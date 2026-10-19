package users

import (
	context "context"
	time "time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleBirthday "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type rawCRUDRepository[T any] interface {
	Create(ctx context.Context, entity T) error
	FindByID(ctx context.Context, id any) (T, error)
	Update(ctx context.Context, entity T) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]T, error)
}
type rawPersonRepository interface {
	directory.DirectoryPersonReads
	rawCRUDRepository[*models.Person]
	FindByIDForUpdate(ctx context.Context, id int64) (*models.Person, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*models.Person, error)
	FindByTagID(ctx context.Context, tagID string) (*models.Person, error)
	FindByAccountID(ctx context.Context, accountID int64) (*models.Person, error)
	FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*models.Person, error)
	ListWithOptions(ctx context.Context, options *ports.QueryOptions) ([]*models.Person, error)
	LinkToAccount(ctx context.Context, personID int64, accountID int64) error
	UnlinkFromAccount(ctx context.Context, personID int64) error
	LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error
	UnlinkFromRFIDCard(ctx context.Context, personID int64) error
	FindWithAccount(ctx context.Context, id int64) (*models.Person, error)
	AnonymizeAndSoftDelete(ctx context.Context, personID int64) error
}
type rawStudentRepository interface {
	directory.StudentSnapshotReads
	rawCRUDRepository[*models.Student]
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*models.Student, error)
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*models.Student, error)
	FindByPersonID(ctx context.Context, personID int64) (*models.Student, error)
	FindByGroupID(ctx context.Context, groupID int64) ([]*models.Student, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*models.Student, error)
	ExistsEnrolledByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday calendar.Date) (bool, error)
	FindEnrolledStudentIDByNameAndBirthday(ctx context.Context, tenantID int64, firstName, lastName string, birthday calendar.Date) (*int64, error)
	ListSchoolClasses(ctx context.Context) ([]string, error)
	FindBirthdaysOn(ctx context.Context, days []peopleBirthday.MonthDay) ([]peopleBirthday.BirthdayEntry, error)
	ListByGroupIDsIncludingAlumni(ctx context.Context, groupIDs []int64) ([]*models.Student, error)
	ListClassRoster(ctx context.Context, schoolClass string) ([]*models.Student, error)
	CountEnrolled(ctx context.Context) (int, error)
	CountWithOptions(ctx context.Context, options *ports.QueryOptions) (int, error)
	CountByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error)
	FindByTeacherIDWithGroups(ctx context.Context, teacherID int64) ([]*models.StudentWithGroupInfo, error)
	FindByTeacherStaffIDsWithGroups(ctx context.Context, staffIDs []int64) ([]*models.StudentWithGroupInfo, error)
	FindAllWithGroups(ctx context.Context) ([]*models.StudentWithGroupInfo, error)
	FindOverlappingWithGroups(ctx context.Context, from, to, today calendar.Date) ([]*models.StudentWithGroupInfo, error)
	FindOverlappingWithGroupsOnDate(ctx context.Context, date string, now time.Time) ([]*models.StudentWithGroupInfo, error)
	UpdateStatus(ctx context.Context, studentID int64, newStatus models.StudentStatus) error
	TransitionStatus(ctx context.Context, studentID int64, expected, next models.StudentStatus) (bool, error)
	FindPendingDueForActivation(ctx context.Context, asOf calendar.Date) ([]*models.Student, error)
	FindActiveDueForDeactivation(ctx context.Context, asOf calendar.Date) ([]*models.Student, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*models.Student, error)
	VerifyCompanionStrandingBatch(ctx context.Context) error
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*models.Student, error)
	FindCareBoundsByIDs(ctx context.Context, ids []int64) (map[int64]calendar.Date, error)
}
type rawGuardianProfileRepository interface {
	directory.GuardianProfileReads
	Create(ctx context.Context, profile *models.GuardianProfile) error
	FindByID(ctx context.Context, id int64) (*models.GuardianProfile, error)
	LockByIDForUpdate(ctx context.Context, id int64) error
	FindByEmail(ctx context.Context, email string) (*models.GuardianProfile, error)
	FindByAccountID(ctx context.Context, accountID int64) (*models.GuardianProfile, error)
	FindWithoutAccount(ctx context.Context) ([]*models.GuardianProfile, error)
	FindInvitable(ctx context.Context) ([]*models.GuardianProfile, error)
	ListWithOptions(ctx context.Context, options *ports.QueryOptions) ([]*models.GuardianProfile, error)
	FindByEmails(ctx context.Context, emails []string) ([]*models.GuardianProfile, error)
	SearchByText(ctx context.Context, searchText string, limit int) ([]*models.GuardianProfile, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*models.GuardianProfile, error)
	FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*models.GuardianProfile, error)
	FindLoginReadyPortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*models.GuardianProfile, error)
	Update(ctx context.Context, profile *models.GuardianProfile) error
	Delete(ctx context.Context, id int64) error
	LinkAccount(ctx context.Context, profileID int64, accountID int64) error
	LoadProfileWithChildren(ctx context.Context, accountID int64) (*models.GuardianProfileWithChildren, error)
}
type rawStudentGuardianRepository interface {
	directory.GuardianRelationshipReads
	Create(ctx context.Context, relationship *models.StudentGuardian) error
	FindByID(ctx context.Context, id any) (*models.StudentGuardian, error)
	Update(ctx context.Context, relationship *models.StudentGuardian) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*models.StudentGuardian, error)
	ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]models.GuardianEmergencyContactRow, error)
	FindByStudentID(ctx context.Context, studentID int64) ([]*models.StudentGuardian, error)
	FindByStudentIDs(ctx context.Context, studentIDs []int64) ([]*models.StudentGuardian, error)
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*models.StudentGuardian, error)
	AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error)
	FilterAccountsWithStudentAccess(ctx context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error)
	GuardianEmailHasStudentPermission(ctx context.Context, email string, studentID, tenantID int64, permission string) (bool, error)
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*models.StudentGuardian, error)
	LinkIfNotExists(ctx context.Context, rel *models.StudentGuardian) (bool, error)
	ListLinkedChildrenForGuardians(ctx context.Context, guardianProfileIDs []int64) ([]*directory.GuardianLinkedChildSnapshot, error)
	UpdateColumns(ctx context.Context, relationship *models.StudentGuardian, columns ...string) (int64, error)
	SetPrimary(ctx context.Context, id int64, isPrimary bool) error
	SetPayer(ctx context.Context, studentID int64, guardianProfileID *int64) error
	ListPaymentAssignments(ctx context.Context) ([]directory.GuardianPaymentAssignment, error)
}
type rawGuardianPhoneNumberRepository interface {
	directory.GuardianPhoneReads
	Create(ctx context.Context, phone *models.GuardianPhoneNumber) error
	FindByID(ctx context.Context, id int64) (*models.GuardianPhoneNumber, error)
	FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*models.GuardianPhoneNumber, error)
	FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*models.GuardianPhoneNumber, error)
	Update(ctx context.Context, phone *models.GuardianPhoneNumber) error
	Delete(ctx context.Context, id int64) error
	SetPrimary(ctx context.Context, id int64, guardianProfileID int64) error
	UnsetAllPrimary(ctx context.Context, guardianProfileID int64) error
	CountByGuardianID(ctx context.Context, guardianProfileID int64) (int, error)
	GetNextPriority(ctx context.Context, guardianProfileID int64) (int, error)
}
