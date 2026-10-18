package careexit_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type bookingApplications struct {
	links   []enrollment.CareExitApplicationLink
	choices []enrollment.SubmittedOfferingChoice
}

func (a bookingApplications) CareExitApplicationLinks(context.Context, []int64) ([]enrollment.CareExitApplicationLink, error) {
	return a.links, nil
}

func (a bookingApplications) SubmittedOfferingChoices(context.Context, []int64) ([]enrollment.SubmittedOfferingChoice, error) {
	return a.choices, nil
}

type bookingStore struct {
	history  []careplan.CareOfferingBooking
	historyQ []int64
	ended    []int64
	restores []careplan.CareOfferingBookingRestore
}

func (s *bookingStore) CareOfferingBookingHistory(_ context.Context, childIDs []int64) ([]careplan.CareOfferingBooking, error) {
	s.historyQ = childIDs
	var result []careplan.CareOfferingBooking
	for _, booking := range s.history {
		if slices.Contains(childIDs, booking.RequestChildID) {
			result = append(result, booking)
		}
	}
	return result, nil
}

func (s *bookingStore) LockCareOfferingBookings(context.Context, []int64) error { return nil }

func (s *bookingStore) EndCareOfferingBookings(_ context.Context, childIDs []int64, _ careplan.Date) (int64, error) {
	s.ended = childIDs
	return int64(len(childIDs)), nil
}

func (s *bookingStore) RestoreCareOfferingBookings(_ context.Context, restores []careplan.CareOfferingBookingRestore) (int64, error) {
	s.restores = restores
	return int64(len(restores)), nil
}

func tenantOf(id int64) func(context.Context) int64 { return func(context.Context) int64 { return id } }

