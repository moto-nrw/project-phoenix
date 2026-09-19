package schoolmembership_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func TestStudentLifecycleCommandsNormalizeAndValidate(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	engine := &recordingEngine{}
	module := schoolmembership.NewModule(engine)
	_, err := module.ChangeClass(ctx, []int64{4, 4, -1, 0, 7}, "1a", "2a")
	if err != nil || !reflect.DeepEqual(engine.studentIDs, []int64{4, 7}) || engine.fromClass != "1a" || engine.toClass != "2a" {
		t.Fatalf("class command lost normalization or comparison: %+v, %v", engine, err)
	}
	_, err = module.Reactivate(ctx, []int64{7, 7, 0}, " active ")
	if err != nil || !reflect.DeepEqual(engine.studentIDs, []int64{7}) || engine.studentStatus != "active" {
		t.Fatalf("reactivation lost normalization: %+v, %v", engine, err)
	}
	calls := engine.calls
	if _, err := module.Graduate(ctx, []int64{0, -1}); err != nil || engine.calls != calls {
		t.Fatalf("empty normalized batch reached persistence: %v", err)
	}
	for _, run := range []func() error{
		func() error { _, err := module.ChangeClass(ctx, []int64{1}, " ", "2a"); return err },
		func() error { _, err := module.Reactivate(ctx, []int64{1}, "alumnus"); return err },
		func() error { _, err := module.TransitionStatus(ctx, 1, "alumnus", "active"); return err },
		func() error { _, err := module.SetStatus(ctx, 1, "alumnus"); return err },
		func() error { _, err := module.EndCare(ctx, []int64{1}, "not-a-date"); return err },
		func() error { _, err := module.ResumeCare(ctx, 1, "2035-01-01", "active", ""); return err },
	} {
		if err := run(); err == nil {
			t.Fatal("invalid lifecycle command accepted")
		}
		if engine.calls != calls {
			t.Fatal("invalid lifecycle command reached persistence")
		}
	}
}

func (e *recordingEngine) TransitionStudentStatus(context.Context, int64, string, string) (bool, error) {
	return false, nil
}

func (e *recordingEngine) EnrollStudent(context.Context, schoolmembership.StudentEnrollment) (int64, error) {
	return 0, nil
}

func (e *recordingEngine) RenewStudentEnrollment(context.Context, schoolmembership.StudentEnrollment) (int64, error) {
	return 0, nil
}

func (e *recordingEngine) AssignStudentGroup(context.Context, int64, *int64) (bool, error) {
	return false, nil
}
