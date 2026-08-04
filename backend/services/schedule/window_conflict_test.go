// Pure unit tests for DetectWindowConflicts — the #2139 conflict matrix the
// Betreuungsplan calendar runs over its visible window. No database: the
// detection is a pure function over instances plus their assignment rows.
//
// Matrix under test (issue #2139):
//
//	two parallel instances without a shared person      → no warning
//	two instances sharing a room                        → no warning
//	same child in two overlapping instances             → warning (any rooms)
//	same staff, overlapping, different rooms            → warning
//	same staff, overlapping, same room                  → no warning
//	same staff, per-row room override diverges          → warning
//	touching edges (endA == startB)                     → no warning
//	cancelled / completed instances                     → no warning
//	staff marked absent                                 → no warning
//
// Plus the fingerprint contract: mirrored warnings share one fingerprint;
// changing person, instance, time, or (for staff) room changes it.
package schedule_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var windowTestDate = timezone.NewDate(2026, time.September, 7) // fixed Monday

func windowInstance(id, roomID int64, startHHMM, endHHMM, status string) *scheduleModels.ActivityInstance {
	parse := func(v string) time.Time {
		t, err := time.Parse("15:04", v)
		if err != nil {
			panic(err)
		}
		return t
	}
	inst := &scheduleModels.ActivityInstance{
		Date:      windowTestDate,
		Title:     "Block",
		StartTime: parse(startHHMM),
		EndTime:   parse(endHHMM),
		RoomID:    roomID,
		Status:    status,
	}
	inst.ID = id
	return inst
}

func staffRow(staffID int64, opts ...func(*scheduleModels.InstanceStaff)) *scheduleModels.InstanceStaff {
	row := &scheduleModels.InstanceStaff{StaffID: staffID}
	for _, opt := range opts {
		opt(row)
	}
	return row
}

func studentRow(studentID int64, status string) *scheduleModels.InstanceStudent {
	return &scheduleModels.InstanceStudent{StudentID: studentID, Status: status}
}

func TestDetectWindowConflicts_ParallelWithoutSharedPersonIsClean(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)},
		},
		{
			Instance: windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(101)},
			Students: []*scheduleModels.InstanceStudent{studentRow(201, scheduleModels.AttendanceStatusExpected)},
		},
	})
	assert.Empty(t, out, "parallel instances without a shared person must not conflict")
}

func TestDetectWindowConflicts_SharedRoomAloneIsClean(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)},
		{Instance: windowInstance(2, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)},
	})
	assert.Empty(t, out, "a shared room alone must not conflict")
}

func TestDetectWindowConflicts_SharedStudentWarnsRegardlessOfRoom(t *testing.T) {
	// Same room on purpose: the room never excuses a child double-booking.
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)},
		},
		{
			Instance: windowInstance(2, 10, "14:30", "15:30", scheduleModels.InstanceStatusPlanned),
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusPresent)},
		},
	})

	require.Len(t, out[1], 1)
	require.Len(t, out[2], 1)
	a, b := out[1][0], out[2][0]
	assert.Equal(t, scheduleSvc.ConflictKindStudent, a.Kind)
	assert.EqualValues(t, 200, a.ResourceID)
	assert.EqualValues(t, 2, a.ConflictingInstanceID)
	assert.EqualValues(t, 1, b.ConflictingInstanceID)
	assert.Equal(t, "14:30", a.OverlapStart)
	assert.Equal(t, "15:00", a.OverlapEnd)
	assert.NotEmpty(t, a.Fingerprint)
	assert.Equal(t, a.Fingerprint, b.Fingerprint, "mirrored warnings share one fingerprint")
	assert.True(t, a.CanOverride)
}

func TestDetectWindowConflicts_AbsentStudentDoesNotWarn(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusAbsent)},
		},
		{
			Instance: windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)},
		},
	})
	assert.Empty(t, out, "an absent row does not claim the child")
}

func TestDetectWindowConflicts_SharedStaffDifferentRoomsWarns(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
		},
		{
			Instance: windowInstance(2, 11, "14:30", "15:30", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
		},
	})

	require.Len(t, out[1], 1)
	require.Len(t, out[2], 1)
	assert.Equal(t, scheduleSvc.ConflictKindStaff, out[1][0].Kind)
	assert.EqualValues(t, 100, out[1][0].ResourceID)
	assert.Equal(t, out[1][0].Fingerprint, out[2][0].Fingerprint)
}

func TestDetectWindowConflicts_SharedStaffSameRoomIsClean(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
		},
		{
			Instance: windowInstance(2, 10, "14:30", "15:30", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
		},
	})
	assert.Empty(t, out, "one staff supervising parallel groups in ONE room is sanctioned")
}

func TestDetectWindowConflicts_StaffRoomOverrideDefinesEffectiveRoom(t *testing.T) {
	overrideRoom := int64(12)

	// Both instances share primary room 10, but instance 2 sends THIS staff
	// member into room 12 via the per-row override → different effective
	// rooms → warning.
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
		},
		{
			Instance: windowInstance(2, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff: []*scheduleModels.InstanceStaff{staffRow(100, func(r *scheduleModels.InstanceStaff) {
				r.RoomID = &overrideRoom
			})},
		},
	})
	require.Len(t, out[1], 1, "diverging per-row room override must warn")

	// And the mirror case: overrides on both sides pointing at the SAME room
	// stay clean even though the primary rooms differ.
	out = scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff: []*scheduleModels.InstanceStaff{staffRow(100, func(r *scheduleModels.InstanceStaff) {
				r.RoomID = &overrideRoom
			})},
		},
		{
			Instance: windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff: []*scheduleModels.InstanceStaff{staffRow(100, func(r *scheduleModels.InstanceStaff) {
				r.RoomID = &overrideRoom
			})},
		},
	})
	assert.Empty(t, out, "matching per-row overrides are the same effective room")
}

