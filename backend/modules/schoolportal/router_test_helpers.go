//go:build testhooks

package schoolportal

import (
	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// Router returns the chi router scoped to /school without a rate limiter on
// the public auth endpoints; tests drive it directly.
func (rs *Resource) Router(configureSession func(chi.Router), protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	return rs.RouterWithAuthRateLimiter(nil, configureSession, protectedSchoolRoutes)
}
