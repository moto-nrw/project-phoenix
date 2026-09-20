package domain

import (
	"errors"
	"strings"
	"time"
)

// Account lifecycle facts and decisions (#3225): the admin staff-view preview,
// staff offboarding, the school identity chain a personnel role requires,
// parent accounts and guardian relative access. The error texts are the wire
// contract the retained auth service established.

// StaffMember is the users.staff fact the module reads through the staff
// directory port.
type StaffMember struct {
	ID       int64
	TenantID int64
	PersonID int64
	Deleted  bool
}

// PersonRecord is the users.persons fact the module reads and, for the school
// identity chain, creates through the staff directory port.
type PersonRecord struct {
	ID        int64
	TenantID  int64
	AccountID *int64
	FirstName string
	LastName  string
	TagID     *string
	Deleted   bool
}

func (p PersonRecord) HasRFIDCard() bool { return p.TagID != nil && *p.TagID != "" }

// PersonName is the display name of a person.
type PersonName struct {
	FirstName string
	LastName  string
}

// CaregiverProfile is the users.teachers fact of a staff member.
type CaregiverProfile struct {
	ID      int64
	StaffID int64
	Deleted bool
}

// --- staff preview ---------------------------------------------------------

var (
	ErrPreviewSelf           = errors.New("cannot preview your own account")
	ErrPreviewTargetNotStaff = errors.New("account is not a staff member at this school")
	ErrPreviewTokenInvalid   = errors.New("not a preview token of this session")
)

const (
	AuthEventStaffPreviewStarted = "staff_preview_started"
	AuthEventStaffPreviewEnded   = "staff_preview_ended"
)

// StaffPreviewSession is a short-lived, access-only token carrying the
// target's identity plus the read-only / acting-admin claims.
type StaffPreviewSession struct {
	AccessToken     string
	ExpiresIn       int64
	TargetAccountID int64
	TargetName      string
}

// StaffPreviewCandidate is one selectable staff member for the preview picker.
type StaffPreviewCandidate struct {
	AccountID int64
	FirstName string
	LastName  string
	Email     string
	Roles     []string
}

// TenantAccount is one account mapped to a school with its role names.
type TenantAccount struct {
	AccountID int64
	Email     string
	Active    bool
	Status    string
	RoleNames []string
}

// StaffPreviewEvent is the audit evidence of one preview start or end.
type StaffPreviewEvent struct {
	AdminAccountID  int64
	TenantID        int64
	TargetAccountID int64
	PreviewID       string
	IPAddress       string
	UserAgent       string
}

// --- staff offboarding -----------------------------------------------------

var ErrStaffOffboardingConflict = errors.New("account access changed since offboarding preview")

// StaffOffboardingPreview contains no credential material. The revision is an
// exact encoding of the access decisions, not an authorization credential.
type StaffOffboardingPreview struct {
	AccountID         int64
	ActiveMembership  bool
	RoleIDs           []int64
	Permissions       int64
	Tokens            int64
	PreserveGuardian  bool
	DeactivateAccount bool
	Revision          string
}

type StaffOffboardingResult struct {
	RolesRevoked            int64
	PermissionsRevoked      int64
	TokensRevoked           int64
	GuardianAccessPreserved bool
	AccountDeactivated      bool
}

// PermissionGrant is one direct account permission row.
type PermissionGrant struct {
	ID           int64
	PermissionID int64
	Granted      bool
}

// --- school identity -------------------------------------------------------

var (
	ErrSchoolIdentityNamesRequired   = errors.New("Vor- und Nachname sind erforderlich, um ein Konto als Personal anzulegen")                                                //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityPersonIsStudent = errors.New("Dieses Konto ist mit dem Datensatz eines Kindes verknüpft und kann nicht als Personal angelegt werden")                   //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityTagUnknown      = errors.New("Der angegebene Transponder ist an dieser Schule nicht bekannt")                                                           //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityTagConflict     = errors.New("Diese Person trägt an dieser Schule bereits einen anderen Transponder; dieser wird über die Personalverwaltung geändert") //nolint:staticcheck // ST1005: user-facing German message
	ErrSchoolIdentityTagTaken        = errors.New("Dieser Transponder ist an dieser Schule bereits einer anderen Person zugeordnet")                                         //nolint:staticcheck // ST1005: user-facing German message
)

