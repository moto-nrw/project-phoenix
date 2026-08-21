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

	// Deactivate sets an RFID card as inactive
	Deactivate(ctx context.Context, id string) error
}

// PersonRepository defines operations for managing persons
type PersonRepository interface {
	base.CRUDRepository[*Person]

	// FindByIDForUpdate retrieves and locks a person row for a transaction.
	FindByIDForUpdate(ctx context.Context, id int64) (*Person, error)

	// FindByIDs retrieves multiple persons by their IDs in a single query
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Person, error)

	// FindByTagID retrieves a person by their RFID tag ID
	FindByTagID(ctx context.Context, tagID string) (*Person, error)

	// FindByAccountID retrieves a person by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Person, error)

	// FindByAccountIDs retrieves persons for the given account IDs in one
	// query, keyed by account ID.
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*Person, error)

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
	base.CRUDRepository[*Student]

	// FindByIDs retrieves multiple students by their IDs in a single query
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Student, error)

	// FindReadScopeByIDs retrieves a lightweight projection of the given students
	// — only id, group_id, person_id, and school_class — without the weekday
	// bus-day / departure hydration FindByIDs performs. It exists for callers that
	// only need read-access gating and name display (e.g. the frequently-polled
	// reminders service) and must not pay for schema probing and jsonb hydration
	// on data they never read. The returned *Student values have ONLY those four
	// fields populated.
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*Student, error)

	// FindByPersonID retrieves a student by their person ID
	FindByPersonID(ctx context.Context, personID int64) (*Student, error)

	// FindByGroupID retrieves students by their group ID
	FindByGroupID(ctx context.Context, groupID int64) ([]*Student, error)

	// FindByGroupIDs retrieves students by multiple group IDs
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*Student, error)

	// FindBySchoolClass retrieves students by their school class
	FindBySchoolClass(ctx context.Context, schoolClass string) ([]*Student, error)

	// ExistsEnrolledByNameAndBirthday reports whether an already-enrolled
	// student (active OR pending — a child approved before its service
	// start date is created pending until activation) with the given
	// (case-insensitive) name and birthday exists in the tenant. Backs the
	// enrollment new_students audience check (#1663).
	// TenantID is filtered explicitly because the enrollment parent
	// submit path runs under an admin transaction where RLS does not
	// narrow the query. Not expressible via the generic List filters:
	// the match spans the joined users.persons row.
	ExistsEnrolledByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday timezone.Date) (bool, error)

	// FindEnrolledStudentIDByNameAndBirthday resolves the single enrolled
	// student matching the (case-insensitive) name and birthday, backing the
	// existing_students re-enrollment path (#1663). Returns the ID only on an
	// unambiguous single match; zero or multiple matches yield (nil, nil) so
	// approval never renews an arbitrary student. Same explicit-tenant,
	// active+pending scope as ExistsEnrolledByNameAndBirthday.
	FindEnrolledStudentIDByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday timezone.Date) (*int64, error)

	// ListSchoolClasses retrieves all distinct non-empty school classes.
	ListSchoolClasses(ctx context.Context) ([]string, error)

	// FindBirthdaysOn returns the non-graduated children whose birthday falls
	// on one of the given annually recurring days (#1542). Children without a
	// stored birth date are omitted, never rendered as an unknown date.
	FindBirthdaysOn(ctx context.Context, days []MonthDay) ([]BirthdayEntry, error)

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

	// FindByTeacherIDWithGroups retrieves students with group names supervised by a teacher
	FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*StudentWithGroupInfo, error)

	// FindAllWithGroups retrieves all students with their group names (LEFT JOIN for students without groups)
	FindAllWithGroups(ctx context.Context) ([]*StudentWithGroupInfo, error)

	// FindByNameAndClass retrieves students by first name, last name, and school class (for import duplicate detection).
	// Alumni are excluded: a graduate is soft-deleted and must not block the
	// import of a new child sharing their name and class.
	FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*Student, error)

	// UpdateStatus changes a student's lifecycle status. Tenant-scoped via context.
	// Unconditional: it overwrites whatever status the row currently carries, so
	// it must NOT be used by background lifecycle work that decided on a status
	// it read earlier — use TransitionStatus for that.
	UpdateStatus(ctx context.Context, studentID int64, newStatus StudentStatus) error

	// TransitionStatus is the compare-and-set form: the row flips to next only
	// while it still holds expected. Returns false for a stale transition.
	// Tenant-scoped via context.
	TransitionStatus(ctx context.Context, studentID int64, expected, next StudentStatus) (bool, error)

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

	// LockStudentClassWrites acquires the per-tenant advisory gate that keeps
	// students from being created in — or moved into — a class while the caller
	// runs. Taken EXCLUSIVELY by the grade transition apply/revert; every
	// ordinary student write takes the shared form inside the repository, so no
	// caller has to remember it. Row locks cannot cover this case: a child who
	// arrives in a mapped class during an apply has no row the apply could have
	// locked, and would otherwise be left behind in a class the transition just
	// emptied while the transition reported success (#405 review).
	//
	// Must be called inside a tenant tx; releases on commit/rollback. Take it
	// BEFORE the recurrence and grade-transition gates.
	LockStudentClassWrites(ctx context.Context) error

	// LockStudentClassWritesShared acquires the SHARED form of the class-writes
	// gate. The repository takes it implicitly in front of every student
	// insert/update/row lock; callers only need it explicitly when they must
	// acquire ANOTHER tenant-wide gate (e.g. the recurrence gate) before their
	// first student row lock — the shared gate has to come first to keep the
	// project-wide order (class-writes → recurrence → rows) acyclic against a
	// concurrently applying grade transition (#2147 review round 12).
	LockStudentClassWritesShared(ctx context.Context) error

	// FindByIDForUpdate retrieves a student by id with SELECT … FOR
	// UPDATE so the caller can re-validate state under the same row
	// lock the next UPDATE on the row will use. Used by the photo
	// upload flow to close a lost-update race against concurrent
	// consent withdrawals from another tab.
	FindByIDForUpdate(ctx context.Context, id int64) (*Student, error)

	// VerifyCompanionStrandingBatch decides the "läuft mit" stranding verdicts
	// that the departure-plan writes of a coordinated multi-child edit deferred
	// into the CompanionStrandingBatch on ctx, now that every plan change and
	// edge removal of that edit is applied. Returns
	// ErrCompanionWouldLoseDeparture when a linked child is genuinely left
	// without a "mit wem" detail, and nil when no batch is open. Callers that
	// opened a batch MUST call this before committing.
	VerifyCompanionStrandingBatch(ctx context.Context) error

	// FindByIDForUpdateNoWait is FindByIDForUpdate that fails immediately
	// (PostgreSQL 55P03) instead of waiting when the row is already locked.
	// Used only where waiting would invert the ascending-id order every
	// companion writer follows and could therefore deadlock — see the
	// lock protocol in api/students and StudentRepository.lockCompanionFarEnds.
	FindByIDForUpdateNoWait(ctx context.Context, id int64) (*Student, error)

	// FindByIDsForUpdate fetches and locks the given student rows in one
	// SELECT … ORDER BY id FOR UPDATE (the project-wide ascending-id lock
	// order), so batch writers acquire all their row locks in one query and
	// overlapping batches serialize instead of deadlocking. Unknown or
	// foreign ids are absent from the returned map.
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*Student, error)

	// FindCareBoundsByIDs returns the last care day of each given child that
	// has one. Deliberately a projection of a single DATE column rather than
	// whole rows: the materializer needs it per date for hundreds of children
	// and must not pay for departure-plan hydration to answer one question
	// (#2487).
	FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]timezone.Date, error)

	// SetEnrolledUntilByIDs writes the enrollment interval's upper bound —
	// the LAST CARE DAY, inclusive — for a whole batch in one statement, and
	// returns how many rows changed. A nil `until` clears the bound, which is
	// what cancelling a planned exit and resuming care both do (#2487).
	SetEnrolledUntilByIDs(ctx context.Context, ids []int64, until *timezone.Date) (int64, error)

	// SetEnrollmentWindowByID reopens one child's care from a new start day:
	// enrolled_from moves to `from`, enrolled_until is cleared and the
	// lifecycle status is recomputed against `today` (#2487).
	SetEnrollmentWindowByID(ctx context.Context, id int64, from timezone.Date, status StudentStatus) error
}

