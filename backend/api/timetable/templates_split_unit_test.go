package timetable

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateTemplatePlanningTrackThreeState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		fragment     string
		wantProvided bool
		wantValue    *int64
	}{
		{name: "omitted preserves", wantProvided: false},
		{name: "null clears", fragment: `"planning_track_id": null`, wantProvided: true},
		{name: "number assigns", fragment: `"planning_track_id": 42`, wantProvided: true, wantValue: testInt64Ptr(42)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &updateTemplateRequest{}
			require.NoError(t, json.Unmarshal([]byte(splitBodyJSON(tc.fragment)), req))

			input := buildUpdateTemplateInput(
				100,
				&parsedUpdateTemplate{req: req},
				200,
				4,
				timezone.Date(""),
			)

			assert.Equal(t, tc.wantProvided, input.Fields.PlanningTrackIDProvided)
			assert.Equal(t, tc.wantValue, input.Fields.PlanningTrackID)
		})
	}
}

func TestBuildTemplateSplitInputPlanningTrackThreeState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		fragment     string
		wantProvided bool
		wantValue    *int64
	}{
		{name: "omitted inherits", wantProvided: false},
		{name: "null clears", fragment: `"planning_track_id": null`, wantProvided: true},
		{name: "number assigns", fragment: `"planning_track_id": 42`, wantProvided: true, wantValue: testInt64Ptr(42)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := &splitTemplateRequest{}
			require.NoError(t, json.Unmarshal([]byte(splitBodyJSON(tc.fragment)), req))

			input, err := buildTemplateSplitInput(100, req)
			require.NoError(t, err)
			assert.Equal(t, tc.wantProvided, input.PlanningTrackIDProvided)
			assert.Equal(t, tc.wantValue, input.PlanningTrackID)
		})
	}
}

func testInt64Ptr(value int64) *int64 {
	return &value
}

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
	t.Parallel()

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

// The split flow must carry the Listenart (#1565) with the same three-state
// contract: omitted inherits the source template's list kind, an explicit
// null/empty clears it, a valid value sets it. Without ListKindProvided a
// plain "this and following" edit would silently drop the successor from its
// automatic daily list.
func TestBuildTemplateSplitInput_ListKindThreeState(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		fragment     string
		wantProvided bool
		wantValueNil bool
		wantValue    string
		wantBindErr  bool
	}{
		{name: "omitted -> inherit (not provided)", fragment: "", wantProvided: false, wantValueNil: true},
		{name: "explicit null -> clear (provided, nil)", fragment: `"list_kind": null`, wantProvided: true, wantValueNil: true},
		{name: "empty string -> clear (provided, nil)", fragment: `"list_kind": ""`, wantProvided: true, wantValueNil: true},
		{name: "value -> set (provided, mensa)", fragment: `"list_kind": "mensa"`, wantProvided: true, wantValue: "mensa"},
		{name: "invalid value -> Bind error", fragment: `"list_kind": "kaffeepause"`, wantBindErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := splitBodyJSON("")
			if tc.fragment != "" {
				body = splitBodyJSON(tc.fragment)
			}
			req := &splitTemplateRequest{}
			require.NoError(t, json.Unmarshal([]byte(body), req))

			err := req.Bind(nil)
			if tc.wantBindErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			in, err := buildTemplateSplitInput(100, req)
			require.NoError(t, err)

			assert.Equal(t, tc.wantProvided, in.ListKindProvided, "ListKindProvided")
			if tc.wantValueNil {
				assert.Nil(t, in.ListKind, "ListKind should be nil")
			} else {
				require.NotNil(t, in.ListKind)
				assert.Equal(t, tc.wantValue, *in.ListKind)
			}
		})
	}
}

// The split flow must carry the offering-source rule (#2137) with the same
// three-state contract as the template PUT (#2147 review round 14): omitted
// inherits the old template's source and filter, an explicit null clears
// them, values set them. Without the pass-through the service always copied
// the old template's rule and a requested change was silently dropped.
func TestBuildTemplateSplitInput_SourceFieldsThreeState(t *testing.T) {
	t.Parallel()

	t.Run("omitted -> inherit (not provided)", func(t *testing.T) {
		req := &splitTemplateRequest{}
		require.NoError(t, json.Unmarshal([]byte(splitBodyJSON("")), req))

		in, err := buildTemplateSplitInput(100, req)
		require.NoError(t, err)

		assert.False(t, in.SourceCareOfferingIDsProvided)
		assert.False(t, in.SourceGradeLevelsProvided)
		assert.Nil(t, in.SourceCareOfferingIDs)
		assert.Nil(t, in.SourceGradeLevels)
	})

	t.Run("explicit null -> clear (provided, nil)", func(t *testing.T) {
		req := &splitTemplateRequest{}
		require.NoError(t, json.Unmarshal([]byte(splitBodyJSON(
			`"source_care_offering_ids": null, "source_grade_levels": null`)), req))

		in, err := buildTemplateSplitInput(100, req)
		require.NoError(t, err)

		assert.True(t, in.SourceCareOfferingIDsProvided)
		assert.True(t, in.SourceGradeLevelsProvided)
		assert.Nil(t, in.SourceCareOfferingIDs)
		assert.Nil(t, in.SourceGradeLevels)
	})

	t.Run("values -> set (provided)", func(t *testing.T) {
		req := &splitTemplateRequest{}
		require.NoError(t, json.Unmarshal([]byte(splitBodyJSON(
			`"source_care_offering_ids": [7, 9], "source_grade_levels": [2, 3]`)), req))

		in, err := buildTemplateSplitInput(100, req)
		require.NoError(t, err)

		require.True(t, in.SourceCareOfferingIDsProvided)
		assert.Equal(t, []int64{7, 9}, in.SourceCareOfferingIDs)
		assert.True(t, in.SourceGradeLevelsProvided)
		assert.Equal(t, []int{2, 3}, in.SourceGradeLevels)
	})
}

func TestSplitTemplateRequestBind_RejectsWeekendWeekdays(t *testing.T) {
	t.Parallel()

	req := &splitTemplateRequest{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"name": "AG Yoga",
		"type": "activity",
		"weekdays": [6],
		"start_time": "14:00",
		"end_time": "15:00",
		"room_id": 3,
		"category_id": 2,
		"effective_date": "2026-05-04"
	}`), req))

	require.ErrorContains(t, req.Bind(nil), "Monday to Friday")
}
