package middleware

import (
	"net/http"
)

// SecurityHeaders adds security headers to all responses
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Prevent clickjacking attacks
		w.Header().Set("X-Frame-Options", "DENY")
		
		// Prevent MIME type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")
		
		// Enable XSS protection in older browsers
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		
		// Control referrer information
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Permissions Policy (replaces Feature-Policy)
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		
		// Content Security Policy - adjust based on your needs
		// This is a strict policy, you may need to relax it for your frontend
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " + // May need to adjust for your frontend framework
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: https:; " +
			"font-src 'self'; " +
			"connect-src 'self'; " +
			"frame-ancestors 'none'; " +
			"base-uri 'self'; " +
			"form-action 'self'"
		w.Header().Set("Content-Security-Policy", csp)
		
		// Strict Transport Security (HSTS) - only for HTTPS
		// Only set this if you're sure the site will always be served over HTTPS
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			// max-age=31536000 = 1 year
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}
		
		next.ServeHTTP(w, r)
	})
}