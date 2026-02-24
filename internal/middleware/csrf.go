package middleware

import (
	"net/http"

	"pdf-studio/internal/services"
)

func CSRFProtection(authSvc *services.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only protect state-changing methods
			if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
				next.ServeHTTP(w, r)
				return
			}

			// Check for JSON content type (API calls) — use session cookie auth
			ct := r.Header.Get("Content-Type")
			if ct == "application/json" || (len(ct) > 16 && ct[:16] == "application/json") {
				// For JSON API calls, the SameSite cookie + origin check is sufficient
				origin := r.Header.Get("Origin")
				if origin != "" {
					host := r.Host
					// Simple origin check
					if origin != "http://"+host && origin != "https://"+host {
						http.Error(w, `{"error":"csrf validation failed"}`, http.StatusForbidden)
						return
					}
				}
				next.ServeHTTP(w, r)
				return
			}

			// For form submissions, check CSRF token
			session := GetSession(r)
			if session == nil {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}

			token := r.FormValue("_csrf")
			if token == "" {
				token = r.Header.Get("X-CSRF-Token")
			}

			if token == "" || !authSvc.ValidateCSRFToken(token, session.ID) {
				http.Error(w, "CSRF token invalid", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
