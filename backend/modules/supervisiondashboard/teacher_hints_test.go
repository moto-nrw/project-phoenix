package supervisiondashboard

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teacherHintReader struct {
	hints []TeacherHint
	err   error
	asked [][]int64
	dates []Date
}

func (r *teacherHintReader) TeacherHints(_ context.Context, ids []int64, date Date) ([]TeacherHint, error) {
	r.asked = append(r.asked, ids)
	r.dates = append(r.dates, date)
	return r.hints, r.err
}

// TestLoadTeacherHintsSharesPlanningGate pins that the Lehrkraft's day hints
// (#4795) share the gate of the planning times: users:read and full access,
// for the children in the projection and the business day.
func TestLoadTeacherHintsSharesPlanningGate(t *testing.T) {
	t.Parallel()
	const date Date = "2026-08-19"
	hint := TeacherHint{StudentID: 42, Body: "Hat Bauchweh", AuthorName: "Frau Kurz"}

	reader := &teacherHintReader{hints: []TeacherHint{hint}}
	svc := newService(&fakes{})
	svc.deps.TeacherHints = reader
	projection := emptyProjection()
	require.NoError(t, svc.loadTeacherHints(context.Background(), projection, Caller{CanReadStudents: true}, []int64{42, 43}, true, date))
	assert.Equal(t, []TeacherHint{hint}, projection.TeacherHints)
	assert.Equal(t, [][]int64{{42, 43}}, reader.asked)
	assert.Equal(t, []Date{date}, reader.dates)

	for name, gate := range map[string]struct {
		caller     Caller
		fullAccess bool
	}{
		"without users:read":  {Caller{}, true},
		"without full access": {Caller{CanReadStudents: true}, false},
	} {
		reader := &teacherHintReader{hints: []TeacherHint{hint}}
		svc := newService(&fakes{})
		svc.deps.TeacherHints = reader
		projection := emptyProjection()
		require.NoError(t, svc.loadTeacherHints(context.Background(), projection, gate.caller, []int64{42}, gate.fullAccess, date), name)
		assert.Empty(t, projection.TeacherHints, name)
		assert.Empty(t, reader.asked, name)
	}

	failure := errors.New("directory unavailable")
	svc = newService(&fakes{})
	svc.deps.TeacherHints = &teacherHintReader{err: failure}
	err := svc.loadTeacherHints(context.Background(), emptyProjection(), Caller{CanReadStudents: true}, []int64{42}, true, date)
	require.ErrorIs(t, err, failure)
}

// TestDashboardAttachesTeacherHints pins that the dashboard itself loads the
// Lehrkraft's hints (#4795) for the children it shows, not only the helper.
func TestDashboardAttachesTeacherHints(t *testing.T) {
	t.Parallel()
	yardGroupID := int64(30)
	hint := TeacherHint{StudentID: 1, Body: "Hat Bauchweh", AuthorName: "Frau Kurz"}
	reader := &teacherHintReader{hints: []TeacherHint{hint}}
	svc := newService(&fakes{
		currentStaffID:    func(context.Context) (*int64, error) { return int64Ptr(77), nil },
		caller:            func(context.Context) (Caller, error) { return Caller{CanReadStudents: true}, nil },
		fullStudentAccess: func(context.Context) (bool, error) { return true, nil },
		supervised:        func(context.Context) ([]Session, error) { return []Session{{ID: 10, RoomName: "Adler"}}, nil },
		yard: func(context.Context, int64) (*SchulhofStatus, error) {
			return &SchulhofStatus{Exists: true, IsUserSupervising: true, ActiveGroupID: &yardGroupID}, nil
		},
		groupVisits: func(_ context.Context, activeGroupID int64) ([]VisitRecord, error) {
			return []VisitRecord{{StudentID: 1, ActiveGroupID: activeGroupID, FirstName: "Erika", LastName: "Muster"}}, nil
		},
	})
	svc.deps.TeacherHints = reader

	projection, err := svc.Dashboard(context.Background(), yardGroupID)

	require.NoError(t, err)
	assert.Equal(t, []TeacherHint{hint}, projection.TeacherHints)
	assert.Equal(t, [][]int64{{1}}, reader.asked)
}
