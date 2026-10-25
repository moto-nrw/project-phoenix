package timetablehttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type rosterTeacherHints struct {
	hints map[int64][]timetable.OperationTeacherHint
	err   error
	asked [][]int64
	days  []calendar.Date
}

func (f *rosterTeacherHints) TeacherHints(_ context.Context, ids []int64, day calendar.Date) (map[int64][]timetable.OperationTeacherHint, error) {
	f.asked = append(f.asked, ids)
	f.days = append(f.days, day)
	return f.hints, f.err
}

// rosterStaff answers whether the caller is verified staff of the tenant.
type rosterStaff bool

func (s rosterStaff) HasCurrentStaff(context.Context) (bool, error) { return bool(s), nil }

var hintToday = calendar.NewDate(2099, 3, 2)

func hintResource(hints TeacherHints, staff bool) *Resource {
	return newResourceForTest(Dependencies{
		TeacherHints:       hints,
		UserContextService: rosterStaff(staff),
		Now:                func() time.Time { return hintToday.BerlinMidnight().Add(10 * time.Hour) },
	})
}

func hintRoster(date calendar.Date) *timetable.OperationRoster {
	return &timetable.OperationRoster{
		Instance: timetable.OperationRosterInstance{Date: date.String()},
		Rows:     []timetable.OperationRosterRow{{StudentID: 7}, {StudentID: 8}},
	}
}

// TestRosterTeacherHints pins the block roster side of #4795: today's hints
// of the Lehrkraft reach the rows of a roster of today, for staff with
// users:read only, on every response that carries the roster.
func TestRosterTeacherHints(t *testing.T) {
	t.Parallel()
	hint := timetable.OperationTeacherHint{Body: "Hat Bauchweh", AuthorName: "Lena Lehrerin"}
	reading := testpkg.WithPermissionsContext(context.Background(), []string{"users:read"})
	withHint := func() *rosterTeacherHints {
		return &rosterTeacherHints{hints: map[int64][]timetable.OperationTeacherHint{7: {hint}}}
	}

	t.Run("today for staff with users:read", func(t *testing.T) {
		t.Parallel()
		hints := withHint()
		got := hintRoster(hintToday)
		require.NoError(t, hintResource(hints, true).presentRoster(reading, got))
		assert.Equal(t, []timetable.OperationTeacherHint{hint}, got.Rows[0].TeacherHints)
		assert.Empty(t, got.Rows[1].TeacherHints)
		assert.Equal(t, [][]int64{{7, 8}}, hints.asked)
		assert.Equal(t, []calendar.Date{hintToday}, hints.days)
	})

	t.Run("another day shows none", func(t *testing.T) {
		t.Parallel()
		for _, day := range []calendar.Date{hintToday.AddDays(-1), hintToday.AddDays(1)} {
			hints := withHint()
			got := hintRoster(day)
			require.NoError(t, hintResource(hints, true).presentRoster(reading, got))
			assert.Empty(t, got.Rows[0].TeacherHints, day)
			assert.Empty(t, hints.asked, day)
		}
	})

	t.Run("without unredacted student access none", func(t *testing.T) {
		t.Parallel()
		hints := withHint()
		got := hintRoster(hintToday)
		require.NoError(t, hintResource(hints, false).presentRoster(reading, got))
		assert.Empty(t, got.Rows[0].TeacherHints)
		assert.Empty(t, hints.asked)
	})

	t.Run("without users:read none", func(t *testing.T) {
		t.Parallel()
		// The school portal's own supervisions carry no users:read.
		hints := withHint()
		got := hintRoster(hintToday)
		school := testpkg.WithPermissionsContext(context.Background(), []string{"supervision:own"})
		require.NoError(t, hintResource(hints, true).presentRoster(school, got))
		assert.Empty(t, got.Rows[0].TeacherHints)
		assert.Empty(t, hints.asked)
	})

	t.Run("a failed read fails the response", func(t *testing.T) {
		t.Parallel()
		failure := errors.New("directory unavailable")
		err := hintResource(&rosterTeacherHints{err: failure}, true).presentRoster(reading, hintRoster(hintToday))
		require.ErrorIs(t, err, failure)
	})

	t.Run("without the port none", func(t *testing.T) {
		t.Parallel()
		require.NoError(t, hintResource(nil, true).presentRoster(reading, hintRoster(hintToday)))
	})
}

// TestRosterResponsesCarryTeacherHints pins that every response carrying the
// roster carries today's hints: the client puts a check-in's roster in place
// of the one it shows, so a response without them would hide every hint.
func TestRosterResponsesCarryTeacherHints(t *testing.T) {
	t.Parallel()
	hints := &rosterTeacherHints{hints: map[int64][]timetable.OperationTeacherHint{
		350: {{Body: "Hat Bauchweh", AuthorName: "Lena Lehrerin"}},
	}}
	resource := hintResource(hints, true)
	resource.OperationsService = &fakeOperationsService{roster: &timetable.OperationRoster{
		Instance: timetable.OperationRosterInstance{Date: hintToday.String()},
		Rows:     []timetable.OperationRosterRow{{StudentID: 350}},
	}}
	cases := []struct {
		method, pattern, path string
		body                  any
		handler               http.HandlerFunc
	}{
		{http.MethodGet, "/instances/{id}/roster", "/instances/250/roster", nil, resource.operationsRoster},
		{http.MethodGet, "/active-groups/{id}/roster", "/active-groups/40/roster", nil, resource.operationsRosterByActiveGroup},
		{http.MethodPost, "/instances/{id}/students/{student_id}/check-in", "/instances/250/students/350/check-in", nil, resource.operationsCheckInStudent},
		{http.MethodPost, "/instances/{id}/students/check-in", "/instances/250/students/check-in", map[string]any{"student_ids": []string{"350"}}, resource.operationsCheckInStudents},
		{http.MethodPost, "/instances/{id}/students/{student_id}/check-out", "/instances/250/students/350/check-out", nil, resource.operationsCheckOutStudent},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			router := operationRouter(tc.method, tc.pattern, tc.handler)
			response := executeOperationRequest(t, router, tc.method, tc.path, tc.body)

			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			var body struct {
				Data timetable.OperationRoster `json:"data"`
			}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
			require.Len(t, body.Data.Rows, 1)
			assert.Equal(t, hints.hints[350], body.Data.Rows[0].TeacherHints)
		})
	}
}
