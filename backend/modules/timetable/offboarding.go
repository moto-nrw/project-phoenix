package timetable

import (
	"context"
	"errors"
	"time"
)

var ErrOffboardingConflict = errors.New("timetable changed since offboarding preview")

type OffboardingCounts struct {
	InstanceAssignments int64
	PlannedSupervisors  int64
}

type OffboardingPreview struct {
	Counts   OffboardingCounts
	Revision string
}

// Offboarding removes operational assignments while retaining history.
type Offboarding struct {
	preview func(context.Context, int64, string) (OffboardingPreview, error)
	execute func(context.Context, int64, string, string) (OffboardingCounts, error)
}

func NewOffboarding(preview func(context.Context, int64, string) (OffboardingPreview, error), execute func(context.Context, int64, string, string) (OffboardingCounts, error)) *Offboarding {
	if preview == nil || execute == nil {
		panic("timetable: offboarding queries and commands are required")
	}
	return &Offboarding{preview: preview, execute: execute}
}

func (o *Offboarding) Preview(ctx context.Context, staffID int64, from string) (OffboardingPreview, error) {
	if err := validateOffboarding(staffID, from); err != nil {
		return OffboardingPreview{}, err
	}
	return o.preview(ctx, staffID, from)
}

func (o *Offboarding) Execute(ctx context.Context, staffID int64, from, revision string) (OffboardingCounts, error) {
	if err := validateOffboarding(staffID, from); err != nil {
		return OffboardingCounts{}, err
	}
	if revision == "" {
		return OffboardingCounts{}, errors.New("timetable: offboarding revision is required")
	}
	return o.execute(ctx, staffID, from, revision)
}

func validateOffboarding(staffID int64, from string) error {
	if staffID <= 0 {
		return errors.New("timetable: staff ID is required")
	}
	_, err := time.Parse("2006-01-02", from)
	return err
}
