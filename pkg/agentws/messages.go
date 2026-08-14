package agentws

const (
	TypeHello          = "hello"
	TypeHeartbeat      = "heartbeat"
	TypeConfig         = "config" // reserved
	TypeLogsFetch      = "logs.fetch"
	TypeLogsBatch      = "logs.batch"
	TypeLogsAck        = "logs.ack"
	TypeLogsStatus     = "logs.status"
	TypeLogsLiveStart  = "logs.live.start"
	TypeLogsLiveStop   = "logs.live.stop"
	TypeAgentUpdate    = "agent.update"
	TypeCatalogPush    = "catalog.push"
	TypeCatalogStatus  = "catalog.status"
	TypeAgentEvents    = "agent.events"
	TypeAgentEventsAck = "agent.events.ack"
)

type Envelope struct {
	Type           string            `json:"type"`
	Hello          *Hello            `json:"hello,omitempty"`
	LogsFetch      *LogsFetch        `json:"logs_fetch,omitempty"`
	LogsBatch      *LogsBatch        `json:"logs_batch,omitempty"`
	LogsAck        *LogsAck          `json:"logs_ack,omitempty"`
	LogsStatus     *LogsStatus       `json:"logs_status,omitempty"`
	LogsLiveStart  *LogsLiveStart    `json:"logs_live_start,omitempty"`
	AgentUpdate    *AgentUpdate      `json:"agent_update,omitempty"`
	CatalogStatus  *CatalogStatus    `json:"catalog_status,omitempty"`
	AgentEvents    *AgentEventsBatch `json:"agent_events,omitempty"`
	AgentEventsAck *AgentEventsAck   `json:"agent_events_ack,omitempty"`
}

type Hello struct {
	Host       string `json:"host,omitempty"`
	Version    string `json:"version,omitempty"`
	SelfUpdate bool   `json:"self_update,omitempty"`
	Arch       string `json:"arch,omitempty"` // GOARCH: arm64 | amd64 (self-update binary selection)
}

type AgentUpdate struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	SHA256  string `json:"sha256"`
}

type CatalogStatus struct {
	Error string `json:"error,omitempty"`
}

type LogsFetch struct {
	SinceDays int `json:"since_days,omitempty"`
}

type LogsLiveStart struct {
	SinceDays int `json:"since_days,omitempty"`
}

type LogLine struct {
	TS       int64  `json:"ts"`
	Source   string `json:"source"` // unbound|dnsmasq|firerouter
	Severity string `json:"severity,omitempty"`
	Line     string `json:"line"`
}

type LogsBatch struct {
	ID      string    `json:"id"`
	Lines   []LogLine `json:"lines"`
	Dropped int       `json:"dropped,omitempty"`
	Done    bool      `json:"done"` // last batch of this invocation; false for live stream chunks
}

type LogsAck struct {
	ID string `json:"id"`
}

type LogsStatus struct {
	Dropped int    `json:"dropped,omitempty"`
	Error   string `json:"error,omitempty"`
	Live    bool   `json:"live,omitempty"`
}

type AgentEventLine struct {
	TS     int64  `json:"ts"`
	Kind   string `json:"kind"`
	Detail string `json:"detail,omitempty"`
}

type AgentEventsBatch struct {
	ID    string           `json:"id"`
	Lines []AgentEventLine `json:"lines"`
}

type AgentEventsAck struct {
	ID string `json:"id"`
}
