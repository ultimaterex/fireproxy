package auth

// Scope is an API key / principal permission level.
type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
	ScopeAdmin Scope = "admin"
)

// HasScope reports whether have includes need, with implications:
// admin implies write implies read.
func HasScope(have []Scope, need Scope) bool {
	for _, s := range have {
		if scopeImplies(s, need) {
			return true
		}
	}
	return false
}

func scopeImplies(have, need Scope) bool {
	switch have {
	case ScopeAdmin:
		return need == ScopeAdmin || need == ScopeWrite || need == ScopeRead
	case ScopeWrite:
		return need == ScopeWrite || need == ScopeRead
	case ScopeRead:
		return need == ScopeRead
	default:
		return false
	}
}
