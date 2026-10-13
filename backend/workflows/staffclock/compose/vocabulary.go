// Package compose names the Staff Clock workflow for the compositions that
// bind it (#2747): its service and the ports and values the root implements
// over the identity, work-session and card owners.
package compose

import "github.com/moto-nrw/project-phoenix/services/iot/staffclock"

type (
	Service                      = staffclock.Service
	Dependencies                 = staffclock.Dependencies
	StaffLookup                  = staffclock.StaffLookup
	TimeClock                    = staffclock.TimeClock
	StaffIdentity                = staffclock.StaffIdentity
	Card                         = staffclock.Card
	Session                      = staffclock.Session
	Break                        = staffclock.Break
	Workday                      = staffclock.Workday
	Stamp                        = staffclock.Stamp
	StampError                   = staffclock.StampError
	PlannedStartNotReachedError  = staffclock.PlannedStartNotReachedError
	DeviationReasonRequiredError = staffclock.DeviationReasonRequiredError
)

var (
	ErrInvalidRFIDTag  = staffclock.ErrInvalidRFIDTag
	ErrRFIDTagNotFound = staffclock.ErrRFIDTagNotFound
	ErrRFIDTagNotStaff = staffclock.ErrRFIDTagNotStaff
	ErrCheckInRaced    = staffclock.ErrCheckInRaced
	ErrStateConflict   = staffclock.ErrStateConflict
	ErrInvalidStamp    = staffclock.ErrInvalidStamp
)

// NewService builds the Staff Clock service over its dependencies.
func NewService(dependencies Dependencies) *Service {
	return staffclock.NewService(dependencies)
}
