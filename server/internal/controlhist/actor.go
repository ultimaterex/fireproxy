package controlhist

import (
	"context"

	"fireproxy/server/internal/auth"
	"fireproxy/server/internal/store"
)

// ActorFromParts resolves actor_kind and actor from auth context parts (unit-testable).
// principalKind is auth.KindSession or auth.KindAPIKey when known.
func ActorFromParts(authEnabled bool, principalKind, authMethod, apiKeyName string) (kind, actor string) {
	if !authEnabled {
		return ActorUser, "admin"
	}
	if principalKind == string(auth.KindAPIKey) {
		if apiKeyName != "" {
			return ActorUser, "api:" + apiKeyName
		}
		return ActorUser, "api:key"
	}
	if authMethod == "oidc" {
		return ActorUser, "oidc"
	}
	return ActorUser, "admin"
}

// ActorFromContext resolves the actor for a request context.
func ActorFromContext(ctx context.Context, p *store.Persist, authEnabled bool) (kind, actor string) {
	if !authEnabled {
		return ActorFromParts(false, "", "", "")
	}
	pr, ok := auth.PrincipalFromContext(ctx)
	if !ok {
		return ActorFromParts(true, "", "", "")
	}
	switch pr.Kind {
	case auth.KindAPIKey:
		name := ""
		if p != nil && pr.APIKeyID != "" {
			if k, ok, _ := p.GetAPIKeyByID(pr.APIKeyID); ok {
				name = k.Name
			}
		}
		return ActorFromParts(true, string(auth.KindAPIKey), "", name)
	case auth.KindSession:
		method := ""
		if p != nil && pr.SessionID != "" {
			if s, ok, _ := auth.LookupSession(p, pr.SessionID); ok {
				method = s.AuthMethod
			}
		}
		return ActorFromParts(true, string(auth.KindSession), method, "")
	default:
		return ActorFromParts(true, "", "", "")
	}
}
