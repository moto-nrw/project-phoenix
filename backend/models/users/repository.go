package users

import (
	"context"
	"database/sql"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// RFIDCardRepository defines operations for managing RFID cards
type RFIDCardRepository interface {
	// Create inserts a new RFID card into the database
	Create(ctx context.Context, card *RFIDCard) error

	// FindByID retrieves an RFID card by its ID
	FindByID(ctx context.Context, id string) (*RFIDCard, error)

	// Update updates an existing RFID card
	Update(ctx context.Context, card *RFIDCard) error

	// Delete removes an RFID card
	Delete(ctx context.Context, id string) error

	// List retrieves RFID cards matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*RFIDCard, error)

	// Activate sets an RFID card as active
	Activate(ctx context.Context, id string) error

	// Deactivate sets an RFID card as inactive
	Deactivate(ctx context.Context, id string) error
}

// PersonRepository defines operations for managing persons
type PersonRepository interface {
	// Create inserts a new person into the database
	Create(ctx context.Context, person *Person) error

	// FindByID retrieves a person by their ID
	FindByID(ctx context.Context, id interface{}) (*Person, error)

	// FindByIDs retrieves multiple persons by their IDs in a single query
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Person, error)

	// FindByTagID retrieves a person by their RFID tag ID
	FindByTagID(ctx context.Context, tagID string) (*Person, error)

	// FindByAccountID retrieves a person by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Person, error)

	// Update updates an existing person
	Update(ctx context.Context, person *Person) error

	// Delete removes a person
	Delete(ctx context.Context, id interface{}) error

	// List retrieves persons matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Person, error)

	// ListWithOptions retrieves persons with type-safe query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Person, error)

	// LinkToAccount associates a person with an account
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error

	// UnlinkFromAccount removes account association from a person
	UnlinkFromAccount(ctx context.Context, personID int64) error

	// LinkToRFIDCard associates a person with an RFID card
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error

	// UnlinkFromRFIDCard removes RFID card association from a person
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error

	// FindWithAccount retrieves a person with their associated account
	FindWithAccount(ctx context.Context, id int64) (*Person, error)

	// AnonymizeAndSoftDelete overwrites the person's PII with placeholder
	// values and stamps deleted_at (GDPR person deletion).
	AnonymizeAndSoftDelete(ctx context.Context, personID int64) error
}

