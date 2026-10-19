package application

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModel "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// mirrorCreateRecorder records the row the mirror inserts; every other
// repository method stays nil and panics if called.
type mirrorCreateRecorder struct {
	ports.ActivityInstanceRecords
	created *scheduleModel.ActivityInstance
}

func (r *mirrorCreateRecorder) Create(_ context.Context, row *scheduleModel.ActivityInstance) error {
	r.created = row
	return nil
}

func TestCreateMirroredSessionInstanceOwnsTheSpontaneousActiveDefaults(t *testing.T) {
	t.Parallel()
	recorder := &mirrorCreateRecorder{}
	records := SessionRecords{Instances: recorder}

	_, err := records.CreateMirroredSessionInstance(context.Background(), timetable.ScheduledInstance{
		Title: "Kiosk-Sitzung", Status: timetable.InstanceStatusPlanned, IsSpontaneous: false,
	})
	if err != nil {
		t.Fatalf("create mirrored session: %v", err)
	}
	if recorder.created == nil {
		t.Fatal("the mirror must insert one instance row")
	}
	if !recorder.created.IsSpontaneous || recorder.created.Status != scheduleModel.InstanceStatusActive {
		t.Errorf("mirrored session = spontaneous %v, status %q; want spontaneous and active", recorder.created.IsSpontaneous, recorder.created.Status)
	}
}
