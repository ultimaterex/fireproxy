package fwapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// ParseInitRules normalizes fw-app init JSON into a RulesSnapshot.
// Accepts either the unwrapped init data object or a full envelope with mtype/data.
func ParseInitRules(raw []byte) (RulesSnapshot, error) {
	data, err := unwrapInitData(raw)
	if err != nil {
		return RulesSnapshot{}, err
	}

	var root initRoot
	if err := json.Unmarshal(data, &root); err != nil {
		return RulesSnapshot{}, fmt.Errorf("fwapp: parse init rules: %w", err)
	}

	hostLabels := map[string]string{}
	for _, h := range root.Hosts {
		mac := NormalizeMAC(h.MAC)
		if mac == "" {
			continue
		}
		label := strings.TrimSpace(h.Name)
		if label == "" {
			label = strings.TrimSpace(h.BName)
		}
		if label == "" {
			label = mac
		}
		hostLabels[mac] = label
	}

	tagLabels := map[string]string{}
	for id, tag := range root.Tags {
		name := strings.TrimSpace(tag.Name)
		if name == "" {
			name = strings.TrimSpace(tag.UID)
		}
		if name == "" {
			name = id
		}
		uid := strings.TrimSpace(tag.UID)
		if uid == "" {
			uid = id
		}
		tagLabels[uid] = name
		tagLabels[id] = name
	}

	rules := make([]Rule, 0, len(root.PolicyRules)+len(root.ScreentimeRules))
	for _, pr := range root.PolicyRules {
		rules = append(rules, normalizePolicyRule(pr, hostLabels, tagLabels))
	}
	for _, sr := range root.ScreentimeRules {
		rules = append(rules, normalizeScreentimeRule(sr, hostLabels, tagLabels))
	}

	exceptions := make([]ExceptionRule, 0, len(root.ExceptionRules))
	for _, er := range root.ExceptionRules {
		exceptions = append(exceptions, normalizeException(er))
	}

	hub := buildHub(rules)
	// ruleGroup chips deferred until lab shows membership / counts.
	scopes := buildScopeChips(rules, root.Hosts, root.Tags, hostLabels, tagLabels)

	return RulesSnapshot{
		Hub:        hub,
		Rules:      rules,
		Exceptions: exceptions,
		Scopes:     scopes,
	}, nil
}

func unwrapInitData(raw []byte) ([]byte, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("fwapp: init rules json: %w", err)
	}
	data, hasData := top["data"]
	_, hasMtype := top["mtype"]
	_, hasPolicy := top["policyRules"]
	if hasData && (hasMtype || !hasPolicy) {
		if len(data) == 0 || string(data) == "null" {
			return nil, fmt.Errorf("fwapp: init envelope missing data")
		}
		return data, nil
	}
	return raw, nil
}

type initRoot struct {
	PolicyRules     []rawPolicyRule    `json:"policyRules"`
	ExceptionRules  []rawExceptionRule `json:"exceptionRules"`
	ScreentimeRules []rawPolicyRule    `json:"screentimeRules"`
	RuleGroups      []rawRuleGroup     `json:"ruleGroups"`
	Hosts           []rawHost          `json:"hosts"`
	Tags            map[string]rawTag  `json:"tags"`
}

type rawPolicyRule struct {
	PID              flexString `json:"pid"`
	Action           string     `json:"action"`
	Type             string     `json:"type"`
	Target           string     `json:"target"`
	Name             string     `json:"_name"`
	Notes            string     `json:"notes"`
	Direction        string     `json:"direction"`
	TrafficDirection string     `json:"trafficDirection"`
	Disabled         flexString `json:"disabled"`
	Scope            []string   `json:"scope"`
	Tag              []string   `json:"tag"`
	HitCount         flexString `json:"hitCount"`
	LastHitTs        flexString `json:"lastHitTs"`
	ActivatedTime    flexString `json:"activatedTime"`
	Timestamp        flexString `json:"timestamp"`
}

type rawExceptionRule struct {
	EID        flexString `json:"eid"`
	Type       string     `json:"type"`
	AlarmType  string     `json:"alarm_type"`
	TargetName string     `json:"target_name"`
	MatchCount flexString `json:"matchCount"`
	Timestamp  flexFloat  `json:"timestamp"`
	Reason     string     `json:"reason"`
	Category   string     `json:"category"`
	IfType     string     `json:"if.type"`
	IfTarget   string     `json:"if.target"`
	PDestName  string     `json:"p.dest.name"`
}

