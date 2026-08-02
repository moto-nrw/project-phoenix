package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentWithGroup represents a student with their group information
type StudentWithGroup struct {
	Student   *userModels.Student `json:"student"`
	GroupName string              `json:"group_name"`
}

// PersonService defines the operations available in the person service layer
type PersonService interface {
	// Get retrieves a person by their ID
	Get(ctx context.Context, id interface{}) (*userModels.Person, error)

	// GetByIDs retrieves multiple persons by their IDs in a single query
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)

	// Create creates a new person
	Create(ctx context.Context, person *userModels.Person) error

	// Update updates an existing person
	Update(ctx context.Context, person *userModels.Person) error

	// Delete removes a person
	Delete(ctx context.Context, id interface{}) error

	// List retrieves persons matching the provided query options
	List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error)

	// FindByTagID finds a person by their RFID tag ID
	FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error)

	// FindByAccountID finds a person by their account ID
	FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error)

	// FindByName finds persons matching the provided name (first name, last name, or both)
	FindByName(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error)

	// LinkToAccount associates a person with an account
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error

	// UnlinkFromAccount removes account association from a person
	UnlinkFromAccount(ctx context.Context, personID int64) error

	// LinkToRFIDCard associates a person with an RFID card
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error

	// LinkStudentToRFIDCard assigns a bracelet to a student, re-reading the
	// student row under a FOR UPDATE lock first so a graduation committing in
	// the meantime is refused with ErrStudentGraduated instead of leaving a tag
	// linked to an alumnus. Every student-facing tag assignment must go through
	// this method rather than LinkToRFIDCard (#405).
	LinkStudentToRFIDCard(ctx context.Context, studentID int64, tagID string) error

	// UnlinkFromRFIDCard removes RFID card association from a person
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error

	// Entity lookups (issue #584). These replaced the now-deleted repository
	// getters (StudentRepository/StaffRepository/TeacherRepository). CONTRACT: each method returns the
	// repository result and error VERBATIM — no wrapping, no sentinel mapping —
	// because callers (IoT device flows, PyrePortal contract) branch on
	// sql.ErrNoRows and render err.Error() into response bodies. Introducing
	// typed errors is follow-up work; see backend-conventions rule 8 deviation
	// note in the #584 PR description.

	// GetStaffByID retrieves a staff member by ID.
	GetStaffByID(ctx context.Context, id int64) (*userModels.Staff, error)

	// GetStaffByPersonID retrieves the staff record belonging to a person.
	GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)

	// ResolveStaffIDByAccountID maps a JWT account id to its staff id via the
	// account → person → staff chain. Used for self-ownership checks and
	// audit attribution; returns a wrapped error when either lookup fails.
	ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error)

	// GetStaffWithPerson retrieves a staff member with person data preloaded.
	GetStaffWithPerson(ctx context.Context, id int64) (*userModels.Staff, error)

	// GetStaffWithPersonByIDs retrieves multiple staff with person data preloaded.
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error)

	// ListStaffWithPerson retrieves all staff with person data preloaded.
	ListStaffWithPerson(ctx context.Context) ([]*userModels.Staff, error)

	// ListStaffByRoles retrieves staff holding any of the given roles, with
	// person data, account ID, and email.
	ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error)

	// GetTeacherByStaffID retrieves the teacher record for a staff member.
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error)

	// GetTeachersByStaffIDs retrieves teacher records for multiple staff members.
	GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error)

	// GetTeachersBySpecialization retrieves teachers by specialization.
	GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error)

	// GetTeacherWithStaffAndPerson retrieves a teacher with staff and person preloaded.
	GetTeacherWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error)

	// ListTeachersWithStaffAndPerson retrieves all teachers with staff and person preloaded.
	ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error)

	// GetStudentByID retrieves a student by ID.
	GetStudentByID(ctx context.Context, id int64) (*userModels.Student, error)

	// GetStudentByIDForUpdate retrieves a student by ID under a SELECT … FOR
	// UPDATE row lock held for the caller's transaction, so a status the caller
	// validates cannot be changed by a concurrent grade transition before the
	// caller's own write commits. Errors are returned verbatim like the other
	// entity lookups (sql.ErrNoRows stays unwrappable).
	GetStudentByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error)

	// GetStudentByPersonID retrieves the student record belonging to a person.
	GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error)

	// GetStudentsByIDs retrieves multiple students by ID.
	GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error)

	// GetStudentsByGroupID retrieves the students of a group.
	GetStudentsByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error)

	// GetStudentsByGroupIDs retrieves the students of multiple groups.
	GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error)

	// GetEligibleStudentsByGroupIDsOnDate retrieves group students enrolled on
	// the requested calendar date. today is the caller's frozen calendar day,
	// so a request crossing Berlin midnight keeps one notion of "today".
	GetEligibleStudentsByGroupIDsOnDate(ctx context.Context, groupIDs []int64, date, today timezone.Date) ([]*userModels.Student, error)

	// CountStudentsByGroupIDs counts students per group in a single query.
	CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)

	// CreateStaffWithTeacher creates a staff record and, when requested, a
	// teacher record in one tenant transaction. A failed teacher creation is
	// deliberately non-fatal (teacherCreationFailed=true, teacher=nil): the
	// staff row still persists, mirroring the historical api/staff behaviour.
	CreateStaffWithTeacher(ctx context.Context, input CreateStaffInput) (staff *userModels.Staff, teacher *userModels.Teacher, teacherCreationFailed bool, err error)

	// UpdateStaffWithTeacher applies the already-mutated directory fields to
	// a freshly locked staff row, reloads it with person data, and applies the
	// requested teacher-record change. Teacher-record failures are non-fatal
	// and reported through the returned TeacherAction.
	UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, TeacherAction, error)

	// UpdatePersonnelNumber sets or clears (nil / empty) the staff member's
	// payroll Personalnummer (#1417). Writes an audit row in the same tenant
	// transaction; returns ErrPersonnelNumberTaken on a per-tenant duplicate
	// and ErrPersonnelNumberInvalid on a malformed value.
	UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*userModels.Staff, error)

	// Staff Stammdaten (#1423). Section-scoped reads/writes for the master
	// data tab. Every write locks the staff row and records field-level
	// audit rows in the same tenant transaction; every financial read
	// writes an audit.data_access_log row before any value is served.

	// GetStaffStammdaten returns the non-sensitive sections (person,
	// contact, contract, qualifications). Financial data has its own
	// permission-gated read path.
	GetStaffStammdaten(ctx context.Context, staffID int64) (*StaffStammdaten, error)

	// UpdateStaffStammdatenPerson replaces the person section.
	UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input StammdatenPersonInput, changedByStaffID int64, note string) error

	// UpdateStaffStammdatenKontakt replaces the contact section.
	UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input StammdatenKontaktInput, changedByStaffID int64, note string) error

	// UpdateStaffStammdatenArbeitsvertrag replaces the contract section.
	UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error

	// ReplaceStaffQualifications replaces the full qualification list.
	ReplaceStaffQualifications(ctx context.Context, staffID int64, inputs []StammdatenQualificationInput, changedByStaffID int64, note string) error

	// GetStaffFinancialMasked serves masked bank & tax data (audited read).
	GetStaffFinancialMasked(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*StaffFinancialMasked, error)

	// RevealStaffFinancial serves the full bank & tax values (audited).
	RevealStaffFinancial(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*StaffFinancialPlain, error)

	// UpdateStaffFinancial replaces the bank & tax section; audit rows carry
	// masked values only.
	UpdateStaffFinancial(ctx context.Context, staffID int64, input StammdatenFinancialInput, changedByAccountID int64, note string) error

	// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
	GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error)

	// GetAllStudentsWithGroups retrieves all students with their group info
	GetAllStudentsWithGroups(ctx context.Context) ([]StudentWithGroup, error)
}

