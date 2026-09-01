package guardians

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	userContextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	guardianSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the guardians API resource
type Resource struct {
	GuardianService    *guardianSvc.GuardianService
	InvitationService  authSvc.GuardianInvitationService
	PersonService      guardianSvc.PersonService
	EducationService   educationSvc.Service
	UserContextService userContextSvc.UserContextService
	// ListExportService renders the Bankverbindungen export (#2608). Assigned
	// after construction like the other export-capable resources.
	ListExportService *listexport.RendererService
	db                *bun.DB
	appEnv            string
}

// NewResource creates a new guardians resource
func NewResource(
	guardianService *guardianSvc.GuardianService,
	invitationService authSvc.GuardianInvitationService,
	personService guardianSvc.PersonService,
	educationService educationSvc.Service,
	userContextService userContextSvc.UserContextService,
	db *bun.DB,
	appEnv string,
) *Resource {
	return &Resource{
		GuardianService:    guardianService,
		InvitationService:  invitationService,
		PersonService:      personService,
		EducationService:   educationService,
		UserContextService: userContextService,
		db:                 db,
		appEnv:             appEnv,
	}
}

// Router returns a configured router for guardian endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Protected routes that require authentication and permissions
	common.ProtectedTenantGroup(r, rs.db, func(r chi.Router, withTx common.Middleware) {

		// Guardian profile CRUD operations
		// Read operations require users:read permission
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listGuardians)
		// Guardian picker search (#1513): same users:read gate as the rest of the
		// guardian reads, so anyone who can reach the student create/detail flows
		// can link a sibling's existing guardian. Returns a minimal,
		// enumeration-resistant projection (name, email, linked-children count) —
		// see searchGuardiansForPicker.
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/search", rs.searchGuardiansForPicker)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}", rs.getGuardian)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/without-account", rs.listGuardiansWithoutAccount)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/invitable", rs.listInvitableGuardians)

		// Write operations
		r.With(common.RequiresPermission(permissions.UsersCreate), withTx).Post("/", rs.createGuardian)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{id}", rs.updateGuardian)
		// Read-only preview of a full delete's blast radius (admin-only check in
		// the handler). Lets the UI show the affected children before confirming
		// without the old destructive "probe DELETE" (#819).
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/{id}/delete-preview", rs.guardianDeletePreview)
		r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteGuardian)

		// Guardian invitations
		r.With(common.RequiresPermission(permissions.UsersCreate), withTx).Post("/{id}/invite", rs.sendInvitation)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/invitations/pending", rs.listPendingInvitations)

		// Student-guardian relationships
		// Anyone with users:read can view guardians (for emergency cases)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/students/{studentId}/guardians", rs.getStudentGuardians)
		r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/students", rs.getGuardianStudents)

		// Relationship writes require users:update plus the per-student check in handlers.
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/students/{studentId}/guardians", rs.linkGuardianToStudent)
		// Atomic create-or-link of one or more guardians for an existing student
		// (#819): the whole batch succeeds or rolls back as one transaction, so a
		// partially-created guardian is never orphaned and the frontend needs no
		// client-side rollback. The per-student gate remains in the handler.
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/students/{studentId}/guardians/batch", rs.createStudentGuardians)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/relationships/{relationshipId}", rs.updateStudentGuardianRelationship)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/students/{studentId}/guardians/{guardianId}", rs.removeGuardianFromStudent)

		// Related-accounts: invite a further guardian to a child by email
		// (staff always allowed; resolves existing-account / existing-profile /
		// new), plus the parent-initiated approval queue.
		r.With(common.RequiresPermission(permissions.UsersCreate), withTx).Post("/students/{studentId}/invite", rs.inviteGuardianToStudent)
		// The approval queue exposes tenant-wide guardian/requester emails and
		// student names, and the UI is admin-only. Gate it with users:manage
		// (not users:read) so non-admin staff with read access cannot enumerate
		// pending parent requests via the API. approve/reject keep their stronger
		// per-student authorization on top of this.
		r.With(common.RequiresPermission(permissions.UsersManage), withTx).Get("/invitations/pending-approval", rs.listPendingApprovals)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/invitations/{invitationId}/approve", rs.approveInvitation)
		r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/invitations/{invitationId}/reject", rs.rejectInvitation)

		// Guardian payment data (#2608). The whole section sits behind
		// guardians:financial, not users:update: an IBAN list is the single
		// most abusable export the school holds, and the directory
		// maintainers are not the school office. Reveal and export are POSTs
		// because both are audited actions, not cacheable reads.
		r.With(common.RequiresPermission(permissions.GuardiansFinancial), withTx).Get("/payment-overview", rs.listPaymentOverview)
		r.With(common.RequiresPermission(permissions.GuardiansFinancial), withTx).Post("/payment-overview/export", rs.exportPaymentOverview)
		r.With(common.RequiresPermission(permissions.GuardiansFinancial), withTx).Put("/students/{studentId}/payer", rs.setStudentPayer)
		r.With(common.RequiresPermission(permissions.GuardiansFinancial), withTx).Get("/{id}/payment", rs.getGuardianPayment)
		r.With(common.RequiresPermission(permissions.GuardiansFinancial), withTx).Put("/{id}/payment", rs.updateGuardianPayment)
		r.With(common.RequiresPermission(permissions.GuardiansFinancial), withTx).Post("/{id}/payment/reveal", rs.revealGuardianPayment)

		// Phone number management (nested under guardian)
		r.Route("/{id}/phone-numbers", func(r chi.Router) {
			r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/", rs.listGuardianPhoneNumbers)
			r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/", rs.addPhoneNumber)
			r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Put("/{phoneId}", rs.updatePhoneNumber)
			r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{phoneId}", rs.deletePhoneNumber)
			r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Post("/{phoneId}/set-primary", rs.setPrimaryPhone)
		})
	})

	return r
}
