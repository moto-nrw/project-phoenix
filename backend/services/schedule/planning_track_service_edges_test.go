package schedule

import (
	"context"
	"errors"
	"testing"
)

func TestPlanningTrackServiceValidationAndMissingEdges(t *testing.T) {
	repo := newPlanningTrackRepoStub()
	service := NewPlanningTrackService(repo, nil)
	invalid := PlanningTrackInput{Name: "", Color: "blue", SortOrder: -1}

	if _, err := service.CreatePlanningTrack(context.Background(), invalid); !errors.Is(err, ErrPlanningTrackInvalid) {
		t.Fatalf("create invalid planning track: got %v", err)
	}
	if _, err := service.UpdatePlanningTrack(context.Background(), 0, invalid); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("update missing planning track: got %v", err)
	}
	if _, err := service.ArchivePlanningTrack(context.Background(), 0); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("archive missing planning track: got %v", err)
	}
	if _, err := service.RestorePlanningTrack(context.Background(), 0); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("restore missing planning track: got %v", err)
	}
}
