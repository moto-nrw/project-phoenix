package ports

import (
	"context"
	"testing"
)

func TestRetentionPlanRunsChildrenThenStaffBeforeDataKinds(t *testing.T) {
	t.Parallel()

	run := func(context.Context) (RetentionOutcome, error) { return RetentionOutcome{}, nil }
	dataKind := RetentionStep{Kind: "data_kind", Run: run}

	steps := RetentionPlan(SubjectRetention{Kind: "children", Run: run}, SubjectRetention{Kind: "staff", Run: run}, []RetentionStep{dataKind})

	if len(steps) != 3 {
		t.Fatalf("steps = %d, want children, staff and the data kind", len(steps))
	}
	for index, want := range []RetentionStep{{Kind: "children", LogsSubjects: true}, {Kind: "staff", LogsSubjects: true}, {Kind: "data_kind"}} {
		if steps[index].Kind != want.Kind || steps[index].LogsSubjects != want.LogsSubjects || steps[index].Run == nil {
			t.Fatalf("step %d = %+v, want %+v", index, steps[index], want)
		}
	}
}

func TestRetentionPlanLeavesOutSubjectsWithoutRun(t *testing.T) {
	t.Parallel()

	run := func(context.Context) (RetentionOutcome, error) { return RetentionOutcome{}, nil }

	steps := RetentionPlan(SubjectRetention{Kind: "children"}, SubjectRetention{Kind: "staff", Run: run}, nil)

	if len(steps) != 1 || steps[0].Kind != "staff" || !steps[0].LogsSubjects {
		t.Fatalf("steps = %+v, want only the staff deletion", steps)
	}
	if RetentionPlan(SubjectRetention{}, SubjectRetention{}, nil) != nil {
		t.Fatal("a plan without steps must be empty")
	}
}
