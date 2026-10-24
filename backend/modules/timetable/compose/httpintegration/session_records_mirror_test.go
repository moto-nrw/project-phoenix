package httpintegration_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mirrorCreateRecorder records the plan the mirror inserts and the session
// it starts; every other method stays nil and panics if called.
type mirrorCreateRecorder struct {
	timetable.ActivityInstanceCapability
	timetablePorts.InstanceSessions
	created *timetable.ActivityInstanceInput
	started *studentpresence.ActivitySessionStart
}

func (r *mirrorCreateRecorder) CreateActivityInstance(_ context.Context, input timetable.ActivityInstanceInput) (timetable.ActivityInstance, error) {
	r.created = &input
	return timetable.ActivityInstance{ID: 91}, nil
}

func (r *mirrorCreateRecorder) StartActivitySession(_ context.Context, start studentpresence.ActivitySessionStart) (studentpresence.ActivitySession, error) {
	r.started = &start
	return studentpresence.ActivitySession{}, nil
}

// FindByID answers the reload after the insert.
func (r *mirrorCreateRecorder) FindByID(context.Context, int64) (*fixtures.ActivityInstance, error) {
	return fixtures.NewActivityInstance(nil), nil
}

func TestCreateMirroredSessionInstanceOwnsTheSpontaneousActiveDefaults(t *testing.T) {
	t.Parallel()
	recorder := &mirrorCreateRecorder{}
	records := compose.Repositories{
		ActivityInstance: timetablePorts.InstanceRows{Plans: recorder, Sessions: recorder, Reads: fixtures.InstanceReadsOf(recorder)},
	}.NativeRecords()
	sessionID := int64(77)

	_, err := records.CreateMirroredSessionInstance(context.Background(), timetable.ScheduledInstance{
		Title: "Kiosk-Sitzung", Status: timetable.InstanceStatusPlanned, IsSpontaneous: false,
		Date: calendar.NewDate(2026, time.April, 20), RoomID: 5, ActiveGroupID: &sessionID,
		StartTime: time.Date(2000, time.January, 1, 14, 0, 0, 0, time.UTC), EndTime: time.Date(2000, time.January, 1, 15, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create mirrored session: %v", err)
	}
	if recorder.created == nil {
		t.Fatal("the mirror must insert one instance row")
	}
	if !recorder.created.IsSpontaneous || recorder.started == nil {
		t.Errorf("mirrored session = spontaneous %v, started %v; want spontaneous and active", recorder.created.IsSpontaneous, recorder.started != nil)
	}
}
