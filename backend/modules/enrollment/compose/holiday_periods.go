package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	applicationNative "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewHolidayPeriods builds the holiday periods of a Träger (#4659). They run
// in the Träger transaction the Träger-Portal route opens (ADR 0105).
func NewHolidayPeriods() enrollment.HolidayPeriods {
	return applicationNative.NewHolidayPeriods(postgres.NewHolidayPeriodStore())
}

// NewHolidayRegistrations builds the public holiday registration (#4661):
// the link resolves in the administrative transaction, the rest runs in the
// Träger transaction the public route opens for the link's Träger (ADR 0110).
func NewHolidayRegistrations(traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox, logger *slog.Logger) enrollment.HolidayRegistrations {
	return applicationNative.NewHolidayRegistrations(applicationNative.HolidayRegistrationDependencies{
		Registrations: postgres.NewHolidayRegistrationStore(), Periods: postgres.NewHolidayPeriodStore(),
		Traeger: traeger, Mail: mail, Logger: logger,
	})
}

// NewHolidayRegistrationRetention builds the hourly deletion of ended holiday
// registrations (#4661, spec #4611 user story 121).
func NewHolidayRegistrationRetention() enrollment.HolidayRegistrationRetention {
	return applicationNative.NewHolidayRegistrationRetention(postgres.NewHolidayRegistrationStore(), nil)
}

// NewHolidayRegistrationReview builds the review of the holiday registrations
// by the Träger-Büro (#4663). It runs in the Träger transaction the
// Träger-Portal route opens (ADR 0114).
func NewHolidayRegistrationReview() enrollment.HolidayRegistrationReview {
	return applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), nil)
}

// NewHolidayHostParticipants builds the hosting school's participant lists
// (#4665, ADR 0116); a nil today uses the Berlin calendar day.
func NewHolidayHostParticipants(log enrollment.HolidayParticipantAccessLog, today func() calendar.Date) enrollment.HolidayHostParticipants {
	return applicationNative.NewHolidayHostParticipants(postgres.NewHolidayHostStore(), log, today)
}
