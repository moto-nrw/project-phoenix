package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// A later pickup decision (#3261) writes to two owners since the presence
// cutover (#2762): Timetable & Activities adds the child to the chosen
// blocks, Student Presence then re-applies the reported day statuses and
// partial absences of each block's date to the new participant. The root
// binds the two steps over the caller's tenant transaction.

// pickupExtensionRules is the Student Presence surface the decision re-applies.
type pickupExtensionRules interface {
	ApplyActiveStatusDaysForInstance(context.Context, int64, string) (int, error)
	ApplyActivePartialAbsencesForInstance(context.Context, int64, string) (int, error)
}

// newPickupExtensions binds the decision over the Timetable pickup extensions and the
// Student Presence attendance rules.
func newPickupExtensions(extensions timetable.PickupExtensionCapability, rules pickupExtensionRules) (timetable.PickupExtensionCapability, error) {
	if extensions == nil || rules == nil {
		return nil, errors.New("pickup extension workflow: timetable and student presence are required")
	}
	return pickupExtensions{PickupExtensionCapability: extensions, rules: rules}, nil
}

type pickupExtensions struct {
	timetable.PickupExtensionCapability
	rules pickupExtensionRules
}

// ResolvePickupExtension adds the child to the chosen blocks and lets Student
// Presence decide the attendance the care plan already reported for those
// days.
func (w pickupExtensions) ResolvePickupExtension(ctx context.Context, taskID int64, blockIDs []int64) (timetable.PickupExtensionResolution, error) {
	resolution, err := w.PickupExtensionCapability.ResolvePickupExtension(ctx, taskID, blockIDs)
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

var _ pickupExtensionRules = (studentpresence.SessionAttendanceRules)(nil)
