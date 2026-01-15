package guardians

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	educationSvc "github.com/moto-nrw/project-phoenix/internal/core/service/education"
	userContextSvc "github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
	userSvc "github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the guardians API resource
type Resource struct {
	GuardianService    userSvc.GuardianService
	PersonService      userSvc.PersonService
	EducationService   educationSvc.Service
	UserContextService userContextSvc.UserContextService
	StudentService     userSvc.StudentService
}

// NewResource creates a new guardians resource
func NewResource(
	guardianService userSvc.GuardianService,
	personService userSvc.PersonService,
	educationService educationSvc.Service,
	userContextService userContextSvc.UserContextService,
	studentService userSvc.StudentService,
) *Resource {
	return &Resource{
		GuardianService:    guardianService,
		PersonService:      personService,
		EducationService:   educationService,
		UserContextService: userContextService,
		StudentService:     studentService,
	}
}

// Router returns a configured router for guardian endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth := jwt.MustTokenAuth()

	// Public routes for guardian invitations (no authentication required)
	r.Get("/invitations/{token}", rs.validateGuardianInvitation)
	r.Post("/invitations/{token}/accept", rs.acceptGuardianInvitation)

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

		// Guardian profile CRUD operations
		// Read operations require users:read permission
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/", rs.listGuardians)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}", rs.getGuardian)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/without-account", rs.listGuardiansWithoutAccount)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/invitable", rs.listInvitableGuardians)

		// Write operations - guardian profile creation allowed for all staff
		// Security enforced when linking guardians to students
		r.Post("/", rs.createGuardian)
		r.With(authorize.RequiresPermission(permissions.UsersUpdate)).Put("/{id}", rs.updateGuardian)
		r.With(authorize.RequiresPermission(permissions.UsersDelete)).Delete("/{id}", rs.deleteGuardian)

		// Guardian invitations
		r.With(authorize.RequiresPermission(permissions.UsersCreate)).Post("/{id}/invite", rs.sendInvitation)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/invitations/pending", rs.listPendingInvitations)

		// Student-guardian relationships
		// Anyone with users:read can view guardians (for emergency cases)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/students/{studentId}/guardians", rs.getStudentGuardians)
		r.With(authorize.RequiresPermission(permissions.UsersRead)).Get("/{id}/students", rs.getGuardianStudents)

		// Create/Update/Delete relationships - custom supervisor permissions checked in handlers
		r.Post("/students/{studentId}/guardians", rs.linkGuardianToStudent)
		r.Put("/relationships/{relationshipId}", rs.updateStudentGuardianRelationship)
		r.Delete("/students/{studentId}/guardians/{guardianId}", rs.removeGuardianFromStudent)
	})

	return r
}
