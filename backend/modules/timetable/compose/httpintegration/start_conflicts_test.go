package httpintegration_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The start check runs against the live layer inside the lifecycle's tenant
// transaction; the lifecycle suites cover its warnings through Start. These
// pin the owner contract itself.

func TestDetectStartConflicts_EmptyInstance_NoWarnings(t *testing.T) {
	t.Parallel()

	db, module := testutil.SetupTimetableModule(t)
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SC-Room-%d", time.Now().UnixNano()))
	instance := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2026, time.April, 20), room.ID,
		testpkg.ActivityInstanceOpts{Title: "Lifecycle-Test"})

	warnings, err := module.ConflictDetection.DetectStartConflicts(testpkg.Ctx(t),
		timetable.StartConflictSubject{InstanceID: instance.ID, RoomID: instance.RoomID})
	require.NoError(t, err)
	assert.Empty(t, warnings, "clean-room, no staff, no students → no warnings")
}

func TestStaffPoolForInstance_UnknownInstanceIsNotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos, err := repositories.NewTimetableTestRepositories(db)
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, db, fmt.Sprintf("SC-Pool-Room-%d", time.Now().UnixNano()))
	instance := testpkg.CreateTestActivityInstance(t, db, calendar.NewDate(2026, time.April, 20), room.ID,
		testpkg.ActivityInstanceOpts{Title: "Pool-Missing"})

	detection := newConflictDetection(t, repos)

	_, err = detection.StaffPoolForInstance(testpkg.Ctx(t), instance.ID)
	require.NoError(t, err, "the block's own tenant reads its pool")

	// The block exists in the parent's tenant only: another tenant must not
	// reach it, and the miss maps onto the owner's not-found sentinel.
	t.Run("other tenant", func(t *testing.T) {
		_, err := detection.StaffPoolForInstance(testpkg.OwnCtx(t), instance.ID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, timetable.ErrActivityInstanceNotFound), "unexpected error: %v", err)
	})
}

// TestWindowConflicts_FingerprintsMatchPersistedAcknowledgements pins the
// digest the root binds: per-user acknowledgements store the first 32 hex
// characters of the SHA-256 of the conflict payload (#2139). The expected
// values were computed independently from the payloads
// ("v1|kind|person|date|instanceA|instanceB|start|end|roomA|roomB"); a
// different digest would silently resurface every acknowledged conflict.
func TestWindowConflicts_FingerprintsMatchPersistedAcknowledgements(t *testing.T) {
	t.Parallel()

	repos, err := repositories.NewTimetableTestRepositories(testpkg.SetupTestDB(t))
	require.NoError(t, err)
	detection := newConflictDetection(t, repos)

	clock := func(hour, minute int) time.Time { return time.Date(1, time.January, 1, hour, minute, 0, 0, time.UTC) }
	date := calendar.NewDate(2026, time.September, 7)
	blocks := []timetable.WindowConflictBlock{
		{
			InstanceID: 2, Date: date, Title: "B", StartTime: clock(14, 30), EndTime: clock(15, 30), RoomID: 11,
			Status:   timetable.InstanceStatusPlanned,
			Staff:    []timetable.WindowConflictStaff{{StaffID: 100}},
			Students: []timetable.WindowConflictStudent{{StudentID: 200, Status: "present"}},
		},
		{
			InstanceID: 1, Date: date, Title: "A", StartTime: clock(14, 0), EndTime: clock(15, 0), RoomID: 10,
			Status:   timetable.InstanceStatusPlanned,
			Staff:    []timetable.WindowConflictStaff{{StaffID: 100}},
			Students: []timetable.WindowConflictStudent{{StudentID: 200, Status: "expected"}},
		},
	}
	warnings := detection.DetectWindowConflicts(blocks)

	fingerprints := map[string]string{}
	for _, warning := range append(warnings[1], warnings[2]...) {
		if previous, seen := fingerprints[warning.Kind]; seen {
			assert.Equal(t, previous, warning.Fingerprint, "mirrored warnings share one fingerprint")
		}
		fingerprints[warning.Kind] = warning.Fingerprint
	}
	assert.Equal(t, map[string]string{
		timetable.ConflictKindStudent: "27771ae0e8ab3256c4b84586c319c38d", // v1|student|200|2026-09-07|1|2|14:30|15:00|0|0
		timetable.ConflictKindStaff:   "d7b12a731c374268d19674ef69ca9eaa", // v1|staff|100|2026-09-07|1|2|14:30|15:00|10|11
	}, fingerprints)
	for _, fingerprint := range fingerprints {
		assert.True(t, timetable.ValidConflictAckFingerprint(fingerprint), "the acknowledgement endpoint accepts %q", fingerprint)
	}
}
