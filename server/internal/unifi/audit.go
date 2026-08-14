package unifi

type AuditGroup[T any] struct {
	Count int `json:"count"`
	Rows  []T `json:"rows"`
}

type Report struct {
	Names   AuditGroup[NameRow]       `json:"names"`
	VLAN    AuditGroup[VLANRow]       `json:"vlan"`
	STP     AuditGroup[STPRow]        `json:"stp"`
	Unknown AuditGroup[UnknownRow]    `json:"unknown"`
	Offline AuditGroup[OfflineRow]    `json:"offline"`
	Pending AuditGroup[PendingDevice] `json:"pending"`
}

type ReportInput struct {
	NameEnabled bool
	NameRows    []NameRow
	VLAN        []VLANRow
	STP         []STPRow
	Unknown     []UnknownRow
	Offline     []OfflineRow
	Pending     []PendingDevice
}

func BuildReport(in ReportInput) Report {
	names := in.NameRows
	if !in.NameEnabled || names == nil {
		names = []NameRow{}
	}
	vlan, stp := in.VLAN, in.STP
	if vlan == nil {
		vlan = []VLANRow{}
	}
	if stp == nil {
		stp = []STPRow{}
	}
	unknown, offline, pending := in.Unknown, in.Offline, in.Pending
	if unknown == nil {
		unknown = []UnknownRow{}
	}
	if offline == nil {
		offline = []OfflineRow{}
	}
	if pending == nil {
		pending = []PendingDevice{}
	}
	return Report{
		Names:   AuditGroup[NameRow]{Count: len(names), Rows: names},
		VLAN:    AuditGroup[VLANRow]{Count: len(vlan), Rows: vlan},
		STP:     AuditGroup[STPRow]{Count: len(stp), Rows: stp},
		Unknown: AuditGroup[UnknownRow]{Count: len(unknown), Rows: unknown},
		Offline: AuditGroup[OfflineRow]{Count: len(offline), Rows: offline},
		Pending: AuditGroup[PendingDevice]{Count: len(pending), Rows: pending},
	}
}
