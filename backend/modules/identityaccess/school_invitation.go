package identityaccess

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Delivery outbox kind of the staff welcome mail an accepted school
// invitation sends, and the entity it relates to.
const (
	MailKindStaffWelcome        = "staff_welcome"
	MailRelatedSchoolInvitation = "school_invitation"
)

// The deployment's invitation link lifetime (invitation_token_expiry_hours)
// is bounded: an unset or non-positive value means two days, and no link
// lives longer than a week. Staff, operator and guardian invitations share it.
const (
	invitationTokenExpiryFallbackHours = 48
	invitationTokenExpiryMaxHours      = 168
)

// InvitationTokenExpiry is the invitation link lifetime for the configured
// hours.
func InvitationTokenExpiry(configuredHours int) time.Duration {
	return boundedLifetime(configuredHours, invitationTokenExpiryFallbackHours, invitationTokenExpiryMaxHours, time.Hour)
}

// boundedLifetime bounds a configured count to (0, upper], falling back to
// fallback for a value at or below zero.
func boundedLifetime(value, fallback, upper int, unit time.Duration) time.Duration {
	if value <= 0 {
		value = fallback
	} else if value > upper {
		value = upper
	}
	return time.Duration(value) * unit
}

// School invitations (#2722): the one-time link that gives an invitee access
// to a school with a role. A link is never stored already expired or spent,
// is redeemable exactly once, and accepting it writes the account, its
// school mapping, the role and the identity chain the role needs in one
// transaction.
var (
	// ErrSchoolInvitationUnavailable reports a module composed without the
	// invitation dependencies.
	ErrSchoolInvitationUnavailable = errors.New("school invitations are not composed")
	ErrInvitationNotFound          = errors.New("invitation not found")
	ErrInvitationExpired           = errors.New("invitation has expired")
	ErrInvitationUsed              = errors.New("invitation has already been used")
	ErrInvitationDeliveryFailed    = errors.New("invitation delivery failed")
	ErrInvitationTenantDeleted     = errors.New("the school for this invitation has been deleted")
	ErrInvitationNameRequired      = errors.New("first name and last name are required")
	// ErrInvitationOwnerRequired reports an acceptance for an address that
	// already has an account, without a session of that account.
	ErrInvitationOwnerRequired = errors.New("sign in to the invited account before accepting")
	ErrInvitationOwnerMismatch = errors.New("the signed-in account does not own this invitation")
	// ErrAccountAlreadyHasTenantAccess reports an invitation to a school the
	// account can already sign in to.
	ErrAccountAlreadyHasTenantAccess = errors.New("account already has access to tenant")
	// ErrInvitationPasswordMismatch reports an acceptance whose password
	// fields differ.
	ErrInvitationPasswordMismatch = errors.New("passwords don't match")
	// ErrRoleGrantNotPermitted reports an inviter handing out a role beyond
	// its own authority: an admin-tier grant requires users:manage.
	ErrRoleGrantNotPermitted = errors.New("du darfst diese Rolle nicht vergeben")
	// ErrLehrkraftNoCaregiver refuses the combination of the Lehrkraft role
	// with the caregiver upgrade, which would defeat the role's class-scoped
	// read-only design (#1772).
	ErrLehrkraftNoCaregiver = errors.New("die Rolle 'Lehrkraft' kann nicht mit Betreuungsrechten kombiniert werden")
)

// InvitationPortal names where an invitation is accepted.
type InvitationPortal string

const (
	InvitationPortalTenant InvitationPortal = "tenant"
	InvitationPortalSchool InvitationPortal = "school"
)

// EmploymentAction is the school's choice for a returning member (ADR 0021).
// An omitted choice continues the former employment, including existing links.
type EmploymentAction string

const (
	EmploymentActionResume EmploymentAction = "resume"
	EmploymentActionNew    EmploymentAction = "new"
)

