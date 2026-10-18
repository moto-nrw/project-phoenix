package peopledirectory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const (
	ownerRuleStudentID int64 = 4201
	ownerRuleOtherID   int64 = 4202
)

func TestPersonDisplayNameTrimsMissingParts(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Ada Lovelace", peopleModule.PersonDisplayName("Ada", "Lovelace"))
	assert.Equal(t, "Ada", peopleModule.PersonDisplayName("Ada", ""))
	assert.Equal(t, "Lovelace", peopleModule.PersonDisplayName("", "Lovelace"))
	assert.Empty(t, peopleModule.PersonDisplayName("", ""))
	assert.Equal(t, "Ada", peopleModule.PersonDisplayName(" Ada", " "), "the outer whitespace goes, as in the reviewer name")
}

func TestPersonIsLiveUntilDeleted(t *testing.T) {
	t.Parallel()
	deletedAt := time.Date(2026, time.October, 18, 9, 0, 0, 0, time.UTC)
	assert.True(t, peopleModule.PersonIsLive(nil))
	assert.False(t, peopleModule.PersonIsLive(&deletedAt))
}

// studentsByID serves ListStudentsByID from fixed rows; every other read of
// the query is unused.
type studentsByID struct {
	peopleModule.StudentQuery
	rows []peopleModule.Student
	err  error
}

func (s studentsByID) ListStudentsByID(context.Context, []int64) ([]peopleModule.Student, error) {
	return s.rows, s.err
}

func TestStudentStillEnrolled(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	cases := map[string]struct {
		rows []peopleModule.Student
		want bool
	}{
		"an active child":             {rows: []peopleModule.Student{{ID: ownerRuleStudentID, Status: peopleModule.StudentStatusActive}}, want: true},
		"a pending child":             {rows: []peopleModule.Student{{ID: ownerRuleStudentID, Status: peopleModule.StudentStatusPending}}, want: true},
		"an inactive child":           {rows: []peopleModule.Student{{ID: ownerRuleStudentID, Status: peopleModule.StudentStatusInactive}}, want: true},
		"a graduate":                  {rows: []peopleModule.Student{{ID: ownerRuleStudentID, Status: peopleModule.StudentStatusAlumnus}}, want: false},
		"a missing child":             {rows: nil, want: false},
		"only another child returned": {rows: []peopleModule.Student{{ID: ownerRuleOtherID, Status: peopleModule.StudentStatusActive}}, want: false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			enrolled, err := peopleModule.StudentStillEnrolled(ctx, studentsByID{rows: tc.rows}, ownerRuleStudentID)
			require.NoError(t, err)
			assert.Equal(t, tc.want, enrolled)
		})
	}

	readErr := errors.New("read failed")
	_, err := peopleModule.StudentStillEnrolled(ctx, studentsByID{err: readErr}, ownerRuleStudentID)
	require.ErrorIs(t, err, readErr)
}

// classRecords serves the class read and records the scope it was asked for.
type classRecords struct {
	rows  []peopleModule.StudentRecord
	err   error
	scope *peopleModule.StudentScope
}

func (c classRecords) ListStudentRecordsByClass(_ context.Context, _ []string, scope peopleModule.StudentScope) ([]peopleModule.StudentRecord, error) {
	*c.scope = scope
	return c.rows, c.err
}

func TestHasActiveClass(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	today := calendar.Date("2026-10-18")
	cases := map[string]struct {
		rows []peopleModule.StudentRecord
		want bool
	}{
		"no enrolled child":               {rows: nil, want: false},
		"a child without a care end":      {rows: []peopleModule.StudentRecord{{ID: ownerRuleStudentID}}, want: true},
		"a child whose last day is today": {rows: []peopleModule.StudentRecord{{ID: ownerRuleStudentID, EnrolledUntil: "2026-10-18"}}, want: true},
		"only children whose care ended": {
			rows: []peopleModule.StudentRecord{{ID: ownerRuleStudentID, EnrolledUntil: "2026-10-17"}}, want: false,
		},
		"one ended and one running child": {
			rows: []peopleModule.StudentRecord{{ID: ownerRuleStudentID, EnrolledUntil: "2026-10-17"}, {ID: ownerRuleOtherID}}, want: true,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var scope peopleModule.StudentScope
			active, err := peopleModule.HasActiveClass(ctx, classRecords{rows: tc.rows, scope: &scope}, "3a", today)
			require.NoError(t, err)
			assert.Equal(t, tc.want, active)
			assert.Equal(t, peopleModule.StudentScopeEnrolled, scope, "graduates never keep a class active")
		})
	}

	var scope peopleModule.StudentScope
	readErr := errors.New("read failed")
	_, err := peopleModule.HasActiveClass(ctx, classRecords{err: readErr, scope: &scope}, "3a", today)
	require.ErrorIs(t, err, readErr)
}

// photoPurge returns fixed references and counts the purges.
type photoPurge struct {
	purged []string
	err    error
	calls  *int
}

func (p photoPurge) PurgeStudentPhotos(context.Context) ([]string, error) {
	*p.calls++
	return p.purged, p.err
}

func TestApplyPhotoFeatureSetting(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	for name, value := range map[string]any{"enabled": true, "non-bool": "false", "nil": nil} {
		t.Run(name+" keeps the photos", func(t *testing.T) {
			t.Parallel()
			calls := 0
			change, err := peopleModule.ApplyPhotoFeatureSetting(ctx, photoPurge{calls: &calls}, value)
			require.NoError(t, err)
			assert.Zero(t, calls)
			assert.Empty(t, change.Purged)
			assert.Equal(t, peopleModule.StudentPhotoSourceFeatureEnabled, change.BroadcastSource)
		})
	}

	t.Run("switching off purges and returns the detached references", func(t *testing.T) {
		t.Parallel()
		calls := 0
		stored := []string{"/uploads/a.jpg", "/uploads/b.jpg"}
		change, err := peopleModule.ApplyPhotoFeatureSetting(ctx, photoPurge{purged: stored, calls: &calls}, false)
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
		assert.Equal(t, stored, change.Purged)
		assert.Equal(t, "tenant_photos_disabled", change.BroadcastSource)
	})

	t.Run("switching off with nothing stored still names the disable broadcast", func(t *testing.T) {
		t.Parallel()
		calls := 0
		change, err := peopleModule.ApplyPhotoFeatureSetting(ctx, photoPurge{calls: &calls}, false)
		require.NoError(t, err)
		assert.Empty(t, change.Purged)
		assert.Equal(t, peopleModule.StudentPhotoSourceFeatureDisabled, change.BroadcastSource)
	})

	t.Run("a failed purge is returned", func(t *testing.T) {
		t.Parallel()
		calls := 0
		purgeErr := errors.New("purge failed")
		_, err := peopleModule.ApplyPhotoFeatureSetting(ctx, photoPurge{err: purgeErr, calls: &calls}, false)
		require.ErrorIs(t, err, purgeErr)
	})
}
