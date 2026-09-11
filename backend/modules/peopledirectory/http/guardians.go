package users

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The guardian adapter serves /api/guardians (#2663): contact persons, their
// phone numbers, their links to children and the bank data behind the
// Bankverbindungen list. It talks to the People Directory only; identity,
// invitations and document rendering are injected by the root.

// Messages the legacy handlers produced verbatim; changing one changes the
// contract the staff screens read.
const (
	msgInvalidGuardianID      = "invalid guardian ID"
	msgInvalidPhoneID         = "invalid phone ID"
	msgInvalidStudentID       = "invalid student ID"
	msgInvalidRelationshipID  = "invalid relationship ID"
	msgGuardianNotFound       = "guardian not found"
	msgStudentNotFound        = "student not found"
	msgRelationshipNotFound   = "relationship not found"
	msgPhoneNotFound          = "phone number not found"
	msgPhoneNotBelongsGuard   = "phone number does not belong to this guardian"
	msgNotAuthenticated       = "user not authenticated"
	msgGuardianStillLinked    = "Erziehungsberechtigte/r kann nicht gelöscht werden: Noch mit Kindern verknüpft"
	msgInvalidPaymentGuardian = "Die erziehungsberechtigte Person konnte nicht zugeordnet werden."
)

// GuardianInvitation is the staff-initiated invitation the per-guardian
// invite endpoint reports; Token is only exposed to the local seeder.
type GuardianInvitation struct {
	ID                int64
	GuardianProfileID int64
	ExpiresAt         time.Time
	EmailSent         bool
	Token             string
}

// PendingGuardianInvitation is one open invitation of the tenant.
type PendingGuardianInvitation struct {
	ID                int64
	GuardianProfileID int64
	CreatedAt         time.Time
	ExpiresAt         time.Time
	EmailSentAt       *time.Time
	EmailError        *string
	EmailRetryCount   int
}

// GuardianInvite invites a further guardian to a child by e-mail.
type GuardianInvite struct {
	StudentID          int64
	Email              string
	FirstName          string
	LastName           string
	RelationshipType   string
	ActorAccountID     int64
	ConfirmRoleUpgrade bool
}

// GuardianInviteResult echoes what the invite resolved to.
type GuardianInviteResult struct {
	Outcome           string
	GuardianProfileID int64
	InvitationID      *int64
	ExistingRole      string
}

// GuardianPendingApproval is one parent-initiated request awaiting staff.
type GuardianPendingApproval struct {
	InvitationID      int64
	GuardianProfileID int64
	GuardianName      string
	GuardianEmail     string
	StudentID         int64
	StudentName       string
	RequestedByEmail  string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	RoleUpgrade       bool
}

// ExportFile is a rendered download.
type ExportFile struct {
	ContentType string
	Filename    string
	Data        []byte
}

// GuardianRuntime carries everything the guardian adapter must not own:
// the protected route group, permission middleware, response rendering,
// the caller's identity, the invitation flows and the document renderer.
type GuardianRuntime struct {
	Protected        func(chi.Router, func(chi.Router, Middleware))
	Permission       func(string) Middleware
	ParsePagination  func(*http.Request) (page, pageSize int)
	Success          func(http.ResponseWriter, *http.Request, int, any, string)
	SuccessPaginated func(http.ResponseWriter, *http.Request, int, any, Pagination, string)
	Failure          func(http.ResponseWriter, *http.Request, FailureKind, error)
	ObserveResponse  func(int, string)

	// ActorID is the authenticated account, 0 when the token carries none.
	ActorID func(*http.Request) int64
	// ActorRole is the comma-joined role list for the GDPR access log.
	ActorRole     func(*http.Request) string
	HasPermission func(*http.Request, string) bool
	IsAdmin       func(*http.Request) bool
	// IsVerifiedStaff reports whether the caller has a staff record.
	IsVerifiedStaff func(context.Context) bool
	// ExposeInvitationToken gates the seed-only raw token in the invite
	// response.
	ExposeInvitationToken func(*http.Request) bool
	MarkRollback          func(context.Context)

	SendInvitation         func(ctx context.Context, guardianID, actorAccountID int64) (GuardianInvitation, error)
	ListPendingInvitations func(context.Context) ([]PendingGuardianInvitation, error)

	InviteGuardianToStudent func(context.Context, GuardianInvite) (GuardianInviteResult, error)
	// InviteFailureKind classifies an invitation failure: forbidden for a
	// school-managed contact, invalid request otherwise.
	InviteFailureKind          func(error) FailureKind
	ListPendingApprovals       func(context.Context) ([]GuardianPendingApproval, error)
	PendingInvitationStudentID func(context.Context, int64) (int64, error)
	ApproveInvitation          func(ctx context.Context, invitationID, actorAccountID int64) error
	RejectInvitation           func(ctx context.Context, invitationID, actorAccountID int64) error

	RenderPaymentExport func(rows []peopledirectory.GuardianPaymentRow, format string) (ExportFile, error)

	Log *slog.Logger
}

// GuardianResource is the guardian HTTP adapter.
type GuardianResource struct {
	directory peopledirectory.Capability
	runtime   GuardianRuntime
}

// NewGuardianResource panics when a dependency is missing: a nil closure
// would only surface as a nil-pointer panic on the first request that needs
// it.
func NewGuardianResource(directory peopledirectory.Capability, runtime GuardianRuntime) *GuardianResource {
	if directory == nil || runtime.Protected == nil || runtime.Permission == nil || runtime.ParsePagination == nil ||
		runtime.Success == nil || runtime.SuccessPaginated == nil || runtime.Failure == nil || runtime.ObserveResponse == nil ||
		runtime.ActorID == nil || runtime.ActorRole == nil || runtime.HasPermission == nil || runtime.IsAdmin == nil ||
		runtime.IsVerifiedStaff == nil || runtime.ExposeInvitationToken == nil || runtime.MarkRollback == nil ||
		runtime.SendInvitation == nil || runtime.ListPendingInvitations == nil ||
		runtime.InviteGuardianToStudent == nil || runtime.InviteFailureKind == nil || runtime.ListPendingApprovals == nil ||
		runtime.PendingInvitationStudentID == nil || runtime.ApproveInvitation == nil || runtime.RejectInvitation == nil ||
		runtime.RenderPaymentExport == nil || runtime.Log == nil {
		panic("guardians HTTP: all dependencies are required")
	}
	return &GuardianResource{directory: directory, runtime: runtime}
}

// Router mounts every guardian route on its own protected router.
func (rs *GuardianResource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(r chi.Router, withTx Middleware) {
		usersRead := rs.runtime.Permission(permissions.UsersRead)
		usersCreate := rs.runtime.Permission(permissions.UsersCreate)
		usersUpdate := rs.runtime.Permission(permissions.UsersUpdate)
		usersDelete := rs.runtime.Permission(permissions.UsersDelete)
		usersManage := rs.runtime.Permission(permissions.UsersManage)
		financial := rs.runtime.Permission(permissions.GuardiansFinancial)

		// Guardian profile reads share the users:read gate; the picker search
		// (#1513) returns a minimal, enumeration-resistant projection.
		r.With(usersRead, withTx).Get("/", rs.listGuardians)
		r.With(usersRead, withTx).Get("/search", rs.searchGuardiansForPicker)
		r.With(usersRead, withTx).Get("/{id}", rs.getGuardian)
		r.With(usersRead, withTx).Get("/without-account", rs.listGuardiansWithoutAccount)
		r.With(usersRead, withTx).Get("/invitable", rs.listInvitableGuardians)

		r.With(usersCreate, withTx).Post("/", rs.createGuardian)
		r.With(usersUpdate, withTx).Put("/{id}", rs.updateGuardian)
		// Read-only preview of a full delete's blast radius (admin-only check in
		// the handler, #819).
		r.With(usersDelete, withTx).Get("/{id}/delete-preview", rs.guardianDeletePreview)
		r.With(usersDelete, withTx).Delete("/{id}", rs.deleteGuardian)

		r.With(usersCreate, withTx).Post("/{id}/invite", rs.sendInvitation)
		r.With(usersRead, withTx).Get("/invitations/pending", rs.listPendingInvitations)

		// Student-guardian relationships: anyone with users:read may view them
		// (emergency cases); writes add the per-student check in the handler.
		r.With(usersRead, withTx).Get("/students/{studentId}/guardians", rs.getStudentGuardians)
		r.With(usersRead, withTx).Get("/{id}/students", rs.getGuardianStudents)
		r.With(usersUpdate, withTx).Post("/students/{studentId}/guardians", rs.linkGuardianToStudent)
		// Atomic create-or-link of one or more guardians (#819).
		r.With(usersUpdate, withTx).Post("/students/{studentId}/guardians/batch", rs.createStudentGuardians)
		r.With(usersUpdate, withTx).Put("/relationships/{relationshipId}", rs.updateStudentGuardianRelationship)
		r.With(usersUpdate, withTx).Delete("/students/{studentId}/guardians/{guardianId}", rs.removeGuardianFromStudent)

		// Related accounts: invite a further guardian by e-mail plus the
		// parent-initiated approval queue. The queue exposes tenant-wide
		// e-mails and student names, so it sits behind users:manage.
		r.With(usersCreate, withTx).Post("/students/{studentId}/invite", rs.inviteGuardianToStudent)
		r.With(usersManage, withTx).Get("/invitations/pending-approval", rs.listPendingApprovals)
		r.With(usersUpdate, withTx).Post("/invitations/{invitationId}/approve", rs.approveInvitation)
		r.With(usersUpdate, withTx).Post("/invitations/{invitationId}/reject", rs.rejectInvitation)

		// Guardian payment data (#2608) sits behind guardians:financial. Reveal
		// and export are POSTs because both are audited actions.
		r.With(financial, withTx).Get("/payment-overview", rs.listPaymentOverview)
		r.With(financial, withTx).Post("/payment-overview/export", rs.exportPaymentOverview)
		r.With(financial, withTx).Put("/students/{studentId}/payer", rs.setStudentPayer)
		r.With(financial, withTx).Get("/{id}/payment", rs.getGuardianPayment)
		r.With(financial, withTx).Put("/{id}/payment", rs.updateGuardianPayment)
		r.With(financial, withTx).Post("/{id}/payment/reveal", rs.revealGuardianPayment)

		r.Route("/{id}/phone-numbers", func(r chi.Router) {
			r.With(usersRead, withTx).Get("/", rs.listGuardianPhoneNumbers)
			r.With(usersUpdate, withTx).Post("/", rs.addPhoneNumber)
			r.With(usersUpdate, withTx).Put("/{phoneId}", rs.updatePhoneNumber)
			r.With(usersUpdate, withTx).Delete("/{phoneId}", rs.deletePhoneNumber)
			r.With(usersUpdate, withTx).Post("/{phoneId}/set-primary", rs.setPrimaryPhone)
		})
	})
	return router
}

