package careexit

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// A later pickup decision (#3261) writes to two owners since the presence
// cutover (#2762): Timetable & Activities adds the child to the chosen
// blocks, Student Presence then re-applies the reported day statuses and
// partial absences of each block's date to the new participant. Both steps
// run on the caller's tenant transaction.

// PickupExtensionRules is the Student Presence surface the decision
// re-applies.
type PickupExtensionRules interface {
	ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date string) (int, error)
	ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string) (int, error)
}

// PickupExtensions are the Timetable pickup-extension reads and the
// resolution the decision coordinates.
type PickupExtensions interface {
	timetable.PickupExtensionQuery
	ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error)
}

// PickupExtensionDecisions are the later-pickup decisions the Timetable
// routes serve (#3261): the open tasks and their child are Timetable's reads,
// the resolution re-applies Student Presence's reported attendance. The root
// binds them to the routes' own port; no Timetable command calls back into
// this workflow (#4825).
type PickupExtensionDecisions interface {
	FindPickupExtensionStudent(ctx context.Context, taskID int64) (int64, error)
	ListOpenPickupExtensions(ctx context.Context, studentID int64) ([]timetable.PickupExtensionTask, error)
	ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error)
}

// NewPickupExtensions binds the decision over the Timetable pickup
// extensions and the Student Presence attendance rules.
func NewPickupExtensions(extensions PickupExtensions, rules PickupExtensionRules) (PickupExtensionDecisions, error) {
	if extensions == nil || rules == nil {
		return nil, errors.New("pickup extension workflow: timetable and student presence are required")
	}
	return pickupExtensions{PickupExtensions: extensions, rules: rules}, nil
}

// pickupExtensions serves Timetable's reads unchanged and resolves through
// both owners.
type pickupExtensions struct {
	PickupExtensions
	rules PickupExtensionRules
}

// ResolvePickupExtension adds the child to the chosen blocks and lets Student
// Presence decide the attendance the care plan already reported for those
// days.
func (w pickupExtensions) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error) {
	resolution, err := w.PickupExtensions.ResolvePickupExtension(ctx, taskID, blockIDs)
	if err != nil {
		return resolution, err
	}
	for _, instance := range resolution.Instances {
		if _, err := w.rules.ApplyActiveStatusDaysForInstance(ctx, instance.ID, instance.Date); err != nil {
			return timetable.PickupExtensionResolution{}, fmt.Errorf("pickup extension: apply day statuses to block %d: %w", instance.ID, err)
		}
		if _, err := w.rules.ApplyActivePartialAbsencesForInstance(ctx, instance.ID, instance.Date); err != nil {
			return timetable.PickupExtensionResolution{}, fmt.Errorf("pickup extension: apply partial absences to block %d: %w", instance.ID, err)
		}
	}
	return resolution, nil
}
