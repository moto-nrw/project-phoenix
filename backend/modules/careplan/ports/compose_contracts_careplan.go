package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// ArrivalScheduleRecords is the owner's storage capability used by arrival commands.
type ArrivalScheduleRecords interface {
	FindArrivalSchedule(context.Context, int64) (careplan.ArrivalSchedule, error)
	ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error)
	CreateArrivalSchedule(context.Context, careplan.ArrivalSchedule) (careplan.ArrivalSchedule, error)
	DeleteArrivalSchedule(context.Context, int64) error
	DeleteArrivalSchedulesByStudent(context.Context, int64) error
	UpsertArrivalSchedule(context.Context, careplan.ArrivalSchedule) (careplan.ArrivalSchedule, error)
	FindArrivalException(context.Context, int64, bool) (careplan.ArrivalException, error)
	ListArrivalExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalException, error)
	CreateArrivalException(context.Context, careplan.ArrivalException) (careplan.ArrivalException, error)
	DeleteArrivalException(context.Context, int64) error
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	UpdateArrivalException(context.Context, careplan.ArrivalException) error
	FindArrivalNote(context.Context, int64) (careplan.ArrivalNote, error)
	ListArrivalNotes(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalNote, error)
	CreateArrivalNote(context.Context, careplan.ArrivalNote) (careplan.ArrivalNote, error)
	DeleteArrivalNote(context.Context, int64) error
	DeleteArrivalNotesByStudent(context.Context, int64) error
	UpdateArrivalNote(context.Context, careplan.ArrivalNote) error
}

type PickupExceptionReader interface {
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
}

// PickupScheduleRecords is the owner's storage capability used by pickup commands.
type PickupScheduleRecords interface {
	FindPickupSchedule(context.Context, int64) (careplan.PickupSchedule, error)
	ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error)
	CreatePickupSchedule(context.Context, careplan.PickupSchedule) (careplan.PickupSchedule, error)
	DeletePickupSchedule(context.Context, int64) error
	DeletePickupSchedulesByStudent(context.Context, int64) error
	UpsertPickupSchedule(context.Context, careplan.PickupSchedule) (careplan.PickupSchedule, error)
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	DeletePickupException(context.Context, int64) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
	UpdatePickupException(context.Context, careplan.PickupException) error
	FindPickupNote(context.Context, int64) (careplan.PickupNote, error)
	ListPickupNotes(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupNote, error)
	CreatePickupNote(context.Context, careplan.PickupNote) (careplan.PickupNote, error)
	DeletePickupNote(context.Context, int64) error
	DeletePickupNotesByStudent(context.Context, int64) error
	UpdatePickupNote(context.Context, careplan.PickupNote) error
	careplan.DepartureExceptions
}

// ParentChildLink is one child a guardian account reaches, with the school it
// attends, as the parent calendar reads it.
type ParentChildLink struct {
	StudentID         int64
	GuardianProfileID int64
	TenantID          int64
	SchoolName        string
}

// ParentPortalSchool is the Organisation & Tenancy school fact the guardian
// portal shows and filters by.
type ParentPortalSchool struct {
	ID        int64
	Name      string
	Slug      string
	Subdomain string
	Active    bool
	Hidden    bool
	Deleted   bool
}

// ParentPortalSchools resolves the schools with the given IDs; an unknown
// school is absent.
type ParentPortalSchools interface {
	ListParentPortalSchools(ctx context.Context, ids []int64) ([]ParentPortalSchool, error)
}
