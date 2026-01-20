package guardians

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/moto-nrw/project-phoenix/models/users"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	userContextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the guardians API resource
type Resource struct {
	GuardianService    guardianSvc.GuardianService
	PersonService      guardianSvc.PersonService
	EducationService   educationSvc.Service
	UserContextService userContextSvc.UserContextService
	StudentRepo        users.StudentRepository
}

// NewResource creates a new guardians resource
func NewResource(
	guardianService guardianSvc.GuardianService,
	personService guardianSvc.PersonService,
	educationService educationSvc.Service,
	userContextService userContextSvc.UserContextService,
	studentRepo users.StudentRepository,
) *Resource {
	return &Resource{
		GuardianService:    guardianService,
		PersonService:      personService,
		EducationService:   educationService,
		UserContextService: userContextService,
		StudentRepo:        studentRepo,
	}
}

// Router returns a configured router for guardian endpoints
// Note: Authentication is handled by tenant middleware in base.go when TENANT_AUTH_ENABLED=true
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Public routes for guardian invitations (no authentication required)
	r.Get("/invitations/{token}", rs.validateGuardianInvitation)
	r.Post("/invitations/{token}/accept", rs.acceptGuardianInvitation)

	// Guardian profile CRUD operations
	// Read operations require guardian:read permission
	r.With(tenant.RequiresPermission("guardian:read")).Get("/", rs.listGuardians)
	r.With(tenant.RequiresPermission("guardian:read")).Get("/{id}", rs.getGuardian)
	r.With(tenant.RequiresPermission("guardian:read")).Get("/without-account", rs.listGuardiansWithoutAccount)
	r.With(tenant.RequiresPermission("guardian:read")).Get("/invitable", rs.listInvitableGuardians)

	// Write operations - guardian profile creation allowed for all staff
	// Security enforced when linking guardians to students
	r.With(tenant.RequiresPermission("guardian:create")).Post("/", rs.createGuardian)
	r.With(tenant.RequiresPermission("guardian:update")).Put("/{id}", rs.updateGuardian)
	r.With(tenant.RequiresPermission("guardian:delete")).Delete("/{id}", rs.deleteGuardian)

	// Guardian invitations
	r.With(tenant.RequiresPermission("guardian:invite")).Post("/{id}/invite", rs.sendInvitation)
	r.With(tenant.RequiresPermission("guardian:read")).Get("/invitations/pending", rs.listPendingInvitations)

	// Student-guardian relationships
	// Anyone with guardian:read can view guardians (for emergency cases)
	r.With(tenant.RequiresPermission("guardian:read")).Get("/students/{studentId}/guardians", rs.getStudentGuardians)
	r.With(tenant.RequiresPermission("guardian:read")).Get("/{id}/students", rs.getGuardianStudents)

	// Create/Update/Delete relationships - custom supervisor permissions checked in handlers
	r.With(tenant.RequiresPermission("guardian:link")).Post("/students/{studentId}/guardians", rs.linkGuardianToStudent)
	r.With(tenant.RequiresPermission("guardian:link")).Put("/relationships/{relationshipId}", rs.updateStudentGuardianRelationship)
	r.With(tenant.RequiresPermission("guardian:link")).Delete("/students/{studentId}/guardians/{guardianId}", rs.removeGuardianFromStudent)

	return r
}
