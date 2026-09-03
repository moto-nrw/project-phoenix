package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// Store is the persistence port over users.staff, users.teachers,
// users.guests and users.class_list_entries. Reads honour the tenant in
// context when one is present.
type Store interface {
	FindStaff(ctx context.Context, id int64, lock string, includeDeleted bool) (domain.Staff, bool, domain.OperationStats, error)
	FindStaffByPerson(context.Context, int64) (domain.Staff, bool, domain.OperationStats, error)
	ListStaff(context.Context, domain.StaffFilter) ([]domain.Staff, domain.OperationStats, error)
	CreateStaff(context.Context, domain.StaffFields) (domain.Staff, domain.OperationStats, error)
	UpdateStaff(context.Context, int64, domain.StaffFields) (domain.Staff, domain.OperationStats, error)
	SoftDeleteStaff(context.Context, int64) (domain.OperationStats, error)
	ClearWorkTimeModel(context.Context, int64) (domain.OperationStats, error)
	SetStaffNotes(context.Context, int64, string) (domain.OperationStats, error)
	SetBirthdayDisplayOptOut(context.Context, int64, bool) (domain.OperationStats, error)
	RebaseWorkTimeModelAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, domain.OperationStats, error)

	FindTeacher(ctx context.Context, id int64, lock string) (domain.Teacher, bool, domain.OperationStats, error)
	FindTeacherByStaff(context.Context, int64) (domain.Teacher, bool, domain.OperationStats, error)
	ListTeachers(context.Context, domain.TeacherFilter) ([]domain.Teacher, domain.OperationStats, error)
	CreateTeacher(context.Context, domain.TeacherFields) (domain.Teacher, domain.OperationStats, error)
	UpdateTeacher(context.Context, int64, domain.TeacherFields) (domain.Teacher, domain.OperationStats, error)
	SoftDeleteTeacher(context.Context, int64) (domain.OperationStats, error)

	FindGuest(ctx context.Context, id int64, lock string) (domain.Guest, bool, domain.OperationStats, error)
	FindGuestByStaff(context.Context, int64) (domain.Guest, bool, domain.OperationStats, error)
	ListGuests(context.Context, domain.GuestFilter) ([]domain.Guest, domain.OperationStats, error)
	CreateGuest(context.Context, domain.GuestFields) (domain.Guest, domain.OperationStats, error)
	UpdateGuest(context.Context, int64, domain.GuestFields) (domain.Guest, domain.OperationStats, error)
	DeleteGuest(context.Context, int64) (domain.OperationStats, error)

	FindClassListEntry(ctx context.Context, id int64, lock string) (domain.ClassListEntry, bool, domain.OperationStats, error)
	ListClassListEntries(context.Context, domain.ClassListEntryFilter) ([]domain.ClassListEntry, domain.OperationStats, error)
	CreateClassListEntry(ctx context.Context, fields domain.ClassListEntryFields, createdBy *int64) (domain.ClassListEntry, domain.OperationStats, error)
	UpdateClassListEntry(context.Context, int64, domain.ClassListEntryFields) (domain.ClassListEntry, domain.OperationStats, error)
	DeleteClassListEntry(context.Context, int64) (domain.OperationStats, error)
}

type Transaction interface {
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(context.Context, func(context.Context) error) error
	// RunRead joins the caller's transaction, else opens a tenant
	// transaction, else an admin transaction for cross-tenant readers.
	RunRead(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
