package collect

import (
	"encoding/json"

	"fireproxy/pkg/inventory"
)

type dapJSON struct {
	State                 string          `json:"state"`
	Reason                string          `json:"reason"`
	LearnedCount          int64           `json:"learnedCount"`
	AllowedDomains        json.RawMessage `json:"allowedDomains"`
	AllowedIPs            json.RawMessage `json:"allowedIPs"`
	AllowedICMPs          json.RawMessage `json:"allowedICMPs"`
	AllowedHashsets       json.RawMessage `json:"allowedHashsets"`
	AllowedCloudDomains   json.RawMessage `json:"allowedCloudDomains"`
	AllowedKnockDomains   json.RawMessage `json:"allowedKnockDomains"`
	AllowedKnockIPs       json.RawMessage `json:"allowedKnockIPs"`
	DomainsAllowedByRules json.RawMessage `json:"domainsAllowedByRules"`
	IPsAllowedByRules     json.RawMessage `json:"ipsAllowedByRules"`
	DomainsBlockedByRules json.RawMessage `json:"domainsBlockedByRules"`
	IPsBlockedByRules     json.RawMessage `json:"ipsBlockedByRules"`
}

func parseDAP(raw string) *inventory.DAP {
	if raw == "" || raw[0] != '{' {
		return nil
	}
	var v dapJSON
	if json.Unmarshal([]byte(raw), &v) != nil || v.State == "" {
		return nil
	}
	trusted := countJSONArr(
		v.AllowedDomains, v.AllowedIPs, v.AllowedICMPs, v.AllowedHashsets,
		v.AllowedCloudDomains, v.AllowedKnockDomains, v.AllowedKnockIPs,
	)
	if trusted == 0 {
		trusted = countJSONArr(v.DomainsAllowedByRules, v.IPsAllowedByRules)
	}
	return &inventory.DAP{
		Status:  v.State,
		Reason:  v.Reason,
		Learned: v.LearnedCount,
		Trusted: trusted,
		Blocked: countJSONArr(v.DomainsBlockedByRules, v.IPsBlockedByRules),
	}
}

func countJSONArr(raws ...json.RawMessage) int64 {
	var n int64
	for _, raw := range raws {
		if len(raw) == 0 || raw[0] != '[' {
			continue
		}
		var arr []json.RawMessage
		if json.Unmarshal(raw, &arr) == nil {
			n += int64(len(arr))
		}
	}
	return n
}
