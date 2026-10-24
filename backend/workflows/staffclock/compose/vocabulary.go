// Package compose names the Staff Clock workflow for the compositions that
// bind it (#2747): its service and the ports and values the root implements
// over the identity, work-session and card owners.
package compose

import (
	staffclock "github.com/moto-nrw/project-phoenix/workflows/staffclock"
	clockApplication "github.com/moto-nrw/project-phoenix/workflows/staffclock/internal/application"
)

type (
	Service       = clockApplication.Service
	Dependencies  = staffclock.Dependencies
	StaffLookup   = staffclock.StaffLookup
	TimeClock     = staffclock.TimeClock
	StaffIdentity = staffclock.StaffIdentity
	Card          = staffclock.Card
	Session       = staffclock.Session
	Break         = staffclock.Break
	Workday       = staffclock.Workday
	Stamp         = staffclock.Stamp
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
	return clockApplication.NewService(dependencies)
}
