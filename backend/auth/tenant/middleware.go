package tenant

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/betterauth"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/uptrace/bun"
)

// Middleware validates BetterAuth sessions and sets tenant context for requests.
//
// This middleware:
// 1. Forwards cookies to BetterAuth to validate the session
// 2. Retrieves the user's role in their active organization
// 3. Loads organization details (OGS) with Träger/Büro hierarchy
// 4. Resolves permissions from the role
// 5. Sets RLS context (SET LOCAL app.ogs_id) for PostgreSQL row-level security
// 6. Sets TenantContext on the request context for use by handlers and services
//
// IMPORTANT: This middleware should be applied AFTER IoT routes are mounted.
// IoT devices use their own authentication path (API key + PIN) and should NOT
// go through this middleware.
func Middleware(baClient *betterauth.Client, db *bun.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Step 1: Validate session with BetterAuth
			session, err := baClient.GetSession(ctx, r)
			if err != nil {
				handleAuthError(w, r, err, "session validation failed")
				return
			}

			// Step 2: Get member role from BetterAuth
			member, err := baClient.GetActiveMember(ctx, r)
			if err != nil {
				handleAuthError(w, r, err, "member lookup failed")
				return
			}

			// Step 3: Load organization details with Träger/Büro hierarchy
			org, err := loadOrganization(ctx, db, session.Session.ActiveOrganizationID)
			if err != nil {
				if logging.Logger != nil {
					logging.Logger.WithFields(map[string]interface{}{
						"error":  err.Error(),
						"org_id": session.Session.ActiveOrganizationID,
					}).Error("Failed to load organization")
				}
				_ = render.Render(w, r, ErrInternalServer)
				return
			}

			// Step 4: Resolve permissions from role
			permissions := GetPermissionsForRole(member.Role)

			// Step 5: Build tenant context
			tc := &TenantContext{
				UserID:      session.User.ID,
				UserEmail:   session.User.Email,
				UserName:    session.User.Name,
				OrgID:       org.ID,
				OrgName:     org.Name,
				OrgSlug:     org.Slug,
				Role:        member.Role,
				Permissions: permissions,
				TraegerID:   org.TraegerID,
				TraegerName: org.TraegerName,
			}

			// Set optional Büro context
			if org.BueroID != nil {
				tc.BueroID = org.BueroID
				tc.BueroName = org.BueroName
			}

			// Step 6: Set RLS context for PostgreSQL
			// Using SET LOCAL ensures the context is scoped to the current transaction.
			_, err = db.ExecContext(ctx, "SET LOCAL app.ogs_id = $1", tc.OrgID)
			if err != nil {
				if logging.Logger != nil {
					logging.Logger.WithFields(map[string]interface{}{
						"error":  err.Error(),
						"ogs_id": tc.OrgID,
					}).Error("Failed to set RLS context")
				}
				_ = render.Render(w, r, ErrInternalServer)
				return
			}

			// Log successful authentication
			if logging.Logger != nil {
				logging.Logger.WithFields(map[string]interface{}{
					"user_id":    tc.UserID,
					"user_email": tc.UserEmail,
					"org_id":     tc.OrgID,
					"org_name":   tc.OrgName,
					"role":       tc.Role,
					"traeger_id": tc.TraegerID,
				}).Debug("Tenant context set")
			}

			// Set context and continue
			ctx = SetTenantContext(ctx, tc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// handleAuthError logs the authentication error and returns appropriate HTTP response.
func handleAuthError(w http.ResponseWriter, r *http.Request, err error, context string) {
	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"error":   err.Error(),
			"path":    r.URL.Path,
			"context": context,
		}).Warn("Authentication failed")
	}

	// Map specific errors to responses
	switch err {
	case betterauth.ErrNoSession:
		_ = render.Render(w, r, ErrUnauthorized)
	case betterauth.ErrNoActiveOrg:
		_ = render.Render(w, r, ErrNoOrganization)
	case betterauth.ErrMemberNotFound:
		_ = render.Render(w, r, NewErrForbidden("not a member of this organization"))
	default:
		// For other errors (network, parsing), return unauthorized
		// Don't expose internal error details to client
		_ = render.Render(w, r, ErrUnauthorized)
	}
}

// organizationWithHierarchy holds organization data with resolved Träger/Büro names.
type organizationWithHierarchy struct {
	ID          string
	Name        string
	Slug        string
	TraegerID   string
	TraegerName string
	BueroID     *string
	BueroName   *string
}

// loadOrganization loads organization details with Träger and Büro hierarchy.
// The organization table is created by BetterAuth with custom fields traegerId and bueroId.
// We join with tenant.traeger and tenant.buero to get the names.
func loadOrganization(ctx context.Context, db *bun.DB, orgID string) (*organizationWithHierarchy, error) {
	var result organizationWithHierarchy

	// Query organization with Träger and optional Büro
	// Note: BetterAuth uses camelCase for custom fields (traegerId, bueroId)
	err := db.NewRaw(`
		SELECT
			o.id,
			o.name,
			o.slug,
			o."traegerId" AS traeger_id,
			t.name AS traeger_name,
			o."bueroId" AS buero_id,
			b.name AS buero_name
		FROM public.organization o
		INNER JOIN tenant.traeger t ON t.id = o."traegerId"
		LEFT JOIN tenant.buero b ON b.id = o."bueroId"
		WHERE o.id = ?
	`, orgID).Scan(ctx,
		&result.ID,
		&result.Name,
		&result.Slug,
		&result.TraegerID,
		&result.TraegerName,
		&result.BueroID,
		&result.BueroName,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrOrgNotFound
		}
		return nil, err
	}

	return &result, nil
}