// SchoolInvitation is an invitation as the owner reports it.
type SchoolInvitation struct {
	ID                  int64
	TenantID            int64
	Email               string
	Token               string
	RoleID              int64
	RoleName            string
	ExpiresAt           time.Time
	UsedAt              *time.Time
	CreatedBy           *int64
	CreatorEmail        string
	FirstName           *string
	LastName            *string
	Position            *string
	CaregiverEnabled    bool
	EmploymentAction    EmploymentAction
	ResetMasterData     bool
	ResetQualifications bool
	ResetWorkTime       bool
	PersonID            *int64
	Delivery            TokenDelivery
	CreatedAt           time.Time
}

// SchoolInvitationRequest describes the invitation to create.
type SchoolInvitationRequest struct {
	Email               string
	RoleID              int64
	TenantID            int64
	FirstName           *string
	LastName            *string
	Position            *string
	CaregiverEnabled    bool
	EmploymentAction    EmploymentAction
	ResetMasterData     bool
	ResetQualifications bool
	ResetWorkTime       bool
	PersonID            *int64
	CreatedBy           int64
	SchoolName          string
	// ActorPermissions are the inviting account's permissions; the empty set
	// grants nothing beyond the user tier.
	ActorPermissions []string
	// OperatorGrant marks an invitation issued from the operator portal.
	OperatorGrant bool
}

// OperatorSchoolInvitation is the school invitation the operator portal
// issues: an operator grant, so the tenant-side role-grant check does not
// apply to the operator-authenticated caller.
func OperatorSchoolInvitation(email string, roleID, tenantID int64, firstName, lastName, position *string, caregiverEnabled bool) SchoolInvitationRequest {
	return SchoolInvitationRequest{
		Email: email, RoleID: roleID, TenantID: tenantID,
		FirstName: firstName, LastName: lastName, Position: position,
		CaregiverEnabled: caregiverEnabled,
		OperatorGrant:    true,
	}
}

// InvitationRegistration is what an invitee supplies when accepting.
type InvitationRegistration struct {
	// OwnerAccessToken proves that the caller holds the invited account's
	// session; it is a backend-signed token, never an account id.
	OwnerAccessToken string
	FirstName        string
	LastName         string
	Password         string
	ConfirmPassword  string
}

// InvitationPreview is the public-safe view of a redeemable invitation.
type InvitationPreview struct {
	Portal               InvitationPortal
	RequiresAccountLogin bool
	Email                string
	RoleName             string
	FirstName            *string
	LastName             *string
	Position             *string
	CaregiverEnabled     bool
	ExpiresAt            time.Time
}

// FormerEmployment is the employment an invitation to an address would
// continue at the school in context (ADR 0021): the latest retired staff
// membership of the address's person, unless the automatic deletion of staff
// erased it. StartedOn and EndedOn are the days the membership was created
// and retired.
type FormerEmployment struct {
	StartedOn    calendar.Date
	EndedOn      calendar.Date
	Resumes      []FormerEmploymentPart
	NotReturning []FormerEmploymentPart
}

// FormerEmploymentPart names a part of a former employment the Einladen
// dialog explains. The values are the wire contract.
type FormerEmploymentPart string

// What a Wiederaufnahme brings back with the staff membership.
const (
	FormerEmploymentMasterData            FormerEmploymentPart = "master_data"
	FormerEmploymentQualifications        FormerEmploymentPart = "qualifications"
	FormerEmploymentWorkTime              FormerEmploymentPart = "work_time"
	FormerEmploymentPastShiftsAndAbsences FormerEmploymentPart = "past_shifts_and_absences"
)

// What never comes back (ADR 0021, decision 5): the exit deletes it, and the
// school sets it anew. It is named, never offered.
const (
	FormerEmploymentStaffDocuments             FormerEmploymentPart = "staff_documents"
	FormerEmploymentFutureShiftsAndAbsences    FormerEmploymentPart = "future_shifts_and_absences"
	FormerEmploymentGroupsClassesSubstitutions FormerEmploymentPart = "groups_classes_substitutions"
)

