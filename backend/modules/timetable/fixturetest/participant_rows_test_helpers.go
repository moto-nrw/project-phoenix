//go:build testhooks

package fixturetest

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	records "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ParticipantRowsOf serves a fake participant repository as the owner's
// participant rows: each filter shape the owner's reads use goes to the
// fake's matching legacy read, and a fake that lacks it fails the read; the
// participant plan and the attendance go through the given parts.
func ParticipantRowsOf(fake any, plans timetable.InstanceStudentCapability, presence ports.ParticipantPresence) ports.ParticipantRows {
	return ports.ParticipantRows{Plans: plans, Presence: presence, Reads: fakeParticipantReads{fake: fake}}
}

type fakeParticipantReads struct {
	ports.LegacyParticipantReads
	fake any
}

func (r fakeParticipantReads) ListLegacyParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) ([]*records.InstanceStudent, error) {
	byID, _ := r.fake.(interface {
		FindByID(context.Context, int64) (*InstanceStudent, error)
	})
	byInstance, _ := r.fake.(interface {
		FindByInstanceID(context.Context, int64) ([]*InstanceStudent, error)
	})
	byInstances, _ := r.fake.(interface {
		FindByInstanceIDs(context.Context, []int64) ([]*InstanceStudent, error)
	})
	byPair, _ := r.fake.(interface {
		FindByInstanceAndStudent(context.Context, int64, int64) (*InstanceStudent, error)
	})
	switch {
	case len(filter.IDs) == 1 && byID != nil:
		row, err := byID.FindByID(ctx, filter.IDs[0])
		return oneParticipant(row), err
	case len(filter.InstanceIDs) == 1 && filter.OrderByCreated && byInstance != nil:
		rows, err := byInstance.FindByInstanceID(ctx, filter.InstanceIDs[0])
		return toNative26(rows), err
	case filter.OrderByInstanceStudent && byInstances != nil:
		rows, err := byInstances.FindByInstanceIDs(ctx, filter.InstanceIDs)
		return toNative26(rows), err
	case len(filter.InstanceIDs) > 0 && len(filter.StudentIDs) > 0 && filter.Date == nil && filter.FromDate == nil && byPair != nil:
		result := []*records.InstanceStudent{}
		for _, instanceID := range filter.InstanceIDs {
			for _, studentID := range filter.StudentIDs {
				row, err := byPair.FindByInstanceAndStudent(ctx, instanceID, studentID)
				if err != nil {
					return nil, err
				}
				result = append(result, oneParticipant(row)...)
			}
		}
		return result, nil
	}
	return nil, fmt.Errorf("fixture participant read is not configured for %+v", filter)
}

func (r fakeParticipantReads) ListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) ([]records.ParallelPresence, error) {
	fake, ok := r.fake.(interface {
		FindPresentInOtherActiveInstances(context.Context, int64, calendar.Date, []int64) ([]ParallelPresence, error)
	})
	if !ok {
		return nil, fmt.Errorf("fixture parallel presence is not configured for %T", r.fake)
	}
	rows, err := fake.FindPresentInOtherActiveInstances(ctx, excludedInstanceID, calendar.Date(date), studentIDs)
	return toNative32(rows), err
}

func oneParticipant(row *InstanceStudent) []*records.InstanceStudent {
	if row == nil {
		return []*records.InstanceStudent{}
	}
	return []*records.InstanceStudent{toNative27(row)}
}
