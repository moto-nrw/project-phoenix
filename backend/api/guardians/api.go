package guardians

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	userContextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Resource defines the guardians API resource
type Resource struct {
	GuardianService    guardianSvc.GuardianService
	PersonService      guardianSvc.PersonService
	EducationService   educationSvc.Service
	UserContextService userContextSvc.UserContextService
	db                 *bun.DB
}

// NewResource creates a new guardians resource
func NewResource(
	guardianService guardianSvc.GuardianService,
	personService guardianSvc.PersonService,
	educationService educationSvc.Service,
	userContextService userContextSvc.UserContextService,
	db *bun.DB,
) *Resource {
	return &Resource{
		GuardianService:    guardianService,
		PersonService:      personService,
		EducationService:   educationService,
		UserContextService: userContextService,
		db:                 db,
	}
}

// Router returns a configured router for guardian endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustNewTokenAuth()

	// Public routes for guardian invitations (no authentication required)
	r.Get("/invitations/{token}", rs.validateGuardianInvitation)
	r.Post("/invitations/{token}/accept", rs.acceptGuardianInvitation)

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)
		r.Use(jwt.TenantMiddleware)
		withTx := tenant.TenantTxMiddleware(rs.db)

		// Guardian profile CRUD operations
		// Read operations require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listGuardians)
		// Guardian picker search (#1513): same users:read gate as the rest of the
		// guardian reads, so anyone who can reach the student create/detail flows
		// can link a sibling's existing guardian. Returns a minimal,
		// enumeration-resistant projection (name, email, linked-children count) —
		// see searchGuardiansForPicker.
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/search", rs.searchGuardiansForPicker)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getGuardian)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/without-account", rs.listGuardiansWithoutAccount)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/invitable", rs.listInvitableGuardians)

		// Write operations - guardian profile creation allowed for all staff
		// Security enforced when linking guardians to students
		r.With(withTx).Post("/", rs.createGuardian)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateGuardian)
		// Read-only preview of a full delete's blast radius (admin-only check in
		// the handler). Lets the UI show the affected children before confirming
		// without the old destructive "probe DELETE" (#819).
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Get("/{id}/delete-preview", rs.guardianDeletePreview)
		r.With(authorize.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteGuardian)

		// Guardian invitations
		r.With(authorize.RequiresPermission(permissions.UsersCreate), withTx).Post("/{id}/invite", rs.sendInvitation)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/invitations/pending", rs.listPendingInvitations)

		// Student-guardian relationships
		// Anyone with users:read can view guardians (for emergency cases)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/students/{studentId}/guardians", rs.getStudentGuardians)
		r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/students", rs.getGuardianStudents)

		// Create/Update/Delete relationships - custom supervisor permissions checked in handlers
		r.With(withTx).Post("/students/{studentId}/guardians", rs.linkGuardianToStudent)
		r.With(withTx).Put("/relationships/{relationshipId}", rs.updateStudentGuardianRelationship)
		r.With(withTx).Delete("/students/{studentId}/guardians/{guardianId}", rs.removeGuardianFromStudent)

		// Phone number management (nested under guardian)
		r.Route("/{id}/phone-numbers", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listGuardianPhoneNumbers)
			r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/", rs.addPhoneNumber)
			r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{phoneId}", rs.updatePhoneNumber)
			r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{phoneId}", rs.deletePhoneNumber)
			r.With(authorize.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{phoneId}/set-primary", rs.setPrimaryPhone)
		})
	})

	return r
}
