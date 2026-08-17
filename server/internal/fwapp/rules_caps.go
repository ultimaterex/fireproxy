package fwapp

// RulesCapabilities is the boolean feature map for Rules UI + API gating.
type RulesCapabilities map[string]bool

// DefaultRulesCapabilities returns write capabilities off until lab fixtures wire them.
func DefaultRulesCapabilities() RulesCapabilities {
	return RulesCapabilities{
		"rule.create.allow":     false,
		"rule.create.block":     false,
		"rule.create.timelimit": false,
		"rule.create.disturb":   false,
		"rule.pause":            false,
		"rule.delete":           false,
		"rule.reset_hits":       false,
		"rule.emergency":        false,
		"rule.diagnose":         false,
	}
}
