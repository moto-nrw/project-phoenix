package timetable

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// splitBodyJSON builds a minimal valid split request body; the required_staff
// fragment is spliced in per-case so the three-state (omitted / null / value)
// decode can be exercised in isolation.
func splitBodyJSON(requiredStaffFragment string) string {
	rs := ""
	if requiredStaffFragment != "" {
		rs = requiredStaffFragment + ","
	}
	return `{
		"name": "AG Yoga",
		"type": "activity",
		"weekdays": [1],
		"start_time": "14:00",
		"end_time": "15:00",
		"room_id": 3,
		"category_id": 2,
		` + rs + `
		"effective_date": "2026-05-04"
	}`
}

// The split flow must distinguish an omitted required_staff (inherit the source
// template's override) from an explicit null (clear it → derive) — the bug the
// review flagged, where clearing was impossible because both became nil (#1839).
func TestBuildTemplateSplitInput_RequiredStaffThreeState(t *testing.T) {
	cases := []struct {
		name         string
		fragment     string
		wantProvided bool
		wantValueNil bool
		wantValue    int
	}{
		{"omitted -> inherit (not provided)", "", false, true, 0},
		{"explicit null -> clear (provided, nil)", `"required_staff": null`, true, true, 0},
		{"number -> override (provided, value)", `"required_staff": 4`, true, false, 4},
		{"zero -> explicit none (provided, 0)", `"required_staff": 0`, true, false, 0},
		{"negative -> normalized to clear (provided, nil)", `"required_staff": -3`, true, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &splitTemplateRequest{}
			require.NoError(t, json.Unmarshal([]byte(splitBodyJSON(tc.fragment)), req))

			in, err := buildTemplateSplitInput(100, req)
			require.NoError(t, err)

			assert.Equal(t, tc.wantProvided, in.RequiredStaffProvided, "RequiredStaffProvided")
			if tc.wantValueNil {
				assert.Nil(t, in.RequiredStaff, "RequiredStaff should be nil")
			} else {
				require.NotNil(t, in.RequiredStaff)
				assert.Equal(t, tc.wantValue, *in.RequiredStaff)
			}
		})
	}
}
