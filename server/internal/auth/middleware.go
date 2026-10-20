package auth

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"fireproxy/server/internal/store"
)

type principalCtxKey struct{}

// Kind identifies how a request was authenticated.
type Kind string

const (
	KindAnonymous Kind = "anonymous"
	KindSession   Kind = "session"
	KindAPIKey    Kind = "apikey"
	KindAgent     Kind = "agent"
)

// Principal is the authenticated caller attached to the request context.
type Principal struct {
	Kind      Kind
	Scopes    []Scope
	SessionID string
	APIKeyID  string
}

// PrincipalFromContext returns the Principal set by Middleware, if any.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}

// Middleware gates HTTP requests: principal resolve, CSRF, scopes, CORS, rate limits.
type Middleware struct {
	Cfg     Config
	Persist *store.Persist
	Agents  AgentVerifier
	Limiter *IPRateLimiter // optional; created lazily for sensitive routes
	touch   *touchGate     // coalesces last-used / idle-deadline writes
}

// Handler wraps next with auth gating and CORS.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	if m == nil {
		return next
	}
	if m.Limiter == nil {
		m.Limiter = NewIPRateLimiter(10, time.Minute)
	}
	if m.touch == nil {
		m.touch = newTouchGate(time.Minute)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.applyCORS(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if m.rateLimited(w, r) {
			return
		}
		p, ok := m.resolve(w, r)
		if !ok {
			return
		}
		// CSRF only for real browser sessions (cookie), not AUTH_DISABLED synthetic admin.
		if p.Kind == KindSession && p.SessionID != "" && isMutating(r.Method) {
			if !m.csrfOK(r) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		if p.Kind != KindAgent && p.Kind != KindAnonymous {
			need := requiredScope(r.Method, r.URL.Path)
			if need != "" && !HasScope(p.Scopes, need) {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
		}
		now := time.Now()
		if p.Kind == KindSession && p.SessionID != "" && m.Persist != nil {
			if m.touch.due("s:"+p.SessionID, now) {
				_ = TouchSessionIdle(m.Persist, m.Cfg, p.SessionID)
			}
		}
		if p.Kind == KindAPIKey && p.APIKeyID != "" && m.Persist != nil {
			if m.touch.due("k:"+p.APIKeyID, now) {
				_ = m.Persist.TouchAPIKeyLastUsed(p.APIKeyID, now.Unix())
			}
		}
		ctx := context.WithValue(r.Context(), principalCtxKey{}, p)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) resolve(w http.ResponseWriter, r *http.Request) (Principal, bool) {
	path := r.URL.Path
	method := r.Method

	if isAgentCredentialRoute(method, path) {
		if m.Agents == nil || !m.Agents.AuthorizeAgent(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return Principal{}, false
		}
		return Principal{Kind: KindAgent}, true
	}

	if token, via := extractAPIKeyCredential(r); via {
		if m.Persist == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return Principal{}, false
		}
		row, ok, err := LookupAPIKey(m.Persist, token)
		if err != nil || !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return Principal{}, false
		}
		return Principal{
			Kind:     KindAPIKey,
			Scopes:   ParseScopesCSV(row.Scopes),
			APIKeyID: row.ID,
		}, true
	}

	if sid := sessionIDFromRequest(r); sid != "" && m.Persist != nil {
		if _, ok, err := LookupSession(m.Persist, sid); err == nil && ok {
			return Principal{
				Kind:      KindSession,
				Scopes:    []Scope{ScopeRead, ScopeWrite, ScopeAdmin},
				SessionID: sid,
			}, true
		}
	}

	if isPublicAllowlist(method, path) {
		return Principal{Kind: KindAnonymous}, true
	}

	if m.Cfg.Disabled {
		return Principal{
			Kind:   KindSession, // synthetic full access; not a real cookie session
			Scopes: []Scope{ScopeRead, ScopeWrite, ScopeAdmin},
		}, true
	}

	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return Principal{}, false
}

func (m *Middleware) rateLimited(w http.ResponseWriter, r *http.Request) bool {
	if m.Limiter == nil {
		return false
	}
	if !isRateLimitedPath(r.Method, r.URL.Path) {
		return false
	}
	ip := clientIP(r, m.Cfg.TrustedProxy)
	if m.Limiter.Allow(ip) {
		return false
	}
	http.Error(w, "too many requests", http.StatusTooManyRequests)
	return true
}

func (m *Middleware) applyCORS(w http.ResponseWriter, r *http.Request) {
	if m.Cfg.Disabled {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		return
	}
	// Auth on: same-origin UI — omit ACAO=* (no broad CORS).
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" && m.originAllowed(r, origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
	}
}

func (m *Middleware) csrfOK(r *http.Request) bool {
	if origin := strings.TrimSpace(r.Header.Get("Origin")); origin != "" {
		return m.originAllowed(r, origin)
	}
	if ref := strings.TrimSpace(r.Header.Get("Referer")); ref != "" {
		u, err := url.Parse(ref)
		if err != nil || u.Host == "" {
			return false
		}
		return m.hostAllowed(r, u.Host)
	}
	return false
}

func (m *Middleware) originAllowed(r *http.Request, origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return m.hostAllowed(r, u.Host)
}

