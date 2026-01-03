package authorize

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// ResourceAuthorizer provides resource-specific authorization middleware
type ResourceAuthorizer struct {
	authService AuthorizationService
}

// NewResourceAuthorizer creates a new resource authorizer
func NewResourceAuthorizer(authService AuthorizationService) *ResourceAuthorizer {
	return &ResourceAuthorizer{
		authService: authService,
	}
}

// RequiresResourceAccess creates middleware that checks resource-specific access
func (ra *ResourceAuthorizer) RequiresResourceAccess(resourceType string, action policy.Action, extractors ...ResourceExtractor) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get claims from context
			claims := jwt.ClaimsFromCtx(r.Context())
			permissions := jwt.PermissionsFromCtx(r.Context())

			// Create subject from claims
			subject := policy.Subject{
				AccountID:   int64(claims.ID),
				Roles:       claims.Roles,
				Permissions: permissions,
			}

			// Extract resource ID and extra context
			var resourceID interface{}
			extra := make(map[string]interface{})

			// Apply extractors
			for _, extractor := range extractors {
				id, extraData := extractor(r)
				if id != nil {
					resourceID = id
				}
				for k, v := range extraData {
					extra[k] = v
				}
			}

			// Create resource
			resource := policy.Resource{
				Type: resourceType,
				ID:   resourceID,
			}

			// Authorize
			allowed, err := ra.authService.AuthorizeResource(r.Context(), subject, resource, action, extra)
			if err != nil {
				if render.Render(w, r, &ErrResponse{
					HTTPStatusCode: http.StatusInternalServerError,
					StatusText:     "Authorization error",
					ErrorText:      err.Error(),
				}) != nil {
					// Error already occurred while sending the response
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
				return
			}

			if !allowed {
				if render.Render(w, r, ErrForbidden) != nil {
					// Error already occurred while sending the response
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ResourceExtractor extracts resource information from the request
type ResourceExtractor func(r *http.Request) (resourceID interface{}, extra map[string]interface{})

// URLParamExtractor extracts resource ID from URL parameter
func URLParamExtractor(paramName string) ResourceExtractor {
	return func(r *http.Request) (interface{}, map[string]interface{}) {
		value := chi.URLParam(r, paramName)
		if value == "" {
			return nil, nil
		}

		// Try to parse as int64
		if id, err := strconv.ParseInt(value, 10, 64); err == nil {
			return id, nil
		}

		return value, nil
	}
}

// StudentIDFromURL extracts student ID from URL parameter
func StudentIDFromURL() ResourceExtractor {
	return func(r *http.Request) (interface{}, map[string]interface{}) {
		idStr := chi.URLParam(r, "id")
		if idStr == "" {
			return nil, nil
		}

		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return nil, nil
		}

		return nil, map[string]interface{}{"student_id": id}
	}
}

// CombinePermissionAndResource creates middleware that checks both permissions and resource access
func CombinePermissionAndResource(permission string, resourceType string, action policy.Action, extractors ...ResourceExtractor) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// First check permission
		permHandler := RequiresPermission(permission)(next)

		// Then check resource access if permission passes
		resourceHandler := GetResourceAuthorizer().RequiresResourceAccess(resourceType, action, extractors...)(permHandler)

		return resourceHandler
	}
}

// Global resource authorizer instance
var defaultResourceAuthorizer *ResourceAuthorizer

// SetResourceAuthorizer sets the default resource authorizer
func SetResourceAuthorizer(ra *ResourceAuthorizer) {
	defaultResourceAuthorizer = ra
}

// GetResourceAuthorizer gets the default resource authorizer
func GetResourceAuthorizer() *ResourceAuthorizer {
	if defaultResourceAuthorizer == nil {
		// Create a default one if not set
		defaultResourceAuthorizer = NewResourceAuthorizer(NewAuthorizationService())
	}
	return defaultResourceAuthorizer
}