// ClassListEntryRepository defines operations for the class-list-only entries
// (#2382). School classes are free-text strings; every class comparison uses
// LOWER(BTRIM(...)) — see models/users.ClassListEntry.
type ClassListEntryRepository interface {
	base.CRUDRepository[*ClassListEntry]
	// FindBySchoolClass returns the entries of one class, name-sorted.
	FindBySchoolClass(ctx context.Context, schoolClass string) ([]*ClassListEntry, error)
	// FindByNameAndClass returns entries matching first name, last name and
	// class case-insensitively (duplicate guard for create and import).
	FindByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]*ClassListEntry, error)
}

// StaffRepository defines operations for managing staff members
type StaffRepository interface {
	base.CRUDRepository[*Staff]

	// FindByIDForUpdate retrieves and locks a staff row for a transaction.
	FindByIDForUpdate(ctx context.Context, id int64) (*Staff, error)

	// FindByPersonID retrieves a staff member by their person ID
	FindByPersonID(ctx context.Context, personID int64) (*Staff, error)

	// ListAllWithPerson retrieves all staff members with their associated person data in a single query
	ListAllWithPerson(ctx context.Context) ([]*Staff, error)

	// FindReachableCalendarStaffIDs returns the subset of the given staff IDs
	// (or all staff when ids is empty) that can use the calendar for the current
	// tenant: an active linked account with an active account_tenants mapping for
	// this tenant and an effective calendar:own permission — resolved like auth
	// (role + direct account_permissions grants, wildcard-aware) so unreachable
	// staff aren't invited as recipients.
	FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error)

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

	// ListStaffWithPermission returns all active staff whose effective
	// permissions match the given permission name (wildcard-aware,
	// tenant-scoped). Used for absence-request email fan-out (#1419).
	ListStaffWithPermission(ctx context.Context, permissionName string) ([]*StaffWithRoleInfo, error)

	// GetStaffContactInfo returns name + account email for one staff member
	// (staff → person → account join). Used for absence-decision emails (#1419).
	GetStaffContactInfo(ctx context.Context, staffID int64) (*StaffWithRoleInfo, error)

	// ListAccountIDsByStaffIDs maps the given staff members to their login
	// accounts. Staff without an account are omitted rather than mapped to
	// zero. For callers that address people by account instead of by staff row.
	ListAccountIDsByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]int64, error)

	// ListAllStaffAccountIDs maps every staff member of the current tenant who
	// can log in to their account: active account, active tenant mapping. The
	// candidate set for anything addressed at the whole team.
	ListAllStaffAccountIDs(ctx context.Context) (map[int64]int64, error)

	// ListStaffByRoles retrieves staff members who have any of the specified roles,
	// including their person data, account ID, and email, using a single JOIN query.
	ListStaffByRoles(ctx context.Context, roles []string) ([]*StaffWithRoleInfo, error)

	// FindBirthdaysOn returns the staff members whose birthday falls on one of
	// the given annually recurring days, excluding everyone who opted out of
	// the display (#1542). The opt-out is applied in the query so no caller
	// can bypass it.
	FindBirthdaysOn(ctx context.Context, days []MonthDay) ([]BirthdayEntry, error)

	// ListBirthdaysForExport returns every staff member with a stored birth
	// date, opt-outs included. Backs the administrative Geburtstagsliste,
	// which is gated on the same permission as the Stammdaten it reads.
	ListBirthdaysForExport(ctx context.Context) ([]BirthdayEntry, error)

	// SetBirthdayDisplayOptOut flips one staff member's dashboard opt-out.
	SetBirthdayDisplayOptOut(ctx context.Context, staffID int64, optOut bool) error
}

