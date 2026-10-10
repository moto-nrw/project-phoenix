package account

import "github.com/go-chi/chi/v5"

// Router returns a configured router for auth endpoints, unthrottled.
func (rs *Resource) Router() chi.Router {
	return rs.RouterWithAuthRateLimiter(nil)
}
