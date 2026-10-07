package timetablehttp

import (
	"encoding/json"
	"net/http"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestApplyDeviations_RefusalCodes drives the Vertretungsplan save through
// its router: a refused save keeps its 400 and now names its code and the
// request field the form marks (#2516).
func TestApplyDeviations_RefusalCodes(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := devRouter(s.ctx, s.res, s.db)
	_, date := futureSubDate(1)
	inst := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: "Codes"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})

	cases := []struct {
		name      string
		body      map[string]any
		wantCode  string
		wantField string
	}{
		{
			name: "self substitution",
			body: map[string]any{"substitutions": []map[string]any{
				{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffA},
			}},
			wantCode: "timetable.substitute_self", wantField: "substitute_staff_id",
		},
		{
			name: "two substitutes for one absence",
			body: map[string]any{"substitutions": []map[string]any{
				{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffX},
				{"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY},
			}},
			wantCode: "timetable.substitute_single_only", wantField: "substitutions",
		},
		{
			name: "empty appointment scope",
			body: map[string]any{"absences": []map[string]any{
				{"staff_id": s.staffA, "instance_ids": []int64{}},
			}},
			wantCode: "timetable.instances_required", wantField: "instance_ids",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := doDev(t, router, inst.ID, tc.body)
			require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
			var body struct {
				Code   string `json:"code"`
				Errors []struct {
					Field string `json:"field"`
				} `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.wantCode, body.Code)
			require.Len(t, body.Errors, 1)
			assert.Equal(t, tc.wantField, body.Errors[0].Field)
		})
	}
}

// TestBulkSubstitution_RefusalCodes drives the Sammel-Vertretung through its
// router: the refusal names its code, its field and the failing day as
// details.date in YYYY-MM-DD (#2516).
func TestBulkSubstitution_RefusalCodes(t *testing.T) {
	t.Parallel()

	s := buildDevModule(t)
	router := bulkRouter(s.ctx, s.res, s.db)
	d1Str, d1 := futureSubDate(1)
	d2Str, d2 := futureSubDate(2)
	inst1 := testpkg.CreateTestActivityInstance(t, s.db, d1, s.roomID, testpkg.ActivityInstanceOpts{Title: "Ok-Tag"})
	inst2 := testpkg.CreateTestActivityInstance(t, s.db, d2, s.roomID, testpkg.ActivityInstanceOpts{Title: "Konflikt-Tag"})
	testpkg.CreateTestInstanceStaff(t, s.db, inst1.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffA, testpkg.InstanceStaffOpts{})
	testpkg.CreateTestInstanceStaff(t, s.db, inst2.ID, s.staffY, testpkg.InstanceStaffOpts{IsAbsent: true})

	type refusal struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
		Errors  []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	decode := func(t *testing.T, raw []byte) refusal {
		t.Helper()
		var body refusal
		require.NoError(t, json.Unmarshal(raw, &body))
		return body
	}

	w := doBulk(t, router, map[string]any{
		"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY, "dates": []string{d1Str, d2Str},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	body := decode(t, w.Body.Bytes())
	assert.Equal(t, "timetable.substitute_absent_on_date", body.Code)
	assert.Equal(t, map[string]any{"date": d2.Format("02.01.2006")}, body.Details)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "substitute_staff_id", body.Errors[0].Field)

	w = doBulk(t, router, map[string]any{
		"absent_staff_id": s.staffA, "substitute_staff_id": s.staffA, "dates": []string{d1Str},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "timetable.substitute_self", decode(t, w.Body.Bytes()).Code)

	w = doBulk(t, router, map[string]any{
		"absent_staff_id": s.staffA, "substitute_staff_id": s.staffY, "dates": []string{},
	})
	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, "timetable.dates_required", decode(t, w.Body.Bytes()).Code)
}
