//go:build testhooks

package application

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// legacyParticipantRows serves a unit fake written against the legacy
// participant repository as the owner's participant rows: each read the
// participant functions issue goes to the fake's matching legacy read, the
// participant insert to its Create, and the Student Presence writes to its
// legacy write. Participants resolved for a write get an ID the bridge maps
// back to their (instance, student) pair. The application's unit tests may
// not name Student Presence, so the bridge lives here.
func legacyParticipantRows(fake any) ports.ParticipantRows {
	refs := &participantRefRegistry{byID: map[int64]scheduleModels.StudentInstanceRef{}}
	return ports.ParticipantRows{
		Plans:    legacyParticipantPlans{fake: fake},
		Presence: legacyParticipantPresence{fake: fake, refs: refs},
		Reads:    legacyParticipantReads{fake: fake, refs: refs},
	}
}

// participantRefRegistry numbers the (instance, student) pairs a write
// resolves.
type participantRefRegistry struct {
	mu   sync.Mutex
	byID map[int64]scheduleModels.StudentInstanceRef
}

func (r *participantRefRegistry) row(instanceID, studentID int64) *scheduleModels.InstanceStudent {
	r.mu.Lock()
	defer r.mu.Unlock()
	id := int64(len(r.byID) + 1)
	r.byID[id] = scheduleModels.StudentInstanceRef{InstanceID: instanceID, StudentID: studentID}
	row := &scheduleModels.InstanceStudent{InstanceID: instanceID, StudentID: studentID}
	row.ID = id
	return row
}

func (r *participantRefRegistry) refsOf(ids []int64) []scheduleModels.StudentInstanceRef {
	r.mu.Lock()
	defer r.mu.Unlock()
	refs := make([]scheduleModels.StudentInstanceRef, 0, len(ids))
	for _, id := range ids {
		refs = append(refs, r.byID[id])
	}
	return refs
}

type legacyParticipantReads struct {
	ports.LegacyParticipantReads
	fake any
	refs *participantRefRegistry
}

func (r legacyParticipantReads) ListLegacyParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error) {
	if filter.ExcludeFinished {
		rows := make([]*scheduleModels.InstanceStudent, 0, len(filter.InstanceIDs)*len(filter.StudentIDs))
		for _, instanceID := range filter.InstanceIDs {
			for _, studentID := range filter.StudentIDs {
				rows = append(rows, r.refs.row(instanceID, studentID))
			}
		}
		return rows, nil
	}
	if filter.OrderByInstanceStudent {
		if fake, ok := r.fake.(interface {
			FindByInstanceIDs(context.Context, []int64) ([]*scheduleModels.InstanceStudent, error)
		}); ok {
			return fake.FindByInstanceIDs(ctx, filter.InstanceIDs)
		}
		if fake, ok := r.fake.(interface {
			FindNotScheduledCandidatesByInstanceIDs(context.Context, []int64) ([]*scheduleModels.InstanceStudent, error)
		}); ok {
			rows, err := fake.FindNotScheduledCandidatesByInstanceIDs(ctx, filter.InstanceIDs)
			return expectedWithoutStatus(rows), err
		}
	}
	return nil, fmt.Errorf("fixture participant read is not configured for %+v", filter)
}

// expectedWithoutStatus reads a fake row without a status as expected, as
// a participant without an attendance row is.
func expectedWithoutStatus(rows []*scheduleModels.InstanceStudent) []*scheduleModels.InstanceStudent {
	for _, row := range rows {
		if row != nil && row.Status == "" {
			row.Status = scheduleModels.AttendanceStatusExpected
		}
	}
	return rows
}

type legacyParticipantPlans struct {
	timetable.InstanceStudentCapability
	fake any
}

func (p legacyParticipantPlans) CreateInstanceStudent(ctx context.Context, input timetable.InstanceStudentInput) (timetable.InstanceStudent, error) {
	fake, ok := p.fake.(interface {
		Create(context.Context, *scheduleModels.InstanceStudent) error
	})
	if !ok {
		return timetable.InstanceStudent{}, fmt.Errorf("fixture participant create is not configured for %T", p.fake)
	}
	row := &scheduleModels.InstanceStudent{InstanceID: input.InstanceID, StudentID: input.StudentID, RoomID: input.RoomID}
	if err := fake.Create(ctx, row); err != nil {
		return timetable.InstanceStudent{}, err
	}
	return timetable.InstanceStudent{ID: row.ID, InstanceID: row.InstanceID, StudentID: row.StudentID, RoomID: row.RoomID}, nil
}

type legacyParticipantPresence struct {
	ports.ParticipantPresence
	fake any
	refs *participantRefRegistry
}

func (p legacyParticipantPresence) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date string) (int, error) {
	fake, ok := p.fake.(interface {
		ApplyActiveStatusDaysForInstance(context.Context, int64, calendar.Date) (int, error)
	})
	if !ok {
		return 0, fmt.Errorf("fixture status days are not configured for %T", p.fake)
	}
	return fake.ApplyActiveStatusDaysForInstance(ctx, instanceID, calendar.Date(date))
}

func (p legacyParticipantPresence) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string) (int, error) {
	fake, ok := p.fake.(interface {
		ApplyActivePartialAbsencesForInstance(context.Context, int64, calendar.Date) (int, error)
	})
	if !ok {
		return 0, fmt.Errorf("fixture partial absences are not configured for %T", p.fake)
	}
	return fake.ApplyActivePartialAbsencesForInstance(ctx, instanceID, calendar.Date(date))
}

func (p legacyParticipantPresence) MarkParticipantsNotScheduled(ctx context.Context, ids []int64) (int64, error) {
	fake, ok := p.fake.(interface {
		MarkNotScheduled(context.Context, []scheduleModels.StudentInstanceRef) error
	})
	if !ok {
		return 0, fmt.Errorf("fixture non-booking marker is not configured for %T", p.fake)
	}
	return int64(len(ids)), fake.MarkNotScheduled(ctx, p.refs.refsOf(ids))
}

func (p legacyParticipantPresence) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []studentpresence.PlannedParticipantRef) error {
	fake, ok := p.fake.(interface {
		MarkExpectedAbsentByActiveGroupIDs(context.Context, []int64, time.Time, []scheduleModels.StudentInstanceRef) error
	})
	if !ok {
		return fmt.Errorf("fixture absent stamp is not configured for %T", p.fake)
	}
	refs := make([]scheduleModels.StudentInstanceRef, 0, len(exclusions))
	for _, ref := range exclusions {
		refs = append(refs, scheduleModels.StudentInstanceRef(ref))
	}
	return fake.MarkExpectedAbsentByActiveGroupIDs(ctx, activeGroupIDs, updatedAt, refs)
}
