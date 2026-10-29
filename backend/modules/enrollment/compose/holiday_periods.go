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
	return applicationNative.NewHolidayPeriods(postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore())
}

// NewHolidayRegistrations builds the public holiday registration (#4661):
// the link resolves in the administrative transaction, the rest runs in the
// Träger transaction the public route opens for the link's Träger (ADR 0110).
func NewHolidayRegistrations(traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox, random func([]byte) error, fingerprint func([]byte) string, logger *slog.Logger) enrollment.HolidayRegistrations {
	return applicationNative.NewHolidayRegistrations(applicationNative.HolidayRegistrationDependencies{
		Registrations: postgres.NewHolidayRegistrationStore(), Periods: postgres.NewHolidayPeriodStore(), Forms: postgres.NewHolidayPeriodStore(),
		Proofs: postgres.NewHolidayRegistrationStore(), Traeger: traeger, Mail: mail, Random: random, Fingerprint: fingerprint, Logger: logger,
	})
}

// NewHolidayRegistrationRetention builds the hourly deletion of ended holiday
// registrations (#4661, #4667); a nil today uses the Berlin calendar day.
func NewHolidayRegistrationRetention(today func() calendar.Date) enrollment.HolidayRegistrationRetention {
	return applicationNative.NewHolidayRegistrationRetention(postgres.NewHolidayRegistrationStore(), today)
}

// NewHolidayRegistrationReview builds the review of the holiday registrations
// by the Träger-Büro (#4663). It runs in the Träger transaction the
// Träger-Portal route opens (ADR 0114).
func NewHolidayRegistrationReview() enrollment.HolidayRegistrationReview {
	return applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), nil)
}

// NewHolidayHostParticipants builds the hosting school's participant lists
// (#4665, ADR 0116); a nil today uses the Berlin calendar day.
func NewHolidayHostParticipants(log enrollment.HolidayParticipantAccessLog, today func() calendar.Date) enrollment.HolidayHostParticipants {
	return applicationNative.NewHolidayHostParticipants(postgres.NewHolidayHostStore(), log, today)
}

// NewHolidayGuests builds the hosting school's holiday guests (#4666,
// ADR 0120); a nil today uses the Berlin calendar day.
func NewHolidayGuests(people enrollment.HolidayGuestPeople, care enrollment.HolidayGuestCareDays, log enrollment.HolidayGuestAccessLog, today func() calendar.Date) enrollment.HolidayGuests {
	return applicationNative.NewHolidayGuests(postgres.NewHolidayGuestStore(), people, care, log, today)
}

// NewHolidayAccessTrail builds the access trail of the holiday registrations
// for the access log of the Träger-Portal (#5326). It reads in the
// administrative transaction of the Träger projection (ADR 0137).
func NewHolidayAccessTrail() enrollment.HolidayAccessTrail {
	return postgres.NewHolidayRegistrationStore()
}