// StudentRepository defines operations for managing students
type StudentRepository interface {
	// Create inserts a new student into the database
	Create(ctx context.Context, student *Student) error

	// FindByID retrieves a student by their ID
	FindByID(ctx context.Context, id interface{}) (*Student, error)

	// FindByIDs retrieves multiple students by their IDs in a single query
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Student, error)

	// FindByPersonID retrieves a student by their person ID
	FindByPersonID(ctx context.Context, personID int64) (*Student, error)

	// FindByGroupID retrieves students by their group ID
	FindByGroupID(ctx context.Context, groupID int64) ([]*Student, error)

	// FindByGroupIDs retrieves students by multiple group IDs
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*Student, error)

	// FindBySchoolClass retrieves students by their school class
	FindBySchoolClass(ctx context.Context, schoolClass string) ([]*Student, error)

	// Update updates an existing student
	Update(ctx context.Context, student *Student) error

	// Delete removes a student
	Delete(ctx context.Context, id interface{}) error

	// List retrieves students matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Student, error)

	// ListWithOptions retrieves students with query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Student, error)

	// CountWithOptions counts students matching the query options
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary
	// key and returns the number of rows affected.
	UpdateColumns(ctx context.Context, student *Student, columns ...string) (int64, error)

	// CountByGroupIDs counts students per group for multiple groups in a single query
	CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)

	// AssignToGroup assigns a student to a group
	AssignToGroup(ctx context.Context, studentID int64, groupID int64) error

	// RemoveFromGroup removes a student from their group
	RemoveFromGroup(ctx context.Context, studentID int64) error

	// FindByTeacherID retrieves students supervised by a teacher (through group assignments)
	FindByTeacherID(ctx context.Context, teacherID int64) ([]*Student, error)

	// FindByTeacherIDWithGroups retrieves students with group names supervised by a teacher
	FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*StudentWithGroupInfo, error)

	// FindAllWithGroups retrieves all students with their group names (LEFT JOIN for students without groups)
	FindAllWithGroups(ctx context.Context) ([]*StudentWithGroupInfo, error)

	// FindByNameAndClass retrieves students by first name, last name, and school class (for import duplicate detection)
	FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*Student, error)

	// UpdateStatus changes a student's lifecycle status. Tenant-scoped via context.
	UpdateStatus(ctx context.Context, studentID int64, newStatus StudentStatus) error

	// FindPendingDueForActivation returns students whose status='pending' AND
	// enrolled_from <= asOf within the current tenant context. Used by the
	// activate-students scheduler tick.
	FindPendingDueForActivation(ctx context.Context, asOf timezone.Date) ([]*Student, error)

	// FindActiveDueForDeactivation returns students whose status='active' AND
	// enrolled_until <= asOf within the current tenant context. Used by the
	// activate-students scheduler tick to flip rows to 'inactive'.
	FindActiveDueForDeactivation(ctx context.Context, asOf timezone.Date) ([]*Student, error)

	// PurgeAllPhotos clears photo_path on every student row visible in the
	// current tenant context (RLS scopes it) and returns the list of stored
	// URLs that were cleared. Caller is responsible for unlinking the
	// underlying files.
	//
	// Used when an admin disables operations.student_photos_enabled - the
	// reviewer flagged that without this, existing photos remain accessible
	// after the toggle. The DB clear runs inside whatever transaction the
	// caller provides via context, so it is atomic with the setting write;
	// file unlinks are best-effort and happen after commit. Acquires the
	// per-tenant photo-feature advisory lock so it serializes against
	// concurrent upload tx's that hold the same lock.
	PurgeAllPhotos(ctx context.Context) ([]string, error)

	// LockPhotoFeature acquires the per-tenant advisory lock that
	// serializes operations affecting the student-photo feature (uploads
	// vs. feature disable). Must be called inside a tenant tx; lock
	// releases on commit/rollback. See implementation for the full race
	// rationale.
	LockPhotoFeature(ctx context.Context) error

	// FindByIDForUpdate retrieves a student by id with SELECT … FOR
	// UPDATE so the caller can re-validate state under the same row
	// lock the next UPDATE on the row will use. Used by the photo
	// upload flow to close a lost-update race against concurrent
	// consent withdrawals from another tab.
	FindByIDForUpdate(ctx context.Context, id int64) (*Student, error)
}

// StaffRepository defines operations for managing staff members
type StaffRepository interface {
	// Create inserts a new staff member into the database
	Create(ctx context.Context, staff *Staff) error

	// FindByID retrieves a staff member by their ID
	FindByID(ctx context.Context, id interface{}) (*Staff, error)

	// FindByPersonID retrieves a staff member by their person ID
	FindByPersonID(ctx context.Context, personID int64) (*Staff, error)

	// Update updates an existing staff member
	Update(ctx context.Context, staff *Staff) error

	// Delete removes a staff member
	Delete(ctx context.Context, id interface{}) error

	// List retrieves staff members matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Staff, error)

	// ListAllWithPerson retrieves all staff members with their associated person data in a single query
	ListAllWithPerson(ctx context.Context) ([]*Staff, error)

	// UpdateNotes updates staff notes
	UpdateNotes(ctx context.Context, id int64, notes string) error

	// ClearWorkTimeModel sets work_time_model_id to NULL. Used by staff
	// offboarding: soft-deleted staff must not keep the reference, or the
	// RESTRICT FK blocks work-time-model deletion while the live-staff
	// pre-check reports zero assignments.
	ClearWorkTimeModel(ctx context.Context, id int64) error

	// FindWithPerson retrieves a staff member with their associated person data
	FindWithPerson(ctx context.Context, id int64) (*Staff, error)

	// FindByIDs retrieves multiple staff members by their IDs in a single query
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Staff, error)

	// FindWithPersonByIDs retrieves multiple staff members with their associated person data in a single query
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*Staff, error)

	// ListStaffByRoles retrieves staff members who have any of the specified roles,
	// including their person data, account ID, and email, using a single JOIN query.
	ListStaffByRoles(ctx context.Context, roles []string) ([]*StaffWithRoleInfo, error)
}

