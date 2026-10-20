package fwapp

// RuleSection is the UI bucket for a normalized rule.
type RuleSection string

const (
	RuleSectionAllow     RuleSection = "allow"
	RuleSectionBlock     RuleSection = "block"
	RuleSectionDisturb   RuleSection = "disturb"
	RuleSectionTimelimit RuleSection = "timelimit"
	RuleSectionOther     RuleSection = "other"
)

// Rule is a normalized policy / screentime rule from fw-app init.
type Rule struct {
	ID               string      `json:"id"`
	Section          RuleSection `json:"section"`
	Action           string      `json:"action"`
	Type             string      `json:"type"`
	Target           string      `json:"target"`
	Name             string      `json:"name,omitempty"`
	Notes            string      `json:"notes,omitempty"`
	Direction        string      `json:"direction,omitempty"`
	TrafficDirection string      `json:"trafficDirection,omitempty"`
	Disabled         bool        `json:"disabled"`
	Scope            []string    `json:"scope,omitempty"`
	Tags             []string    `json:"tags,omitempty"`
	ScopeLabel       string      `json:"scopeLabel,omitempty"`
	HitCount         int64       `json:"hitCount"`
	LastHitTs        float64     `json:"lastHitTs,omitempty"`
	ActivatedTime    string      `json:"activatedTime,omitempty"`
	Timestamp        string      `json:"timestamp,omitempty"`
	Purpose          string      `json:"purpose,omitempty"`
	Method           string      `json:"method,omitempty"`
	AlarmType        string      `json:"alarmType,omitempty"`
	ReadOnly         bool        `json:"readOnly"`
}

// ExceptionRule is a normalized exception from init exceptionRules.
type ExceptionRule struct {
	ID         string  `json:"id"`
	Type       string  `json:"type,omitempty"`
	AlarmType  string  `json:"alarmType,omitempty"`
	Target     string  `json:"target,omitempty"`
	TargetName string  `json:"targetName,omitempty"`
	MatchCount int64   `json:"matchCount"`
	Timestamp  float64 `json:"timestamp,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	Category   string  `json:"category,omitempty"`
	IfType     string  `json:"ifType,omitempty"`
	IfTarget   string  `json:"ifTarget,omitempty"`
}

// ScopeChipKind classifies a hybrid-list filter chip.
type ScopeChipKind string

const (
	ScopeChipAll    ScopeChipKind = "all"
	ScopeChipDevice ScopeChipKind = "device"
	ScopeChipTag    ScopeChipKind = "tag"
	ScopeChipGroup  ScopeChipKind = "group"
)

// ScopeChip is a filter chip built from hosts / tags / ruleGroups.
type ScopeChip struct {
	ID    string        `json:"id"`
	Kind  ScopeChipKind `json:"kind"`
	Label string        `json:"label"`
	Count int           `json:"count"`
}

// RulesHub summarizes hit totals and rule counts for the Rules hub strip.
type RulesHub struct {
	TotalRules int   `json:"totalRules"`
	TotalHits  int64 `json:"totalHits"`
	AllowHits  int64 `json:"allowHits"`
	BlockHits  int64 `json:"blockHits"`
	AllowCount int   `json:"allowCount"`
	BlockCount int   `json:"blockCount"`
}

// CatalogItem is a selectable id from init (apps, etc).
type CatalogItem struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// RuleCatalog is init-backed pickers for Add Rule (not hardcoded app/target lists).
type RuleCatalog struct {
	Apps []CatalogItem `json:"apps,omitempty"`
}

// RulesSnapshot is the normalized init-backed Rules read model.
type RulesSnapshot struct {
	Hub        RulesHub        `json:"hub"`
	Rules      []Rule          `json:"rules"`
	DapRules   []Rule          `json:"dapRules"`
	Exceptions []ExceptionRule `json:"exceptions"`
	Scopes     []ScopeChip     `json:"scopes"`
	Catalog    RuleCatalog     `json:"catalog"`
	Hosts      []HostPolicy    `json:"hosts,omitempty"`
}
