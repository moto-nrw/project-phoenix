package enrollment

import (
	"context"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The hint on a known child in the review of the holiday registrations
// (#6683, ADR 0214): the Träger-Büro sees that a registered child is known at
// a school of the Träger. moto links nothing and opens no data of the
// school: the hint names the school and nothing else. The public form still
// never looks a child up (ADR 0110 point 4).

// HolidayAuditKnownChildren records that the Träger-Büro compared the
// children of registrations with the children of its schools. The entry
// names the registration asked for, if one was, and the children found,
// never their names.
const HolidayAuditKnownChildren = "known_children"

// HolidayRegisteredChild is a child of a registration that is not cancelled,
// as the parents named it.
type HolidayRegisteredChild struct {
	RegistrationID int64
	ChildID        int64
	FirstName      string
	LastName       string
	BirthDate      calendar.Date
}

// HolidayRegisteredChildren reads the children of the registrations of one
// period of a Träger that are not cancelled, or of the one registrationID
// names, for the comparison with the children of its schools. It joins the
// Träger projection's administrative transaction and never opens one; every
// statement filters on the Träger it is given.
type HolidayRegisteredChildren interface {
	HolidayRegisteredChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]HolidayRegisteredChild, error)
}

// HolidayKnownChild is a registered child with the one school of the Träger
// that knows it.
type HolidayKnownChild struct {
	RegistrationID int64 `json:"registration_id"`
	ChildID        int64 `json:"child_id"`
	SchoolID       int64 `json:"school_id"`
}

// HolidayKnownChildren finds the known children of a period of the Träger,
// or of the one registration registrationID names. The root binds it to the
// Träger projection, which reads in its administrative transaction before
// the route opens the Träger transaction.
type HolidayKnownChildren interface {
	KnownHolidayChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]HolidayKnownChild, error)
}

// HolidayKnownChildTrail records the comparison in the trail of the period
// and answers its result. It runs in the Träger transaction of the
// reviewer's Träger; a period of another Träger is ErrHolidayPeriodNotFound.
type HolidayKnownChildTrail interface {
	RecordKnownHolidayChildren(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, known []HolidayKnownChild) ([]HolidayKnownChild, error)
}