// TeacherRepository defines operations for managing teachers
type TeacherRepository interface {
	// Create inserts a new teacher into the database
	Create(ctx context.Context, teacher *Teacher) error

	// ListActiveCaregivers returns every active caregiver for the tenant in
	// context (teachers with an active account, tenant mapping, and system
	// user/teacher role), ordered by name.
	ListActiveCaregivers(ctx context.Context) ([]*ActiveCaregiver, error)

	// FindActiveCaregiverByAccountID returns the active caregiver bound to
	// the account, or nil when the account is not an active caregiver.
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*ActiveCaregiver, error)

	// FindByID retrieves a teacher by their ID
	FindByID(ctx context.Context, id interface{}) (*Teacher, error)

	// FindByStaffID retrieves a teacher by their staff ID
	FindByStaffID(ctx context.Context, staffID int64) (*Teacher, error)

	// FindByStaffIDs retrieves teachers by multiple staff IDs in a single query
	// Returns a map of staff_id -> Teacher for efficient lookup
	FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*Teacher, error)

	// FindBySpecialization retrieves teachers by their specialization
	FindBySpecialization(ctx context.Context, specialization string) ([]*Teacher, error)

	// Update updates an existing teacher
	Update(ctx context.Context, teacher *Teacher) error

	// Delete removes a teacher
	Delete(ctx context.Context, id interface{}) error

	// List retrieves teachers matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Teacher, error)

	// ListWithOptions retrieves teachers matching the query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Teacher, error)

	// FindByGroupID retrieves teachers assigned to a group
	FindByGroupID(ctx context.Context, groupID int64) ([]*Teacher, error)

	// UpdateQualifications updates a teacher's qualifications
	UpdateQualifications(ctx context.Context, id int64, qualifications string) error

	// FindWithStaffAndPerson retrieves a teacher with their associated staff and person data
	FindWithStaffAndPerson(ctx context.Context, id int64) (*Teacher, error)

	// ListAllWithStaffAndPerson retrieves all teachers with their staff and person data in a single query
	ListAllWithStaffAndPerson(ctx context.Context) ([]*Teacher, error)

	// FindWithStaffAndPersonByIDs retrieves teachers with staff and person data for multiple IDs
	FindWithStaffAndPersonByIDs(ctx context.Context, ids []int64) ([]*Teacher, error)
}

// GuestRepository defines operations for managing guests
type GuestRepository interface {
	// Create inserts a new guest into the database
	Create(ctx context.Context, guest *Guest) error

	// FindByID retrieves a guest by their ID
	FindByID(ctx context.Context, id interface{}) (*Guest, error)

	// FindByStaffID retrieves a guest by their staff ID
	FindByStaffID(ctx context.Context, staffID int64) (*Guest, error)

	// FindByOrganization retrieves guests by their organization
	FindByOrganization(ctx context.Context, organization string) ([]*Guest, error)

	// FindByExpertise retrieves guests by their activity expertise
	FindByExpertise(ctx context.Context, expertise string) ([]*Guest, error)

	// Update updates an existing guest
	Update(ctx context.Context, guest *Guest) error

	// Delete removes a guest
	Delete(ctx context.Context, id interface{}) error

	// List retrieves guests matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Guest, error)

	// FindActive retrieves currently active guests
	FindActive(ctx context.Context) ([]*Guest, error)

	// SetDateRange sets a guest's start and end dates
	SetDateRange(ctx context.Context, id int64, startDate, endDate timezone.Date) error
}

// ProfileRepository defines operations for managing profiles
type ProfileRepository interface {
	// Create inserts a new profile into the database
	Create(ctx context.Context, profile *Profile) error

	// FindByID retrieves a profile by its ID
	FindByID(ctx context.Context, id interface{}) (*Profile, error)

	// FindByAccountID retrieves a profile by account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Profile, error)

	// Update updates an existing profile
	Update(ctx context.Context, profile *Profile) error

	// Delete removes a profile
	Delete(ctx context.Context, id interface{}) error

	// List retrieves profiles matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*Profile, error)

	// UpdateAvatar updates a profile's avatar
	UpdateAvatar(ctx context.Context, id int64, avatar string) error

	// UpdateBio updates a profile's bio
	UpdateBio(ctx context.Context, id int64, bio string) error

	// UpdateSettings updates a profile's settings
	UpdateSettings(ctx context.Context, id int64, settings string) error
}