// RoleFacts are the classification facts of a role: its tier (base_role) and,
// for system roles seeded before the column existed, its name. The role
// classification itself is the public package's; the application reads it
// through the role policy port.
type RoleFacts struct {
	ID       int64
	TenantID *int64
	Name     string
	IsSystem bool
	BaseRole *string
}

// SchoolIdentityInput describes the identity to provision.
type SchoolIdentityInput struct {
	AccountID int64
	TenantID  int64
	Role      *RoleFacts

	// FirstName/LastName are consulted only when no person exists yet.
	FirstName string
	LastName  string
	TagID     *string

	// PersonID names an existing, account-less person at this school the
	// account should be linked to before a new one would be created (#2600).
	// It is a hint, not a command.
	PersonID *int64

	// Position fills users.teachers.role when a caregiver profile is created.
	Position string

	// CaregiverUpgrade provisions the caregiver profile for a role that would
	// not get one on its own.
	CaregiverUpgrade bool

	// CreatePerson allows the caller to refuse inventing an identity for an
	// account that has none at this school yet.
	CreatePerson bool
}

// SchoolIdentity is what the account carries at the school afterwards.
// TeacherID is zero for roles that run without a caregiver profile.
type SchoolIdentity struct {
	PersonID  int64
	StaffID   int64
	TeacherID int64
}

// --- parent accounts -------------------------------------------------------

var (
	ErrParentAccountNotFound = errors.New("parent account not found")
	ErrEmailAlreadyExists    = errors.New("Diese E-Mail-Adresse ist bereits registriert") //nolint:staticcheck // ST1005: user-facing German message
	ErrUsernameAlreadyExists = errors.New("Dieser Benutzername ist bereits vergeben")     //nolint:staticcheck // ST1005: user-facing German message
)

// ParentAccount is one row of auth.accounts_parents.
type ParentAccount struct {
	ID           int64
	TenantID     int64
	Email        string
	Username     string
	Active       bool
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ParentAccountFilter narrows ListParentAccounts. Zero values do not filter.
type ParentAccountFilter struct {
	Email  string
	Active *bool
}

// --- guardian relative access ---------------------------------------------

var (
	ErrCannotRemovePrimaryGuardian      = errors.New("the primary guardian cannot be removed by a parent")
	ErrCannotRemoveStaffManagedGuardian = errors.New("staff-managed guardian contacts cannot be removed by a parent")
	ErrCannotRemoveOwnAccess            = errors.New("a parent cannot remove their own access to a child")
	ErrCannotRemovePayerGuardian        = errors.New("Diese Person ist als Zahler für das Kind eingetragen und kann nicht entfernt werden. Bitte wenden Sie sich an die Schule.") //nolint:staticcheck // ST1005: user-facing German message
	ErrInviteSocialWorkerManaged        = errors.New("a social-worker contact is managed by the school and cannot be invited to the parents portal")
	ErrGuardianInvitationNotFound       = errors.New("invitation not found")
	ErrGuardianInvitationExpired        = errors.New("invitation has expired")
)

// GuardianInvitationValidationError marks input rejected by the guardian
// invitation flows while retaining the original caller-facing explanation.
type GuardianInvitationValidationError struct{ Err error }

func (e *GuardianInvitationValidationError) Error() string { return e.Err.Error() }
func (e *GuardianInvitationValidationError) Unwrap() error { return e.Err }

// Guardian invitation approval states, as auth.guardian_invitations stores them.
const (
	GuardianInvitationApprovalNotRequired = "not_required"
	GuardianInvitationApprovalPending     = "pending"
	GuardianInvitationApprovalApproved    = "approved"
	GuardianInvitationApprovalRejected    = "rejected"
)

// GuardianRoleClass classifies a students_guardians role for the invite flow.
type GuardianRoleClass int

const (
	// GuardianRoleRestricted: emergency contact, pickup only, custom.
	GuardianRoleRestricted GuardianRoleClass = iota
	// GuardianRoleFull: primary, legal or co guardian.
	GuardianRoleFull
	// GuardianRoleSocialWorker: a school-managed professional contact.
	GuardianRoleSocialWorker
)

// GuardianProfile is the users.guardian_profiles fact the invite flow reads
// and creates.
type GuardianProfile struct {
	ID         int64
	TenantID   int64
	FirstName  string
	LastName   string
	Email      string
	AccountID  *int64
	HasAccount bool
}

func (p GuardianProfile) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(p.FirstName) + " " + strings.TrimSpace(p.LastName))
}

