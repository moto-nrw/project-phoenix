package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupApprovalDirectory = ports.PickupApprovalDirectory
type PickupApprovalPresence = ports.PickupApprovalPresence
type PickupApprovalExceptions = ports.PickupApprovalExceptions
type PickupApprovalExcusal = ports.PickupApprovalExcusal
type PickupApprovalLocker = ports.PickupApprovalLocker

type PickupApprovalDependencies struct {
	People     PickupApprovalDirectory
	Presence   PickupApprovalPresence
	Exceptions PickupApprovalExceptions
	Excusal    PickupApprovalExcusal
	Locker     PickupApprovalLocker
	Today      func() calendar.Date
	// Fingerprint returns the lowercase hexadecimal SHA-256 of the content.
	Fingerprint func([]byte) string
}

func NewPickupApprovals(deps PickupApprovalDependencies) (carerequests.PickupApprovals, error) {
	if deps.People == nil || deps.Presence == nil || deps.Exceptions == nil || deps.Excusal == nil || deps.Locker == nil || deps.Today == nil || deps.Fingerprint == nil {
		return nil, errors.New("pickup approvals: people, presence, exceptions, excusal, care-day locker, clock, and impact fingerprint are required")
	}
	return &application.PickupApprovals{
		People: deps.People, Presence: deps.Presence, Exceptions: deps.Exceptions,
		Excusal: deps.Excusal, Locker: deps.Locker, Today: deps.Today, Fingerprint: deps.Fingerprint,
	}, nil
}