// PersonGuardianRepository defines operations for managing person-guardian relationships
type PersonGuardianRepository interface {
	// Create inserts a new person-guardian relationship into the database
	Create(ctx context.Context, relationship *PersonGuardian) error

	// FindByID retrieves a relationship by its ID
	FindByID(ctx context.Context, id interface{}) (*PersonGuardian, error)

	// FindByPersonID retrieves relationships by person ID
	FindByPersonID(ctx context.Context, personID int64) ([]*PersonGuardian, error)

	// FindByGuardianID retrieves relationships by guardian account ID
	FindByGuardianID(ctx context.Context, guardianID int64) ([]*PersonGuardian, error)

	// FindPrimaryByPersonID retrieves the primary guardian for a person
	FindPrimaryByPersonID(ctx context.Context, personID int64) (*PersonGuardian, error)

	// FindByRelationshipType retrieves relationships by relationship type
	FindByRelationshipType(ctx context.Context, personID int64, relationshipType RelationshipType) ([]*PersonGuardian, error)

	// Update updates an existing relationship
	Update(ctx context.Context, relationship *PersonGuardian) error

	// Delete removes a relationship
	Delete(ctx context.Context, id interface{}) error

	// List retrieves relationships matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*PersonGuardian, error)

	// SetPrimary sets a guardian as the primary guardian for a person
	SetPrimary(ctx context.Context, id int64, isPrimary bool) error

	// UpdatePermissions updates a guardian's permissions
	UpdatePermissions(ctx context.Context, id int64, permissions string) error

	// FindWithPerson retrieves a relationship with the associated person loaded
	FindWithPerson(ctx context.Context, id int64) (*PersonGuardian, error)

	// GrantPermissionToGuardian grants a specific permission to a guardian
	GrantPermissionToGuardian(ctx context.Context, id int64, permission string) error

	// RevokePermissionFromGuardian revokes a specific permission from a guardian
	RevokePermissionFromGuardian(ctx context.Context, id int64, permission string) error
}

// StudentGuardianRepository defines operations for managing student-guardian relationships
// GuardianEmergencyContactRow is one (guardian, phone number) projection row
// for the emergency contact list; the consumer aggregates rows per student.
type GuardianEmergencyContactRow struct {
	StudentID   int64          `bun:"student_id"`
	FirstName   sql.NullString `bun:"first_name"`
	LastName    sql.NullString `bun:"last_name"`
	PhoneNumber sql.NullString `bun:"phone_number"`
}

