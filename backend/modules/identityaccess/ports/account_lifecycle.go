package ports

import (
	"context"
)

// StaffMember is the users.staff fact of a school.
type StaffMember struct {
	ID       int64
	TenantID int64
	PersonID int64
	Deleted  bool
}

// PersonRecord is the users.persons fact the lifecycle reads and, for the
// school identity chain, creates.
type PersonRecord struct {
	ID        int64
	TenantID  int64
	AccountID *int64
	FirstName string
	LastName  string
	TagID     *string
	Deleted   bool
}

// PersonName is a person's display name.
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

// StaffDirectory reads and writes the People Directory and School Membership
// facts of the tenant in context. Missing rows report found=false.
type StaffDirectory interface {
	FindStaff(ctx context.Context, staffID int64) (StaffMember, bool, error)
	FindPerson(ctx context.Context, personID int64) (PersonRecord, bool, error)
	FindPersonByAccount(ctx context.Context, accountID int64) (PersonRecord, bool, error)
	FindPersonByTag(ctx context.Context, tagID string) (PersonRecord, bool, error)
	FindPersonNames(ctx context.Context, accountIDs []int64) (map[int64]PersonName, error)
	CreatePerson(ctx context.Context, person PersonRecord) (int64, error)
	LinkPersonToAccount(ctx context.Context, personID, accountID int64) error
	LinkPersonToRFIDCard(ctx context.Context, personID int64, tagID string) error
	IsStudentPerson(ctx context.Context, personID int64) (bool, error)
	FindStaffByPerson(ctx context.Context, personID int64) (StaffMember, bool, error)
	CreateStaff(ctx context.Context, tenantID, personID int64) (int64, error)
	FindCaregiverProfile(ctx context.Context, staffID int64) (CaregiverProfile, bool, error)
	CreateCaregiverProfile(ctx context.Context, tenantID, staffID int64, position string) (int64, error)
	// HasLiveCaregiverProfile reports whether the account's live person at the
	// school carries a live staff record with a live caregiver profile.
	HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error)
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

// PreviewAudit records the staff preview evidence on the caller's
// transaction; the lock is transaction-scoped.
type PreviewAudit interface {
	RecordStaffPreviewStart(ctx context.Context, event StaffPreviewEvent) error
	RecordStaffPreviewEndOnce(ctx context.Context, event StaffPreviewEvent) (bool, error)
	LockStaffPreview(ctx context.Context, adminAccountID int64, previewID string) error
	StaffPreviewEnded(ctx context.Context, adminAccountID int64, previewID string) (bool, error)
}

// GuardianProfile is the users.guardian_profiles fact.
type GuardianProfile struct {
	ID         int64
	TenantID   int64
	FirstName  string
	LastName   string
	Email      string
	AccountID  *int64
	HasAccount bool
}

// StudentGuardianLink is one student-guardian relationship.
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

// GuardianRoleClass classifies a stored guardian role for the invite flow.
type GuardianRoleClass int

const (
	GuardianRoleRestricted GuardianRoleClass = iota
	GuardianRoleFull
	GuardianRoleSocialWorker
)

// GuardianDirectory reads and writes guardian profiles, student-guardian
// relationships and students of the tenant in context.
type GuardianDirectory interface {
	FindGuardianProfileByEmail(ctx context.Context, email string) (GuardianProfile, bool, error)
	FindGuardianProfile(ctx context.Context, id int64) (GuardianProfile, bool, error)
	// LockGuardianProfile locks the profile for the caller's transaction and
	// reports whether it exists in that tenant.
	LockGuardianProfile(ctx context.Context, id int64) (bool, error)
	FindGuardianProfiles(ctx context.Context, ids []int64) (map[int64]GuardianProfile, error)
	CreateGuardianProfile(ctx context.Context, profile GuardianProfile) (int64, error)
	DeleteGuardianProfile(ctx context.Context, id int64) error
	LinkGuardianProfileToAccount(ctx context.Context, profileID, accountID int64) error
	FindStudentGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (StudentGuardianLink, bool, error)
	LinkStudentGuardianIfAbsent(ctx context.Context, link StudentGuardianLink) (bool, error)
	PromoteStudentGuardianLink(ctx context.Context, linkID int64) error
	ListStudentGuardianLinksByStudents(ctx context.Context, studentIDs []int64) ([]StudentGuardianLink, error)
	ListStudentGuardianLinksByProfile(ctx context.Context, guardianProfileID int64) ([]StudentGuardianLink, error)
	DeleteStudentGuardianLink(ctx context.Context, linkID int64) error
	GuardianRoleClass(role string) GuardianRoleClass
	FindStudents(ctx context.Context, ids []int64) (map[int64]Student, error)
	LockStudent(ctx context.Context, studentID int64) error
	FindPersonNamesByIDs(ctx context.Context, personIDs []int64) (map[int64]PersonName, error)
}

// GuardianEnrollments claims the enrollment requests a guardian filed
// before they had an account, on the caller's transaction.
type GuardianEnrollments interface {
	ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error)
}
