package careplan

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ParentPortalStudent is the People Directory student fact the guardian portal
// reads, alumni included.
type ParentPortalStudent struct {
	ID            int64
	TenantID      int64
	PersonID      int64
	SchoolClass   string
	Status        string
	EnrolledFrom  calendar.Date
	EnrolledUntil calendar.Date
}

// ParentPortalGuardianLink is one student-guardian relationship of an
// account, with the granted parents-portal permissions.
type ParentPortalGuardianLink struct {
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	Permissions       []string
}

// ParentPortalPerson is a person the People Directory still shows; a
// soft-deleted person is absent.
type ParentPortalPerson struct {
	ID        int64
	FirstName string
	LastName  string
}

// ParentPortalDirectory is the People Directory read of the guardian portal.
// Students and guardian links are read across every tenant the admin
// transaction can see.
type ParentPortalDirectory interface {
	ListParentPortalStudents(ctx context.Context, ids []int64) ([]ParentPortalStudent, error)
	ListParentPortalGuardianLinks(ctx context.Context, accountID int64) ([]ParentPortalGuardianLink, error)
	ListParentPortalPersons(ctx context.Context, ids []int64) ([]ParentPortalPerson, error)
}

type ParentPortalChild struct {
	// Student identity (per-tenant id; the (TenantID, StudentID) pair
	// is globally unique).
	StudentID int64 `json:"student_id"`
	TenantID  int64 `json:"tenant_id"`
	// PersonID is the student's person reference; the names below are
	// attached by the composition layer through the People Directory.
	PersonID    int64  `json:"-"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class,omitempty"`

	// Lifecycle status as set by the activate-students scheduler:
	// pending → active → inactive (or alumnus).
	Status        string         `json:"status"`
	EnrolledFrom  *calendar.Date `json:"enrolled_from,omitempty"`
	EnrolledUntil *calendar.Date `json:"enrolled_until,omitempty"`

	// School context — repeated on every child of the same school so
	// the frontend can render grouped cards without an extra fetch.
	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`

	// GuardianPermissions is the matching auth.guardian_student_access.permissions
	// map for the calling account's guardian relationship. It is internal to the
	// parent service; handlers must not serialize it.
	GuardianPermissions map[string]bool `json:"-"`
	GuardianProfileID   int64           `json:"-"`
}

type ParentPortalPhase struct {
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`
	// SchoolSubdomain is the school's tenant ROUTING identifier — the one
	// /auth/tenant/resolve and the parent enrollment endpoints accept. Slug is
	// only unique per organization and is not a routing key, so links must be
	// built from this field (#1663).
	SchoolSubdomain string `json:"school_subdomain"`
	PhaseID         int64  `json:"phase_id"`
	PhaseName       string `json:"phase_name"`
	// PhaseNameTranslations maps locale → the school's translation of
	// PhaseName, limited to translations that still match it (#3377).
	PhaseNameTranslations map[string]string `json:"phase_name_translations,omitempty"`
	PhaseKind             string            `json:"phase_kind"`
	ServiceStartDate      calendar.Date     `json:"service_start_date"`
	ServiceEndDate        calendar.Date     `json:"service_end_date"`
	EnrollmentOpenAt      *time.Time        `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt     *time.Time        `json:"enrollment_close_at,omitempty"`
	AlreadyLinked         bool              `json:"already_linked"`
	// Audience mirrors enrollment.phases.audience (#1663) so the picker
	// can label restricted phases. Phases the account is NOT eligible
	// for are already filtered out by the repository.
	Audience      string `json:"audience"`
	HasFamilyLink bool   `json:"-"`
	// EnrolledSubmitPersonIDs are the persons of the ACTIVE or PENDING
	// students whose guardian relationship grants submit permission. The
	// composition layer checks them against the People Directory to decide
	// the existing_students eligibility (a soft-deleted person does not
	// count); the repository itself never reads person rows.
	EnrolledSubmitPersonIDs []int64 `json:"-"`
}

type ParentPortalSubmitStatus struct {
	// Linked: the account has an active auth.account_tenants mapping.
	Linked bool
	// HasGuardianLink: at least one student-guardian relationship exists
	// via a guardian profile owned by the account at this school.
	HasGuardianLink bool
	// HasSubmitPermission: at least one of those rows grants
	// parent_portal.enrollment.submit, backed by an ACTIVE account_tenants
	// mapping. This is the linked_parents audience fact — it deliberately
	// accepts a relationship to an inactive child, because enrolling a
	// genuinely NEW sibling is exactly what that audience is for.
	HasSubmitPermission bool
	// HasEnrolledSubmitPermission narrows HasSubmitPermission to a
	// relationship pointing at a student that is still ACTIVE or PENDING
	// (person not soft-deleted) — the same "enrolled" scope the submit-time
	// student matcher uses. This is the existing_students audience fact:
	// without such a child, a re-enrollment can only ever fail
	// (ErrChildNotEnrolled / ErrChildEnrollmentNotPermitted), so the picker
	// (ListEnrollable) hides those phases and the form gate must refuse them
	// with the same fact.
	HasEnrolledSubmitPermission bool
	// EnrolledSubmitPersonIDs carries the candidate persons behind
	// HasEnrolledSubmitPermission for the composition layer; see
	// ParentPortalPhase.EnrolledSubmitPersonIDs.
	EnrolledSubmitPersonIDs []int64
}

type ParentPortalEnrollment struct {
	RequestID   int64      `json:"request_id"`
	TenantID    int64      `json:"tenant_id"`
	StatusToken string     `json:"status_token"`
	SubmittedAt time.Time  `json:"submitted_at"`
	WithdrawnAt *time.Time `json:"withdrawn_at,omitempty"`

	PhaseID          int64         `json:"phase_id"`
	PhaseName        string        `json:"phase_name"`
	ServiceStartDate calendar.Date `json:"service_start_date"`
	ServiceEndDate   calendar.Date `json:"service_end_date"`

	// ShowStatusReasonToParent mirrors the owning phase flag. Internal
	// plumbing only (json:"-"): the parent service uses it to redact
	// admin-internal child status reasons before returning, and the API
	// handler maps an explicit response struct, so it never reaches the
	// wire. See workflows/parentportal/care ListEnrollmentsForAccount.
	ShowStatusReasonToParent bool `json:"-"`

	SchoolName string `json:"school_name"`
	SchoolSlug string `json:"school_slug"`

	Children []ParentPortalEnrollmentChild `json:"children"`
}

type ParentPortalEnrollmentChild struct {
	ChildID      int64   `json:"child_id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason,omitempty"`
}

// ParentPortalChildQueries reads the account-scoped child projection. The
// caller supplies the administrative unit of work; authorization is still
// checked per relationship by the consuming workflow.
type ParentPortalChildQueries interface {
	ListByAccount(context.Context, int64) ([]*ParentPortalChild, error)
	FindForAccount(context.Context, int64, int64) (*ParentPortalChild, error)
}

type ParentPortalPhaseQueries interface {
	ListEnrollable(context.Context, int64) ([]*ParentPortalPhase, error)
	GuardianSubmitStatus(context.Context, int64, int64) (*ParentPortalSubmitStatus, error)
}

type ParentPortalEnrollmentQueries interface {
	ListByAccount(context.Context, int64) ([]*ParentPortalEnrollment, error)
}
