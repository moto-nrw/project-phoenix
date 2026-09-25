package care

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

// The read ports below are the rows the child flows read and lock. They name
// only reads and row locks: every write goes through an owner command.

// ChildReads resolves the guardian's children across schools.
type ChildReads interface {
	FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error)
	ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error)
}

// EnrollablePhaseReads lists the schools a parent may enroll a child at.
type EnrollablePhaseReads interface {
	ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error)
	GuardianSubmitStatus(ctx context.Context, accountID, tenantID int64) (*parentModels.GuardianSubmitStatus, error)
}

// EnrollmentRequestReads lists the parent's enrollment requests.
type EnrollmentRequestReads interface {
	ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error)
}

// StudentReads reads and locks the child's row.
type StudentReads interface {
	FindByID(ctx context.Context, id any) (*usersModels.Student, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.Student, error)
}

// PersonReads reads the child's person row.
type PersonReads interface {
	FindByID(ctx context.Context, id any) (*usersModels.Person, error)
}

// GuardianProfileReads reads and locks guardian profiles.
type GuardianProfileReads interface {
	FindByID(ctx context.Context, id int64) (*usersModels.GuardianProfile, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.GuardianProfile, error)
	FindByEmail(ctx context.Context, email string) (*usersModels.GuardianProfile, error)
	FindByAccountID(ctx context.Context, accountID int64) (*usersModels.GuardianProfile, error)
	LockByIDForUpdate(ctx context.Context, id int64) error
}

// GuardianPhoneReads reads guardian phone numbers.
type GuardianPhoneReads interface {
	FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*usersModels.GuardianPhoneNumber, error)
	FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*usersModels.GuardianPhoneNumber, error)
}

// StudentGuardianReads reads and locks the guardian relationships.
type StudentGuardianReads interface {
	FindByStudentID(ctx context.Context, studentID int64) ([]*usersModels.StudentGuardian, error)
	FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*usersModels.StudentGuardian, error)
	FindByStudentAndGuardianForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*usersModels.StudentGuardian, error)
	ListLinkedChildrenForGuardians(ctx context.Context, guardianProfileIDs []int64) ([]*usersModels.GuardianLinkedChild, error)
	AccountHasStudentPermission(ctx context.Context, accountID, studentID, tenantID int64, permission string) (bool, error)
}

// DataRequestReads reads the child's Stammdaten change requests.
type DataRequestReads interface {
	FindByID(ctx context.Context, id any) (*usersModels.StudentDataChangeRequest, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*usersModels.StudentDataChangeRequest, error)
	HasPendingForField(ctx context.Context, studentID int64, target, fieldKey string) (bool, error)
	ListByStudent(ctx context.Context, studentID int64, statuses []string, limit int) ([]*usersModels.StudentDataChangeRequest, error)
	ListParentVisibleByStudent(ctx context.Context, studentID int64, limit int) ([]*usersModels.StudentDataChangeRequest, error)
}

// StatusDayReads reads the child's active status days.
type StatusDayReads interface {
	FindActiveByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*absencerecords.StudentStatusDay, error)
}

// CareOfferingReads reads care offerings by ID.
type CareOfferingReads interface {
	ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error)
}

// CarePeriod is one approved enrollment that created the child, together
// with the care window of its phase. The offering view needs the window to
// decide which care period is the current one for a child that is enrolled
// across several school years (#1665).
type CarePeriod struct {
	RequestChildID   int64
	RequestID        int64
	PhaseID          int64
	PhaseName        string
	ServiceStartDate timezone.Date
	ServiceEndDate   timezone.Date
}

// CarePeriodReads lists the child's care periods from Enrollment, latest
// window first.
type CarePeriodReads interface {
	CarePeriods(ctx context.Context, studentID int64) ([]*CarePeriod, error)
}

// OfferingBooking is one care-offering selection of an enrollment. ValidUntil
// is exclusive, matching student enrollments.
type OfferingBooking struct {
	CareOfferingID int64
	SelectedDays   []string
	ValidFrom      *timezone.Date
	ValidUntil     *timezone.Date
}

// OfferingHistoryReads lists every offering selection one enrollment child
// ever had, from Enrollment.
type OfferingHistoryReads interface {
	OfferingHistory(ctx context.Context, requestChildID int64) ([]*OfferingBooking, error)
}
