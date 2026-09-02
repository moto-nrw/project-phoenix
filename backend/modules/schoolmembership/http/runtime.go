package staff

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// Middleware is the chi middleware shape the root hands in.
type Middleware = func(http.Handler) http.Handler

// FailureKind selects the response shape the root renders for an error the
// adapter classifies itself. The wire format stays with the shared renderer
// so it matches every other staff endpoint.
type FailureKind string

const (
	FailureInvalidRequest FailureKind = "invalid_request"
	FailureUnauthorized   FailureKind = "unauthorized"
	FailureForbidden      FailureKind = "forbidden"
	FailureNotFound       FailureKind = "not_found"
	FailureConflict       FailureKind = "conflict"
	FailureInternal       FailureKind = "internal"
)

// Messages the legacy handlers produced verbatim. They are reproduced here
// because api/common is out of reach for this package; changing one of them
// changes the contract the staff screens read.
const (
	msgInvalidStaffID = "invalid staff ID"
	msgStaffNotFound  = "staff member not found"
	msgPersonNotFound = "person not found"
)

// Person is the People Directory entry behind a staff row. The adapter never
// reads persons itself; the root resolves them.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
	TagID     string
	AccountID *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Group is one group a Betreuungskraft is assigned to.
type Group struct {
	ID   int64
	Name string
}

// TeacherAction describes the teacher-record outcome of an update; it mirrors
// the service-side outcome the root translates.
type TeacherAction string

const (
	TeacherActionNone         TeacherAction = "none"
	TeacherActionExisting     TeacherAction = "existing"
	TeacherActionUpdated      TeacherAction = "updated"
	TeacherActionUpdateFailed TeacherAction = "update_failed"
	TeacherActionCreated      TeacherAction = "created"
	TeacherActionCreateFailed TeacherAction = "create_failed"
)

// CreateStaffInput is the parsed and validated create body plus the caller's
// permissions, which the create flow needs because it may adopt an existing
// staff record instead of creating a second one (#2906).
type CreateStaffInput struct {
	PersonID         int64
	StaffNotes       string
	IsTeacher        bool
	Specialization   string
	Role             string
	Qualifications   string
	ActorPermissions []string
}

// CreateStaffResult carries the created rows; TeacherCreationFailed reports
// the historical "staff created, teacher record failed" branch.
type CreateStaffResult struct {
	Staff                 schoolmembership.Staff
	Teacher               *schoolmembership.Teacher
	TeacherCreationFailed bool
}

// UpdateStaffInput is the parsed update body already merged with the stored
// record (PersonID is the target person, reassignment included).
type UpdateStaffInput struct {
	StaffID        int64
	PersonID       int64
	StaffNotes     string
	IsTeacher      bool
	Specialization string
	Role           string
	Qualifications string
}

// UpdateStaffResult carries the persisted staff row and the teacher outcome.
type UpdateStaffResult struct {
	Staff   schoolmembership.Staff
	Teacher *schoolmembership.Teacher
	Action  TeacherAction
}

