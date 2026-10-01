package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// BirthdayStore is the read and opt-out surface of the birthday display over
// the person, student and staff rows.
type BirthdayStore interface {
	StudentBirthdaysOn(ctx context.Context, days []domain.MonthDay) ([]domain.BirthdayEntry, error)
	StaffBirthdaysOn(ctx context.Context, days []domain.MonthDay) ([]domain.BirthdayEntry, error)
	StaffBirthdaysForExport(ctx context.Context) ([]domain.BirthdayEntry, error)
	FindPersonByAccount(ctx context.Context, accountID int64) (personID int64, found bool, err error)
	FindStaffByPerson(ctx context.Context, personID int64) (domain.BirthdayStaff, bool, error)
	SetStaffBirthdayOptOut(ctx context.Context, staffID int64, optOut bool) error
}

// BirthdaySettings resolves the two tenant settings that govern the display.
type BirthdaySettings interface {
	BirthdayDisplayEnabled(ctx context.Context) (bool, error)
	BirthdayDisplayIncludesStaff(ctx context.Context) (bool, error)
}