type rawHost struct {
	MAC   string `json:"mac"`
	Name  string `json:"name"`
	BName string `json:"bname"`
}

type rawTag struct {
	UID  string `json:"uid"`
	Name string `json:"name"`
}

type rawRuleGroup struct {
	UID  string `json:"uid"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

func normalizePolicyRule(pr rawPolicyRule, hosts, tags map[string]string) Rule {
	section := sectionFromAction(pr.Action)
	return finalizeRule(pr, section, hosts, tags)
}

func normalizeScreentimeRule(pr rawPolicyRule, hosts, tags map[string]string) Rule {
	return finalizeRule(pr, RuleSectionTimelimit, hosts, tags)
}

func finalizeRule(pr rawPolicyRule, section RuleSection, hosts, tags map[string]string) Rule {
	scope := make([]string, 0, len(pr.Scope))
	for _, mac := range pr.Scope {
		mac = NormalizeMAC(mac)
		if mac == "" {
			continue
		}
		scope = append(scope, mac)
	}
	tagRefs := append([]string(nil), pr.Tag...)
	r := Rule{
		ID:               strings.TrimSpace(string(pr.PID)),
		Section:          section,
		Action:           strings.TrimSpace(pr.Action),
		Type:             strings.TrimSpace(pr.Type),
		Target:           strings.TrimSpace(pr.Target),
		Name:             strings.TrimSpace(pr.Name),
		Notes:            strings.TrimSpace(pr.Notes),
		Direction:        strings.TrimSpace(pr.Direction),
		TrafficDirection: strings.TrimSpace(pr.TrafficDirection),
		Disabled:         parseDisabled(pr.Disabled),
		Scope:            scope,
		Tags:             tagRefs,
		HitCount:         parseInt64(pr.HitCount),
		LastHitTs:        parseFloat64(pr.LastHitTs),
		ActivatedTime:    strings.TrimSpace(string(pr.ActivatedTime)),
		Timestamp:        strings.TrimSpace(string(pr.Timestamp)),
	}
	r.ScopeLabel = scopeLabel(scope, tagRefs, hosts, tags)
	return r
}

func normalizeException(er rawExceptionRule) ExceptionRule {
	target := strings.TrimSpace(er.IfTarget)
	if target == "" {
		target = strings.TrimSpace(er.PDestName)
	}
	targetName := strings.TrimSpace(er.TargetName)
	if targetName == "" {
		targetName = strings.TrimSpace(er.PDestName)
	}
	return ExceptionRule{
		ID:         strings.TrimSpace(string(er.EID)),
		Type:       strings.TrimSpace(er.Type),
		AlarmType:  strings.TrimSpace(er.AlarmType),
		Target:     target,
		TargetName: targetName,
		MatchCount: parseInt64(er.MatchCount),
		Timestamp:  float64(er.Timestamp),
		Reason:     strings.TrimSpace(er.Reason),
		Category:   strings.TrimSpace(er.Category),
		IfType:     strings.TrimSpace(er.IfType),
		IfTarget:   strings.TrimSpace(er.IfTarget),
	}
}

func sectionFromAction(action string) RuleSection {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "allow":
		return RuleSectionAllow
	case "block":
		return RuleSectionBlock
	case "disturb":
		return RuleSectionDisturb
	default:
		return RuleSectionOther
	}
}

func buildHub(rules []Rule) RulesHub {
	hub := RulesHub{TotalRules: len(rules)}
	for _, r := range rules {
		switch r.Section {
		case RuleSectionAllow:
			hub.AllowCount++
			hub.AllowHits += r.HitCount
		case RuleSectionBlock:
			hub.BlockCount++
			hub.BlockHits += r.HitCount
		}
		hub.TotalHits += r.HitCount
	}
	return hub
}

func buildScopeChips(rules []Rule, hosts []rawHost, tags map[string]rawTag, hostLabels, tagLabels map[string]string) []ScopeChip {
	deviceCounts := map[string]int{}
	tagCounts := map[string]int{}
	allCount := len(rules)

	for _, r := range rules {
		seenDev := map[string]struct{}{}
		seenTag := map[string]struct{}{}
		for _, mac := range r.Scope {
			mac = NormalizeMAC(mac)
			if mac == "" {
				continue
			}
			if _, ok := seenDev[mac]; ok {
				continue
			}
			seenDev[mac] = struct{}{}
			deviceCounts[mac]++
		}
		for _, ref := range r.Tags {
			ref = strings.TrimSpace(ref)
			if id, ok := strings.CutPrefix(ref, "tag:"); ok && id != "" {
				if _, seen := seenTag[id]; seen {
					continue
				}
				seenTag[id] = struct{}{}
				tagCounts[id]++
			}
		}
	}

	chips := []ScopeChip{{
		ID:    "all",
		Kind:  ScopeChipAll,
		Label: "All Devices",
		Count: allCount,
	}}

	// Prefer hosts list order for device chips; include unlabeled MACs seen on rules.
	seenHost := map[string]struct{}{}
	for _, h := range hosts {
		mac := NormalizeMAC(h.MAC)
		if mac == "" {
			continue
		}
		seenHost[mac] = struct{}{}
		label := hostLabels[mac]
		if label == "" {
			label = mac
		}
		chips = append(chips, ScopeChip{
			ID:    mac,
			Kind:  ScopeChipDevice,
			Label: label,
			Count: deviceCounts[mac],
		})
	}
	for mac, n := range deviceCounts {
		if _, ok := seenHost[mac]; ok {
			continue
		}
		label := hostLabels[mac]
		if label == "" {
			label = mac
		}
		chips = append(chips, ScopeChip{
			ID:    mac,
			Kind:  ScopeChipDevice,
			Label: label,
			Count: n,
		})
	}

	emittedTag := map[string]struct{}{}
	for id, tag := range tags {
		uid := strings.TrimSpace(tag.UID)
		if uid == "" {
			uid = id
		}
		if _, ok := emittedTag[uid]; ok {
			continue
		}
		emittedTag[uid] = struct{}{}
		label := tagLabels[uid]
		if label == "" {
			label = strings.TrimSpace(tag.Name)
		}
		if label == "" {
			label = uid
		}
		chips = append(chips, ScopeChip{
			ID:    "tag:" + uid,
			Kind:  ScopeChipTag,
			Label: label,
			Count: tagCounts[uid],
		})
	}
	for id, n := range tagCounts {
		if _, ok := emittedTag[id]; ok {
			continue
		}
		chips = append(chips, ScopeChip{
			ID:    "tag:" + id,
			Kind:  ScopeChipTag,
			Label: id,
			Count: n,
		})
	}

	return chips
}

func scopeLabel(scope, tagRefs []string, hosts, tags map[string]string) string {
	parts := make([]string, 0, len(scope)+len(tagRefs))
	for _, mac := range scope {
		mac = NormalizeMAC(mac)
		if mac == "" {
			continue
		}
		if label, ok := hosts[mac]; ok {
			parts = append(parts, label)
		} else {
			parts = append(parts, mac)
		}
	}
	for _, ref := range tagRefs {
		ref = strings.TrimSpace(ref)
		if id, ok := strings.CutPrefix(ref, "tag:"); ok {
			if label, ok := tags[id]; ok {
				parts = append(parts, label)
			} else {
				parts = append(parts, ref)
			}
			continue
		}
		if ref != "" {
			parts = append(parts, ref)
		}
	}
	return strings.Join(parts, ", ")
}

func parseDisabled(v flexString) bool {
	s := strings.TrimSpace(string(v))
	if s == "" {
		return false
	}
	switch strings.ToLower(s) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func parseInt64(v flexString) int64 {
	s := strings.TrimSpace(string(v))
	if s == "" {
		return 0
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f)
	}
	return 0
}

func parseFloat64(v flexString) float64 {
	s := strings.TrimSpace(string(v))
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// flexString accepts JSON string or number.
type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	b = bytesTrimSpace(b)
	if string(b) == "null" {
		*f = ""
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	*f = flexString(b)
	return nil
}

// flexFloat accepts JSON number or numeric string.
type flexFloat float64

func (f *flexFloat) UnmarshalJSON(b []byte) error {
	b = bytesTrimSpace(b)
	if string(b) == "null" {
		*f = 0
		return nil
	}
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			*f = 0
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return err
		}
		*f = flexFloat(v)
		return nil
	}
	var v float64
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*f = flexFloat(v)
	return nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
