package staffshifts

import (
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The route is composed and driven end to end from the Workforce HTTP
// composition; these tests pin the payload decoding and the wire mapping the
// adapter owns itself.

func TestOptionalFieldsTrackPresence(t *testing.T) {
	t.Parallel()

	var omitted ShiftRequest
	require.NoError(t, json.Unmarshal([]byte(`{"staff_id":7}`), &omitted))
	assert.False(t, omitted.ShiftTypeID.Present)
	assert.False(t, omitted.ChangeReason.Present)
	assert.False(t, omitted.Cancelled.Present)

	var explicit ShiftRequest
	require.NoError(t, json.Unmarshal([]byte(`{"shift_type_id":null,"change_reason":null,"cancelled":false,"origin_shift_id":"9223372036854775807"}`), &explicit))
	assert.True(t, explicit.ShiftTypeID.Present)
	assert.Nil(t, explicit.ShiftTypeID.Value)
	assert.True(t, explicit.ChangeReason.Present)
	assert.Nil(t, explicit.ChangeReason.Value)
	assert.True(t, explicit.Cancelled.Present)
	require.NotNil(t, explicit.OriginShiftID.Value)
	assert.Equal(t, int64(9223372036854775807), *explicit.OriginShiftID.Value, "string ids keep every bigint digit")

	var numeric ShiftRequest
	require.NoError(t, json.Unmarshal([]byte(`{"shift_type_id":5,"origin_shift_id":42}`), &numeric))
	require.NotNil(t, numeric.ShiftTypeID.Value)
	assert.Equal(t, int64(5), *numeric.ShiftTypeID.Value)

	var rejected CancellationRequest
	assert.Error(t, json.Unmarshal([]byte(`{"cancelled":null}`), &rejected), "a null cancelled flag is never an implicit false")
	assert.Error(t, json.Unmarshal([]byte(`{"break_minutes":null}`), &rejected), "a null break is never an implicit zero")
	assert.Error(t, json.Unmarshal([]byte(`{"shift_type_id":"abc"}`), &rejected))
}

func TestBuildShiftWidensClocksAndValidatesDates(t *testing.T) {
	t.Parallel()

	notes := "Frühdienst"
	input, err := buildShift(ShiftRequest{StaffID: 7, Date: "2026-07-07", StartTime: "09:00", EndTime: "15:30", BreakMinutes: 20, Notes: &notes})
	require.NoError(t, err)
	assert.Equal(t, "2026-07-07", input.Date)
	assert.Equal(t, "09:00:00", input.StartTime)
	assert.Equal(t, "15:30:00", input.EndTime)
	assert.Equal(t, "Frühdienst", input.Notes)

	_, err = buildShift(ShiftRequest{Date: "07.07.2026", StartTime: "09:00", EndTime: "15:30"})
	assert.EqualError(t, err, "date must be YYYY-MM-DD")
	_, err = buildShift(ShiftRequest{Date: "2026-07-07", StartTime: "9", EndTime: "15:30"})
	assert.EqualError(t, err, "start_time must be HH:MM")
	_, err = buildShift(ShiftRequest{Date: "2026-07-07", StartTime: "09:00", EndTime: "15h"})
	assert.EqualError(t, err, "end_time must be HH:MM")
}

func TestBuildMoveRequiresAnExplicitShiftType(t *testing.T) {
	t.Parallel()

	_, err := buildMove(9, MoveShiftRequest{SourceStaffID: 7, TargetStaffID: 8, Date: "2026-07-07", StartTime: "09:00", EndTime: "15:00"})
	assert.EqualError(t, err, "shift_type_id is required")

	typeID := int64(5)
	input, err := buildMove(9, MoveShiftRequest{SourceStaffID: 7, TargetStaffID: 8, Date: "2026-07-07", StartTime: "09:00", EndTime: "15:00", BreakMinutes: 20, ShiftTypeID: optionalID{Present: true, Value: &typeID}})
	require.NoError(t, err)
	assert.Equal(t, int64(9), input.ShiftID)
	assert.Equal(t, "15:00:00", input.EndTime)
	assert.Equal(t, &typeID, input.ShiftTypeID)
}

func TestBuildCancellationRejectsPartialOriginEdits(t *testing.T) {
	t.Parallel()

	decode := func(t *testing.T, payload string) CancellationRequest {
		t.Helper()
		var request CancellationRequest
		require.NoError(t, json.Unmarshal([]byte(payload), &request))
		return request
	}

	full, err := buildCancellation(5, decode(t, `{"cancelled": true, "change_reason": "krank", "start_time": "09:00", "end_time": "14:00", "break_minutes": 30, "shift_type_id": 7, "replacements": [{"staff_id": 8, "start_time": "09:00", "end_time": "11:00"}]}`))
	require.NoError(t, err)
	assert.True(t, full.ApplyOriginEdits)
	assert.Equal(t, "09:00:00", full.StartTime)
	assert.Equal(t, 30, full.BreakMinutes)
	require.Len(t, full.Replacements, 1)
	assert.Equal(t, "11:00:00", full.Replacements[0].EndTime)

	preserved, err := buildCancellation(5, decode(t, `{"cancelled": false, "change_reason": "zurück"}`))
	require.NoError(t, err)
	assert.False(t, preserved.ApplyOriginEdits)
	assert.False(t, preserved.Cancelled)

	_, err = buildCancellation(5, decode(t, `{"change_reason": "krank"}`))
	assert.EqualError(t, err, "cancelled is required")

	for name, payload := range map[string]string{
		"missing break_minutes": `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "shift_type_id": 7}`,
		"missing shift_type_id": `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": 30}`,
		"missing end_time":      `{"cancelled": true, "start_time": "09:00", "break_minutes": 30, "shift_type_id": 7}`,
		"only break_minutes":    `{"cancelled": true, "break_minutes": 30}`,
		"only shift_type_id":    `{"cancelled": true, "shift_type_id": 7}`,
	} {
		_, err := buildCancellation(5, decode(t, payload))
		assert.EqualError(t, err, "origin shift edits require start_time, end_time, break_minutes and shift_type_id together", name)
	}
}

func TestSeriesPayloadMapping(t *testing.T) {
	t.Parallel()

	var request SeriesRequest
	require.NoError(t, json.Unmarshal([]byte(`{"staff_id": 5, "weekdays": [1, 3], "start_time": "09:00", "end_time": "12:00", "break_minutes": 15, "calendar_period_id": 8, "valid_from": "2026-09-01", "valid_until": "2026-12-01"}`), &request))
	input, err := buildSeries(request)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 3}, input.Weekdays)
	assert.Equal(t, workforce.WeekPatternEvery, input.WeekPattern, "an omitted pattern means every week")
	assert.Equal(t, "2026-12-01", input.ValidUntil)

	_, err = toWeekdays([]int{1, 65538})
	assert.EqualError(t, err, "weekdays must be between 1 (Monday) and 7 (Sunday)")
	kept, err := toWeekdays(nil)
	require.NoError(t, err)
	assert.Nil(t, kept, "omitted weekdays keep the predecessor's rule on split")

	var split SeriesRequest
	require.NoError(t, json.Unmarshal([]byte(`{"effective_date": "2026-10-05", "start_time": "10:00", "end_time": "13:00", "occurrence_shift_id": "9007199254740993", "valid_until": null}`), &split))
	splitInput, err := buildSplit(11, split)
	require.NoError(t, err)
	assert.Equal(t, int64(9007199254740993), splitInput.OccurrenceShiftID)
	assert.True(t, splitInput.ValidUntilSet)
	assert.Empty(t, splitInput.ValidUntil)
	assert.False(t, splitInput.ShiftTypeIDSet)
}