// --- shared helpers ---

func (rs *GuardianResource) succeed(w http.ResponseWriter, r *http.Request, status int, data any, message string) {
	rs.runtime.Success(w, r, status, data, message)
	rs.runtime.ObserveResponse(status, "none")
}

func (rs *GuardianResource) fail(w http.ResponseWriter, r *http.Request, kind FailureKind, err error) {
	rs.runtime.Failure(w, r, kind, err)
	rs.runtime.ObserveResponse(statusOf(kind), string(kind))
}

func (rs *GuardianResource) failMessage(w http.ResponseWriter, r *http.Request, kind FailureKind, message string) {
	rs.fail(w, r, kind, errors.New(message))
}

// moduleFailure renders a directory error: not found, invalid input and
// the guardian conflicts keep their status; everything else is a 500 with
// the cause kept for the logs.
func (rs *GuardianResource) moduleFailure(w http.ResponseWriter, r *http.Request, err error) {
	rs.runtime.Failure(w, r, classifyGuardianError(err), err)
	rs.runtime.ObserveResponse(statusOf(classifyGuardianError(err)), peopledirectory.ErrorCode(err))
}

func classifyGuardianError(err error) FailureKind {
	switch {
	case errors.Is(err, peopledirectory.ErrGuardianNotFound), errors.Is(err, peopledirectory.ErrGuardianPhoneNotFound),
		errors.Is(err, peopledirectory.ErrGuardianLinkNotFound), errors.Is(err, peopledirectory.ErrStudentNotFound),
		errors.Is(err, peopledirectory.ErrPersonNotFound):
		return FailureNotFound
	case errors.Is(err, peopledirectory.ErrInvalidGuardian), errors.Is(err, peopledirectory.ErrInvalidStudent),
		errors.Is(err, peopledirectory.ErrInvalidPerson):
		return FailureInvalidRequest
	case errors.Is(err, peopledirectory.ErrGuardianForceDeleteRequiresAdmin), errors.Is(err, peopledirectory.ErrPayerRemovalRequiresFinancial):
		return FailureForbidden
	case errors.Is(err, peopledirectory.ErrGuardianStillLinked), errors.Is(err, peopledirectory.ErrGuardianDeletePreviewChanged),
		errors.Is(err, peopledirectory.ErrGuardianLinkConflict):
		return FailureConflict
	default:
		return FailureInternal
	}
}

