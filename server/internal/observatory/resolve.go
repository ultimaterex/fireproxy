package observatory

import "time"

// Pick selects the observatory data source.
//
// Rules (locked):
//  1. Agent online + agent age within TTL → agent
//  2. Agent online + agent age ≥ TTL + warm init → fw-app-init
//  3. Agent online + age ≥ TTL + cold init → empty (do not serve expired agent)
//  4. Agent offline + warm init → fw-app-init even if catalog recent
//  5. Agent offline + cold init / unpaired → empty
func Pick(agentOnline bool, agentAge, agentTTL time.Duration, initOK bool, initAt time.Time) (Provenance, bool) {
	if !agentOnline {
		if initOK {
			return Provenance{Source: SourceFWAppInit, FetchedAt: initAt}, true
		}
		return Provenance{Source: SourceEmpty}, false
	}

	if agentAge < agentTTL {
		return Provenance{Source: SourceAgent}, false
	}

	if initOK {
		return Provenance{Source: SourceFWAppInit, FetchedAt: initAt}, true
	}
	return Provenance{Source: SourceEmpty}, false
}
