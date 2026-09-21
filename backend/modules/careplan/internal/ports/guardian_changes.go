package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// GuardianAbsenceRecords is the owner persistence a guardian absence report
// needs. Every call joins the caller's tenant transaction.
type GuardianAbsenceRecords interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	ClearStudentStatusDays(context.Context, int64, string, []careplan.Date, time.Time, string) error
	UpsertStudentStatusDay(context.Context, careplan.StudentStatusDay) (careplan.StudentStatusDay, error)
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
}

// GuardianPickupRecords is the owner persistence of the guardian pickup leg.
type GuardianPickupRecords interface {
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	UpdatePickupException(context.Context, careplan.PickupException) error
	DeletePickupException(context.Context, int64) error
}

// GuardianPickupExcusal couples the guardian pickup leg with the derived
// block excusal. A nil value skips the coupling.
type GuardianPickupExcusal interface {
	Sync(context.Context, int64) (bool, error)
	ReleaseBeforeDelete(context.Context, *careplan.PickupException) error
}