// TeacherRepository defines operations for managing teachers
type TeacherRepository interface {
	base.CRUDRepository[*Teacher]

	// ListActiveCaregivers returns every active caregiver for the tenant in
	// context (teachers with an active account, tenant mapping, and system
	// user/teacher role), ordered by name.
	ListActiveCaregivers(ctx context.Context) ([]*ActiveCaregiver, error)

	// FindActiveCaregiverByAccountID returns the active caregiver bound to
	// the account, or nil when the account is not an active caregiver.
	FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*ActiveCaregiver, error)

	// FindByStaffID retrieves a teacher by their staff ID
	FindByStaffID(ctx context.Context, staffID int64) (*Teacher, error)

	// FindByStaffIDs retrieves teachers by multiple staff IDs in a single query
	// Returns a map of staff_id -> Teacher for efficient lookup
	FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*Teacher, error)

	// FindBySpecialization retrieves teachers by their specialization
	FindBySpecialization(ctx context.Context, specialization string) ([]*Teacher, error)

	// ListWithOptions retrieves teachers matching the query options
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Teacher, error)

	// FindByGroupID retrieves teachers assigned to a group
	FindByGroupID(ctx context.Context, groupID int64) ([]*Teacher, error)

	// FindWithStaffAndPerson retrieves a teacher with their associated staff and person data
	FindWithStaffAndPerson(ctx context.Context, id int64) (*Teacher, error)

	// ListAllWithStaffAndPerson retrieves all teachers with their staff and person data in a single query
	ListAllWithStaffAndPerson(ctx context.Context) ([]*Teacher, error)

	// FindWithStaffAndPersonByIDs retrieves teachers with staff and person data for multiple IDs
	FindWithStaffAndPersonByIDs(ctx context.Context, ids []int64) ([]*Teacher, error)
}

