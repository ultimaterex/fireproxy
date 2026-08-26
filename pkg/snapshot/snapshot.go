// Package snapshot defines the FireProxy agent → server JSON contract.
package snapshot

// Snapshot is one collection tick from fireproxy-agent.
type Snapshot struct {
	TS             int64                 `json:"ts"`
	Host           string                `json:"host"`
	Load           Load                  `json:"load"`
	CPU            *CPU                  `json:"cpu,omitempty"`
	DNSRestarts    int64                 `json:"dns_restarts"`
	DNSSvcs        []DNSSvc              `json:"dns_svcs,omitempty"`
	DNSBlocksDelta *int64                `json:"dns_blocks_delta,omitempty"`
	Unbound        *Unbound              `json:"unbound,omitempty"`
	Ifaces         map[string]IfaceStats `json:"ifaces"`
	WAN            map[string]WANLink    `json:"wan"`
	Disks          []Disk                `json:"disks,omitempty"`
	NICMetrics     []NICMetric           `json:"nic_metrics,omitempty"`
}

// Disk is one filesystem mount (init fallback / optional agent).
type Disk struct {
	Mount      string  `json:"mount"`
	Filesystem string  `json:"filesystem,omitempty"`
	Capacity   float64 `json:"capacity"` // used fraction 0–1
	Size       int64   `json:"size,omitempty"`
	Used       int64   `json:"used,omitempty"`
	Available  int64   `json:"available,omitempty"`
}

// NICMetric is coarse rx/tx percentile traffic (bytes), not live Mbps.
type NICMetric struct {
	Name     string `json:"name"`
	RxMedian int64  `json:"rx_median,omitempty"`
	TxMedian int64  `json:"tx_median,omitempty"`
	RxPt90   int64  `json:"rx_pt90,omitempty"`
	TxPt90   int64  `json:"tx_pt90,omitempty"`
}

// Unbound is cache stats from unbound-control stats_noreset.
type Unbound struct {
	Queries int64   `json:"queries"`
	Hits    int64   `json:"hits"`
	Misses  int64   `json:"misses"`
	HitPct  float64 `json:"hit_pct"`
}

// DNSSvc is one DNS-stack process (unbound, dnsmasq, firerouter).
type DNSSvc struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Since int64  `json:"since,omitempty"`
}

// Load is /proc/loadavg style averages.
type Load struct {
	M1    float64 `json:"m1"`
	M5    float64 `json:"m5"`
	M15   float64 `json:"m15"`
	Cores int     `json:"cores,omitempty"` // logical CPUs; used for pressure %
}

// CPU is an optional short /proc/stat sample (percentages).
type CPU struct {
	User    float64 `json:"user"`
	Sys     float64 `json:"sys"`
	Idle    float64 `json:"idle"`
	Softirq float64 `json:"softirq"`
}

// IfaceStats are raw counters and link state from sysfs.
type IfaceStats struct {
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	SpeedMbps *int   `json:"speed_mbps"` // null for bridges without speed
	Carrier   bool   `json:"carrier"`
}

// WANLink is dual-WAN readiness from Redis event:state:cache (or equivalent).
type WANLink struct {
	Name   string `json:"name"`
	Ready  bool   `json:"ready"`
	Active bool   `json:"active"`
}