func TestWireFormatKeepsBigintIdentifiersAsStrings(t *testing.T) {
	t.Parallel()

	seriesID := int64(9223372036854775807)
	originShiftID := int64(9223372036854775805)
	encoded, err := json.Marshal(ToShiftResponse(workforce.PlannedShift{StaffShift: workforce.StaffShift{
		ID: 9223372036854775806, SeriesID: &seriesID, OriginShiftID: &originShiftID, SeriesOccurrenceDate: "2026-07-06",
		StartTime: "08:00:00", EndTime: "16:30:00",
	}}))
	require.NoError(t, err)
	var body map[string]any
	require.NoError(t, json.Unmarshal(encoded, &body))
	assert.Equal(t, "9223372036854775806", body["id"])
	assert.Equal(t, "9223372036854775807", body["series_id"])
	assert.Equal(t, "9223372036854775805", body["origin_shift_id"])
	assert.Equal(t, "2026-07-06", body["series_occurrence_date"])
	assert.Equal(t, "08:00", body["start_time"])
	assert.Equal(t, "16:30", body["end_time"])

	encoded, err = json.Marshal(toSeriesResponse(workforce.StaffShiftSeriesResult{SeriesID: 9223372036854775807, OldSeriesID: 9223372036854775806}))
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"series_id":"9223372036854775807"`)
	assert.Contains(t, string(encoded), `"old_series_id":"9223372036854775806"`)
	assert.Contains(t, string(encoded), `"skipped_dates":[]`)

	detail := toSeriesDetailResponse(workforce.StaffShiftSeries{ID: 11, Weekdays: []int{1, 3}, StartTime: "09:00:00", EndTime: "12:00:00", ValidFrom: "2026-09-01"})
	assert.Equal(t, "09:00", detail.StartTime)
	assert.Nil(t, detail.ValidUntil)
	encoded, err = json.Marshal(detail)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"id":"11"`)
}

func TestClassifyMapsCapabilityErrors(t *testing.T) {
	t.Parallel()

	assert.Equal(t, FailureConflict, classify(workforce.ErrStaffShiftOverlap))
	assert.Equal(t, FailureNotFound, classify(workforce.ErrShiftSeriesNotFound))
	assert.Equal(t, FailureInvalid, classify(&workforce.InvalidStaffShiftError{Reason: "x"}))
	assert.Equal(t, FailureInvalid, classify(workforce.ErrShiftTypeNotFound), "an unknown type on a shift is client input")
	assert.Equal(t, FailureForbidden, classify(workforce.ErrPlanExportForbidden))
	assert.Equal(t, FailureInternal, classify(assert.AnError))
}
