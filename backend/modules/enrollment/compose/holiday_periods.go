package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	applicationNative "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewHolidayPeriods builds the holiday periods of a Träger (#4659) in the
// Träger transaction the Träger-Portal route opens (ADR 0105).
func NewHolidayPeriods() enrollment.HolidayPeriods {
	return applicationNative.NewHolidayPeriods(postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore())
}

// NewHolidayRegistrations builds the public holiday registration (ADR 0110, #4722).
func NewHolidayRegistrations(traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox, health enrollment.HolidayHealthCipher, random func([]byte) error, fingerprint func([]byte) string, logger *slog.Logger) enrollment.HolidayRegistrations {
	return applicationNative.NewHolidayRegistrations(applicationNative.HolidayRegistrationDependencies{
		Registrations: postgres.NewHolidayRegistrationStore(), Periods: postgres.NewHolidayPeriodStore(), Forms: postgres.NewHolidayPeriodStore(),
		Proofs: postgres.NewHolidayRegistrationStore(), Traeger: traeger, Mail: mail, Health: health, Random: random, Fingerprint: fingerprint, Logger: logger,
	})
}

// NewHolidayRegistrationRetention deletes ended holiday registrations hourly
// (#4661, #4667); a nil today uses the Berlin calendar day.
func NewHolidayRegistrationRetention(today func() calendar.Date) enrollment.HolidayRegistrationRetention {
	return applicationNative.NewHolidayRegistrationRetention(postgres.NewHolidayRegistrationStore(), today)
}

// NewHolidayRegistrationReview builds the Träger-Büro's review (ADR 0114, #4722).
func NewHolidayRegistrationReview(health enrollment.HolidayHealthCipher) enrollment.HolidayRegistrationReview {
	return applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), health, nil)
}

// NewHolidayHostParticipants builds the hosting school's participant lists (ADR 0116).
func NewHolidayHostParticipants(log enrollment.HolidayParticipantAccessLog, health enrollment.HolidayHealthCipher, students enrollment.HolidayStudentIdentities, today func() calendar.Date) enrollment.HolidayHostParticipants {
	return applicationNative.NewHolidayHostParticipants(postgres.NewHolidayHostStore(), log, health, students, today)
}

// NewHolidayGuests builds the hosting school's holiday guests (ADR 0120, #4722).
func NewHolidayGuests(people enrollment.HolidayGuestPeople, care enrollment.HolidayGuestCareDays, health enrollment.HolidayHealthCipher, log enrollment.HolidayGuestAccessLog, today func() calendar.Date) enrollment.HolidayGuests {
	return applicationNative.NewHolidayGuests(postgres.NewHolidayGuestStore(), people, care, health, log, today)
}

// NewHolidayAccessTrail builds the holiday trail of the Träger-Portal's access
// log (#5326), read in the Träger projection's transaction (ADR 0137).
func NewHolidayAccessTrail() enrollment.HolidayAccessTrail {
	return postgres.NewHolidayRegistrationStore()
}
