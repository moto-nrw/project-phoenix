package compose

import (
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/uptrace/bun"
)

// StaffAdminDependencies are the collaborators the personnel-record
// administration cannot own: the staff and person rows behind a record, and the
// audit trail. Both are required, so an unaudited composition fails at startup
// instead of at the first change.
type StaffAdminDependencies struct {
	DB       *bun.DB
	Subjects workforce.StaffAdminSubjects
	Audit    workforce.StaffAdminAudit
	// Allows evaluates a required permission against a held set with the
	// shared wildcard rules; the root binds the security runtime's matcher.
	Allows  func(required string, held []string) bool
	Observe func(Observation)
	// Now is the clock access-log rows and cleanup delays are measured
	// against; nil means the wall clock. Tests pin it.
	Now    func() time.Time
	Logger *slog.Logger
}

// NewStaffAdmin composes the personnel-record administration over the
// Workforce-owned record rows.
func NewStaffAdmin(dependencies StaffAdminDependencies) (*workforce.StaffAdmin, error) {
	if dependencies.DB == nil || dependencies.Subjects == nil || dependencies.Audit == nil || dependencies.Allows == nil {
		return nil, errors.New("staff admin: database, staff subjects, audit trail and permission matcher are required")
	}
	store := postgres.New(postgres.DatabaseRuntime(dependencies.DB))
	now := dependencies.Now
	if now == nil {
		now = time.Now
	}
	logger := dependencies.Logger
	if logger == nil {
		logger = slog.Default()
	}
	observe := application.NewOptionalObserver(dependencies.Observe)
	unit := transaction{lock: store.AcquireXactLock}
	records := application.NewRecords(store, unit, application.Clock{Time: now}, observe)
	admin := application.NewStaffAdmin(
		records, unit, application.NewStaffSubjects(dependencies.Subjects), application.NewStaffAudit(dependencies.Audit),
		domain.DocumentAuthority{
			Health: permissions.StaffDocumentsHealth, Financial: permissions.StaffFinancial, General: permissions.StaffDocuments,
			Allows: dependencies.Allows,
		},
		application.Clock{Time: now}, logger.Warn,
	)
	return workforce.NewStaffAdmin(application.NewStaffAdminEngine(admin, records)), nil
}

// --- documents --------------------------------------------------------------
