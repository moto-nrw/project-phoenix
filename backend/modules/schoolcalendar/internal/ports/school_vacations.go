package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
)

// SchoolVacationStore is the persistence port over calendar.school_vacations
// (no tenant) and calendar.school_vacation_acknowledgements (one row per
// school and window). Every call runs in the transaction of its context.
type SchoolVacationStore interface {
	// ListSchoolVacations lists the region's windows touching [from, to]
	// ordered by start date, each marked when the school of the tenant
	// transaction confirmed it.
	ListSchoolVacations(ctx context.Context, region, from, to string) ([]schoolcalendar.SchoolVacation, error)
	// LastSchoolVacationSync is the newest sync time of the region's stored
	// windows; nil while none is stored.
	LastSchoolVacationSync(ctx context.Context, region string) (*time.Time, error)
	// HasSchoolVacation reports whether the region has exactly this window.
	HasSchoolVacation(ctx context.Context, region, start, end string) (bool, error)
	// AcknowledgeSchoolVacation records the confirmation of the school of
	// the tenant transaction; an existing confirmation stays unchanged.
	AcknowledgeSchoolVacation(ctx context.Context, region, start, end string, accountID *int64) error
	// ReplaceSchoolVacations replaces the region's windows touching
	// [from, to] with records and returns how many it stored.
	ReplaceSchoolVacations(ctx context.Context, region, from, to string, records []domain.SchoolVacationRecord) (int, error)
}

// SchoolVacationSource fetches the vacation windows of every federal state
// touching [from, to] from the external calendar.
type SchoolVacationSource func(ctx context.Context, from, to string) ([]domain.SchoolVacationRecord, error)

// AdminTransaction runs fn in the cross-tenant administrative transaction.
type AdminTransaction func(ctx context.Context, fn func(context.Context) error) error
