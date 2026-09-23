package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type requestDiffBoundary struct {
	modes                map[string][]string
	arrivals             []*careplan.ArrivalSchedule
	exception, effective *time.Time
	fail                 error
	calls                []string
}

func (b *requestDiffBoundary) DepartureModesForStudent(context.Context, int64) (map[string][]string, error) {
	b.calls = append(b.calls, "student")
	return b.modes, b.fail
}
func (b *requestDiffBoundary) GetStudentArrivalSchedules(context.Context, int64) ([]*careplan.ArrivalSchedule, error) {
	b.calls = append(b.calls, "arrivals")
	return b.arrivals, nil
}
func (b *requestDiffBoundary) GetStudentPickupSchedules(context.Context, int64) ([]*careplan.PickupSchedule, error) {
	b.calls = append(b.calls, "pickups")
	return nil, nil
}
func (b *requestDiffBoundary) ExceptionPickupTime(context.Context, int64, careplan.Date) (*time.Time, error) {
	b.calls = append(b.calls, "exception")
	return b.exception, b.fail
}
func (b *requestDiffBoundary) EffectivePickupTime(context.Context, int64, careplan.Date) (*time.Time, error) {
	b.calls = append(b.calls, "effective")
	return b.effective, b.fail
}

func TestNativeRequestDiffKeepsUntimedCareDaysAndDepartureModes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Diff", "Modes", "1a")
	b := &requestDiffBoundary{modes: map[string][]string{"mon": nil, "tue": {"bus", "pickup"}}, arrivals: []*careplan.ArrivalSchedule{{StudentID: student.ID, Weekday: 1}}}
	service, err := compose.NewRequestDiffs(b, b, b, nil)
	require.NoError(t, err)
	request := &carerequests.Request{StudentID: student.ID, RequestKind: "weekly_schedule", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"scheduled":false,"mode":"pickup"},{"weekday":2,"mode":"alone"}]}`)}
	diff, err := service.Weekly(testpkg.Ctx(t), student.ID, request.Payload)
	require.NoError(t, err)
	require.Len(t, diff, 3)
	require.Equal(t, carerequests.KindScheduled, diff[0].CareKind)
	require.NotEqual(t, diff[0].Old, diff[0].New, "untimed arrival still identifies a scheduled day")
	require.NotEqual(t, "Keine Angaben", diff[0].Old)
	require.Equal(t, []string{"alone"}, diff[1].OldModes)
	require.Equal(t, []string{"bus", "pickup"}, diff[2].OldModes)
	require.Equal(t, []string{"student", "arrivals", "pickups"}, b.calls)
	snapshot := service.Snapshot(testpkg.Ctx(t), request)
	require.NotNil(t, snapshot)
	b.modes["tue"][0] = "alone"
	require.Equal(t, []string{"bus", "pickup"}, snapshot.Entries()[2].OldModes)
	b.fail = errors.New("directory unavailable")
	require.Nil(t, service.Snapshot(testpkg.Ctx(t), request), "presentation failure must not block the decision")
}

func TestNativePickupSnapshotPrefersStoredTimeThenExceptionThenEffective(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Diff", "Pickup", "1a")
	exception := time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC)
	effective := time.Date(1, 1, 1, 16, 0, 0, 0, time.UTC)
	b := &requestDiffBoundary{exception: &exception, effective: &effective}
	service, err := compose.NewRequestDiffs(b, b, b, nil)
	require.NoError(t, err)
	request := &carerequests.Request{StudentID: student.ID, RequestKind: "pickup_change", Payload: json.RawMessage(`{"date":"2031-02-03","pickup_time":"14:00","reason":"Termin","previous_pickup_time":"13:00"}`)}
	snapshot := service.Snapshot(testpkg.Ctx(t), request)
	require.NotNil(t, snapshot)
	require.Equal(t, "13:00", snapshot.Entries()[0].Old)
	require.Empty(t, b.calls, "frozen submission baseline needs no live query")
	request.Payload = json.RawMessage(`{"date":"2031-02-03","pickup_time":"14:00","reason":"Termin"}`)
	snapshot = service.Snapshot(testpkg.Ctx(t), request)
	require.NotNil(t, snapshot)
	require.Equal(t, "15:00", snapshot.Entries()[0].Old)
	require.Equal(t, []string{"exception"}, b.calls)
	b.exception, b.calls = nil, nil
	snapshot = service.Snapshot(testpkg.Ctx(t), request)
	require.NotNil(t, snapshot)
	require.Equal(t, "16:00", snapshot.Entries()[0].Old)
	require.Equal(t, []string{"exception", "effective"}, b.calls)
	b.fail, b.calls = errors.New("exception unavailable"), nil
	require.Nil(t, service.Snapshot(testpkg.Ctx(t), request))
	require.Equal(t, []string{"exception"}, b.calls, "a failed query must not silently select another baseline")
}
