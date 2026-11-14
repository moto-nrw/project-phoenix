package carecontract

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Amendment of a contract (#4681, ADR 0088 section 2). The parents request a
// change of their child's care (other days or times) with the existing
// change request of the parents portal, and the school approves it there;
// the approval changes the bookings from the requested date. An active
// contract follows: it gets a new version with that date as its effective
// date and the booked care from then on. Earlier versions stay as they were
// and remain readable.
//
// Invariants (each has a test):
//   - The effective versions of a contract form one timeline without
//     overlap: a version holds from its effective date to the day before the
//     next one, or to its own end. VersionsInForce therefore answers at most
//     one version per contract and day.
//   - A change with effective date D makes a version from D. A version
//     already planned from E > D stays; the new one holds from D to E-1, and
//     the planned version does not take the change over automatically.
//   - A new version never starts before the day it is made (Berlin date),
//     so no change reaches back into a month that may be billed already.
//     The database refuses it, and an overlap.
//   - A module never runs longer in a version than it is booked: a service
//     carries the last booked day of a module that ends before the version
//     does, and ServicesOn drops it after that day.
//   - A run changes nothing when the bookings still match the version in
//     force, so repeated runs create no second version.

// The sources of a version.
const (
	// SourceConclusion is the first version of a contract.
	SourceConclusion = "abschluss"
	// SourceBookingChange is a later version made from an approved change of
	// the booked care.
	SourceBookingChange = "buchungsaenderung"
	// SourceImport is the first version of a contract taken over from an
	// import file (#4723).
	SourceImport = "import"
)

// The reasons an amendment run leaves a contract unchanged although its
// bookings changed.
const (
	// SkipCareEnds: the child books no care from the date on. That is an
	// end of the contract (#4682), not a new version.
	SkipCareEnds = "care_ends"
	// SkipApplicationGone: the application child is no longer accepted.
	SkipApplicationGone = "application_gone"
)

var ErrVersionNotFound = errors.New("care contract version not found")

// AmendmentRun answers one run over the active contracts of a Träger: the
// new versions and, per contract left unchanged although its bookings
// changed, why.
type AmendmentRun struct {
	Amended []AmendedContract `json:"amended"`
	Skipped []AmendmentSkip   `json:"skipped"`
}

type AmendedContract struct {
	ContractID    int64         `json:"contract_id"`
	VersionNumber int           `json:"version_number"`
	EffectiveFrom calendar.Date `json:"effective_from"`
}

type AmendmentSkip struct {
	ContractID int64  `json:"contract_id"`
	Reason     string `json:"reason"`
}

// Conclusion.AmendFromBookings makes the new versions. It reads each school
// in its own tenant transaction and writes each contract in its own Träger
// transaction, so it must be called outside any transaction. It acts for no
// account: the school approved the change, and the trail records the new
// version.

// VersionPeriod is the version of one contract in force over a span of
// days: From and Until are inclusive and lie within the span asked for.
type VersionPeriod struct {
	ContractID int64         `json:"contract_id"`
	SchoolID   int64         `json:"school_id"`
	StudentID  *int64        `json:"student_id"`
	From       calendar.Date `json:"from"`
	Until      calendar.Date `json:"until"`
	Version    Version       `json:"version"`
}

// ContractVersions reads the versions in force for the parents' billing
// (#4684). It runs in the Träger transaction the caller holds.
type ContractVersions interface {
	// VersionsInForce answers, for every active, terminated or ended
	// contract of the Träger (or only contractIDs), each version in force on
	// a day between from and to (both inclusive), ordered by contract and
	// day. A contract appears once per version, never twice for one day. A
	// terminated contract holds until its last day; a withdrawn contract on
	// no day (#4682).
	VersionsInForce(ctx context.Context, organizationID int64, from, to calendar.Date, contractIDs ...int64) ([]VersionPeriod, error)
}

// ServicesOn are the services of the version on one day: the modules booked
// on its weekday, without a module whose last booked day has passed. It
// does not check that the day lies in the version; VersionsInForce does.
func (v Version) ServicesOn(day calendar.Date) []Service {
	weekday := strings.ToLower(day.Weekday().String()[:3])
	services := []Service{}
	for _, service := range v.Services {
		if service.Weekday == weekday && (service.LastDay == nil || !service.LastDay.Before(day)) {
			services = append(services, service)
		}
	}
	return services
}
