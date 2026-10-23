package compose

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	internalports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// The birthday display reads persons, students and staff through a store the
// composition root binds over the retained repositories, and two tenant
// settings it binds over the Settings Platform.
type (
	BirthdayEntry    = domain.BirthdayEntry
	BirthdayMonthDay = domain.MonthDay
	BirthdayStaff    = domain.BirthdayStaff
)

const BirthdayKindStaff = domain.BirthdayKindStaff

// BirthdayDependencies wires the birthday capability.
type BirthdayDependencies struct {
	Store    internalports.BirthdayStore
	Settings internalports.BirthdaySettings
	Logger   *slog.Logger
	// Now is injectable so tests can pin a weekday; nil means time.Now.
	Now func() time.Time
}

// NewBirthdays returns the birthday display capability.
func NewBirthdays(deps BirthdayDependencies) peopledirectory.Birthdays {
	return application.NewBirthdayView(application.NewBirthdayService(deps.Store, deps.Settings, deps.Logger, deps.Now))
}