// SchoolInvitationDeliveryRecorder records how an invitation mail fared; the
// root's mail delivery reports through it.
type SchoolInvitationDeliveryRecorder interface {
	RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error
}

// SchoolInvitations is the capability the invitation routes and the staff
// import consume. Flow errors arrive in the AuthenticationError envelope.
type SchoolInvitations interface {
	CreateSchoolInvitation(ctx context.Context, request SchoolInvitationRequest) (SchoolInvitation, error)
	ValidateSchoolInvitation(ctx context.Context, token string) (InvitationPreview, error)
	// FindFormerEmployment reports the former employment an invitation to the
	// address would continue at the school in context. Found is false for an
	// address without an account and for one without a former employment
	// here alike, so the answer never tells whether an account exists.
	FindFormerEmployment(ctx context.Context, email string) (FormerEmployment, bool, error)
	// AcceptSchoolInvitation returns the account the invitee signs in with.
	AcceptSchoolInvitation(ctx context.Context, token string, registration InvitationRegistration) (Account, error)
	ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	ListPendingSchoolInvitations(ctx context.Context) ([]SchoolInvitation, error)
	// RevokeTenantSchoolInvitations spends every pending invitation of the
	// school and returns how many it spent.
	RevokeTenantSchoolInvitations(ctx context.Context, tenantID int64) (int, error)
	DeleteExpiredSchoolInvitations(ctx context.Context) (int, error)
	SchoolInvitationDeliveryRecorder
	// SchoolInvitationDeliverySent reports whether the invitation mail was
	// accepted by the transport. Delivery uses it to order its welcome mail.
	SchoolInvitationDeliverySent(ctx context.Context, id int64) (bool, error)
	// SchoolInvitationSubdomain resolves the school host of an accepted
	// invitation; it is best-effort and answers "" when it cannot.
	SchoolInvitationSubdomain(ctx context.Context, token string) string
}

func (m *Module) CreateSchoolInvitation(ctx context.Context, request SchoolInvitationRequest) (SchoolInvitation, error) {
	return m.engine.CreateSchoolInvitation(ctx, request)
}

func (m *Module) ValidateSchoolInvitation(ctx context.Context, token string) (InvitationPreview, error) {
	return m.engine.ValidateSchoolInvitation(ctx, token)
}

func (m *Module) FindFormerEmployment(ctx context.Context, email string) (FormerEmployment, bool, error) {
	return m.engine.FindFormerEmployment(ctx, email)
}

func (m *Module) AcceptSchoolInvitation(ctx context.Context, token string, registration InvitationRegistration) (Account, error) {
	return m.engine.AcceptSchoolInvitation(ctx, token, registration)
}

func (m *Module) ResendSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return m.engine.ResendSchoolInvitation(ctx, invitationID, actorAccountID)
}

func (m *Module) RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return m.engine.RevokeSchoolInvitation(ctx, invitationID, actorAccountID)
}

func (m *Module) ListPendingSchoolInvitations(ctx context.Context) ([]SchoolInvitation, error) {
	return m.engine.ListPendingSchoolInvitations(ctx)
}

func (m *Module) RevokeTenantSchoolInvitations(ctx context.Context, tenantID int64) (int, error) {
	return m.engine.RevokeTenantSchoolInvitations(ctx, tenantID)
}

func (m *Module) DeleteExpiredSchoolInvitations(ctx context.Context) (int, error) {
	return m.engine.DeleteExpiredSchoolInvitations(ctx)
}

func (m *Module) RecordSchoolInvitationDelivery(ctx context.Context, id int64, delivery TokenDelivery) error {
	return m.engine.RecordSchoolInvitationDelivery(ctx, id, delivery)
}

func (m *Module) SchoolInvitationDeliverySent(ctx context.Context, id int64) (bool, error) {
	return m.engine.SchoolInvitationDeliverySent(ctx, id)
}

func (m *Module) SchoolInvitationSubdomain(ctx context.Context, token string) string {
	return m.engine.SchoolInvitationSubdomain(ctx, token)
}
