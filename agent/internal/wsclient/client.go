package wsclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"fireproxy/agent/internal/agentevents"
	"fireproxy/agent/internal/svclogs"
	"fireproxy/agent/internal/update"
	"fireproxy/pkg/agentws"

	"github.com/gorilla/websocket"
)

// Client maintains the agent WebSocket session.
type Client struct {
	URL        string
	AgentToken string
	Host       string
	Version    string
	SelfUpdate bool
	Collector  *svclogs.Collector
	Follow     *svclogs.Follow
	OnUpdate   func(*agentws.AgentUpdate)
	OnCatalog  func() error
	Events     *agentevents.Store

	mu       sync.Mutex
	inflight bool
	conn     *websocket.Conn
	acks     map[string]chan struct{}
	liveCtx  context.Context

	// writeMu serializes conn.WriteMessage calls. gorilla/websocket permits at
	// most one concurrent writer, but the hello, heartbeat, log batches, agent
	// events, and catalog-status replies are all written from different
	// goroutines on the same conn.
	writeMu sync.Mutex
}

// Run reconnects forever until ctx is cancelled.
func (c *Client) Run(ctx context.Context) {
	c.acks = map[string]chan struct{}{}
	c.liveCtx = ctx
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := c.session(ctx)
		if c.Follow != nil {
			c.Follow.Stop()
		}
		if ctx.Err() != nil {
			return
		}
		log.Printf("fireproxy-agent: ws disconnected: %v; retry in %s", err, backoff)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

// HourlySync triggers a coalesced sync on the active connection (no-op if offline).
func (c *Client) HourlySync() {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return
	}
	if c.Follow != nil && c.Follow.Active() {
		return // live follow covers new lines
	}
	if err := c.syncOnce(conn, 0); err != nil {
		log.Printf("fireproxy-agent: hourly log sync: %v", err)
	}
}

func (c *Client) syncOnce(conn *websocket.Conn, sinceDays int) error {
	c.mu.Lock()
	if c.inflight {
		c.mu.Unlock()
		return nil
	}
	c.inflight = true
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.inflight = false
		c.mu.Unlock()
	}()

	if c.Collector == nil {
		return nil
	}
	if sinceDays > 0 {
		c.Collector.SinceDays = sinceDays
	}
	batches, pending, err := c.Collector.Collect(svclogs.MaxBatchesPerInvocation)
	if err != nil {
		return err
	}
	for _, b := range batches {
		if err := c.sendBatch(conn, b); err != nil {
			return err
		}
	}
	return c.Collector.ApplyCursors(pending)
}

func (c *Client) sendBatch(conn *websocket.Conn, b agentws.LogsBatch) error {
	ch := make(chan struct{}, 1)
	c.mu.Lock()
	c.acks[b.ID] = ch
	c.mu.Unlock()
	env := agentws.Envelope{Type: agentws.TypeLogsBatch, LogsBatch: &b}
	if err := c.writeEnv(conn, env); err != nil {
		c.mu.Lock()
		delete(c.acks, b.ID)
		c.mu.Unlock()
		return err
	}
	select {
	case <-ch:
	case <-time.After(10 * time.Second):
		c.mu.Lock()
		delete(c.acks, b.ID)
		c.mu.Unlock()
		return fmt.Errorf("ack timeout for %s", b.ID)
	}
	c.mu.Lock()
	delete(c.acks, b.ID)
	c.mu.Unlock()
	return nil
}