// CreateStaffInput carries the payload for CreateStaffWithTeacher.
type CreateStaffInput struct {
	PersonID       int64
	StaffNotes     string
	IsTeacher      bool
	Specialization string
	Role           string
	Qualifications string
}

// TeacherAction describes the teacher-record outcome of UpdateStaffWithTeacher.
type TeacherAction int

const (
	// TeacherActionNone — not a teacher request and no existing teacher record.
	TeacherActionNone TeacherAction = iota
	// TeacherActionExisting — not a teacher request, existing record untouched.
	TeacherActionExisting
	// TeacherActionUpdated — existing teacher record updated.
	TeacherActionUpdated
	// TeacherActionUpdateFailed — teacher update failed; staff update persisted.
	TeacherActionUpdateFailed
	// TeacherActionCreated — new teacher record created.
	TeacherActionCreated
	// TeacherActionCreateFailed — teacher creation failed; staff update persisted.
	TeacherActionCreateFailed
)

// StaffOffboardingService fully offboards a staff member within a tenant:
// soft-deletes the staff/teacher rows, cleans up planned assignments, and
// revokes the linked account's access for the tenant.
type StaffOffboardingService interface {
	// OffboardStaff offboards the staff member. deletedByStaffID identifies
	// the acting staff member in time-tracking tombstones; deletedBy is the
	// account username used by the broader data-deletion audit.
	OffboardStaff(ctx context.Context, staffID, deletedByStaffID int64, deletedBy string) error
}

// CaregiverCapabilityService manages the operational caregiver capability of an
// existing account inside a tenant.
type CaregiverCapabilityService interface {
	GetCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error)
	EnableCaregiverCapability(ctx context.Context, accountID int64, input userModels.EnableCaregiverCapabilityInput) (*userModels.CaregiverCapabilityState, error)
	DisableCaregiverCapability(ctx context.Context, accountID int64) (*userModels.CaregiverCapabilityState, error)
}
