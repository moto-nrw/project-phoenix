package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	applicationNative "github.com/moto-nrw/project-phoenix/modules/enrollment/internal/application"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewHolidayPeriods builds a Träger's holiday periods (#4659) in the Träger transaction (ADR 0105); schoolCalendar (#4702) may be nil only for a reader.
func NewHolidayPeriods(today func() calendar.Date, schoolCalendar enrollment.HolidaySchoolCalendar) enrollment.HolidayPeriods {
	return applicationNative.NewHolidayPeriods(postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), today, schoolCalendar)
}

// NewHolidayRegistrations builds the public holiday registration (ADR 0110, #4722); schoolCalendar (#4702) may be nil only for links and quotes; wishModes (#4703).
func NewHolidayRegistrations(traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox, health enrollment.HolidayHealthCipher, schoolCalendar enrollment.HolidaySchoolCalendar, wishModes enrollment.HolidayWishModes, random func([]byte) error, fingerprint func([]byte) string, logger *slog.Logger) enrollment.HolidayRegistrations {
	return applicationNative.NewHolidayRegistrations(applicationNative.HolidayRegistrationDependencies{
		Registrations: postgres.NewHolidayRegistrationStore(), Periods: postgres.NewHolidayPeriodStore(), Forms: postgres.NewHolidayPeriodStore(),
		Proofs: postgres.NewHolidayRegistrationStore(), Places: postgres.NewHolidayRegistrationStore(), Traeger: traeger, Mail: mail, Health: health, Calendar: schoolCalendar, WishModes: wishModes, Random: random, Fingerprint: fingerprint, Logger: logger,
	})
}

// NewHolidayWishes builds the prefill's sealed statements of the holiday care wish (school tenant transaction) and the office's decision on it (#4703).
func NewHolidayWishes(health enrollment.HolidayHealthCipher) (enrollment.HolidayWishProofs, enrollment.HolidayWishReview) {
	review, _ := applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), health, nil, nil, nil)
	return applicationNative.NewHolidayWishProofs(newStore(), health, nil), applicationNative.NewHolidayWishReview(review)
}

// NewHolidayRegistrationRetention deletes ended holiday registrations hourly (#4661, #4667); nil today: Berlin day.
func NewHolidayRegistrationRetention(today func() calendar.Date) enrollment.HolidayRegistrationRetention {
	return applicationNative.NewHolidayRegistrationRetention(postgres.NewHolidayRegistrationStore(), today)
}

// NewHolidayRegistrationReview builds the Träger-Büro's review (ADR 0114, #4722) and places (#4699); nil mail moves nobody up; schoolCalendar (#4702) may be nil without rebooking.
func NewHolidayRegistrationReview(health enrollment.HolidayHealthCipher, mail enrollment.TraegerMailOutbox, schoolCalendar enrollment.HolidaySchoolCalendar) (enrollment.HolidayRegistrationReview, enrollment.HolidayPlaces) {
	return applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), health, nil, mail, schoolCalendar)
}

// NewHolidayHealthExportFields opens the sealed health information for the return export (#3838, #6328); only the administrative export CLI binds it, never HTTP.
func NewHolidayHealthExportFields(health enrollment.HolidayHealthCipher) enrollment.HolidayHealthExportFields {
	return applicationNative.NewHolidayHealthExportFields(health)
}

// NewHolidayHostParticipants builds the hosting school's participant lists (ADR 0116).
func NewHolidayHostParticipants(log enrollment.HolidayParticipantAccessLog, health enrollment.HolidayHealthCipher, students enrollment.HolidayStudentIdentities, today func() calendar.Date) enrollment.HolidayHostParticipants {
	return applicationNative.NewHolidayHostParticipants(postgres.NewHolidayHostStore(), log, health, students, today)
}

// NewHolidayGuests builds the hosting school's holiday guests (ADR 0120, #4722, #5630).
func NewHolidayGuests(people enrollment.HolidayGuestPeople, schools enrollment.HolidayHostSchoolNames, care enrollment.HolidayGuestCareDays, health enrollment.HolidayHealthCipher, log enrollment.HolidayGuestAccessLog, today func() calendar.Date) enrollment.HolidayGuests {
	return applicationNative.NewHolidayGuests(postgres.NewHolidayGuestStore(), people, schools, care, health, log, today)
}

// NewHolidayGuestBadgeList builds only the hints „Gast aus …“, for the tablet (PyrePortal#499); nil today: Berlin day.
func NewHolidayGuestBadgeList(today func() calendar.Date) enrollment.HolidayGuestBadgeList {
	return applicationNative.NewHolidayGuestBadgeList(postgres.NewHolidayGuestStore(), today)
}

// NewHolidayGuestBookedCare builds the read of the guests' booked days for Soll und Ist (#5994), in the hosting school's tenant transaction.
func NewHolidayGuestBookedCare(children enrollment.HolidayStudentsByBirthDate) enrollment.HolidayGuestBookedCare {
	return applicationNative.NewHolidayGuestBookedCare(postgres.NewHolidayGuestStore(), children)
}

// NewHolidayAccessTrail builds the Träger access log's holiday trail (#5326), read in the Träger projection's transaction (ADR 0137).
func NewHolidayAccessTrail() enrollment.HolidayAccessTrail {
	return postgres.NewHolidayRegistrationStore()
}

// NewOpenEnrollments counts the children waiting for a decision at each school, for the Träger dashboard (#4675).
func NewOpenEnrollments() enrollment.OpenEnrollments {
	return postgres.NewOpenEnrollmentStore()
}

// NewBookedChildren ties approved children of the online enrollment to their student, for the Träger's occupancy (#4676, ADR 0154).
func NewBookedChildren() enrollment.BookedChildren {
	return postgres.NewBookedChildStore()
}

// NewContractApplications reads the accepted application children of a school for the Träger's contracts (#4678, ADR 0153), in that school's tenant transaction.
func NewContractApplications(fingerprint func([]byte) string) enrollment.ContractApplications {
	return postgres.NewContractApplicationStore(fingerprint)
}