func (c *Client) session(ctx context.Context) error {
	hdr := http.Header{}
	if c.AgentToken != "" {
		hdr.Set("Authorization", "Bearer "+c.AgentToken)
		hdr.Set("X-API-Key", c.AgentToken)
	}
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, c.URL, hdr)
	if err != nil {
		return err
	}
	defer conn.Close()

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.conn == conn {
			c.conn = nil
		}
		c.mu.Unlock()
	}()

	ver := c.Version
	if ver == "" {
		ver = "0.0.0"
	}
	_ = c.writeEnv(conn, agentws.Envelope{
		Type: agentws.TypeHello,
		Hello: &agentws.Hello{
			Host:       c.Host,
			Version:    ver,
			SelfUpdate: c.SelfUpdate,
			Arch:       runtime.GOARCH,
			SHA256:     update.SelfSHA256(),
		},
	})
	go c.flushAgentEvents(conn)

	heartbeat := time.NewTicker(45 * time.Second)
	defer heartbeat.Stop()

	errCh := make(chan error, 1)
	go func() {
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				errCh <- err
				return
			}
			var env agentws.Envelope
			if err := json.Unmarshal(data, &env); err != nil {
				continue
			}
			switch env.Type {
			case agentws.TypeLogsFetch:
				since := 0
				if env.LogsFetch != nil {
					since = env.LogsFetch.SinceDays
				}
				go func(days int) {
					if err := c.syncOnce(conn, days); err != nil {
						log.Printf("fireproxy-agent: logs fetch: %v", err)
					}
				}(since)
			case agentws.TypeLogsLiveStart:
				since := 0
				if env.LogsLiveStart != nil {
					since = env.LogsLiveStart.SinceDays
				}
				go c.startLive(conn, since)
			case agentws.TypeLogsLiveStop:
				if c.Follow != nil {
					c.Follow.Stop()
					log.Printf("fireproxy-agent: live logs stopped")
				}
			case agentws.TypeLogsAck:
				if env.LogsAck == nil {
					continue
				}
				c.mu.Lock()
				ch := c.acks[env.LogsAck.ID]
				c.mu.Unlock()
				if ch != nil {
					select {
					case ch <- struct{}{}:
					default:
					}
				}
			case agentws.TypeAgentEventsAck:
				if env.AgentEventsAck == nil {
					continue
				}
				c.mu.Lock()
				ch := c.acks[env.AgentEventsAck.ID]
				c.mu.Unlock()
				if ch != nil {
					select {
					case ch <- struct{}{}:
					default:
					}
				}
			case agentws.TypeAgentUpdate:
				if !c.SelfUpdate || env.AgentUpdate == nil {
					continue
				}
				go func(u *agentws.AgentUpdate) {
					if c.OnUpdate != nil {
						c.OnUpdate(u)
					}
				}(env.AgentUpdate)
			case agentws.TypeCatalogPush:
				go func() {
					st := &agentws.CatalogStatus{}
					if c.OnCatalog != nil {
						if err := c.OnCatalog(); err != nil {
							st.Error = err.Error()
						}
					}
					_ = c.writeEnv(conn, agentws.Envelope{
						Type:          agentws.TypeCatalogStatus,
						CatalogStatus: st,
					})
				}()
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case <-heartbeat.C:
			_ = c.writeEnv(conn, agentws.Envelope{Type: agentws.TypeHeartbeat})
		}
	}
}

func (c *Client) startLive(conn *websocket.Conn, sinceDays int) {
	// Seed catch-up once before follow.
	if err := c.syncOnce(conn, sinceDays); err != nil {
		log.Printf("fireproxy-agent: live seed: %v", err)
	}
	if c.Follow == nil {
		return
	}
	c.Follow.Sink = func(b agentws.LogsBatch) error {
		c.mu.Lock()
		active := c.conn
		c.mu.Unlock()
		if active != conn {
			return fmt.Errorf("conn changed")
		}
		return c.sendBatch(conn, b)
	}
	parent := c.liveCtx
	if parent == nil {
		parent = context.Background()
	}
	c.Follow.Start(parent)
	log.Printf("fireproxy-agent: live logs started")
}

func (c *Client) flushAgentEvents(conn *websocket.Conn) {
	if c.Events == nil {
		return
	}
	lines, err := c.Events.Pending()
	if err != nil || len(lines) == 0 {
		return
	}
	id := fmt.Sprintf("ae-%d", time.Now().UnixNano())
	ch := make(chan struct{}, 1)
	c.mu.Lock()
	c.acks[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.acks, id)
		c.mu.Unlock()
	}()
	if err := c.writeEnv(conn, agentws.Envelope{
		Type: agentws.TypeAgentEvents,
		AgentEvents: &agentws.AgentEventsBatch{
			ID:    id,
			Lines: lines,
		},
	}); err != nil {
		return
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	select {
	case <-ch:
		_ = c.Events.Drop(len(lines))
	case <-timer.C:
	}
}

func (c *Client) writeEnv(conn *websocket.Conn, env agentws.Envelope) error {
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	// Serialize the frame write only; ack waits happen outside this lock so a
	// slow batch never blocks heartbeats.
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return conn.WriteMessage(websocket.TextMessage, b)
}

// JournalCtlRunner implements svclogs.JournalRunner via journalctl.
func JournalCtlRunner(unit, afterCursor string, sinceDays, limit int) (stdout string, newCursor string, err error) {
	args := []string{"-u", unit, "--no-pager", "-o", "json", "-n", fmt.Sprintf("%d", limit)}
	if afterCursor != "" && afterCursor != svclogs.CursorNow {
		args = append(args, "--after-cursor", afterCursor)
	} else {
		if sinceDays <= 0 {
			sinceDays = svclogs.DefaultSeedDays
		}
		args = append(args, "--since", fmt.Sprintf("%d days ago", sinceDays))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "journalctl", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", afterCursor, nil
	}
	stdout = string(out)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "" {
			continue
		}
		var row struct {
			Cursor string `json:"__CURSOR"`
		}
		if json.Unmarshal([]byte(lines[i]), &row) == nil && row.Cursor != "" {
			newCursor = row.Cursor
			break
		}
	}
	if newCursor == "" {
		newCursor = afterCursor
	}
	return stdout, newCursor, nil
}
