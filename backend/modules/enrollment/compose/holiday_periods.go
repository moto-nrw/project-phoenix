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

// NewHolidayWeekHosts builds the read of the host school of each holiday week for the Elternabrechnung (#6624).
func NewHolidayWeekHosts() enrollment.HolidayWeekHosts { return postgres.NewHolidayPeriodStore() }

// NewHolidayRegistrations builds the public holiday registration (ADR 0110, #4722); schoolCalendar (#4702) may be nil only for links and quotes; wishModes (#4703); retention (#6691); rateLimitRefusals reports refusals to the monitoring (#6726), nil reports nowhere.
func NewHolidayRegistrations(traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailOutbox, health enrollment.HolidayHealthCipher, schoolCalendar enrollment.HolidaySchoolCalendar, wishModes enrollment.HolidayWishModes, retention enrollment.HolidayRetentionSettings, random func([]byte) error, fingerprint func([]byte) string, logger *slog.Logger, rateLimitRefusals func(bucket string)) enrollment.HolidayRegistrations {
	return applicationNative.NewHolidayRegistrations(applicationNative.HolidayRegistrationDependencies{
		Registrations: postgres.NewHolidayRegistrationStore(), Periods: postgres.NewHolidayPeriodStore(), Forms: postgres.NewHolidayPeriodStore(),
		Proofs: postgres.NewHolidayRegistrationStore(), Places: postgres.NewHolidayRegistrationStore(), Criteria: postgres.NewHolidayRegistrationStore(), Traeger: traeger, Mail: mail, Health: health, Calendar: schoolCalendar, WishModes: wishModes, Retention: retention, Random: random, Fingerprint: fingerprint, Logger: logger, RateLimitRefusals: rateLimitRefusals,
	})
}

// NewHolidayRegistrationAnnouncements builds the read of the published periods the parents app announces (#6671, ADR 0207), in the administrative transaction.
func NewHolidayRegistrationAnnouncements() enrollment.HolidayRegistrationAnnouncements {
	return postgres.NewHolidayRegistrationStore()
}

// NewHolidayWishes builds the prefill's sealed statements of the holiday care wish (school tenant transaction) and the office's decision on it (#4703).
func NewHolidayWishes(health enrollment.HolidayHealthCipher) (enrollment.HolidayWishProofs, enrollment.HolidayWishReview) {
	review, _, _, _ := applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), health, nil, nil, nil, nil)
	return applicationNative.NewHolidayWishProofs(newStore(), health, nil), applicationNative.NewHolidayWishReview(review)
}

// NewHolidayRegistrationRetention deletes ended holiday registrations hourly (#4661, #4667); nil today: Berlin day.
func NewHolidayRegistrationRetention(mail enrollment.HolidayRegistrationMailErasure, today func() calendar.Date) enrollment.HolidayRegistrationRetention {
	return applicationNative.NewHolidayRegistrationRetention(postgres.NewHolidayRegistrationStore(), mail, today)
}

// NewHolidayRegistrationReview builds the Träger-Büro's review (ADR 0114, #4722), places (#4699), allocation by criteria (#4700) and the reads for the files (#6691); nil mail moves nobody up and decides nothing; traeger names the Träger and the host schools in those mails (#6437); schoolCalendar (#4702) may be nil without rebooking.
func NewHolidayRegistrationReview(health enrollment.HolidayHealthCipher, mail enrollment.TraegerMailOutbox, traeger enrollment.HolidayTraegerDirectory, schoolCalendar enrollment.HolidaySchoolCalendar) (enrollment.HolidayRegistrationReview, enrollment.HolidayPlaces, enrollment.HolidayAllocation, enrollment.HolidayRegistrationRecords) {
	return applicationNative.NewHolidayRegistrationReview(postgres.NewHolidayRegistrationStore(), postgres.NewHolidayPeriodStore(), postgres.NewHolidayPeriodStore(), health, nil, mail, traeger, schoolCalendar)
}

// NewHolidayHealthExportFields opens the sealed health information for the return export (#3838, #6328); only the administrative export CLI binds it, never HTTP.
func NewHolidayHealthExportFields(health enrollment.HolidayHealthCipher) enrollment.HolidayHealthExportFields {
	return applicationNative.NewHolidayHealthExportFields(health)
}

// NewHolidayHostParticipants builds the hosting school's participant lists (ADR 0116).
func NewHolidayHostParticipants(log enrollment.HolidayParticipantAccessLog, health enrollment.HolidayHealthCipher, students enrollment.HolidayStudentIdentities, today func() calendar.Date) enrollment.HolidayHostParticipants {
	return applicationNative.NewHolidayHostParticipants(postgres.NewHolidayHostStore(), log, health, students, today)
}

// NewHolidayGuests builds the hosting school's holiday guests (ADR 0120, #4722, #5630) and the planned days of its own children (#6678).
func NewHolidayGuests(people enrollment.HolidayGuestPeople, schools enrollment.HolidayHostSchoolNames, care enrollment.HolidayGuestCareDays, children enrollment.HolidayStudentsByBirthDate, ownPlans enrollment.HolidayOwnChildPlans, health enrollment.HolidayHealthCipher, log enrollment.HolidayGuestAccessLog, today func() calendar.Date) enrollment.HolidayGuests {
	return applicationNative.NewHolidayGuests(postgres.NewHolidayGuestStore(), people, schools, care, children, ownPlans, health, log, today)
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

// NewHolidayClosures builds the notices of closing days in booked holiday weeks (#6434) in the Träger transaction; nil today: Berlin day.
func NewHolidayClosures(schoolCalendar enrollment.HolidaySchoolCalendar, operationCalendar enrollment.HolidayClosureCalendar, traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailDelivery, random func([]byte) error, keyed func(key, content []byte) string, today func() calendar.Date) enrollment.HolidayClosures {
	return applicationNative.NewHolidayClosures(postgres.NewHolidayRegistrationStore(), schoolCalendar, operationCalendar, traeger, mail, random, keyed, today)
}