func (m *Middleware) hostAllowed(r *http.Request, host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	reqHost := strings.ToLower(strings.TrimSpace(r.Host))
	if hostsMatch(host, reqHost) {
		return true
	}
	if po := strings.TrimSpace(m.Cfg.PublicOrigin); po != "" {
		u, err := url.Parse(po)
		if err == nil && u.Host != "" && hostsMatch(host, strings.ToLower(u.Host)) {
			return true
		}
	}
	return false
}

// hostsMatch compares request Host / Origin hosts. nginx $host strips the port,
// so "localhost" must match Origin "localhost:3080".
func hostsMatch(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ah, ap, aErr := net.SplitHostPort(a)
	bh, bp, bErr := net.SplitHostPort(b)
	switch {
	case aErr == nil && bErr == nil:
		return ah == bh && ap == bp
	case aErr == nil && bErr != nil:
		return ah == b
	case aErr != nil && bErr == nil:
		return a == bh
	default:
		return false
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isAgentCredentialRoute(method, path string) bool {
	switch path {
	case "/v1/ingest", "/v1/catalog", "/v1/devices":
		return method == http.MethodPost
	case "/v1/agent/ws":
		return method == http.MethodGet
	default:
		return false
	}
}

func isPublicAllowlist(method, path string) bool {
	if path == "/healthz" {
		return true
	}
	if path == "/install-agent.sh" && method == http.MethodGet {
		return true
	}
	if strings.HasPrefix(path, "/agent/") {
		return true
	}
	if path == "/v1/agent/enroll" && method == http.MethodPost {
		return true
	}
	if path == "/v1/auth/login" && method == http.MethodPost {
		return true
	}
	if path == "/v1/auth/login-options" && method == http.MethodGet {
		return true
	}
	if path == "/v1/auth/oidc/start" && method == http.MethodGet {
		return true
	}
	if path == "/v1/auth/oidc/callback" && method == http.MethodGet {
		return true
	}
	if path == "/v1/auth/logout" && method == http.MethodPost {
		return true
	}
	if path == "/v1/auth/me" && method == http.MethodGet {
		return true
	}
	return false
}

func isRateLimitedPath(method, path string) bool {
	if path == "/v1/auth/login" && method == http.MethodPost {
		return true
	}
	if path == "/v1/auth/oidc/callback" && method == http.MethodGet {
		return true
	}
	if path == "/v1/agent/enroll" && method == http.MethodPost {
		return true
	}
	return false
}

// requiredScope returns the minimum scope for a non-agent route, or "" if none.
func requiredScope(method, path string) Scope {
	if isIAMPath(path) || (path == "/v1/agent/enroll/code" && method == http.MethodPost) ||
		(path == "/v1/agent/update" && method == http.MethodPost) {
		return ScopeAdmin
	}
	if method == http.MethodGet || method == http.MethodHead {
		return ScopeRead
	}
	// WebSocket upgrades are GET; logs WS already covered. Mutating methods:
	if isMutating(method) {
		return ScopeWrite
	}
	return ScopeRead
}

func isIAMPath(path string) bool {
	if strings.HasPrefix(path, "/v1/auth/api-keys") {
		return true
	}
	if strings.HasPrefix(path, "/v1/auth/agent-credentials") {
		return true
	}
	// Admin OIDC settings (not public start/callback).
	if path == "/v1/auth/oidc" || strings.HasPrefix(path, "/v1/auth/oidc/") {
		if path == "/v1/auth/oidc/start" || path == "/v1/auth/oidc/callback" {
			return false
		}
		return true
	}
	return false
}

func sessionIDFromRequest(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil || c == nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

// extractBearerOrAPIKey pulls a token from the X-API-Key header or an
// Authorization: Bearer header. It is the single source of truth for
// credential parsing, shared by the API-key and agent auth paths so their
// Bearer/X-API-Key handling cannot drift apart.
func extractBearerOrAPIKey(r *http.Request) (token string, present bool) {
	if k := strings.TrimSpace(r.Header.Get("X-API-Key")); k != "" {
		return k, true
	}
	authz := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(authz) > len(prefix) && strings.EqualFold(authz[:len(prefix)], prefix) {
		if t := strings.TrimSpace(authz[len(prefix):]); t != "" {
			return t, true
		}
	}
	return "", false
}

// extractAPIKeyCredential returns the token if an explicit API key header is present.
// Presence (even invalid) takes precedence over the session cookie.
func extractAPIKeyCredential(r *http.Request) (token string, present bool) {
	return extractBearerOrAPIKey(r)
}

func clientIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			// Use the rightmost entry: our trusted proxy appends the address it
			// actually observed. Earlier entries are client-supplied and can be
			// spoofed to rotate around a per-IP rate limit. Requires the proxy
			// to append (nginx: $proxy_add_x_forwarded_for), not overwrite.
			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				if ip := strings.TrimSpace(parts[i]); ip != "" {
					return ip
				}
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// CheckOriginSameHost returns a websocket CheckOrigin that allows empty Origin
// (non-browser agents) or Origin host matching the request Host / publicOrigin.
func CheckOriginSameHost(publicOrigin string) func(*http.Request) bool {
	return func(r *http.Request) bool {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" {
			return false
		}
		if hostsMatch(u.Host, r.Host) {
			return true
		}
		if po := strings.TrimSpace(publicOrigin); po != "" {
			pu, err := url.Parse(po)
			if err == nil && pu.Host != "" && hostsMatch(u.Host, pu.Host) {
				return true
			}
		}
		return false
	}
}
