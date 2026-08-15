package fwapp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"fireproxy/server/internal/tplink"
)

// Service owns pairing state and LAN verification.
type Service struct {
	mu     sync.Mutex
	vault  *CredentialVault
	lan    *LANClient
	pairFn func(ctx context.Context, req PairRequest) (Creds, error) // test hook; nil = PairWithCloud

	lastPingOK bool
	lastPingAt time.Time
	lastErr    string
	state      string // unpaired | ready | lan-ok | lan-down | error

	speedJobs map[string]*SpeedtestJob
	// IndexSpeedtest merges LAN history into the server catalog (optional).
	IndexSpeedtest func(results []SpeedtestResult)
}

// NewService builds a Service using persist KV and FIREPROXY_SECRETS_KEY.
func NewService(store Store, secretsKeyEnv string) *Service {
	key, _ := tplink.KeyFromEnv(secretsKeyEnv)
	if secretsKeyEnv == "" {
		key, _ = tplink.KeyFromEnv(os.Getenv("FIREPROXY_SECRETS_KEY"))
	}
	return &Service{
		vault: &CredentialVault{Store: store, Key: key},
		lan:   NewLANClient(),
		state: "unpaired",
	}
}

// NewServiceWithVault is for tests.
func NewServiceWithVault(v *CredentialVault, lan *LANClient) *Service {
	if lan == nil {
		lan = NewLANClient()
	}
	return &Service{vault: v, lan: lan, state: "unpaired"}
}

func (s *Service) secretsReady() bool {
	return s.vault != nil && len(s.vault.Key) == 32
}

// Status returns non-secret pairing/LAN state.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{SecretsReady: s.secretsReady(), State: "unpaired"}
	if !s.secretsReady() {
		st.State = "error"
		st.LastError = "FIREPROXY_SECRETS_KEY not set"
		return st
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		st.State = "error"
		st.LastError = err.Error()
		return st
	}
	if !ok || c.SymKey == "" {
		st.State = "unpaired"
		return st
	}
	st.Paired = true
	st.BoxIP = c.BoxIP
	st.GidHint = gidHint(c.Gid)
	st.Email = c.Email
	st.DeviceName = c.DeviceName
	if !c.PairedAt.IsZero() {
		st.PairedAt = c.PairedAt.UTC().Format(time.RFC3339)
	}
	st.LastPingOK = s.lastPingOK
	if !s.lastPingAt.IsZero() {
		st.LastPingAt = s.lastPingAt.UTC().Format(time.RFC3339)
	}
	st.LastError = s.lastErr
	// Vault is source of truth for paired; ignore stale "unpaired" after restart.
	switch s.state {
	case "lan-ok", "lan-down", "ready", "error":
		st.State = s.state
	default:
		if s.lastPingOK {
			st.State = "lan-ok"
		} else {
			st.State = "ready"
		}
	}
	return st
}