func (rs *GuardianResource) parseIDParam(w http.ResponseWriter, r *http.Request, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, name), 10, 64)
	if err != nil || id <= 0 {
		rs.failMessage(w, r, FailureInvalidRequest, message)
		return 0, false
	}
	return id, true
}

func (rs *GuardianResource) parseGuardianID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return rs.parseIDParam(w, r, "id", msgInvalidGuardianID)
}

func (rs *GuardianResource) parseStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return rs.parseIDParam(w, r, "studentId", msgInvalidStudentID)
}

// actingAccountID reads the authenticated account, rendering a 401 when the
// token carries none.
func (rs *GuardianResource) actingAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id := rs.runtime.ActorID(r)
	if id <= 0 {
		rs.failMessage(w, r, FailureUnauthorized, msgNotAuthenticated)
		return 0, false
	}
	return id, true
}

// activeStudent resolves a non-graduated student of the tenant; graduated
// children are a soft delete and their guardian links stay immutable.
func (rs *GuardianResource) activeStudent(ctx context.Context, studentID int64) (peopledirectory.Student, bool) {
	students, err := rs.directory.ListStudentsByID(ctx, []int64{studentID})
	if err != nil || len(students) == 0 || students[0].IsAlumnus() {
		return peopledirectory.Student{}, false
	}
	return students[0], true
}

