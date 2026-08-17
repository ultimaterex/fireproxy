package fwapp

// RulesCapabilities is the boolean feature map for Rules UI + API gating.
type RulesCapabilities map[string]bool

// DefaultRulesCapabilities returns write capabilities; create allow/block + pause/delete
// are on after lab-proven policy:* NetBot cmds.
func DefaultRulesCapabilities() RulesCapabilities {
	return RulesCapabilities{
		"rule.create.allow":     true,
		"rule.create.block":     true,
		"rule.create.timelimit": false,
		"rule.create.disturb":   false,
		"rule.pause":            true,
		"rule.delete":           true,
		"rule.reset_hits":       false,
		"rule.emergency":        false,
		"rule.diagnose":         false,
	}
}
