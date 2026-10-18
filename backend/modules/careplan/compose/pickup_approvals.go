package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupApprovalDependencies struct {
	People     ports.PickupApprovalDirectory
	Presence   ports.PickupApprovalPresence
	Exceptions ports.PickupApprovalExceptions
	Excusal    ports.PickupApprovalExcusal
	Locker     ports.PickupApprovalLocker
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