// Pair runs cloud join once, persists creds, then requires LAN ping/init to succeed.
func (s *Service) Pair(ctx context.Context, req PairRequest) (Status, error) {
	if !s.secretsReady() {
		return s.Status(), fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	fn := s.pairFn
	if fn == nil {
		fn = PairWithCloud
	}
	creds, err := fn(ctx, req)
	if err != nil {
		s.mu.Lock()
		s.state = "error"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return s.Status(), err
	}
	if err := s.vault.Save(creds); err != nil {
		return s.Status(), err
	}
	s.mu.Lock()
	s.state = "ready"
	s.lastErr = ""
	s.mu.Unlock()

	if err := s.lan.PingInitReady(ctx, creds); err != nil {
		s.mu.Lock()
		s.lastPingOK = false
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-down"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return s.Status(), fmt.Errorf("paired but LAN verify failed: %w", err)
	}
	s.mu.Lock()
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	return s.Status(), nil
}

// Wake sends Wake-on-LAN for mac via LAN cmd wol:wake (target = MAC).
func (s *Service) Wake(ctx context.Context, mac string) (Status, error) {
	mac, err := ParseMAC(mac)
	if err != nil {
		return s.Status(), err
	}
	if !s.secretsReady() {
		return s.Status(), fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return s.Status(), err
	}
	if !ok || c.SymKey == "" {
		return s.Status(), ErrNotPaired
	}
	_, err = s.lan.SendTo(ctx, c, MTypeCmd, map[string]any{"item": "wol:wake"}, mac)
	if err != nil {
		s.mu.Lock()
		s.lastPingOK = false
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-down"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return s.Status(), err
	}
	s.mu.Lock()
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	return s.Status(), nil
}

// Ping verifies LAN-only control (never cloud).
func (s *Service) Ping(ctx context.Context) (Status, error) {
	if !s.secretsReady() {
		return s.Status(), fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return s.Status(), err
	}
	if !ok || c.SymKey == "" {
		return s.Status(), ErrNotPaired
	}
	// Fail fast: Test connection should surface errors immediately.
	// Pair still uses PingInitReady for post-join box init.
	if err := s.lan.PingInit(ctx, c); err != nil {
		s.mu.Lock()
		s.lastPingOK = false
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-down"
		s.lastErr = err.Error()
		s.mu.Unlock()
		return s.Status(), err
	}
	s.mu.Lock()
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	return s.Status(), nil
}

// RunSpeedtest runs an internet speedtest on the given WAN via LAN only.
func (s *Service) RunSpeedtest(ctx context.Context, wanUUID, serverID string) (SpeedtestResult, Status, error) {
	var zero SpeedtestResult
	if !s.secretsReady() {
		return zero, s.Status(), fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return zero, s.Status(), err
	}
	if !ok || c.SymKey == "" {
		return zero, s.Status(), ErrNotPaired
	}
	res, err := s.lan.RunSpeedtest(ctx, c, wanUUID, serverID)
	if err != nil {
		s.mu.Lock()
		if errorsIsLocal(err) {
			s.lastPingOK = false
			s.state = "lan-down"
			s.lastErr = err.Error()
			s.lastPingAt = time.Now().UTC()
		}
		s.mu.Unlock()
		return zero, s.Status(), err
	}
	s.mu.Lock()
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	return res, s.Status(), nil
}

// SpeedtestJob is an async LAN speedtest (avoids reverse-proxy timeouts on long POSTs).
type SpeedtestJob struct {
	ID        string           `json:"id"`
	State     string           `json:"state"` // queued|running|done|error
	WanUUID   string           `json:"wan_uuid,omitempty"`
	ServerID  string           `json:"server_id,omitempty"`
	Result    *SpeedtestResult `json:"result,omitempty"`
	Error     string           `json:"error,omitempty"`
	updatedAt time.Time        // for TTL / eviction; not exported
}

const (
	speedJobTTL = 30 * time.Minute
	speedJobMax = 32
)

// pruneSpeedJobsLocked drops finished jobs past TTL and caps map size. Caller holds s.mu.
func (s *Service) pruneSpeedJobsLocked(now time.Time) {
	if s.speedJobs == nil {
		return
	}
	for id, j := range s.speedJobs {
		if j == nil {
			delete(s.speedJobs, id)
			continue
		}
		if j.State == "done" || j.State == "error" {
			if j.updatedAt.IsZero() || now.Sub(j.updatedAt) > speedJobTTL {
				delete(s.speedJobs, id)
			}
		}
	}
	for len(s.speedJobs) > speedJobMax {
		var oldestID string
		var oldest time.Time
		for id, j := range s.speedJobs {
			if j == nil || j.State == "queued" || j.State == "running" {
				continue
			}
			t := j.updatedAt
			if t.IsZero() {
				t = now.Add(-speedJobTTL)
			}
			if oldestID == "" || t.Before(oldest) {
				oldestID = id
				oldest = t
			}
		}
		if oldestID == "" {
			break
		}
		delete(s.speedJobs, oldestID)
	}
}

// StartSpeedtest queues a background speedtest and returns immediately.
func (s *Service) StartSpeedtest(wanUUID, serverID string) (SpeedtestJob, Status, error) {
	var zero SpeedtestJob
	if !s.secretsReady() {
		return zero, s.Status(), fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return zero, s.Status(), err
	}
	if !ok || c.SymKey == "" {
		return zero, s.Status(), ErrNotPaired
	}
	wanUUID = strings.TrimSpace(wanUUID)
	if wanUUID == "" {
		return zero, s.Status(), fmt.Errorf("wan_uuid required")
	}
	serverID = strings.TrimSpace(serverID)

	id := RandomID()
	now := time.Now()
	job := SpeedtestJob{ID: id, State: "queued", WanUUID: wanUUID, ServerID: serverID, updatedAt: now}
	s.mu.Lock()
	if s.speedJobs == nil {
		s.speedJobs = map[string]*SpeedtestJob{}
	}
	s.pruneSpeedJobsLocked(now)
	s.speedJobs[id] = &job
	s.mu.Unlock()

	go func(creds Creds, jobID, wan, server string) {
		s.mu.Lock()
		if j := s.speedJobs[jobID]; j != nil {
			j.State = "running"
			j.updatedAt = time.Now()
		}
		s.mu.Unlock()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		res, err := s.lan.RunSpeedtest(ctx, creds, wan, server)
		if err != nil {
			s.mu.Lock()
			defer s.mu.Unlock()
			j := s.speedJobs[jobID]
			if j == nil {
				return
			}
			j.State = "error"
			j.Error = err.Error()
			j.updatedAt = time.Now()
			if errorsIsLocal(err) {
				s.lastPingOK = false
				s.state = "lan-down"
				s.lastErr = err.Error()
				s.lastPingAt = time.Now().UTC()
			}
			return
		}

		s.indexAfterRun(creds, &res, server)

		s.mu.Lock()
		defer s.mu.Unlock()
		j := s.speedJobs[jobID]
		if j == nil {
			return
		}
		cp := res
		j.State = "done"
		j.Result = &cp
		j.updatedAt = time.Now()
		s.lastPingOK = true
		s.lastPingAt = time.Now().UTC()
		s.state = "lan-ok"
		s.lastErr = ""
	}(c, id, wanUUID, serverID)

	return job, s.Status(), nil
}

// SyncSpeedtestHistory pulls get internetSpeedtestResults and indexes server metadata.
func (s *Service) SyncSpeedtestHistory(ctx context.Context) (n int, st Status, err error) {
	st = s.Status()
	if !s.secretsReady() {
		return 0, st, fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	c, ok, err := s.vault.Load()
	if err != nil {
		return 0, st, err
	}
	if !ok || c.SymKey == "" {
		return 0, st, ErrNotPaired
	}
	begin := time.Now().Add(-90 * 24 * time.Hour).Unix()
	hist, err := s.lan.GetSpeedtestResults(ctx, c, begin)
	if err != nil {
		s.mu.Lock()
		if errorsIsLocal(err) {
			s.lastPingOK = false
			s.state = "lan-down"
			s.lastErr = err.Error()
			s.lastPingAt = time.Now().UTC()
		}
		s.mu.Unlock()
		return 0, s.Status(), err
	}
	s.enrichResults(ctx, hist)
	s.mu.Lock()
	index := s.IndexSpeedtest
	s.lastPingOK = true
	s.lastPingAt = time.Now().UTC()
	s.state = "lan-ok"
	s.lastErr = ""
	s.mu.Unlock()
	if index != nil && len(hist) > 0 {
		index(hist)
	}
	return len(hist), s.Status(), nil
}

// indexAfterRun writes the cmd result, then reconciles from get internetSpeedtestResults.
func (s *Service) indexAfterRun(creds Creds, run *SpeedtestResult, requestedServerID string) {
	if run == nil {
		return
	}
	if run.ServerID == "" && requestedServerID != "" {
		run.ServerID = requestedServerID
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	seed := []SpeedtestResult{*run}
	s.enrichResults(ctx, seed)
	*run = seed[0]

	s.mu.Lock()
	index := s.IndexSpeedtest
	s.mu.Unlock()
	if index == nil {
		return
	}
	batch := []SpeedtestResult{*run}
	begin := run.TS - 7*24*3600
	if begin < 0 {
		begin = 0
	}
	if hist, err := s.lan.GetSpeedtestResults(ctx, creds, begin); err == nil {
		s.enrichResults(ctx, hist)
		for _, h := range hist {
			if h.WanUUID == "" {
				h.WanUUID = run.WanUUID
			}
			batch = append(batch, h)
		}
	}
	index(batch)
}

func (s *Service) enrichResults(ctx context.Context, results []SpeedtestResult) {
	need := false
	for i := range results {
		if results[i].ServerID != "" && results[i].Server == "" {
			need = true
			break
		}
	}
	if !need {
		return
	}
	servers, err := FetchOoklaServers(ctx, 50)
	if err != nil || len(servers) == 0 {
		return
	}
	byID := map[string]OoklaServer{}
	for _, srv := range servers {
		byID[srv.ID] = srv
	}
	for i := range results {
		r := &results[i]
		if r.Server != "" || r.ServerID == "" {
			continue
		}
		if srv, ok := byID[r.ServerID]; ok {
			r.Server = srv.Sponsor
			if r.Location == "" {
				r.Location = srv.Name
			}
		}
	}
}

// SpeedtestJobStatus returns a job snapshot.
func (s *Service) SpeedtestJobStatus(id string) (SpeedtestJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneSpeedJobsLocked(time.Now())
	j := s.speedJobs[id]
	if j == nil {
		return SpeedtestJob{}, false
	}
	out := *j
	if j.Result != nil {
		cp := *j.Result
		out.Result = &cp
	}
	return out, true
}

func errorsIsLocal(err error) bool {
	return err != nil && (errors.Is(err, ErrLocalUnreach) || strings.Contains(strings.ToLower(err.Error()), "unreachable"))
}

// Unpair wipes local credentials.
func (s *Service) Unpair() error {
	if !s.secretsReady() {
		return fmt.Errorf("FIREPROXY_SECRETS_KEY required")
	}
	if err := s.vault.Clear(); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastPingOK = false
	s.lastPingAt = time.Time{}
	s.lastErr = ""
	s.state = "unpaired"
	s.mu.Unlock()
	return nil
}

// SetPairFn overrides cloud pairing (tests).
func (s *Service) SetPairFn(fn func(ctx context.Context, req PairRequest) (Creds, error)) {
	s.pairFn = fn
}

// SetLAN overrides the LAN client (tests).
func (s *Service) SetLAN(lan *LANClient) {
	s.lan = lan
}
