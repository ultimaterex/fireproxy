package loghub

import (
	"encoding/json"
	"net/http"
	"sync"

	"fireproxy/pkg/agentws"
	"fireproxy/server/internal/auth"

	"github.com/gorilla/websocket"
)

// Hub fans log lines out to browser Live viewers.
type Hub struct {
	// PublicOrigin is AUTH_PUBLIC_ORIGIN for WebSocket CheckOrigin.
	PublicOrigin string
	OnFirst      func() // first UI socket connected
	OnEmpty      func() // last UI socket closed

	mu    sync.Mutex
	conns map[*websocket.Conn]struct{}
}

func (h *Hub) viewers() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

// ServeWS upgrades a browser Live session.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: auth.CheckOriginSameHost(h.PublicOrigin),
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	if h.conns == nil {
		h.conns = map[*websocket.Conn]struct{}{}
	}
	first := len(h.conns) == 0
	h.conns[c] = struct{}{}
	onFirst := h.OnFirst
	h.mu.Unlock()
	if first && onFirst != nil {
		onFirst()
	}

	defer func() {
		h.mu.Lock()
		delete(h.conns, c)
		empty := len(h.conns) == 0
		onEmpty := h.OnEmpty
		h.mu.Unlock()
		_ = c.Close()
		if empty && onEmpty != nil {
			onEmpty()
		}
	}()

	// Read until client disconnects (ignore payloads).
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			return
		}
	}
}

// Broadcast sends a logs.batch envelope to all Live viewers.
func (h *Hub) Broadcast(batch *agentws.LogsBatch) {
	if batch == nil || len(batch.Lines) == 0 {
		return
	}
	env := agentws.Envelope{Type: agentws.TypeLogsBatch, LogsBatch: batch}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		_ = c.WriteMessage(websocket.TextMessage, b)
	}
}

// BroadcastStatus sends logs.status to viewers.
func (h *Hub) BroadcastStatus(st *agentws.LogsStatus) {
	if st == nil {
		return
	}
	env := agentws.Envelope{Type: agentws.TypeLogsStatus, LogsStatus: st}
	b, err := json.Marshal(env)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.conns {
		_ = c.WriteMessage(websocket.TextMessage, b)
	}
}