// canModifyStudent is the per-student write gate on top of users:update:
// the student must be active, and the caller an admin or a verified staff
// member (#2329).
func (rs *GuardianResource) canModifyStudent(r *http.Request, studentID int64) (bool, error) {
	if _, ok := rs.activeStudent(r.Context(), studentID); !ok {
		return false, errors.New(msgStudentNotFound)
	}
	if rs.runtime.IsAdmin(r) {
		return true, nil
	}
	if !rs.runtime.IsVerifiedStaff(r.Context()) {
		return false, errors.New("insufficient permissions to modify this student's guardians")
	}
	return true, nil
}

// canModifyGuardian lets admins change any profile and verified staff the
// profiles that are linked to at least one child (#2329).
func (rs *GuardianResource) canModifyGuardian(r *http.Request, guardianID int64) (bool, error) {
	if rs.runtime.IsAdmin(r) {
		return true, nil
	}
	if !rs.runtime.IsVerifiedStaff(r.Context()) {
		return false, errors.New("only staff members can modify guardian profiles")
	}
	students, err := rs.directory.ListGuardianStudents(r.Context(), guardianID)
	if err != nil {
		return false, errors.New("failed to get guardian's students")
	}
	if len(students) == 0 {
		return false, errors.New("only administrators can modify guardians with no linked students")
	}
	return true, nil
}
