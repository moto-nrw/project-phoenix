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
			subject := createSubjectFromContext(r)
			resourceID, extra := applyExtractors(r, extractors)

			resource := policy.Resource{
				Type: resourceType,
				ID:   resourceID,
			}

			allowed, err := ra.authService.AuthorizeResource(r.Context(), subject, resource, action, extra)
			if err != nil {
				handleAuthorizationError(w, r, err)
				return
			}

			if !allowed {
				handleForbiddenResponse(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// createSubjectFromContext creates a policy subject from JWT context
func createSubjectFromContext(r *http.Request) policy.Subject {
	claims := jwt.ClaimsFromCtx(r.Context())
	permissions := jwt.PermissionsFromCtx(r.Context())

	return policy.Subject{
		AccountID:   int64(claims.ID),
		Roles:       claims.Roles,
		Permissions: permissions,
	}
}

// applyExtractors applies all resource extractors and collects resource ID and extra data
func applyExtractors(r *http.Request, extractors []ResourceExtractor) (interface{}, map[string]interface{}) {
	var resourceID interface{}
	extra := make(map[string]interface{})

	for _, extractor := range extractors {
		id, extraData := extractor(r)
		if id != nil {
			resourceID = id
		}
		for k, v := range extraData {
			extra[k] = v
		}
	}

	return resourceID, extra
}

// handleAuthorizationError handles authorization error response
func handleAuthorizationError(w http.ResponseWriter, r *http.Request, err error) {
	if renderErr := render.Render(w, r, &ErrResponse{
		HTTPStatusCode: http.StatusInternalServerError,
		StatusText:     "Authorization error",
		ErrorText:      err.Error(),
	}); renderErr != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleForbiddenResponse handles forbidden response
func handleForbiddenResponse(w http.ResponseWriter, r *http.Request) {
	if renderErr := render.Render(w, r, ErrForbidden); renderErr != nil {
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
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
