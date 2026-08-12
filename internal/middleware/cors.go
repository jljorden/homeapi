package middleware

import "net/http"

func CORS(next http.Handler) http.Handler {
	// allowedOrigins is a small whitelist of permitted origins.
	allowedOrigins := map[string]struct{}{
		"https://localhost.com:3000": {},
		"http://localhost.com:3000":  {},
		"https://jljorden.com":       {},
		"https://jljorden.com:13443": {},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			if _, ok := allowedOrigins[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				// Signal that the response varies by Origin for caches
				w.Header().Set("Vary", "Origin")
			}
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
