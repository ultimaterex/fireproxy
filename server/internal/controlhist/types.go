package controlhist

// Scheme identifiers for control history rows.
const (
	SchemeFirewalla = "firewalla"
	SchemeUnifi     = "unifi"
)

// Action verbs (scheme-local; uniqueness is scheme+action).
const (
	ActionHostRename   = "host.rename"
	ActionHostDNS      = "host.dns"
	ActionHostWOL      = "host.wol"
	ActionSpeedtestRun = "speedtest.run"
	ActionClientRename = "client.rename"
)

// Actor kind values stored in control_events.actor_kind.
const (
	ActorUser   = "user"
	ActorSystem = "system"
)

// Result vocabulary for control_events.result.
const (
	ResultOK    = "ok"
	Result400   = "400"
	Result409   = "409"
	ResultBusy  = "busy"
	Result502   = "502"
	ResultError = "error"
)

// Outcome is one control write attempt passed to Recorder.Record.
type Outcome struct {
	Scheme, Action, Target, Summary string
	ActorKind, Actor                string
	Before, After                   map[string]any // After ignored unless Err==nil
	Err                             error
	Skip                            error // optional explicit skip sentinel (module off)
}
