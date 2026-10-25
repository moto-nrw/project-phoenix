package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedExternalAGBodyPlansTheExternalAsOnlyAndResponsibleStaff(t *testing.T) {
	t.Parallel()

	students := make([]int64, 20)
	for index := range students {
		students[index] = int64(200 + index)
	}
	thursday := seedDate{Time: time.Date(2026, time.October, 22, 0, 0, 0, 0, time.UTC)}
	saturday := seedDate{Time: time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)}

	body := seedExternalAGBody(thursday, 11, 12, 13, students)

	assert.Equal(t, "AG Musik", body["name"])
	assert.Equal(t, "activity", body["type"])
	assert.Equal(t, 0, body["week_pattern"], "weekly")
	assert.Equal(t, []int{4}, body["weekdays"], "on the weekday of the seed")
	assert.Equal(t, []int64{13}, body["staff_ids"])
	assert.Equal(t, int64(13), body["primary_staff_id"])
	assert.Equal(t, int64(11), body["room_id"])
	assert.Equal(t, int64(12), body["category_id"])
	assert.Equal(t, students[12:], body["student_ids"])
	assert.Equal(t, "2026-10-22", body["materialize_from"])
	assert.Equal(t, []int{1}, seedExternalAGBody(saturday, 11, 12, 13, students)["weekdays"], "a weekend seed plans Monday")
}

func TestSeedExternalAGStepPlansTheAcceptedExternal(t *testing.T) {
	t.Parallel()

	var templates []map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		w.Header().Set("Content-Type", "application/json")
		require.Equal(t, "/api/timetable/templates", r.URL.Path)
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		templates = append(templates, body)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"template_id":41}}`)
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.roomIDs = map[string]int64{"Musikraum": 24}
	fs.categoryIDs = map[string]int64{"Musik": 25}
	for index := range 10 {
		fs.studentIDByIndex[index] = int64(300 + index)
	}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, Values: map[string]any{
		seedExternalCaregiverIDsKey: map[string]int64{"Jonas Becker": 77, "Leonie Wagner": 78},
	}}

	require.NoError(t, seedExternalAGStep{}.Run(t.Context(), rt))

	require.Len(t, templates, 1)
	assert.Equal(t, []any{float64(77)}, templates[0]["staff_ids"])
	assert.Equal(t, float64(77), templates[0]["primary_staff_id"])
	assert.Equal(t, float64(24), templates[0]["room_id"])
}

func TestSeedExternalAGStepFailsWithoutTheExternal(t *testing.T) {
	t.Parallel()

	fs := NewFixedSeeder(newTestClient("http://127.0.0.1:1", false), false, "")
	fs.roomIDs = map[string]int64{"Musikraum": 24}
	fs.categoryIDs = map[string]int64{"Musik": 25}
	rt := &Runtime{Client: fs.client, FixedSeeder: fs, Values: map[string]any{}}

	require.Error(t, seedExternalAGStep{}.Run(t.Context(), rt))
}