// GuestRepository defines operations for managing guests
type GuestRepository interface {
	base.CRUDRepository[*Guest]

	// FindByStaffID retrieves a guest by their staff ID
	FindByStaffID(ctx context.Context, staffID int64) (*Guest, error)

	// FindActive retrieves currently active guests
	FindActive(ctx context.Context) ([]*Guest, error)
}

// ProfileRepository defines operations for managing profiles
type ProfileRepository interface {
	base.CRUDRepository[*Profile]

	// FindByAccountID retrieves a profile by account ID
	FindByAccountID(ctx context.Context, accountID int64) (*Profile, error)

	// UpdateAvatar updates a profile's avatar
	UpdateAvatar(ctx context.Context, id int64, avatar string) error
}

// StudentGuardianRepository defines operations for managing student-guardian relationships
// GuardianEmergencyContactRow is one (guardian, phone number) projection row
// for the emergency contact list; the consumer aggregates rows per student.
type GuardianEmergencyContactRow struct {
	StudentID         int64          `bun:"student_id"`
	GuardianProfileID int64          `bun:"guardian_profile_id"`
	FirstName         sql.NullString `bun:"first_name"`
	LastName          sql.NullString `bun:"last_name"`
	Email             sql.NullString `bun:"email"`
	PhoneNumber       sql.NullString `bun:"phone_number"`
}

type StudentGuardianRepository interface {
	base.CRUDRepository[*StudentGuardian]

	// ListEmergencyContactRows returns guardian/phone rows for the given
	// students, emergency contacts and primary entries first.
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]GuardianEmergencyContactRow, error)

	// FindByStudentID retrieves relationships by student ID
	FindByStudentID(ctx context.Context, studentID int64) ([]*StudentGuardian, error)

	// FindByStudentIDs retrieves relationships for many students in one query,
	// avoiding an N+1 when resolving whole-school / group / class parent targets.
	FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*StudentGuardian, error)

	// FindByGuardianProfileID retrieves relationships by guardian profile ID
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*StudentGuardian, error)

	// AccountHasStudentPermission reports whether the guardian account holds the
	// named parent_portal.* permission on its relationship to the given student
	// at the tenant, backed by an ACTIVE auth.account_tenants mapping. It is the
	// per-child authorization probe for parent-portal actions that resolve a
	// concrete student only deep inside a service — e.g. existing_students
	// re-enrollment (#1663), where a school-wide submit flag is too coarse to
	// prove authority over one specific child. tenant_id is passed explicitly so
	// the check is correct even under an admin transaction (RLS bypassed). A
	// deactivated guardian's lingering relationship rows report false.
	AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error)

	// FilterAccountsWithStudentAccess is the batched sibling of
	// AccountHasStudentPermission: it returns the subset of guardianAccountIDs
	// whose relationship to at least ONE of studentIDs still carries the named
	// parent_portal.* permission, backed by an ACTIVE account_tenants mapping.
	// It exists for delivery paths holding a recipient list that was resolved in
	// an earlier transaction — a notification about a child must be re-checked
	// where it is SENT, because access can be revoked in between (#1671). The
	// result keeps the caller's order and can only narrow the input.
	FilterAccountsWithStudentAccess(ctx context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error)

	// GuardianEmailHasStudentPermission is the accountless sibling of
	// AccountHasStudentPermission: the same per-child probe keyed on the
	// guardian's EMAIL, for flows whose submitter has no portal account — a late
	// enrollment invite names a guardian email and may reach a guardian who never
	// logged in (#1663). guardian_profiles is unique on (tenant_id, LOWER(email)),
	// so at most one profile per school answers. An active account_tenants mapping
	// is required only when the profile carries an account, so a deactivated
	// guardian's lingering relationship rows still report false.
	GuardianEmailHasStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error)

	// FindByStudentAndGuardianForUpdate returns the relationship row joining the
	// student and guardian profile, locked FOR UPDATE for the current
	// transaction, or ErrStudentGuardianNotFound when none exists. The row lock
	// serializes against a concurrent staff edit/delete of the SAME relationship
	// row (a plain UPDATE/DELETE takes the conflicting row lock), so a caller that
	// reads role/account state and then writes cannot have that state changed out
	// from under it between the authorization check and the write. Must run inside
	// a tenant transaction (RLS-scoped via TenantWhere).
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*StudentGuardian, error)

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

	// UpdateColumns writes only the named columns of the relationship row
	// (matched by primary key, tenant-scoped). Use it to edit a bounded subset
	// of fields without clobbering columns the caller does not own — e.g. a
	// parent editing only pickup flags/notes must not overwrite guardian_role,
	// permissions, or relationship_type a staff editor may have changed
	// concurrently. Returns the number of rows affected.
	UpdateColumns(ctx context.Context, relationship *StudentGuardian, columns ...string) (int64, error)

	// SetPrimary sets a guardian as the primary guardian for a student
	SetPrimary(ctx context.Context, id int64, isPrimary bool) error
}