type StudentGuardianRepository interface {
	// Create inserts a new student-guardian relationship into the database
	Create(ctx context.Context, relationship *StudentGuardian) error

	// ListEmergencyContactRows returns guardian/phone rows for the given
	// students, emergency contacts and primary entries first.
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]GuardianEmergencyContactRow, error)

	// FindByID retrieves a relationship by its ID
	FindByID(ctx context.Context, id interface{}) (*StudentGuardian, error)

	// FindByStudentID retrieves relationships by student ID
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindByGuardianProfileID retrieves relationships by guardian profile ID
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*StudentGuardian, error)

	// LinkIfNotExists inserts the student↔guardian relationship, treating a
	// duplicate (same tenant_id + student_id + guardian_profile_id) as a no-op
	// via ON CONFLICT DO NOTHING. Returns true when a new row was inserted, false
	// when the link already existed. Race-safe and transaction-safe — a duplicate
	// never raises a unique violation (which would abort the surrounding tenant
	// tx). On conflict the existing row is left untouched.
	LinkIfNotExists(ctx context.Context, rel *StudentGuardian) (bool, error)

	// ListLinkedChildrenForGuardians returns, in a single query, the children
	// linked to any of the given guardian profiles (id + name only). Backs the
	// guardian picker search so it never falls into a per-guardian N+1. Tenant
	// isolation is enforced by RLS on the ambient tenant transaction (the query
	// carries no explicit tenant predicate).
	ListLinkedChildrenForGuardians(ctx context.Context, guardianProfileIDs []int64) ([]*GuardianLinkedChild, error)

	// FindPrimaryByStudentID retrieves the primary guardian for a student
	FindPrimaryByStudentID(ctx context.Context, studentID int64) (*StudentGuardian, error)

	// FindEmergencyContactsByStudentID retrieves all emergency contacts for a student
	FindEmergencyContactsByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindPickupAuthoritiesByStudentID retrieves all guardians who can pickup a student
	FindPickupAuthoritiesByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindByRelationshipType retrieves relationships by relationship type
	FindByRelationshipType(ctx context.Context, studentID int64, relationshipType string) ([]*StudentGuardian, error)

	// Update updates an existing relationship
	Update(ctx context.Context, relationship *StudentGuardian) error

	// Delete removes a relationship
	Delete(ctx context.Context, id interface{}) error

	// List retrieves relationships matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*StudentGuardian, error)

	// SetPrimary sets a guardian as the primary guardian for a student
	SetPrimary(ctx context.Context, id int64, isPrimary bool) error

	// SetEmergencyContact sets whether a guardian is an emergency contact
	SetEmergencyContact(ctx context.Context, id int64, isEmergencyContact bool) error

	// SetCanPickup sets whether a guardian can pickup a student
	SetCanPickup(ctx context.Context, id int64, canPickup bool) error

	// UpdatePermissions updates a guardian's permissions
	UpdatePermissions(ctx context.Context, id int64, permissions string) error
}

// StudentRetentionSetting is the projection used by the GDPR visit-cleanup
// worklist: one row per distinct (student, retention-days) pair with an
// accepted privacy consent.
type StudentRetentionSetting struct {
	StudentID         int64 `bun:"student_id"`
	DataRetentionDays int   `bun:"data_retention_days"`
}

// PrivacyConsentRepository defines operations for managing privacy consents
type PrivacyConsentRepository interface {
	// Create inserts a new privacy consent into the database
	Create(ctx context.Context, consent *PrivacyConsent) error

	// FindByID retrieves a privacy consent by its ID
	FindByID(ctx context.Context, id interface{}) (*PrivacyConsent, error)

	// FindByStudentID retrieves privacy consents for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*PrivacyConsent, error)

	// FindByStudentIDAndPolicyVersion retrieves a privacy consent for a student and policy version
	FindByStudentIDAndPolicyVersion(ctx context.Context, studentID int64, policyVersion string) (*PrivacyConsent, error)

	// FindActiveByStudentID retrieves active privacy consents for a student
	FindActiveByStudentID(ctx context.Context, studentID int64) ([]*PrivacyConsent, error)

	// FindExpired retrieves all expired privacy consents
	FindExpired(ctx context.Context) ([]*PrivacyConsent, error)

	// FindNeedingRenewal retrieves all privacy consents that need renewal
	FindNeedingRenewal(ctx context.Context) ([]*PrivacyConsent, error)

	// Update updates an existing privacy consent
	Update(ctx context.Context, consent *PrivacyConsent) error

	// Delete removes a privacy consent
	Delete(ctx context.Context, id interface{}) error

	// List retrieves privacy consents matching the filters
	List(ctx context.Context, filters map[string]interface{}) ([]*PrivacyConsent, error)

	// Accept marks a privacy consent as accepted
	Accept(ctx context.Context, id int64, acceptedAt time.Time) error

	// Revoke revokes a privacy consent
	Revoke(ctx context.Context, id int64) error

	// SetExpiryDate sets the expiry date for a privacy consent
	SetExpiryDate(ctx context.Context, id int64, expiresAt time.Time) error

	// SetRenewalRequired sets whether renewal is required for a privacy consent
	SetRenewalRequired(ctx context.Context, id int64, renewalRequired bool) error

	// UpdateDetails updates the details for a privacy consent
	UpdateDetails(ctx context.Context, id int64, details string) error

	// ListAcceptedRetentionSettings returns the distinct (student_id,
	// data_retention_days) pairs of accepted privacy consents, ordered by
	// student_id. Feeds the GDPR visit-cleanup worklist.
	ListAcceptedRetentionSettings(ctx context.Context) ([]StudentRetentionSetting, error)
}

