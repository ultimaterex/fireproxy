package tplink

import (
	"regexp"
	"sort"
	"strings"
	"sync"

	"fireproxy/pkg/inventory"
)

var reTPLinkVendor = regexp.MustCompile(`(?i)tp-?link`)

// Easy Smart / JetStream model names commonly seen in DHCP/hostname.
var reSwitchModel = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:tl-?sg|tl-?sl|tl-?st|sg10|sg1[0-9]|jet\s*stream|easy\s*smart)`)

// Candidate is a detected Easy Smart switch not yet configured.
type Candidate struct {
	MAC    string `json:"mac"`
	IP     string `json:"ip,omitempty"`
	Name   string `json:"name,omitempty"`
	Vendor string `json:"vendor,omitempty"`
	Type   string `json:"type,omitempty"`
}

func NormalizeMAC(mac string) string {
	mac = strings.ToUpper(strings.TrimSpace(mac))
	mac = strings.ReplaceAll(mac, "-", ":")
	return mac
}

func IsTPLinkVendor(vendor string) bool {
	return reTPLinkVendor.MatchString(vendor)
}

// LooksLikeSwitch uses Firewalla detect.type and/or Easy Smart model name patterns.
func LooksLikeSwitch(typ, name string) bool {
	if strings.EqualFold(strings.TrimSpace(typ), "switch") {
		return true
	}
	return reSwitchModel.MatchString(name)
}

type hint struct {
	MAC, IP, Name, Vendor, Type string
}

// CandidatesFromCatalog finds Easy Smart switches among TP-Link-looking hosts.
// When cache is non-nil, unknown hosts with an IP are soft-probed once (result cached by MAC).
func CandidatesFromCatalog(cat inventory.Catalog, configured []Config, cache *ProbeCache) []Candidate {
	skipMAC := map[string]bool{}
	skipIP := map[string]bool{}
	for _, c := range configured {
		if c.MAC != "" {
			skipMAC[NormalizeMAC(c.MAC)] = true
		}
		if c.IP != "" {
			skipIP[c.IP] = true
		}
	}

	hints := map[string]hint{}
	for _, d := range cat.Devices {
		mac := NormalizeMAC(d.MAC)
		if mac == "" || skipMAC[mac] {
			continue
		}
		if d.IP != "" && skipIP[d.IP] {
			continue
		}
		if !IsTPLinkVendor(d.Vendor) {
			continue
		}
		h := hints[mac]
		h.MAC = mac
		if d.IP != "" {
			h.IP = d.IP
		}
		if d.Name != "" {
			h.Name = d.Name
		}
		if d.Vendor != "" {
			h.Vendor = d.Vendor
		}
		if d.Type != "" {
			h.Type = d.Type
		}
		hints[mac] = h
	}

	type job struct {
		h hint
	}
	var needProbe []job
	var confirmed []Candidate

	for _, h := range hints {
		if cache == nil {
			// No probe cache/persist: fall back to type/name only.
			if LooksLikeSwitch(h.Type, h.Name) {
				confirmed = append(confirmed, Candidate{
					MAC: h.MAC, IP: h.IP, Name: h.Name, Vendor: h.Vendor, Type: h.Type,
				})
			}
			continue
		}
		if r, ok := cache.Get(h.MAC); ok && r.EasySmart && r.Via == "http" {
			confirmed = append(confirmed, Candidate{
				MAC: h.MAC, IP: h.IP, Name: h.Name, Vendor: h.Vendor, Type: h.Type,
			})
			continue
		}
		if r, ok := cache.Get(h.MAC); ok && !r.EasySmart && (h.IP == "" || r.IP == h.IP) {
			continue
		}
		if h.IP == "" {
			continue
		}
		needProbe = append(needProbe, job{h: h})
	}

	if cache != nil && len(needProbe) > 0 {
		// Probe hinted switches first so true Easy Smart boxes surface quickly.
		sort.SliceStable(needProbe, func(i, j int) bool {
			hi := LooksLikeSwitch(needProbe[i].h.Type, needProbe[i].h.Name)
			hj := LooksLikeSwitch(needProbe[j].h.Type, needProbe[j].h.Name)
			if hi != hj {
				return hi
			}
			return needProbe[i].h.IP < needProbe[j].h.IP
		})
		var mu sync.Mutex
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for _, j := range needProbe {
			j := j
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				ok, _ := cache.Resolve(j.h.MAC, j.h.IP)
				if !ok {
					return
				}
				mu.Lock()
				confirmed = append(confirmed, Candidate{
					MAC: j.h.MAC, IP: j.h.IP, Name: j.h.Name, Vendor: j.h.Vendor, Type: j.h.Type,
				})
				mu.Unlock()
			}()
		}
		wg.Wait()
	}

	sort.SliceStable(confirmed, func(i, j int) bool {
		ai, aj := confirmed[i].IP != "", confirmed[j].IP != ""
		if ai != aj {
			return ai
		}
		if confirmed[i].IP != confirmed[j].IP {
			return confirmed[i].IP < confirmed[j].IP
		}
		return confirmed[i].MAC < confirmed[j].MAC
	})
	return confirmed
}
