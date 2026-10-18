package care

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// These consumer ports retain only the Care Plan operations used by the workflow.
type ArrivalSchedules interface {
	GetStudentArrivalExceptionForDate(ctx context.Context, id int64, date calendar.Date) (*ArrivalException, error)
	GetStudentArrivalScheduleForWeekday(ctx context.Context, id int64, weekday int) (*ArrivalSchedule, error)
	GetStudentArrivalSchedules(ctx context.Context, id int64) ([]*ArrivalSchedule, error)
}
type PickupSchedules interface {
	GetEffectivePickupTimeForDate(ctx context.Context, id int64, date calendar.Date) (*EffectivePickupTime, error)
	GetStudentPickupSchedules(ctx context.Context, id int64) ([]*PickupSchedule, error)
}
type GuardianAbsenceReports interface {
	ReportGuardianAbsence(ctx context.Context, report GuardianAbsenceReport) ([]StudentStatusDay, error)
}
type GuardianPickupExceptions interface {
	ApplyGuardianPickupException(ctx context.Context, change GuardianPickupChange) error
	WithdrawGuardianPickupException(ctx context.Context, change PickupException) error
}
type ExcusedAbsenceRequests interface {
	EditRequest(ctx context.Context, input ExcusedRequestEditInput) (*ExcusedAbsenceRequest, error)
	ListForStudent(ctx context.Context, id int64, since time.Time) ([]*ExcusedAbsenceRequest, error)
	Submit(ctx context.Context, input ExcusedRequestCreateInput) (*ExcusedAbsenceRequest, error)
}
type CareExceptions interface {
	LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error
	ListPickupExceptions(ctx context.Context, filter StudentScheduleFilter) ([]PickupException, error)
	ListArrivalExceptions(ctx context.Context, filter StudentScheduleFilter) ([]ArrivalException, error)
	ListDepartureExceptions(ctx context.Context, filter StudentScheduleFilter) ([]DepartureException, error)
}
type OfferingChangeRequests interface {
	CatalogAt(ctx context.Context, id int64, date calendar.Date) (*OfferingChangeCatalog, error)
	EarliestEffectiveFrom(ctx context.Context) (calendar.Date, error)
	Edit(ctx context.Context, id int64, input CreateOfferingChangeInput, version string) (*OfferingChangeRequest, error)
	GetForStudent(ctx context.Context, id int64) (*OfferingChangeView, error)
	SubmitOfferingChange(ctx context.Context, input CreateOfferingChangeInput) (*OfferingChangeRequest, error)
	CourseCatalog(ctx context.Context, studentID int64, accountID int64) (*CourseCatalog, error)
	CreateCourseRequest(ctx context.Context, input CreateCourseRequestInput) (*OfferingChangeRequest, error)
	WithdrawCourseRequest(ctx context.Context, studentID int64, accountID int64, offeringID int64) error
}
