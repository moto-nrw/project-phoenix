package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mirrorTimetableStub struct {
	exists                         bool
	lookupErr, createErr, staffErr error
	title                          string
	created                        []MirroredSession
	staff                          []int64
}

func (s *mirrorTimetableStub) HasInstance(context.Context, int64) (bool, error) {
	return s.exists, s.lookupErr
}
func (s *mirrorTimetableStub) CreateInstance(_ context.Context, instance MirroredSession) (MirroredSession, error) {
	instance.ID = 77
	s.created = append(s.created, instance)
	return instance, s.createErr
}
func (s *mirrorTimetableStub) AddStaff(_ context.Context, _ int64, staffID int64) error {
	s.staff = append(s.staff, staffID)
	return s.staffErr
}
func (s *mirrorTimetableStub) ActivityTitle(context.Context, int64) (string, error) {
	return s.title, nil
}
func (s *mirrorTimetableStub) CalendarParts(instant time.Time) (string, int) {
	local := instant.In(berlin)
	return local.Format("2006-01-02"), local.Hour()*60 + local.Minute()
}

func TestSessionMirrorPreservesClockStaffAndPublication(t *testing.T) {
	t.Parallel()
	store := &mirrorTimetableStub{title: "Werkstatt"}
	var published []MirroredSession
	mirror := NewSessionMirror(store, func(_ context.Context, instance MirroredSession) { published = append(published, instance) }, nil)
	groupID := int64(44)
	startedAt := time.Date(2026, 5, 12, 21, 45, 0, 0, time.UTC)
	mirror.MirrorSession(t.Context(), LifecycleSession{ID: 66, ActivityID: &groupID, RoomID: 55, StartTime: startedAt}, []int64{101, 0, 101, 202})
	require.Len(t, store.created, 1)
	instance := store.created[0]
	assert.Equal(t, int64(77), instance.ID)
	assert.Equal(t, &groupID, instance.ActivityGroupID)
	assert.Equal(t, "Werkstatt", instance.Title)
	assert.Equal(t, int64(55), instance.RoomID)
	assert.Equal(t, int64(101), *instance.CreatedBy)
	assert.Equal(t, instance.CreatedBy, instance.StartedBy)
	assert.Equal(t, startedAt, *instance.StartedAt)
	assert.Equal(t, "2026-05-12", instance.Date)
	assert.Equal(t, "23:30:00", instance.StartTime)
	assert.Equal(t, "23:59:00", instance.EndTime)
	assert.Equal(t, []int64{101, 202}, store.staff)
	assert.Equal(t, store.created, published)
}

func TestSessionMirrorFallbacks(t *testing.T) {
	t.Parallel()
	store := &mirrorTimetableStub{}
	mirror := NewSessionMirror(store, nil, nil)
	mirror.MirrorSession(t.Context(), LifecycleSession{StartTime: time.Date(2026, 5, 12, 7, 5, 0, 0, time.UTC)}, []int64{-1, 0})
	require.Len(t, store.created, 1)
	assert.Equal(t, "RFID-Aktivität", store.created[0].Title)
	assert.Equal(t, "09:05:00", store.created[0].StartTime)
	assert.Nil(t, store.created[0].CreatedBy)
	assert.Empty(t, store.staff)
}

func TestSessionMirrorFailureBoundaries(t *testing.T) {
	t.Parallel()
	unavailable := errors.New("unavailable")
	for _, tc := range []struct {
		name                         string
		store                        mirrorTimetableStub
		creates, staff, publications int
	}{
		{"already mirrored", mirrorTimetableStub{exists: true}, 0, 0, 0},
		{"lookup failure", mirrorTimetableStub{lookupErr: unavailable}, 0, 0, 0},
		{"instance failure", mirrorTimetableStub{createErr: unavailable}, 1, 0, 0},
		{"staff failure continues", mirrorTimetableStub{staffErr: unavailable}, 1, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store := tc.store
			publications := 0
			mirror := NewSessionMirror(&store, func(context.Context, MirroredSession) { publications++ }, nil)
			mirror.MirrorSession(t.Context(), LifecycleSession{ID: 66, StartTime: fixedNow}, []int64{101, 202})
			assert.Len(t, store.created, tc.creates)
			assert.Len(t, store.staff, tc.staff)
			assert.Equal(t, tc.publications, publications)
		})
	}
}