// StudentCompanionRepository defines operations for the child-to-child
// departure links ("läuft mit" / Laufgemeinschaft, users.student_companions).
//
// An edge is stored once in normalized low/high order, so every read has to
// consider both endpoint columns — that is why there is no generic
// List(filters) usage here: a filter map cannot express the OR.
type StudentCompanionRepository interface {
	base.CRUDRepository[*StudentCompanion]

	// ListForStudent returns every edge touching the student, all weekdays.
	ListForStudent(ctx context.Context, studentID int64) ([]*StudentCompanion, error)

	// ListLinksForStudent returns the edges folded per companion, with names.
	ListLinksForStudent(ctx context.Context, studentID int64) ([]CompanionLink, error)

	// ListLinksForStudents is the bulk form of ListLinksForStudent, for the
	// offline lists that render the "mit wem" detail of a whole school.
	ListLinksForStudents(ctx context.Context, studentIDs []int64) (map[int64][]CompanionLink, error)

	// ReplaceForStudent makes the given edges the student's complete set.
	ReplaceForStudent(ctx context.Context, studentID int64, edges []*StudentCompanion) error

	// CompanionIDsForWeekday bulk-resolves companions for one weekday.
	CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error)

	// CompanionCountsExcluding returns, per student, the number of DISTINCT
	// other children they are linked to, ignoring links to excludeID.
	CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error)

	// CompanionDaysCoveredExcluding returns, per student, the weekday keys on
	// which they keep at least one edge to a child other than excludeID. The
	// "mit wem" cover is per weekday, so removal checks need this per-day view.
	CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error)
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
	base.CRUDRepository[*PrivacyConsent]

	// FindByStudentID retrieves privacy consents for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*PrivacyConsent, error)

	// FindActiveByStudentID retrieves active privacy consents for a student
	FindActiveByStudentID(ctx context.Context, studentID int64) ([]*PrivacyConsent, error)

	// Accept marks a privacy consent as accepted
	Accept(ctx context.Context, id int64, acceptedAt time.Time) error

	// Revoke revokes a privacy consent
	Revoke(ctx context.Context, id int64) error

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

	// LockByIDForUpdate locks a guardian profile row for the current
	// transaction. Used by full-delete flows to block concurrent link inserts.
	LockByIDForUpdate(ctx context.Context, id int64) error

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

	// FindByIDs retrieves guardian profiles for the given ids in a single query,
	// keyed by id. Missing ids are simply absent from the map.
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*GuardianProfile, error)

	// FindActivePortalProfilesByIDs retrieves guardian profiles with a linked
	// account and active account_tenants membership for the current tenant.
	FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*GuardianProfile, error)

	// Update updates an existing guardian profile
	Update(ctx context.Context, profile *GuardianProfile) error

	// UpdatePortalLocaleByAccountID updates portal_locale for every guardian
	// profile linked to the given parent account.
	UpdatePortalLocaleByAccountID(ctx context.Context, accountID int64, locale string) error

	// Delete removes a guardian profile
	Delete(ctx context.Context, id int64) error

	// LinkAccount links a guardian profile to a parent account
	LinkAccount(ctx context.Context, profileID int64, accountID int64) error

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

	// FindByGuardianIDs retrieves all phone numbers for the given guardian
	// profile ids in a single query, grouped by profile id. Each group is
	// ordered primary-first, matching FindByGuardianID.
	FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*GuardianPhoneNumber, error)

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
