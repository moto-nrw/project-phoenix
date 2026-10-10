package schoolportal

import "github.com/go-chi/chi/v5"

// Router returns the chi router scoped to /school without a rate limiter on
// the public auth endpoints; tests drive it directly.
func (rs *Resource) Router() chi.Router {
	return rs.RouterWithAuthRateLimiter(nil)
}
