package compose

import (
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// conflictFingerprintLength is the length of the persisted fingerprint: the
// first 16 bytes of the SHA-256, hex-encoded. Acknowledgements stored before
// the move keep matching because the payload and the prefix are unchanged.
const conflictFingerprintLength = 32

// DetectWindowConflicts computes person double-bookings between the given
// planned or running blocks, keyed by instance ID. It implements the #2139
// conflict matrix for the calendar view:
//
//   - pure time overlap or a shared room alone: no warning
//   - same child in two overlapping blocks: warning, regardless of rooms
//   - same (non-absent) staff in two overlapping blocks: warning only when the
//     effective rooms differ (per-row multi-room override, else the block's
//     primary room) — the same concrete room is sanctioned
//   - touching edges (endA == startB): no warning (half-open comparison)
//   - completed or cancelled blocks: never conflict
//
// Every warning appears on both involved blocks with the same fingerprint, so
// the calendar can deduplicate by fingerprint and a per-user acknowledgement
// hides the pair at once.
func (d *conflictDetection) DetectWindowConflicts(blocks []timetable.WindowConflictBlock) map[int64][]timetable.InstanceConflictWarning {
	relevant := windowConflictCandidates(blocks)
	out := make(map[int64][]timetable.InstanceConflictWarning)
	for i := range relevant {
		for j := i + 1; j < len(relevant); j++ {
			a, b := relevant[i], relevant[j]
			if a.Date != b.Date {
				break // sorted by date — no later same-date partner exists
			}
			overlapStart, overlapEnd, overlaps := windowOverlap(a, b)
			if !overlaps {
				continue
			}
			d.appendPairStaffConflicts(out, a, b, overlapStart, overlapEnd)
			d.appendPairStudentConflicts(out, a, b, overlapStart, overlapEnd)
		}
	}
	return out
}

// windowConflictCandidates keeps the planned and running blocks, sorted by
// date and instance ID.
func windowConflictCandidates(blocks []timetable.WindowConflictBlock) []timetable.WindowConflictBlock {
	relevant := make([]timetable.WindowConflictBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Status == scheduleModels.InstanceStatusPlanned || block.Status == scheduleModels.InstanceStatusActive {
			relevant = append(relevant, block)
		}
	}
	sort.Slice(relevant, func(i, j int) bool {
		if relevant[i].Date != relevant[j].Date {
			return relevant[i].Date.Before(relevant[j].Date)
		}
		return relevant[i].InstanceID < relevant[j].InstanceID
	})
	return relevant
}

// windowOverlap returns the intersection of the two blocks' wall-clock
// windows. Half-open: touching edges do not overlap.
func windowOverlap(a, b timetable.WindowConflictBlock) (time.Time, time.Time, bool) {
	aStart := timezone.NormalizeWallClock(a.StartTime)
	aEnd := timezone.NormalizeWallClock(a.EndTime)
	bStart := timezone.NormalizeWallClock(b.StartTime)
	bEnd := timezone.NormalizeWallClock(b.EndTime)
	if !aStart.Before(bEnd) || !bStart.Before(aEnd) {
		return time.Time{}, time.Time{}, false
	}
	overlapStart, overlapEnd := aStart, aEnd
	if bStart.After(overlapStart) {
		overlapStart = bStart
	}
	if bEnd.Before(overlapEnd) {
		overlapEnd = bEnd
	}
	return overlapStart, overlapEnd, true
}

// appendPairStaffConflicts emits mirrored staff warnings for one overlapping
// pair. Absent rows are skipped on both sides — staff marked out for a block
// are not bound by it.
func (d *conflictDetection) appendPairStaffConflicts(out map[int64][]timetable.InstanceConflictWarning, a, b timetable.WindowConflictBlock, overlapStart, overlapEnd time.Time) {
	bByStaff := make(map[int64]timetable.WindowConflictStaff, len(b.Staff))
	for _, row := range b.Staff {
		if !row.IsAbsent {
			bByStaff[row.StaffID] = row
		}
	}
	for _, aRow := range a.Staff {
		if aRow.IsAbsent {
			continue
		}
		bRow, ok := bByStaff[aRow.StaffID]
		if !ok {
			continue
		}
		roomA := effectiveRoom(a.RoomID, aRow.RoomID)
		roomB := effectiveRoom(b.RoomID, bRow.RoomID)
		if roomA == roomB {
			continue // same concrete room — sanctioned parallel supervision
		}
		fingerprint := d.conflictFingerprint(timetable.ConflictKindStaff, aRow.StaffID, a.Date,
			a.InstanceID, b.InstanceID, overlapStart, overlapEnd, roomA, roomB)
		appendMirroredWarnings(out, a, b, timetable.InstanceConflictWarning{
			Kind:        timetable.ConflictKindStaff,
			ResourceID:  aRow.StaffID,
			CanOverride: true,
			Fingerprint: fingerprint,
		}, overlapStart, overlapEnd, "Personal ist von %s–%s auch bei „%s“ eingeplant (anderer Raum).")
	}
}

// appendPairStudentConflicts emits mirrored student warnings for one
// overlapping pair. Rows in status expected or present count — both mean the
// plan claims the child for that window; absent rows do not.
func (d *conflictDetection) appendPairStudentConflicts(out map[int64][]timetable.InstanceConflictWarning, a, b timetable.WindowConflictBlock, overlapStart, overlapEnd time.Time) {
	bStudents := make(map[int64]bool, len(b.Students))
	for _, row := range b.Students {
		if claimsStudent(row.Status) {
			bStudents[row.StudentID] = true
		}
	}
	for _, aRow := range a.Students {
		if !claimsStudent(aRow.Status) || !bStudents[aRow.StudentID] {
			continue
		}
		// Rooms are irrelevant to a child double-booking (the conflict exists
		// either way), so the fingerprint pins them to zero: moving one block
		// to another room must not resurface an acknowledged child conflict.
		fingerprint := d.conflictFingerprint(timetable.ConflictKindStudent, aRow.StudentID, a.Date,
			a.InstanceID, b.InstanceID, overlapStart, overlapEnd, 0, 0)
		appendMirroredWarnings(out, a, b, timetable.InstanceConflictWarning{
			Kind:        timetable.ConflictKindStudent,
			ResourceID:  aRow.StudentID,
			CanOverride: true,
			Fingerprint: fingerprint,
		}, overlapStart, overlapEnd, "Kind ist von %s–%s auch bei „%s“ eingeplant.")
	}
}

func claimsStudent(status string) bool {
	return status == scheduleModels.AttendanceStatusExpected || status == scheduleModels.AttendanceStatusPresent
}

// appendMirroredWarnings attaches the warning to both involved blocks, each
// naming the other block as the conflicting one. The message format takes
// (overlap start, overlap end, other title).
func appendMirroredWarnings(
	out map[int64][]timetable.InstanceConflictWarning,
	a, b timetable.WindowConflictBlock,
	warning timetable.InstanceConflictWarning,
	overlapStart, overlapEnd time.Time,
	messageFormat string,
) {
	startLabel := overlapStart.Format("15:04")
	endLabel := overlapEnd.Format("15:04")
	warning.OverlapStart = startLabel
	warning.OverlapEnd = endLabel

	forA := warning
	forA.ConflictingInstanceID = b.InstanceID
	forA.ConflictingTitle = b.Title
	forA.Message = fmt.Sprintf(messageFormat, startLabel, endLabel, b.Title)
	out[a.InstanceID] = append(out[a.InstanceID], forA)

	forB := warning
	forB.ConflictingInstanceID = a.InstanceID
	forB.ConflictingTitle = a.Title
	forB.Message = fmt.Sprintf(messageFormat, startLabel, endLabel, a.Title)
	out[b.InstanceID] = append(out[b.InstanceID], forB)
}

// conflictFingerprint derives the stable identity of one concrete conflict:
// kind, person, both blocks (order-normalized), date, overlap window and the
// two effective rooms (zero for student conflicts, where rooms do not define
// the conflict). Any change to person, blocks, time or a relevant room yields
// a new fingerprint, so an acknowledgement stops matching and the warning
// resurfaces (#2139).
func (d *conflictDetection) conflictFingerprint(
	kind string,
	resourceID int64,
	date timezone.Date,
	instanceA, instanceB int64,
	overlapStart, overlapEnd time.Time,
	roomA, roomB int64,
) string {
	if instanceB < instanceA {
		instanceA, instanceB = instanceB, instanceA
		roomA, roomB = roomB, roomA
	}
	payload := fmt.Sprintf("v1|%s|%d|%s|%d|%d|%s|%s|%d|%d",
		kind, resourceID, date.String(), instanceA, instanceB,
		overlapStart.Format("15:04"), overlapEnd.Format("15:04"), roomA, roomB)
	hash := d.deps.ContentHash([]byte(payload))
	if len(hash) < conflictFingerprintLength {
		return hash
	}
	return hash[:conflictFingerprintLength]
}