func TestBookingSnapshotsSelectTheRunningBookingsOfTheSchool(t *testing.T) {
	t.Parallel()
	student, otherStudent := int64(11), int64(12)
	note := "Mittagessen"
	before, until, after := careplan.Date("2026-09-01"), careplan.Date("2026-10-19"), careplan.Date("2026-11-01")
	store := &bookingStore{history: []careplan.CareOfferingBooking{
		{ID: 103, TenantID: 50, RequestChildID: 21, CareOfferingID: 31, ManualSelectedDays: []string{"mon"}},
		{ID: 101, TenantID: 50, RequestChildID: 21, CareOfferingID: 32, ValidFrom: &after},
		{ID: 102, TenantID: 50, RequestChildID: 21, CareOfferingID: 33, ValidUntil: &until},
		{ID: 104, TenantID: 60, RequestChildID: 21, CareOfferingID: 34},
		{ID: 105, TenantID: 50, RequestChildID: 21, CareOfferingID: 35, ValidFrom: &before},
	}}
	applications := bookingApplications{
		links: []enrollment.CareExitApplicationLink{
			{ID: 21, CreatedStudentID: &student}, {ID: 22, CreatedStudentID: &otherStudent}, {ID: 23},
		},
		choices: []enrollment.SubmittedOfferingChoice{{RequestChildID: 21, CareOfferingID: 31, Notes: &note}},
	}
	bookings := careexit.NewBookings(applications, store, tenantOf(50))

	snapshots, err := bookings.CareExitOfferingSnapshots(context.Background(), []int64{student}, enrollment.Date(until), nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{21}, store.historyQ, "only the children created for the selected students")
	require.Len(t, snapshots, 3, "an ended booking and another school's booking are not part of the exit")
	assert.Equal(t, []int64{101, 103, 105}, []int64{snapshots[0].SourceRowID, snapshots[1].SourceRowID, snapshots[2].SourceRowID})
	assert.True(t, snapshots[0].WasDeleted, "a booking that starts after the end is deleted, not capped")
	assert.False(t, snapshots[1].WasDeleted)
	assert.False(t, snapshots[2].WasDeleted)
	assert.Equal(t, student, snapshots[1].StudentID)
	var row enrollment.RequestChildOffering
	require.NoError(t, json.Unmarshal(snapshots[1].Snapshot, &row))
	assert.Equal(t, &note, row.Notes)
	assert.Equal(t, []string{"mon"}, row.SelectedDays)

	none, err := bookings.CareExitOfferingSnapshots(context.Background(), []int64{student}, enrollment.Date(until), new(int64(22)))
	require.NoError(t, err)
	assert.Empty(t, none, "a source child of another student narrows the snapshot to nothing")
	empty, err := bookings.CareExitOfferingSnapshots(context.Background(), nil, enrollment.Date(until), nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestBookingEndHonoursTheSourceChild(t *testing.T) {
	t.Parallel()
	store := &bookingStore{}
	bookings := careexit.NewBookings(bookingApplications{}, store, tenantOf(50))

	changed, err := bookings.EndCareExitOfferingLinks(context.Background(), []int64{21, 22}, new(int64(22)), "2026-10-19")
	require.NoError(t, err)
	assert.EqualValues(t, 1, changed)
	assert.Equal(t, []int64{22}, store.ended)

	store.ended = nil
	changed, err = bookings.EndCareExitOfferingLinks(context.Background(), []int64{21}, new(int64(22)), "2026-10-19")
	require.NoError(t, err)
	assert.Zero(t, changed)
	assert.Nil(t, store.ended, "a source child outside the exit ends nothing")
}

func TestBookingRestoreReplaysTheSnapshots(t *testing.T) {
	t.Parallel()
	store := &bookingStore{}
	bookings := careexit.NewBookings(bookingApplications{}, store, tenantOf(50))
	legacy, err := json.Marshal(enrollment.RequestChildOffering{ID: 201, TenantID: 50, RequestChildID: 21, CareOfferingID: 31, SelectedDays: []string{"tue"}})
	require.NoError(t, err)
	split, err := json.Marshal(enrollment.RequestChildOffering{ID: 202, TenantID: 50, RequestChildID: 21, CareOfferingID: 32,
		SelectedDays: []string{"wed"}, AutomaticSelectedDays: []string{"thu"}})
	require.NoError(t, err)

	restored, err := bookings.RestoreCareExitOfferingLinks(context.Background(), []enrollment.CareExitOfferingSnapshotRestore{
		{SourceRowID: 201, WasDeleted: true, Snapshot: legacy}, {SourceRowID: 202, Snapshot: split},
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, restored)
	assert.True(t, store.restores[0].WasDeleted)
	assert.Equal(t, []string{"tue"}, store.restores[0].Booking.ManualSelectedDays, "a snapshot before the day split restores its days as manual ones")
	assert.Empty(t, store.restores[1].Booking.ManualSelectedDays)
	assert.Equal(t, []string{"thu"}, store.restores[1].Booking.AutomaticSelectedDays)

	_, err = bookings.RestoreCareExitOfferingLinks(context.Background(), []enrollment.CareExitOfferingSnapshotRestore{{SourceRowID: 999, Snapshot: legacy}})
	require.EqualError(t, err, "care-exit snapshot booking identity mismatch")
	_, err = bookings.RestoreCareExitOfferingLinks(context.Background(), []enrollment.CareExitOfferingSnapshotRestore{{SourceRowID: 201, Snapshot: json.RawMessage(`{`)}})
	require.ErrorContains(t, err, "decode care-exit offering snapshot")
}

type pickupExtensionTimetable struct {
	timetable.PickupExtensionCapability
	resolution timetable.PickupExtensionResolution
}

func (p pickupExtensionTimetable) ResolvePickupExtension(context.Context, int64, []int64) (timetable.PickupExtensionResolution, error) {
	return p.resolution, nil
}

type pickupExtensionRules struct {
	applied []string
	fail    error
}

func (r *pickupExtensionRules) ApplyActiveStatusDaysForInstance(_ context.Context, _ int64, date string) (int, error) {
	r.applied = append(r.applied, "status "+date)
	return 0, r.fail
}

func (r *pickupExtensionRules) ApplyActivePartialAbsencesForInstance(_ context.Context, _ int64, date string) (int, error) {
	r.applied = append(r.applied, "partial "+date)
	return 0, nil
}

func TestPickupExtensionReappliesTheReportedAttendancePerBlock(t *testing.T) {
	t.Parallel()
	resolution := timetable.PickupExtensionResolution{StudentID: 11, Instances: []timetable.PickupExtensionInstance{
		{ID: 301, Date: "2026-10-19"}, {ID: 302, Date: "2026-10-20"},
	}}
	rules := &pickupExtensionRules{}
	extensions, err := careexit.NewPickupExtensions(pickupExtensionTimetable{resolution: resolution}, rules)
	require.NoError(t, err)

	got, err := extensions.ResolvePickupExtension(context.Background(), 41, []int64{301, 302})
	require.NoError(t, err)
	assert.Equal(t, resolution, got)
	assert.Equal(t, []string{"status 2026-10-19", "partial 2026-10-19", "status 2026-10-20", "partial 2026-10-20"}, rules.applied)

	rules = &pickupExtensionRules{fail: errors.New("refused")}
	extensions, err = careexit.NewPickupExtensions(pickupExtensionTimetable{resolution: resolution}, rules)
	require.NoError(t, err)
	_, err = extensions.ResolvePickupExtension(context.Background(), 41, []int64{301})
	require.EqualError(t, err, "pickup extension: apply day statuses to block 301: refused")

	_, err = careexit.NewPickupExtensions(nil, rules)
	require.EqualError(t, err, "pickup extension workflow: timetable and student presence are required")
}
