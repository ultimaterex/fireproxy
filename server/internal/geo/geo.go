package geo

import (
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/oschwald/geoip2-golang"

	pkggeo "fireproxy/pkg/geo"
	"fireproxy/pkg/inventory"
)

// Lookup returns an ISO country code for an IP.
type Lookup interface {
	Country(ip string) string
}

type mmdb struct {
	db *geoip2.Reader
}

func (g *mmdb) Country(ip string) string {
	if g == nil || g.db == nil {
		return ""
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ""
	}
	rec, err := g.db.Country(parsed)
	if err != nil || rec == nil {
		return ""
	}
	return strings.ToUpper(rec.Country.IsoCode)
}

func (g *mmdb) Close() error {
	if g == nil || g.db == nil {
		return nil
	}
	err := g.db.Close()
	g.db = nil
	return err
}

// OpenMMDB opens a GeoLite2-Country.mmdb. Returns nil if missing/unreadable.
func OpenMMDB(path string) Lookup {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	db, err := geoip2.Open(path)
	if err != nil {
		return nil
	}
	return &mmdb{db: db}
}

// ResolvePath picks GEOIP_MMDB, else <data-dir>/GeoLite2-Country.mmdb.
func ResolvePath(explicit, dataPath string) string {
	if p := strings.TrimSpace(explicit); p != "" {
		return p
	}
	if dataPath = strings.TrimSpace(dataPath); dataPath != "" {
		return filepath.Join(filepath.Dir(dataPath), "GeoLite2-Country.mmdb")
	}
	return "GeoLite2-Country.mmdb"
}

// EnrichBox fills country/region from the public IP.
func EnrichBox(b *inventory.Box, look Lookup) {
	if b == nil {
		return
	}
	cc := pkggeo.Code(b.Country)
	if cc == "" && b.PublicIP != "" && look != nil {
		cc = pkggeo.Code(look.Country(b.PublicIP))
	}
	b.Country = cc
	if cc != "" {
		b.Region = pkggeo.Name(cc)
	}
}

// Enrich fills dest country/region from MMDB and re-ranks dests + regions.
// Intel country from the agent wins when already set.
// Agent region Devices/Targets are kept and merged onto re-ranked regions by country.
func Enrich(d *inventory.Dashboard, look Lookup) {
	if d == nil {
		return
	}
	prevRegions := d.TopRegions
	flows := fill(d.DestFlows, look)
	if len(flows) > 0 {
		d.TopDestUpload = rank(flows, 5, "up")
		d.TopDestDownload = rank(flows, 5, "down")
		d.TopRegions = regions(flows, 5)
		attachRegionSlices(d.TopRegions, prevRegions)
		return
	}
	d.TopDestUpload = fill(d.TopDestUpload, look)
	d.TopDestDownload = fill(d.TopDestDownload, look)
}

func attachRegionSlices(dst, src []inventory.RankedFlow) {
	if len(dst) == 0 || len(src) == 0 {
		return
	}
	by := map[string]inventory.RankedFlow{}
	for _, r := range src {
		cc := pkggeo.Code(r.Country)
		if cc == "" {
			cc = pkggeo.Code(r.ID)
		}
		if cc == "" {
			continue
		}
		by[cc] = r
	}
	for i := range dst {
		cc := pkggeo.Code(dst[i].Country)
		if cc == "" {
			cc = pkggeo.Code(dst[i].ID)
		}
		prev, ok := by[cc]
		if !ok {
			continue
		}
		if len(dst[i].Devices) == 0 {
			dst[i].Devices = prev.Devices
		}
		if len(dst[i].Targets) == 0 {
			dst[i].Targets = prev.Targets
		}
	}
}

