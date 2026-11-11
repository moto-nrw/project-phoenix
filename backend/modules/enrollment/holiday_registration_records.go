package enrollment

import "context"

// The file of a holiday registration (#6691): the Träger-Büro downloads a
// registration as a PDF, one or all of a period, to keep it beyond the
// retention. A record is everything the family entered, the booked weeks,
// the state and the declarations word for word with name and time. It names
// the health information only when the caller asks for it; the route admits
// that only with the right traeger_health:read, as on the screen.

// HolidayRegistrationRecord is one registration as its file names it: the
// detail with contacts, answers, criteria and consent texts, without the
// history and the proofs. Health is nil unless the health information was
// asked for.
type HolidayRegistrationRecord struct {
	Detail HolidayRegistrationDetail
	Health *HolidayRegistrationHealthDetail
}

// HolidayRegistrationRecordSet are the records of one period.
type HolidayRegistrationRecordSet struct {
	Period  HolidayPeriod
	Records []HolidayRegistrationRecord
}

// HolidayRegistrationRecords reads the registrations of a period for their
// files, in the Träger transaction of the reviewer's Träger. Every read
// records itself in the audit trail: one registration as a read of it, a
// period as an export, and with health information a health read of every
// registration named. A failed audit entry fails the read. The statement
// count does not grow with the registrations.
type HolidayRegistrationRecords interface {
	// HolidayRegistrationRecord reads one registration of the period;
	// ErrHolidayRegistrationEntryNotFound outside it.
	HolidayRegistrationRecord(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, withHealth bool) (HolidayRegistrationRecordSet, error)
	// HolidayRegistrationRecords reads the registrations of the filter, in
	// the order of the list.
	HolidayRegistrationRecords(ctx context.Context, reviewer HolidayReviewer, periodID int64, filter HolidayRegistrationFilter, withHealth bool) (HolidayRegistrationRecordSet, error)
}