// StudentGuardianLink is one users.students_guardians row.
type StudentGuardianLink struct {
	ID                int64
	TenantID          int64
	StudentID         int64
	GuardianProfileID int64
	RelationshipType  string
	GuardianRole      string
	IsPrimary         bool
	IsPayer           bool
	EmergencyPriority int
}

// Student is the users.students fact the approval queue resolves names through.
type Student struct {
	ID       int64
	PersonID int64
}

// GuardianInvitation is one auth.guardian_invitations row.
type GuardianInvitation struct {
	ID                          int64
	TenantID                    int64
	Token                       string
	GuardianProfileID           int64
	CreatedBy                   int64
	ExpiresAt                   time.Time
	AcceptedAt                  *time.Time
	EmailSentAt                 *time.Time
	EmailError                  *string
	StudentID                   *int64
	RequestedByAccountID        *int64
	ApprovalStatus              string
	ApprovedBy                  *int64
	ApprovedAt                  *time.Time
	ProfileCreatedForInvitation bool
	RoleUpgrade                 bool
	CreatedAt                   time.Time
}

func (i GuardianInvitation) IsPendingApproval() bool {
	return i.ApprovalStatus == GuardianInvitationApprovalPending
}

// InviteToStudentRequest is the unified "invite an email to a child" input.
type InviteToStudentRequest struct {
	StudentID        int64
	Email            string
	FirstName        string
	LastName         string
	RelationshipType string
	CreatedBy        int64
	// RequestedByParentAccountID is set when a parent initiated the invite.
	RequestedByParentAccountID *int64
	// RequireApproval queues the invite for staff approval.
	RequireApproval bool
	// ConfirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link to legal_guardian (#2172).
	ConfirmRoleUpgrade bool
}

// InviteToStudentOutcome describes what the resolve logic did.
type InviteToStudentOutcome string

const (
	InviteOutcomeLinkedExistingAccount     InviteToStudentOutcome = "linked_existing_account"
	InviteOutcomeAlreadyLinked             InviteToStudentOutcome = "already_linked"
	InviteOutcomeInvited                   InviteToStudentOutcome = "invited"
	InviteOutcomePendingApproval           InviteToStudentOutcome = "pending_approval"
	InviteOutcomeExistingContactRestricted InviteToStudentOutcome = "existing_contact_restricted"
)

// InviteToStudentResult is returned by InviteToStudent.
type InviteToStudentResult struct {
	Outcome           InviteToStudentOutcome
	GuardianProfileID int64
	InvitationID      *int64
	ExistingRole      string
}

// RevokeAccessRequest removes one account's access to one child.
type RevokeAccessRequest struct {
	StudentID         int64
	GuardianProfileID int64
	ActorAccountID    int64
	ByParent          bool
	MayClearPayer     bool
}

// PendingApprovalView is the staff-facing, name-resolved projection of a
// parent-initiated invitation awaiting approval.
type PendingApprovalView struct {
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