func fill(rows []inventory.RankedFlow, look Lookup) []inventory.RankedFlow {
	if len(rows) == 0 {
		return nil
	}
	out := append([]inventory.RankedFlow(nil), rows...)
	for i := range out {
		cc := pkggeo.Code(out[i].Country)
		ip := strings.TrimSpace(out[i].DestIP)
		if ip == "" && net.ParseIP(out[i].ID) != nil {
			ip = out[i].ID
			out[i].DestIP = ip
		}
		if cc == "" && ip != "" && look != nil {
			cc = pkggeo.Code(look.Country(ip))
		}
		out[i].Country = cc
		if cc != "" {
			out[i].Region = pkggeo.Name(cc)
		}
	}
	return out
}

func rank(rows []inventory.RankedFlow, n int, key string) []inventory.RankedFlow {
	out := append([]inventory.RankedFlow(nil), rows...)
	sortFlows(out, key)
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func regions(rows []inventory.RankedFlow, n int) []inventory.RankedFlow {
	by := map[string]*inventory.RankedFlow{}
	byDev := map[string]map[string]*inventory.RankedFlow{}
	byTgt := map[string]map[string]*inventory.RankedFlow{}
	order := []string{}
	for _, r := range rows {
		cc := pkggeo.Code(r.Country)
		if cc == "" {
			continue
		}
		a := by[cc]
		if a == nil {
			a = &inventory.RankedFlow{ID: cc, Name: pkggeo.Name(cc), Country: cc}
			by[cc] = a
			order = append(order, cc)
		}
		a.Upload += r.Upload
		a.Download += r.Download
		a.Bytes += r.Bytes
		if a.Bytes == 0 {
			a.Bytes = a.Upload + a.Download
		}
		tid := r.ID
		if tid == "" {
			tid = r.Name
		}
		if tid != "" {
			addNestedFlow(byTgt, cc, tid, inventory.RankedFlow{
				ID: tid, Name: r.Name, DestIP: r.DestIP, Country: cc, Region: pkggeo.Name(cc),
				Upload: r.Upload, Download: r.Download, Bytes: r.Bytes,
			})
		}
		for _, d := range r.Devices {
			did := d.ID
			if did == "" {
				did = d.Name
			}
			if did == "" {
				continue
			}
			addNestedFlow(byDev, cc, did, d)
		}
	}
	out := make([]inventory.RankedFlow, 0, len(order))
	for _, cc := range order {
		row := *by[cc]
		row.Devices = topFlows(byDev[cc], 8)
		row.Targets = topFlows(byTgt[cc], 8)
		out = append(out, row)
	}
	sortFlows(out, "total")
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func addNestedFlow(m map[string]map[string]*inventory.RankedFlow, outer, id string, add inventory.RankedFlow) {
	inner := m[outer]
	if inner == nil {
		inner = map[string]*inventory.RankedFlow{}
		m[outer] = inner
	}
	a := inner[id]
	if a == nil {
		cp := add
		if cp.ID == "" {
			cp.ID = id
		}
		if cp.Bytes == 0 {
			cp.Bytes = cp.Upload + cp.Download
		}
		inner[id] = &cp
		return
	}
	a.Upload += add.Upload
	a.Download += add.Download
	a.Bytes += add.Bytes
	if a.Bytes == 0 {
		a.Bytes = a.Upload + a.Download
	}
	if a.Name == "" {
		a.Name = add.Name
	}
	if a.Type == "" {
		a.Type = add.Type
	}
	if a.DestIP == "" {
		a.DestIP = add.DestIP
	}
}

func topFlows(m map[string]*inventory.RankedFlow, n int) []inventory.RankedFlow {
	if len(m) == 0 {
		return nil
	}
	out := make([]inventory.RankedFlow, 0, len(m))
	for _, a := range m {
		out = append(out, *a)
	}
	sortFlows(out, "total")
	if n > 0 && len(out) > n {
		out = out[:n]
	}
	return out
}

func sortFlows(out []inventory.RankedFlow, key string) {
	sort.Slice(out, func(i, j int) bool {
		vi, vj := out[i].Bytes, out[j].Bytes
		switch key {
		case "up":
			vi, vj = out[i].Upload, out[j].Upload
		case "down":
			vi, vj = out[i].Download, out[j].Download
		}
		if vi != vj {
			return vi > vj
		}
		return out[i].Name < out[j].Name
	})
}
