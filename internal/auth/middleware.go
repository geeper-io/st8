package auth

import (
	"net/http"
	"strings"
)

// routeVerb maps "METHOD /path" to the required verb.
var routeVerb = map[string]string{
	"GET /v1/state":        VerbRead,
	"GET /v1/log":          VerbRead,
	"GET /v1/branches":     VerbRead,
	"POST /v1/diff":        VerbRead,
	"POST /v1/apply":       VerbWrite,
	"POST /v1/checkpoints": VerbWrite,
	"POST /v1/rollback":    VerbWrite,
	"POST /v1/branches":    VerbWrite,
	"POST /v1/restore":     VerbWrite,
	"POST /v1/gc":          VerbAdmin,
}

// unauthenticatedPaths are always allowed without a token.
var unauthenticatedPaths = map[string]bool{
	"/healthz": true,
	"/readyz":  true,
	"/metrics": true,
}

// Middleware returns an HTTP middleware that enforces token-based RBAC.
//
// It validates the Bearer token and the required HTTP verb for the route, then
// attaches the Principal to the request context. Namespace, branch, and
// key-prefix enforcement is handled by the service layer.
func Middleware(cfg *Config, next http.Handler) http.Handler {
	index := make(map[string]int, len(cfg.Tokens))
	for i, entry := range cfg.Tokens {
		index[entry.Token] = i
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if unauthenticatedPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		raw := r.Header.Get("Authorization")
		if !strings.HasPrefix(raw, "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		tokenValue := strings.TrimPrefix(raw, "Bearer ")

		idx, ok := index[tokenValue]
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		entry := cfg.Tokens[idx]
		principal := &Principal{Name: entry.Name, Allow: entry.Allow}

		verb, known := routeVerb[r.Method+" "+r.URL.Path]
		if known && !principal.AllowsVerb(verb) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r.WithContext(ContextWithPrincipal(r.Context(), principal)))
	})
}