// Runtime carries everything the adapter must not own: the protected route
// group, permission middleware, response rendering, and every foreign lookup
// or business flow that still lives outside the School Membership capability.
//
// The three delegated renderers (WriteFailure, SchoolClassFailure,
// PINFailure) both render AND observe the response — the adapter cannot know
// which status the root's error rules produce for a sentinel it may not
// import. Every failure the adapter classifies itself goes through Failure
// and is observed here.
type Runtime struct {
	// Protected wraps the router in the authenticated tenant group and hands
	// back the per-request transaction middleware.
	Protected func(chi.Router, func(chi.Router, Middleware))
	// Permission builds the route gate; more than one permission means "any
	// of them is enough".
	Permission func(...string) Middleware

	Success         func(http.ResponseWriter, *http.Request, int, any, string)
	Failure         func(http.ResponseWriter, *http.Request, FailureKind, error)
	ObserveResponse func(int, string)
	// ServeAvatar resolves the stored avatar path and streams the image, or
	// answers 404 when the path does not resolve.
	ServeAvatar func(http.ResponseWriter, *http.Request, string)

	// WriteFailure renders (and observes) the create/update/offboard
	// sentinels: adoption not permitted -> 403, Lehrkraft caregiver profile
	// -> 409, staff in use -> 409, constraint violation -> 409, else 500.
	WriteFailure func(http.ResponseWriter, *http.Request, error)
	// SchoolClassFailure renders (and observes) the class-teacher sentinels:
	// unknown staff -> 404, empty class name -> 400, else 500.
	SchoolClassFailure func(http.ResponseWriter, *http.Request, error)
	// PINFailure renders (and observes) the PIN sentinels: account not found
	// -> 404, locked -> 403, missing current PIN -> 400, wrong current PIN
	// -> 401, else 500.
	PINFailure func(http.ResponseWriter, *http.Request, error)

	// Permissions returns the caller's granted permissions; HasPermission is
	// the wildcard-aware matcher (admin:* matches every tier).
	Permissions   func(context.Context) []string
	HasPermission func(required string, granted []string) bool
	// CurrentAccountID is 0 when the token carries no account.
	CurrentAccountID func(context.Context) int64
	CurrentUsername  func(context.Context) string

	// Person / Persons resolve People Directory entries for staff rows.
	Person         func(context.Context, int64) (Person, error)
	PersonNotFound func(error) bool
	Persons        func(context.Context, []int64) ([]Person, error)
	// PersonIDByAccount reports the person linked to an account, if any.
	PersonIDByAccount func(context.Context, int64) (int64, bool, error)

	// The presence and account enrichments below are non-critical: an error
	// is logged and the field stays empty.
	PresentStaffIDs func(context.Context) ([]int64, error)
	WorkStatusMap   func(context.Context) (map[int64]string, error)
	AbsenceMap      func(context.Context) (map[int64]string, error)
	AbsenceLabelMap func(context.Context) (map[int64]string, error)
	AccountRoles    func(context.Context, []int64) (map[int64]string, error)
	AccountEmails   func(context.Context, []int64) (map[int64]string, error)
	AccountAvatars  func(context.Context, []int64) (map[int64]string, error)
	AccountHasRole  func(context.Context, int64, string) bool

	// GrantDefaultPermissions grants the default account permissions after a
	// create; the root decides what a Lehrkraft may and may not receive.
	GrantDefaultPermissions func(context.Context, int64, bool)
	// RetryQueuedDocumentCleanups drains the queued staff-document file
	// cleanups; the directory listing triggers it with source "directory".
	RetryQueuedDocumentCleanups func(context.Context, string)

	TeacherGroups    func(context.Context, int64) ([]Group, error)
	SchoolClasses    func(context.Context, int64) ([]string, error)
	SetSchoolClasses func(context.Context, int64, []string, int64) error
	ActiveCaregivers func(context.Context) ([]StaffWithRoleResponse, error)
	StaffByRoles     func(context.Context, []string) ([]StaffWithRoleResponse, error)

	CreateStaff func(context.Context, CreateStaffInput) (CreateStaffResult, error)
	UpdateStaff func(context.Context, UpdateStaffInput) (UpdateStaffResult, error)
	Offboard    func(context.Context, int64, string) error

	PINStatus func(context.Context, int64) (bool, *time.Time, error)
	// PINPreflight runs the account-state checks that must answer BEFORE the
	// staff-only check, so a locked account keeps reading "account is
	// temporarily locked due to failed PIN attempts" instead of the
	// staff-only message. Its error goes to PINFailure.
	PINPreflight func(context.Context, int64) error
	UpdatePIN    func(context.Context, int64, *string, string) error

	Log *slog.Logger
}

// Resource is the School Membership HTTP adapter.
type Resource struct {
	membership schoolmembership.Capability
	runtime    Runtime
}

