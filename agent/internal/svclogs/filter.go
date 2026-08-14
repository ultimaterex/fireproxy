package svclogs

import "strings"

var quietAllow = []string{
	"restart",
	"fail",
	"[acl][blocked]",
	"wan",
	"error",
	"fatal",
	"crit",
}

var quietDeny = []string{
	"zero ttl record",
}

// QuietKeep returns true for err/warn severities or allowlisted line text.
// Denylisted spam is always dropped.
func QuietKeep(severity, line string) bool {
	low := strings.ToLower(line)
	for _, d := range quietDeny {
		if strings.Contains(low, d) {
			return false
		}
	}
	s := strings.ToLower(strings.TrimSpace(severity))
	switch s {
	case "err", "error", "crit", "critical", "alert", "emerg", "emergency", "warning", "warn":
		return true
	}
	for _, a := range quietAllow {
		if strings.Contains(low, a) {
			return true
		}
	}
	return false
}
