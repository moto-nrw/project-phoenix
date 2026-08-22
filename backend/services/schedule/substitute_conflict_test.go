package schedule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ci is a concise constructor for SubstituteConflictInstance used by the
// table tests below.
func ci(id int64, start, end int, startHHMM string) SubstituteConflictInstance {
	return SubstituteConflictInstance{ID: id, StartMin: start, EndMin: end, StartHHMM: startHHMM}
}

func TestDetectSubstituteTimeConflicts_EmptyInputs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, DetectSubstituteTimeConflicts(nil, nil))
	assert.Nil(t, DetectSubstituteTimeConflicts(
		[]SubstituteConflictInstance{ci(1, 840, 900, "14:00")},
		nil,
	))
	assert.Nil(t, DetectSubstituteTimeConflicts(
		nil,
		[]SubstituteConflictInstance{ci(2, 870, 930, "14:30")},
	))
}

func TestDetectSubstituteTimeConflicts_OverlapCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		target       SubstituteConflictInstance
		foreign      SubstituteConflictInstance
		wantWarning  bool
		wantStartMsg string // only checked when wantWarning=true
	}{
		{
			name:         "target starts before, overlaps middle",
			target:       ci(1, 840, 900, "14:00"), // 14:00–15:00
			foreign:      ci(99, 870, 930, "14:30"),
			wantWarning:  true,
			wantStartMsg: "14:30",
		},
		{
			name:        "target after foreign ends, no overlap",
			target:      ci(1, 900, 960, "15:00"),  // 15:00–16:00
			foreign:     ci(99, 780, 900, "13:00"), // 13:00–15:00 (edge)
			wantWarning: false,                     // target.start == foreign.end → half-open no overlap
		},
		{
			name:        "target before foreign starts, no overlap",
			target:      ci(1, 840, 900, "14:00"), // 14:00–15:00
			foreign:     ci(99, 900, 960, "15:00"),
			wantWarning: false,
		},
		{
			name:         "target fully inside foreign window",
			target:       ci(1, 855, 885, "14:15"), // 14:15–14:45
			foreign:      ci(99, 840, 900, "14:00"),
			wantWarning:  true,
			wantStartMsg: "14:00",
		},
		{
			name:         "foreign fully inside target window",
			target:       ci(1, 840, 960, "14:00"), // 14:00–16:00
			foreign:      ci(99, 870, 930, "14:30"),
			wantWarning:  true,
			wantStartMsg: "14:30",
		},
		{
			name:         "identical window",
			target:       ci(1, 840, 900, "14:00"),
			foreign:      ci(99, 840, 900, "14:00"),
			wantWarning:  true,
			wantStartMsg: "14:00",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := DetectSubstituteTimeConflicts(
				[]SubstituteConflictInstance{tc.target},
				[]SubstituteConflictInstance{tc.foreign},
			)
			if !tc.wantWarning {
				assert.Empty(t, out)
				return
			}
			if assert.Len(t, out, 1) {
				assert.Equal(t, SubstituteConflictKind, out[0].Kind)
				assert.Equal(t, tc.target.ID, out[0].InstanceID)
				assert.Equal(t, tc.foreign.ID, out[0].OtherID)
				assert.Contains(t, out[0].Message, tc.wantStartMsg)
			}
		})
	}
}

func TestDetectSubstituteTimeConflicts_SameInstanceIgnored(t *testing.T) {
	t.Parallel()

	// When an ID appears in both targets and foreigns (shouldn't happen
	// because the caller pre-filters, but be defensive), it must not flag
	// itself as a conflict.
	tgt := ci(1, 840, 900, "14:00")
	out := DetectSubstituteTimeConflicts(
		[]SubstituteConflictInstance{tgt},
		[]SubstituteConflictInstance{tgt},
	)
	assert.Empty(t, out)
}

func TestDetectSubstituteTimeConflicts_MultipleTargetsMultipleForeigns(t *testing.T) {
	t.Parallel()

	// Tgt1 14:00–15:00, Tgt2 16:00–17:00.
	// Foreign A 14:30–15:30 → overlaps Tgt1 only.
	// Foreign B 16:30–17:30 → overlaps Tgt2 only.
	// Foreign C 12:00–13:00 → overlaps neither.
	targets := []SubstituteConflictInstance{
		ci(1, 840, 900, "14:00"),
		ci(2, 960, 1020, "16:00"),
	}
	foreigns := []SubstituteConflictInstance{
		ci(100, 870, 930, "14:30"),
		ci(101, 990, 1050, "16:30"),
		ci(102, 720, 780, "12:00"),
	}

	out := DetectSubstituteTimeConflicts(targets, foreigns)
	if assert.Len(t, out, 2) {
		// Iteration order: targets outer, foreigns inner → Tgt1 first.
		assert.Equal(t, int64(1), out[0].InstanceID)
		assert.Equal(t, int64(100), out[0].OtherID)
		assert.Equal(t, int64(2), out[1].InstanceID)
		assert.Equal(t, int64(101), out[1].OtherID)
	}
}

func TestMinutesOfTime(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 0, MinutesOfTime(0, 0))
	assert.Equal(t, 60, MinutesOfTime(1, 0))
	assert.Equal(t, 90, MinutesOfTime(1, 30))
	assert.Equal(t, 14*60+30, MinutesOfTime(14, 30))
	assert.Equal(t, 23*60+59, MinutesOfTime(23, 59))
}
