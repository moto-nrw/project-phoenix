package api

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nativeParentExceptionSource struct {
	owners.ParentCareExceptionSource
	context context.Context
	filter  careplan.StudentScheduleFilter
	rows    []careplan.PickupException
	err     error
}

func (s *nativeParentExceptionSource) ListPickupExceptions(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
	if ctx != s.context {
		return nil, errors.New("parent exception adapter changed the caller context")
	}
	s.filter = filter
	return s.rows, s.err
}

func TestParentCarePlanNativeExceptionReadRetainsOwnerSemantics(t *testing.T) {
	t.Parallel()
	day := calendar.NewDate(2026, time.October, 17)
	clock := calendar.NormalizeWallClock(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC))
	actor := int64(19)
	reason := "partial absence"
	created := time.Date(2026, 10, 16, 10, 12, 0, 0, time.UTC)
	cause := errors.New("owner read failure")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	source := &nativeParentExceptionSource{context: ctx, rows: []careplan.PickupException{{ID: 11, TenantID: 13, CreatedAt: created, UpdatedAt: created.Add(time.Minute), StudentID: 17, ExceptionDate: careplan.Date(day), PickupTime: &clock, Reason: &reason, ExcusedFrom: &clock, ExcusedReason: &reason, ExcusedCreatedBy: &actor, ExcusedOwnsPickupTime: true, Source: careplan.ExceptionSourceGuardian, CreatedBy: actor, CreatedByGuardian: &actor}}, err: cause}
	port := owners.NewParentCareExceptions(source)
	rows, err := readParentCareExceptionFixture(t, port.ListPickupExceptions, ctx, day)
	require.ErrorIs(t, err, cause)
	require.Len(t, rows, 1)
	assert.Equal(t, []int64{17}, source.filter.StudentIDs)
	assert.Equal(t, careplan.Date(day), source.filter.Date)
	assert.Equal(t, int64(11), rows[0].ID)
	assert.Equal(t, int64(13), rows[0].TenantID)
	assert.Equal(t, created, rows[0].CreatedAt)
	assert.Equal(t, created.Add(time.Minute), rows[0].UpdatedAt)
	assert.Equal(t, day, rows[0].ExceptionDate)
	assert.Equal(t, &actor, rows[0].CreatedByGuardian)
	assert.Equal(t, source.rows[0].HasManualPartialAbsence(), rows[0].HasManualPartialAbsence())
	source.rows = nil
	source.err = nil
	rows, err = readParentCareExceptionFixture(t, port.ListPickupExceptions, ctx, day)
	require.NoError(t, err)
	assert.Nil(t, rows)
	source.rows = []careplan.PickupException{}
	rows, err = readParentCareExceptionFixture(t, port.ListPickupExceptions, ctx, day)
	require.NoError(t, err)
	require.NotNil(t, rows)
	assert.Empty(t, rows)
}

type nativeParentOfferingSource struct {
	careplan.OfferingChangeRequests
	careplan.CourseRequests
	view *careplan.OfferingChangeView
}

func (s nativeParentOfferingSource) GetForStudent(context.Context, int64) (*careplan.OfferingChangeView, error) {
	return s.view, nil
}
func TestParentCarePlanNativeRequestRetainsRawSnapshotsAndEmptyRelations(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{"large":9007199254740993,"nested":{"value":9223372036854775807}}`)
	source := nativeParentOfferingSource{view: &careplan.OfferingChangeView{Request: &careplan.OfferingChangeRequest{ID: 23, Payload: raw, DecisionSnapshot: json.RawMessage(`{}`)}, Diff: []careplan.OfferingChangeDiffEntry{}, LastDecision: &careplan.OfferingChangeDecision{Requested: []careplan.OfferingChangeRequestedItem{}, AppliedDiff: nil}}}
	port := owners.NewParentOfferingChangeRequests(source)
	view, err := port.GetForStudent(context.Background(), 31)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.Request)
	assert.Equal(t, int64(23), view.Request.ID)
	assert.Equal(t, raw, view.Request.Payload)
	assert.Equal(t, json.RawMessage(`{}`), view.Request.DecisionSnapshot)
	assert.NotNil(t, view.Diff)
	assert.Empty(t, view.Diff)
	require.NotNil(t, view.LastDecision)
	assert.NotNil(t, view.LastDecision.Requested)
	assert.Nil(t, view.LastDecision.AppliedDiff)
}
func TestParentCarePlanCutoffBindingKeepsLazyClockAndCanonicalErrors(t *testing.T) {
	t.Parallel()
	policy := owners.NewParentCarePlanPolicy()
	now := time.Date(2026, 10, 17, 11, 0, 0, 0, calendar.Berlin)
	cutoff, err := policy.NewSameDayCutoffAt(" 11:00 ", func() time.Time { return now })
	require.NoError(t, err)
	assert.Equal(t, "11:00", cutoff.Clock)
	assert.False(t, cutoff.Closed(calendar.DateFromTime(now)))
	now = now.Add(time.Second)
	assert.True(t, cutoff.Closed(calendar.DateFromTime(now)))
	assert.False(t, cutoff.Closed(calendar.DateFromTime(now).AddDays(1)))
	_, err = policy.NewSameDayCutoffAt("11:00", nil)
	_, canonical := careplan.NewSameDayCutoffAt("11:00", nil)
	require.EqualError(t, err, canonical.Error())
	assert.Same(t, careplan.ErrCourseRequestsDisabled, policy.Error("CourseRequestsDisabled"))
	assert.Same(t, careplan.ErrStudentDataRequestFieldPending, policy.Error("StudentDataRequestFieldPending"))
}

// The fixture's inferred type is the consumer's DTO. JSON constructs only its
// native filter values, without granting API tests an application import.
func readParentCareExceptionFixture[T, R any](t *testing.T, read func(context.Context, T) ([]R, error), ctx context.Context, day calendar.Date) ([]R, error) {
	t.Helper()
	raw, err := json.Marshal(struct {
		StudentIDs []int64
		Date       calendar.Date
	}{[]int64{17}, day})
	require.NoError(t, err)
	var filter T
	require.NoError(t, json.Unmarshal(raw, &filter))
	return read(ctx, filter)
}
