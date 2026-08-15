package agenthub

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"fireproxy/pkg/agentws"
	"fireproxy/server/internal/agentpkg"
	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/loghub"
	"fireproxy/server/internal/store"

	"github.com/gorilla/websocket"
)

const fetchWait = 12 * time.Second
const catalogWait = 90 * time.Second

// AgentInfo is last-known agent presence.
type AgentInfo struct {
	Online     bool   `json:"online"`
	Host       string `json:"host,omitempty"`
	Version    string `json:"version,omitempty"`
	SelfUpdate bool   `json:"self_update,omitempty"`
	Arch       string `json:"arch,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	LastSeen   int64  `json:"last_seen,omitempty"`
}

// Hub manages one agent WebSocket and log ingest.
type Hub struct {
	Auth    auth.AgentVerifier
	Persist *store.Persist
	Package *agentpkg.Package
	// PublicOrigin is AUTH_PUBLIC_ORIGIN for WebSocket CheckOrigin.
	PublicOrigin string
	// PublicBase resolves the URL prefix for agent.update downloads.
	PublicBase func() string
	// UI is optional Live fan-out to browsers.
	UI *loghub.Hub
	// LogRetentionDays resolves seed window for fetch/live (default 7).
	LogRetentionDays func() int

	mu     sync.Mutex
	conn   *websocket.Conn
	online bool
	info   AgentInfo

	fetchMu   sync.Mutex
	fetchWait chan fetchResult

	catalogMu   sync.Mutex
	catalogWait chan string
}

type fetchResult struct {
	status  string
	dropped int
}

func (h *Hub) Online() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.online
}

func (h *Hub) Info() AgentInfo {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := h.info
	out.Online = h.online
	return out
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	if h.Auth == nil || !h.Auth.AuthorizeAgent(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	upgrader := websocket.Upgrader{
		CheckOrigin: auth.CheckOriginSameHost(h.PublicOrigin),
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	if h.conn != nil {
		_ = h.conn.Close()
	}
	h.conn = c
	h.online = true
	h.info.Online = true
	h.info.LastSeen = time.Now().Unix()
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		cleared := false
		if h.conn == c {
			h.conn = nil
			h.online = false
			h.info.Online = false
			cleared = true
		}
		h.mu.Unlock()
		if cleared && h.Persist != nil {
			_ = h.Persist.SetAgentLastOfflineTS(time.Now().Unix())
		}
		_ = c.Close()
		h.failFetch("agent_offline")
		h.failCatalog("agent_offline")
	}()

	for {
		_, data, err := c.ReadMessage()
		if err != nil {
			return
		}
		var env agentws.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		switch env.Type {
		case agentws.TypeHello:
			h.onHello(env.Hello)
		case agentws.TypeHeartbeat:
			h.mu.Lock()
			h.info.LastSeen = time.Now().Unix()
			h.mu.Unlock()
		case agentws.TypeLogsBatch:
			if env.LogsBatch == nil {
				continue
			}
			dropped := env.LogsBatch.Dropped
			if h.Persist != nil && len(env.LogsBatch.Lines) > 0 {
				_ = h.Persist.InsertLogLines(env.LogsBatch.Lines)
			}
			_ = h.write(agentws.Envelope{
				Type:    agentws.TypeLogsAck,
				LogsAck: &agentws.LogsAck{ID: env.LogsBatch.ID},
			})
			if h.UI != nil {
				h.UI.Broadcast(env.LogsBatch)
			}
			if env.LogsBatch.Done {
				h.completeFetch("ok", dropped)
			}
		case agentws.TypeCatalogStatus:
			status := "ok"
			if env.CatalogStatus != nil && env.CatalogStatus.Error != "" {
				status = env.CatalogStatus.Error
			}
			h.completeCatalog(status)
		case agentws.TypeAgentEvents:
			if env.AgentEvents == nil {
				continue
			}
			if h.Persist != nil {
				for _, ln := range env.AgentEvents.Lines {
					_ = h.Persist.InsertAgentEvent(store.AgentEvent{
						TS:     ln.TS,
						Kind:   ln.Kind,
						Detail: ln.Detail,
					})
				}
			}
			_ = h.write(agentws.Envelope{
				Type:           agentws.TypeAgentEventsAck,
				AgentEventsAck: &agentws.AgentEventsAck{ID: env.AgentEvents.ID},
			})
		}
	}
}

func (h *Hub) onHello(hello *agentws.Hello) {
	h.mu.Lock()
	h.info.LastSeen = time.Now().Unix()
	if hello != nil {
		h.info.Host = hello.Host
		h.info.Version = hello.Version
		h.info.SelfUpdate = hello.SelfUpdate
		h.info.Arch = hello.Arch
		h.info.SHA256 = strings.TrimSpace(strings.ToLower(hello.SHA256))
	}
	selfUpdate := h.info.SelfUpdate
	agentVer := h.info.Version
	agentSHA := h.info.SHA256
	arch := h.info.Arch
	h.mu.Unlock()

	h.recordHelloEvents(agentVer)

	if selfUpdate {
		h.MaybeAutoUpdate(agentVer, agentSHA, arch)
	}
}

func (h *Hub) recordHelloEvents(helloVer string) {
	if h.Persist == nil {
		return
	}
	now := time.Now().Unix()
	lastVer, _ := h.Persist.AgentLastVersion()
	offlineTS, _ := h.Persist.AgentLastOfflineTS()
	for _, pe := range DecideHelloEvents(lastVer, offlineTS, helloVer, now) {
		_ = h.Persist.InsertAgentEvent(store.AgentEvent{
			TS:      now,
			Kind:    pe.Kind,
			Detail:  pe.Detail,
			FromVer: pe.FromVer,
			ToVer:   pe.ToVer,
		})
	}
	if helloVer != "" {
		_ = h.Persist.SetAgentLastVersion(helloVer)
	}
	_ = h.Persist.ClearAgentLastOfflineTS()
}

// MaybeAutoUpdate sends agent.update when the packaged binary should replace the agent.
// Uses SHA when the agent reports one (so VERSION=dev rebuilds still update); else semver.
func (h *Hub) MaybeAutoUpdate(agentVer, agentSHA, arch string) {
	if h.Package == nil || h.Package.Missing() {
		return
	}
	bin, ok := h.Package.Bin(arch)
	if !ok {
		return
	}
	if !agentpkg.NeedsUpdate(h.Package.Version, agentVer, bin.SHA256, agentSHA) {
		return
	}
	_ = h.SendUpdate()
}

// SendUpdate pushes agent.update to the connected agent.
func (h *Hub) SendUpdate() error {
	if h.Package == nil || h.Package.Missing() {
		return errString("agent package missing")
	}
	base := ""
	if h.PublicBase != nil {
		base = strings.TrimRight(h.PublicBase(), "/")
	}
	if base == "" {
		return errString("public base URL required")
	}
	h.mu.Lock()
	arch := h.info.Arch
	h.mu.Unlock()
	bin, ok := h.Package.Bin(arch)
	if !ok {
		return errString("no agent binary for arch " + arch)
	}
	url := base + "/agent/fireproxy-agent"
	if bin.Arch != "" {
		url += "?arch=" + bin.Arch
	}
	return h.write(agentws.Envelope{
		Type: agentws.TypeAgentUpdate,
		AgentUpdate: &agentws.AgentUpdate{
			Version: h.Package.Version,
			URL:     url,
			SHA256:  bin.SHA256,
		},
	})
}

// RequestCatalogPush asks the agent to collect and POST catalog, then waits for catalog.status.
func (h *Hub) RequestCatalogPush(ctx context.Context) string {
	if !h.Online() {
		return "agent_offline"
	}
	h.catalogMu.Lock()
	if h.catalogWait != nil {
		h.catalogMu.Unlock()
		return "busy"
	}
	ch := make(chan string, 1)
	h.catalogWait = ch
	h.catalogMu.Unlock()
	// Clear our waiter on any exit. Without this, a timeout would leave
	// catalogWait non-nil and permanently reject later requests as "busy".
	defer func() {
		h.catalogMu.Lock()
		if h.catalogWait == ch {
			h.catalogWait = nil
		}
		h.catalogMu.Unlock()
	}()

	if err := h.write(agentws.Envelope{Type: agentws.TypeCatalogPush}); err != nil {
		return "agent_offline"
	}

	wait := catalogWait
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 && d < wait {
			wait = d
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "timeout"
	case <-timer.C:
		return "timeout"
	case status := <-ch:
		return status
	}
}

func (h *Hub) RequestFetch(ctx context.Context) (status string, dropped int) {
	if !h.Online() {
		return "agent_offline", 0
	}
	h.fetchMu.Lock()
	if h.fetchWait != nil {
		h.fetchMu.Unlock()
		return "busy", 0
	}
	ch := make(chan fetchResult, 1)
	h.fetchWait = ch
	h.fetchMu.Unlock()
	// Clear our waiter on any exit so a timeout can't wedge later requests.
	defer func() {
		h.fetchMu.Lock()
		if h.fetchWait == ch {
			h.fetchWait = nil
		}
		h.fetchMu.Unlock()
	}()

	if err := h.write(agentws.Envelope{
		Type:      agentws.TypeLogsFetch,
		LogsFetch: &agentws.LogsFetch{SinceDays: h.sinceDays()},
	}); err != nil {
		return "agent_offline", 0
	}

	wait := fetchWait
	if dl, ok := ctx.Deadline(); ok {
		if d := time.Until(dl); d > 0 && d < wait {
			wait = d
		}
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return "timeout", 0
	case <-timer.C:
		return "timeout", 0
	case res := <-ch:
		return res.status, res.dropped
	}
}

// StartLive asks the agent to begin journal follow.
func (h *Hub) StartLive() error {
	if !h.Online() {
		return errOffline
	}
	return h.write(agentws.Envelope{
		Type:          agentws.TypeLogsLiveStart,
		LogsLiveStart: &agentws.LogsLiveStart{SinceDays: h.sinceDays()},
	})
}

// StopLive asks the agent to stop journal follow.
func (h *Hub) StopLive() error {
	if !h.Online() {
		return nil
	}
	return h.write(agentws.Envelope{Type: agentws.TypeLogsLiveStop})
}

func (h *Hub) sinceDays() int {
	if h.LogRetentionDays != nil {
		if d := h.LogRetentionDays(); d > 0 {
			return d
		}
	}
	if h.Persist != nil {
		if d := h.Persist.LogRetentionDays(); d > 0 {
			return d
		}
	}
	return 7
}

func (h *Hub) write(env agentws.Envelope) error {
	h.mu.Lock()
	c := h.conn
	h.mu.Unlock()
	if c == nil {
		return errOffline
	}
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conn != c {
		return errOffline
	}
	return c.WriteMessage(websocket.TextMessage, b)
}

func (h *Hub) completeFetch(status string, dropped int) {
	h.fetchMu.Lock()
	ch := h.fetchWait
	h.fetchWait = nil
	h.fetchMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- fetchResult{status: status, dropped: dropped}:
	default:
	}
}

func (h *Hub) failFetch(status string) {
	h.completeFetch(status, 0)
}

func (h *Hub) completeCatalog(status string) {
	h.catalogMu.Lock()
	ch := h.catalogWait
	h.catalogWait = nil
	h.catalogMu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- status:
	default:
	}
}

func (h *Hub) failCatalog(status string) {
	h.completeCatalog(status)
}

var errOffline = errString("agent offline")

type errString string

func (e errString) Error() string { return string(e) }