// NewResource panics when a dependency is missing: a nil closure would only
// surface as a nil-pointer panic on the first request that needs it.
func NewResource(membership schoolmembership.Capability, runtime Runtime) *Resource {
	if membership == nil || runtime.Protected == nil || runtime.Permission == nil ||
		runtime.Success == nil || runtime.Failure == nil || runtime.ObserveResponse == nil ||
		runtime.ServeAvatar == nil || runtime.WriteFailure == nil || runtime.SchoolClassFailure == nil ||
		runtime.PINFailure == nil || runtime.Permissions == nil || runtime.HasPermission == nil ||
		runtime.CurrentAccountID == nil || runtime.CurrentUsername == nil ||
		runtime.Person == nil || runtime.PersonNotFound == nil || runtime.Persons == nil || runtime.PersonIDByAccount == nil ||
		runtime.PresentStaffIDs == nil || runtime.WorkStatusMap == nil || runtime.AbsenceMap == nil ||
		runtime.AbsenceLabelMap == nil || runtime.AccountRoles == nil || runtime.AccountEmails == nil ||
		runtime.AccountAvatars == nil || runtime.AccountHasRole == nil ||
		runtime.GrantDefaultPermissions == nil || runtime.RetryQueuedDocumentCleanups == nil ||
		runtime.TeacherGroups == nil || runtime.SchoolClasses == nil || runtime.SetSchoolClasses == nil ||
		runtime.ActiveCaregivers == nil || runtime.StaffByRoles == nil ||
		runtime.CreateStaff == nil || runtime.UpdateStaff == nil || runtime.Offboard == nil ||
		runtime.PINStatus == nil || runtime.PINPreflight == nil || runtime.UpdatePIN == nil || runtime.Log == nil {
		panic("staff HTTP: all dependencies are required")
	}
	return &Resource{membership: membership, runtime: runtime}
}

// Router mounts the membership routes on their own protected router. The
// composition root shares one /staff router with the workforce routes and
// calls Register instead.
func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	rs.runtime.Protected(router, func(protected chi.Router, withTx Middleware) {
		rs.Register(protected, withTx)
	})
	return router
}

// Register adds the membership routes to an already-protected router.
//
// The permission tiers are the ones #2906 settled on: the directory read is
// open to the personnel tiers as well because the response is field-scoped
// per caller, while replacing a Lehrkraft's classes is users:manage — the
// rows scope the Lehrkraft student day view, so users:update (held by every
// ordinary user) must not widen it.
func (rs *Resource) Register(r chi.Router, withTx Middleware) {
	directoryGate := rs.runtime.Permission(permissions.UsersRead, permissions.StaffManage, permissions.StaffStammdaten)
	profileGate := rs.runtime.Permission(permissions.UsersRead, permissions.StaffManage, permissions.StaffStammdaten, permissions.TimeTrackingManage)
	documentsGate := rs.runtime.Permission(permissions.StaffDocuments, permissions.StaffFinancial, permissions.StaffDocumentsHealth)
	usersRead := rs.runtime.Permission(permissions.UsersRead)

	r.With(directoryGate, withTx).Get("/", rs.listStaff)
	r.With(documentsGate, withTx).Get("/documents-directory", rs.listDocumentDirectory)
	r.With(rs.runtime.Permission(permissions.StaffFinancial), withTx).Get("/financial-profile/{id}", rs.getFinancialProfile)
	r.With(documentsGate, withTx).Get("/documents-profile/{id}", rs.getDocumentProfile)
	r.With(profileGate, withTx).Get("/{id}", rs.getStaff)
	r.With(profileGate, withTx).Get("/{id}/avatar", rs.serveStaffAvatar)
	r.With(usersRead, withTx).Get("/{id}/groups", rs.getStaffGroups)
	r.With(usersRead, withTx).Get("/{id}/school-classes", rs.getStaffSchoolClasses)
	r.With(rs.runtime.Permission(permissions.UsersManage), withTx).Put("/{id}/school-classes", rs.updateStaffSchoolClasses)
	r.With(usersRead, withTx).Get("/available", rs.getAvailableStaff)
	r.With(usersRead, withTx).Get("/by-role", rs.getStaffByRole)

	r.With(rs.runtime.Permission(permissions.UsersCreate), withTx).Post("/", rs.createStaff)
	r.With(rs.runtime.Permission(permissions.StaffManage), withTx).Put("/{id}", rs.updateStaff)
	r.With(rs.runtime.Permission(permissions.UsersDelete), withTx).Delete("/{id}", rs.deleteStaff)

	// PIN management: every authenticated staff member manages their own PIN,
	// so the protected group is the only gate.
	r.With(withTx).Get("/pin", rs.getPINStatus)
	r.With(withTx).Put("/pin", rs.updatePIN)
}
