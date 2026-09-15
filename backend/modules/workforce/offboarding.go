package workforce

import (
	"context"
	"errors"
	"time"
)

var ErrOffboardingConflict = errors.New("workforce changed since offboarding preview")
var ErrOffboardingInUse = errors.New("staff has an active or future group handover")

type OffboardingCounts struct {
	Absences      int64
	Shifts        int64
	Series        int64
	Substitutions int64
}

type OffboardingPreview struct {
	Counts   OffboardingCounts
	Revision string
	Blocked  bool
}

// Offboarding removes operational plans, not work-session or decided absence
// history. Its audit callback must append in the supplied UnitOfWork.
type Offboarding struct {
	lock    func(context.Context, int64) error
	preview func(context.Context, int64, string) (OffboardingPreview, error)
	execute func(context.Context, int64, int64, string, string) (OffboardingCounts, error)
}

func NewOffboarding(lock func(context.Context, int64) error, preview func(context.Context, int64, string) (OffboardingPreview, error), execute func(context.Context, int64, int64, string, string) (OffboardingCounts, error)) *Offboarding {
	if lock == nil || preview == nil || execute == nil {
		panic("workforce: offboarding queries and commands are required")
	}
	return &Offboarding{lock: lock, preview: preview, execute: execute}
}

// Lock holds the balance, absence and shift advisory locks, in that order.
// Workflows call it before acquiring Membership row locks; an ambient tenant
// transaction is required so the lock lifetime covers the entire workflow.
func (o *Offboarding) Lock(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("workforce: staff ID is required")
	}
	return o.lock(ctx, staffID)
}

func (o *Offboarding) Preview(ctx context.Context, staffID int64, from string) (OffboardingPreview, error) {
	if err := validateOffboarding(staffID, from); err != nil {
		return OffboardingPreview{}, err
	}
	return o.preview(ctx, staffID, from)
}

func (o *Offboarding) Execute(ctx context.Context, staffID, actorID int64, from, revision string) (OffboardingCounts, error) {
	if err := validateOffboarding(staffID, from); err != nil {
		return OffboardingCounts{}, err
	}
	if actorID <= 0 || revision == "" {
		return OffboardingCounts{}, errors.New("workforce: actor and offboarding revision are required")
	}
	return o.execute(ctx, staffID, actorID, from, revision)
}

func validateOffboarding(staffID int64, from string) error {
	if staffID <= 0 {
		return errors.New("workforce: staff ID is required")
	}
	_, err := time.Parse(DateLayout, from)
	return err
}