func TestDetectWindowConflicts_AbsentStaffDoesNotWarn(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff: []*scheduleModels.InstanceStaff{staffRow(100, func(r *scheduleModels.InstanceStaff) {
				r.IsAbsent = true
			})},
		},
		{
			Instance: windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Staff:    []*scheduleModels.InstanceStaff{staffRow(100)},
		},
	})
	assert.Empty(t, out, "absent staff are not bound by the instance")
}

func TestDetectWindowConflicts_TouchingEdgesAreClean(t *testing.T) {
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{
			Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned),
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)},
		},
		{
			Instance: windowInstance(2, 11, "15:00", "16:00", scheduleModels.InstanceStatusPlanned),
			Students: []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)},
		},
	})
	assert.Empty(t, out, "endA == startB is adjacency, not overlap")
}

func TestDetectWindowConflicts_CancelledAndCompletedAreIgnored(t *testing.T) {
	shared := []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)}
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusCancelled), Students: shared},
		{Instance: windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusCompleted), Students: shared},
		{Instance: windowInstance(3, 12, "14:00", "15:00", scheduleModels.InstanceStatusPlanned), Students: shared},
	})
	assert.Empty(t, out, "cancelled/completed instances are history, not conflicts")
}

func TestDetectWindowConflicts_DifferentDaysNeverConflict(t *testing.T) {
	shared := []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)}
	a := windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
	b := windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
	b.Date = windowTestDate.AddDays(1)

	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{Instance: a, Students: shared},
		{Instance: b, Students: shared},
	})
	assert.Empty(t, out)
}

func TestDetectWindowConflicts_ActiveInstancesParticipate(t *testing.T) {
	shared := []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)}
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{Instance: windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusActive), Students: shared},
		{Instance: windowInstance(2, 11, "14:00", "15:00", scheduleModels.InstanceStatusPlanned), Students: shared},
	})
	require.Len(t, out[1], 1, "a running block still claims its people")
}

// --- Fingerprint contract ----------------------------------------------------

func windowStudentFingerprint(t *testing.T, mutate func(a, b *scheduleModels.ActivityInstance)) string {
	t.Helper()
	a := windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
	b := windowInstance(2, 11, "14:30", "15:30", scheduleModels.InstanceStatusPlanned)
	if mutate != nil {
		mutate(a, b)
	}
	shared := []*scheduleModels.InstanceStudent{studentRow(200, scheduleModels.AttendanceStatusExpected)}
	out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
		{Instance: a, Students: shared},
		{Instance: b, Students: shared},
	})
	require.Len(t, out[a.ID], 1)
	return out[a.ID][0].Fingerprint
}

func TestDetectWindowConflicts_FingerprintIsStable(t *testing.T) {
	assert.Equal(t, windowStudentFingerprint(t, nil), windowStudentFingerprint(t, nil),
		"identical conflicts must produce identical fingerprints across runs")
}

func TestDetectWindowConflicts_FingerprintChangesWithTimeDateAndInstances(t *testing.T) {
	base := windowStudentFingerprint(t, nil)

	shifted := windowStudentFingerprint(t, func(_, b *scheduleModels.ActivityInstance) {
		b.StartTime = mustClock(t, "14:45")
	})
	assert.NotEqual(t, base, shifted, "a changed overlap window must re-surface the warning")

	movedDay := windowStudentFingerprint(t, func(a, b *scheduleModels.ActivityInstance) {
		a.Date = windowTestDate.AddDays(7)
		b.Date = windowTestDate.AddDays(7)
	})
	assert.NotEqual(t, base, movedDay, "another date is another conflict")

	otherInstance := windowStudentFingerprint(t, func(_, b *scheduleModels.ActivityInstance) {
		b.ID = 99
	})
	assert.NotEqual(t, base, otherInstance, "another instance pair is another conflict")
}

func TestDetectWindowConflicts_StudentFingerprintIgnoresRooms(t *testing.T) {
	base := windowStudentFingerprint(t, nil)
	movedRoom := windowStudentFingerprint(t, func(_, b *scheduleModels.ActivityInstance) {
		b.RoomID = 42
	})
	assert.Equal(t, base, movedRoom,
		"rooms do not define a child conflict — moving a block must not resurface an acknowledged one")
}

func TestDetectWindowConflicts_StaffFingerprintTracksRooms(t *testing.T) {
	run := func(roomB int64) string {
		a := windowInstance(1, 10, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
		b := windowInstance(2, roomB, "14:00", "15:00", scheduleModels.InstanceStatusPlanned)
		out := scheduleSvc.DetectWindowConflicts([]scheduleSvc.WindowConflictInput{
			{Instance: a, Staff: []*scheduleModels.InstanceStaff{staffRow(100)}},
			{Instance: b, Staff: []*scheduleModels.InstanceStaff{staffRow(100)}},
		})
		require.Len(t, out[1], 1)
		return out[1][0].Fingerprint
	}
	assert.NotEqual(t, run(11), run(12),
		"a staff conflict's rooms are part of its identity")
}

func mustClock(t *testing.T, hhmm string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", hhmm)
	require.NoError(t, err)
	return parsed
}
