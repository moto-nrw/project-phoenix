package schedule

import (
	"strings"
	"testing"
	"time"
)

func TestPlanningTrackValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		track PlanningTrack
		valid bool
	}{
		{name: "valid", track: PlanningTrack{Name: " Früh ", Color: "#5080D8", SortOrder: 0}, valid: true},
		{name: "empty name", track: PlanningTrack{Name: " ", Color: "#5080D8"}},
		{name: "long name", track: PlanningTrack{Name: strings.Repeat("a", 101), Color: "#5080D8"}},
		{name: "invalid color", track: PlanningTrack{Name: "Früh", Color: "blue"}},
		{name: "negative order", track: PlanningTrack{Name: "Früh", Color: "#5080D8", SortOrder: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.track.Validate()
			if tt.valid && err != nil {
				t.Fatalf("validate planning track: %v", err)
			}
			if !tt.valid && err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
	track := PlanningTrack{Name: " Früh ", Color: "#5080D8"}
	if err := track.Validate(); err != nil || track.Name != "Früh" {
		t.Fatalf("validate should trim the name: track=%#v err=%v", track, err)
	}
	if track.IsArchived() {
		t.Fatal("new planning track must be active")
	}
	now := time.Now()
	track.ArchivedAt = &now
	if !track.IsArchived() {
		t.Fatal("archived planning track was not recognized")
	}
}