// GuardianProfileRepository defines operations for managing guardian profiles
type GuardianProfileRepository interface {
	// Create inserts a new guardian profile into the database
	Create(ctx context.Context, profile *GuardianProfile) error

	// FindByID retrieves a guardian profile by their ID
	FindByID(ctx context.Context, id int64) (*GuardianProfile, error)

	// FindByEmail retrieves a guardian profile by their email address
	FindByEmail(ctx context.Context, email string) (*GuardianProfile, error)

	// FindByAccountID retrieves a guardian profile by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*GuardianProfile, error)

	// FindWithoutAccount retrieves guardian profiles without portal accounts
	FindWithoutAccount(ctx context.Context) ([]*GuardianProfile, error)

	// FindInvitable retrieves guardians who can be invited (has email, no account)
	FindInvitable(ctx context.Context) ([]*GuardianProfile, error)

	// ListWithOptions retrieves guardian profiles with pagination and filters
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*GuardianProfile, error)

	// SearchByText retrieves guardian profiles whose first name, last name, or
	// email matches the search text (case-insensitive substring). Tenant-scoped
	// via RLS; results are capped by limit to keep the picker payload small.
	SearchByText(ctx context.Context, searchText string, limit int) ([]*GuardianProfile, error)

	// Count returns the total number of guardian profiles
	Count(ctx context.Context) (int, error)

	// Update updates an existing guardian profile
	Update(ctx context.Context, profile *GuardianProfile) error

	// Delete removes a guardian profile
	Delete(ctx context.Context, id int64) error

	// LinkAccount links a guardian profile to a parent account
	LinkAccount(ctx context.Context, profileID int64, accountID int64) error

	// UnlinkAccount unlinks a guardian profile from their account
	UnlinkAccount(ctx context.Context, profileID int64) error

	// GetStudentCount returns the number of students for a guardian
	GetStudentCount(ctx context.Context, profileID int64) (int, error)

	// LoadProfileWithChildren returns the guardian profile linked to the
	// given account along with their primary phone and a summary of
	// every active student linked via users.students_guardians. Returns
	// (nil, nil) when no profile exists in the current tenant context
	// — callers fall through to claims-derived defaults instead of
	// erroring. RLS narrows reads to the tenant in context.
	LoadProfileWithChildren(ctx context.Context, accountID int64) (*GuardianProfileWithChildren, error)
}

// GuardianPhoneNumberRepository defines operations for managing guardian phone numbers
type GuardianPhoneNumberRepository interface {
	// Create inserts a new phone number into the database
	Create(ctx context.Context, phone *GuardianPhoneNumber) error

	// FindByID retrieves a phone number by its ID
	FindByID(ctx context.Context, id int64) (*GuardianPhoneNumber, error)

	// FindByGuardianID retrieves all phone numbers for a guardian profile
	FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*GuardianPhoneNumber, error)

	// GetPrimary retrieves the primary phone number for a guardian
	GetPrimary(ctx context.Context, guardianProfileID int64) (*GuardianPhoneNumber, error)

	// Update updates an existing phone number
	Update(ctx context.Context, phone *GuardianPhoneNumber) error

	// Delete removes a phone number
	Delete(ctx context.Context, id int64) error

	// SetPrimary sets a phone number as primary and unsets others for the guardian
	SetPrimary(ctx context.Context, id int64, guardianProfileID int64) error

	// UnsetAllPrimary unsets primary flag for all phone numbers of a guardian
	UnsetAllPrimary(ctx context.Context, guardianProfileID int64) error

	// CountByGuardianID returns the number of phone numbers for a guardian
	CountByGuardianID(ctx context.Context, guardianProfileID int64) (int, error)

	// DeleteByGuardianID removes all phone numbers for a guardian
	DeleteByGuardianID(ctx context.Context, guardianProfileID int64) error

	// GetNextPriority returns the next priority value for a guardian's phone numbers
	GetNextPriority(ctx context.Context, guardianProfileID int64) (int, error)
}
