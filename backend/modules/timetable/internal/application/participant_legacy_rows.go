package application

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The instance lifecycle, the materializer, the roster maintenance, the
// attendance mirror and correction, the operational day and the conflict
// reads still compute on the legacy instance_students row, which used to be
// one row and now has two owners: the participant is Timetable's, the
// attendance is Student Presence's, and a participant without an attendance
// row is expected (#2762). They read it through the joined presence read,
// write the participant through the owner's capability and the attendance
// through Student Presence. These functions keep the attendance restore, the
// participant resolution and the operation names the retained repository
// reported.

// createLegacyParticipant lists the child on the block and restores any
// attendance the row carries beyond expected.
func createLegacyParticipant(ctx context.Context, rows ports.ParticipantRows, value *scheduleModels.InstanceStudent) error {
	if value == nil {
		return errors.New("instance student cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := rows.Plans.CreateInstanceStudent(ctx, publicInstanceStudentInput(value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	attendance := studentpresence.ExpectedSessionAttendance(created.ID)
	if legacyAttendanceDiffers(value) {
		if err := rows.Presence.RestoreSessionAttendance(ctx, []studentpresence.SessionAttendanceRestore{legacyAttendanceRestore(created.ID, value)}); err != nil {
			return WrapDatabaseError("create", err)
		}
		stored, err := rows.Presence.ListSessionAttendance(ctx, []int64{created.ID})
		if err != nil {
			return WrapDatabaseError("create", err)
		}
		if len(stored) > 0 {
			attendance = stored[0]
		}
	}
	replaceLegacyInstanceStudent(value, created, attendance)
	return nil
}

// findLegacyParticipant reads one participant with its attendance; an ID
// <= 0 is not found.
func findLegacyParticipant(ctx context.Context, reads ports.LegacyParticipantReads, id int64) (*scheduleModels.InstanceStudent, error) {
	if id <= 0 {
		return nil, WrapNotFoundDatabaseError("find by id")
	}
	rows, err := listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{IDs: []int64{id}}, "find by id")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, WrapNotFoundDatabaseError("find by id")
	}
	return rows[0], nil
}

func deleteLegacyParticipant(ctx context.Context, plans timetable.InstanceStudentCapability, id int64) error {
	if err := plans.DeleteInstanceStudent(ctx, id); err != nil {
		return WrapDatabaseError("delete", err)
	}
	return nil
}

// participantsOfInstance lists a block's participants in creation order.
func participantsOfInstance(ctx context.Context, reads ports.LegacyParticipantReads, instanceID int64) ([]*scheduleModels.InstanceStudent, error) {
	return listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true}, "find by instance id")
}

// participantsOfInstances lists the participants of the blocks; no blocks
// list nothing.
func participantsOfInstances(ctx context.Context, reads ports.LegacyParticipantReads, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true}, "find by instance ids")
}

// expectedParticipantsOf lists the participants of the blocks still expected.
func expectedParticipantsOf(ctx context.Context, reads ports.LegacyParticipantReads, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := participantsOfInstances(ctx, reads, instanceIDs)
	if err != nil {
		return nil, err
	}
	return filterLegacyInstanceStudents(rows, func(row *scheduleModels.InstanceStudent) bool {
		return row.Status == scheduleModels.AttendanceStatusExpected
	}), nil
}

// notScheduledCandidatesOf mirrors the rows Student Presence's non-booking
// marker can still change: expected rows and absences a day status or an
// excusal owns, never a decision somebody made by hand.
func notScheduledCandidatesOf(ctx context.Context, reads ports.LegacyParticipantReads, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := participantsOfInstances(ctx, reads, instanceIDs)
	if err != nil {
		return nil, err
	}
	return filterLegacyInstanceStudents(rows, func(row *scheduleModels.InstanceStudent) bool {
		if row.ManualStatusAt != nil {
			return false
		}
		return row.Status == scheduleModels.AttendanceStatusExpected ||
			(row.Status == scheduleModels.AttendanceStatusAbsent && (row.StudentStatusDayID != nil || row.PickupExceptionID != nil))
	}), nil
}

// parallelPresence lists the students present in another running block of
// the day, latest block first.
func parallelPresence(ctx context.Context, reads ports.LegacyParticipantReads, excludedID int64, date calendar.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	result, err := reads.ListParallelPresence(ctx, excludedID, date.String(), positiveInstanceStudentIDs(studentIDs))
	if err != nil {
		return nil, WrapDatabaseError("find present in other active instances", err)
	}
	return result, nil
}

// participantsOfStudentBetween lists a student's participations in an
// inclusive date range.
func participantsOfStudentBetween(ctx context.Context, reads ports.LegacyParticipantReads, studentID int64, from, to calendar.Date) ([]*scheduleModels.InstanceStudent, error) {
	fromText, toText := from.String(), to.String()
	return listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{StudentIDs: []int64{studentID}, FromDate: &fromText,
		ToDate: &toText, OrderByActivityDateTime: true}, "find by student and date range")
}

// participantsOfStudentsOn lists the students' participations of a day.
func participantsOfStudentsOn(ctx context.Context, reads ports.LegacyParticipantReads, studentIDs []int64, date calendar.Date) ([]*scheduleModels.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	text := date.String()
	return listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{StudentIDs: studentIDs, Date: &text,
		OrderByStudentActivityTime: true}, "find by student ids and date")
}

// participantOf reads a student's participation in a block, or nil if the
// student is not listed there.
func participantOf(ctx context.Context, reads ports.LegacyParticipantReads, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error) {
	values, err := listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}, StudentIDs: []int64{studentID}}, "find by instance and student")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}

// currentCandidates lists the running blocks of a student.
func currentCandidates(ctx context.Context, reads ports.LegacyParticipantReads, studentID int64, date calendar.Date, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	return currentParticipants(ctx, reads, []int64{studentID}, date, at, "find current student slot candidates")
}

// currentCandidatesOfStudents lists the running blocks of the students.
func currentCandidatesOfStudents(ctx context.Context, reads ports.LegacyParticipantReads, studentIDs []int64, date calendar.Date, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return currentParticipants(ctx, reads, studentIDs, date, at, "find current student slot candidates batch")
}

// currentParticipants lists the running blocks of the students; a block
// that ended is no candidate.
func currentParticipants(ctx context.Context, reads ports.LegacyParticipantReads, studentIDs []int64, date calendar.Date, at time.Time, operation string) ([]*scheduleModels.InstanceStudent, error) {
	dateText, clock := date.String(), InstanceStudentWallClock(at)
	return listLegacyParticipants(ctx, reads, timetable.InstanceStudentFilter{StudentIDs: studentIDs, Date: &dateText, CurrentTime: &clock,
		OrderByStudentActivityTime: true}, operation)
}

func deleteParticipantsOfInstance(ctx context.Context, plans timetable.InstanceStudentCapability, instanceID int64) error {
	if err := plans.DeleteInstanceStudentsByInstance(ctx, instanceID); err != nil {
		return WrapDatabaseError("delete by instance id", err)
	}
	return nil
}

// checkInParticipant opens the observed presence of a listed child; it
// reports whether a row changed.
func checkInParticipant(ctx context.Context, rows ports.ParticipantRows, instanceID, studentID int64, checkedInAt time.Time) (bool, error) {
	ids, err := participantIDsOf(ctx, rows.Reads, []scheduleModels.InstanceStudentKey{{InstanceID: instanceID, StudentID: studentID}}, false)
	if err != nil {
		return false, WrapDatabaseError("update attendance from checkin", err)
	}
	if len(ids) == 0 {
		return false, nil
	}
	changed, err := rows.Presence.CheckInParticipants(ctx, ids, checkedInAt)
	if err != nil {
		return false, WrapDatabaseError("update attendance from checkin", err)
	}
	return changed > 0, nil
}

func checkInParticipants(ctx context.Context, rows ports.ParticipantRows, keys []scheduleModels.InstanceStudentKey, checkedInAt time.Time) error {
	ids, err := participantIDsOf(ctx, rows.Reads, keys, false)
	if err != nil {
		return WrapDatabaseError("update attendance from checkin batch", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := rows.Presence.CheckInParticipants(ctx, ids, checkedInAt); err != nil {
		return WrapDatabaseError("update attendance from checkin batch", err)
	}
	return nil
}

func checkOutParticipant(ctx context.Context, rows ports.ParticipantRows, instanceID, studentID int64, checkedOutAt time.Time) error {
	return checkOutParticipants(ctx, rows, []scheduleModels.InstanceStudentKey{{InstanceID: instanceID, StudentID: studentID}}, checkedOutAt)
}

func checkOutParticipants(ctx context.Context, rows ports.ParticipantRows, keys []scheduleModels.InstanceStudentKey, checkedOutAt time.Time) error {
	ids, err := participantIDsOf(ctx, rows.Reads, keys, false)
	if err != nil {
		return WrapDatabaseError("update slot attendance checkout batch", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := rows.Presence.CheckOutParticipants(ctx, ids, checkedOutAt); err != nil {
		return WrapDatabaseError("update slot attendance checkout batch", err)
	}
	return nil
}

// createUnplannedPresentIfAbsent lists the child on the block if the plan
// does not, then opens the presence: as a walk-in for a new participant, as
// a check-in for a planned one.
func createUnplannedPresentIfAbsent(ctx context.Context, rows ports.ParticipantRows, instanceID, studentID int64, checkedInAt time.Time) (*scheduleModels.InstanceStudent, error) {
	participant, inserted, err := rows.Plans.EnsureInstanceStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, WrapDatabaseError("create unplanned slot attendance", err)
	}
	if inserted {
		if _, err := rows.Presence.CheckInWalkIn(ctx, participant.ID, checkedInAt); err != nil {
			return nil, WrapDatabaseError("create unplanned slot attendance", err)
		}
	} else if _, err := rows.Presence.CheckInParticipants(ctx, []int64{participant.ID}, checkedInAt); err != nil {
		return nil, WrapDatabaseError("create unplanned slot attendance", err)
	}
	attendance, err := attendanceByParticipant(ctx, rows.Presence, []int64{participant.ID})
	if err != nil {
		return nil, WrapDatabaseError("create unplanned slot attendance", err)
	}
	return legacyInstanceStudent(participant, attendance), nil
}

// reconcileAttendanceInterval replaces a listed child's observed interval
// when the row still carries the caller's previous one.
func reconcileAttendanceInterval(ctx context.Context, rows ports.ParticipantRows, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error) {
	ids, err := participantIDsOf(ctx, rows.Reads, []scheduleModels.InstanceStudentKey{{InstanceID: instanceID, StudentID: studentID}}, false)
	if err != nil {
		return false, WrapDatabaseError("reconcile slot attendance interval", err)
	}
	if len(ids) == 0 {
		return false, nil
	}
	updated, err := rows.Presence.ReconcileParticipantInterval(ctx, ids[0], previousCheckIn, previousCheckOut, updatedCheckIn, updatedCheckOut)
	if err != nil {
		return false, WrapDatabaseError("reconcile slot attendance interval", err)
	}
	return updated, nil
}

// patchAttendanceFields writes only the fields the patch carries; callers
// validate cross-field invariants before.
func patchAttendanceFields(ctx context.Context, presence ports.ParticipantPresence, id int64, patch scheduleModels.AttendanceFieldPatch) error {
	if !patch.HasChanges() {
		return nil
	}
	if err := presence.PatchSessionAttendance(ctx, id, studentpresence.SessionAttendancePatch{Status: patch.Status, Substatus: patch.Substatus,
		SubstatusClear: patch.SubstatusClear, Note: patch.Note, NoteClear: patch.NoteClear}); err != nil {
		return WrapDatabaseError("update attendance fields", err)
	}
	return nil
}

// bulkUpdateParticipantStatus moves a block's participants from one status
// to another, sparing the excluded students.
func bulkUpdateParticipantStatus(ctx context.Context, rows ports.ParticipantRows, instanceID int64, fromStatus, toStatus string, excludedStudentIDs []int64) (int, error) {
	participants, err := rows.Plans.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}})
	if err != nil {
		return 0, WrapDatabaseError("bulk update status", err)
	}
	excluded := positiveInstanceStudentIDs(excludedStudentIDs)
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		if !slices.Contains(excluded, participant.StudentID) {
			ids = append(ids, participant.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	changed, err := rows.Presence.TransitionParticipants(ctx, ids, fromStatus, toStatus, time.Time{})
	if err != nil {
		return 0, WrapDatabaseError("bulk update status", err)
	}
	return int(changed), nil
}

// markParticipantsNotScheduled marks the listed children of blocks that are
// not finished as never booked.
func markParticipantsNotScheduled(ctx context.Context, rows ports.ParticipantRows, refs []scheduleModels.StudentInstanceRef) error {
	keys := make([]scheduleModels.InstanceStudentKey, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, scheduleModels.InstanceStudentKey{InstanceID: ref.InstanceID, StudentID: ref.StudentID})
	}
	ids, err := participantIDsOf(ctx, rows.Reads, keys, true)
	if err != nil {
		return WrapDatabaseError("mark attendance rows not scheduled", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := rows.Presence.MarkParticipantsNotScheduled(ctx, ids); err != nil {
		return WrapDatabaseError("mark attendance rows not scheduled", err)
	}
	return nil
}

// markExpectedAbsentBySessions stamps the still expected children of the
// ended sessions absent, except the excluded ones.
func markExpectedAbsentBySessions(ctx context.Context, presence ports.ParticipantPresence, activeGroupIDs []int64, updatedAt time.Time, exclusions []scheduleModels.StudentInstanceRef) error {
	if len(activeGroupIDs) == 0 {
		return nil
	}
	refs := make([]studentpresence.PlannedParticipantRef, 0, len(exclusions))
	for _, ref := range exclusions {
		if ref.InstanceID > 0 && ref.StudentID > 0 {
			refs = append(refs, studentpresence.PlannedParticipantRef(ref))
		}
	}
	if err := presence.MarkExpectedAbsentByActiveGroupIDs(ctx, positiveInstanceStudentIDs(activeGroupIDs), updatedAt, refs); err != nil {
		return WrapDatabaseError("mark expected absent by active group ids", err)
	}
	return nil
}

func applyActiveStatusDaysToInstance(ctx context.Context, presence ports.ParticipantPresence, instanceID int64, date calendar.Date) (int, error) {
	changed, err := presence.ApplyActiveStatusDaysForInstance(ctx, instanceID, date.String())
	if err != nil {
		return 0, WrapDatabaseError("apply active status days to instance", err)
	}
	return changed, nil
}

func applyActivePartialAbsencesToInstance(ctx context.Context, presence ports.ParticipantPresence, instanceID int64, date calendar.Date) (int, error) {
	changed, err := presence.ApplyActivePartialAbsencesForInstance(ctx, instanceID, date.String())
	if err != nil {
		return 0, WrapDatabaseError("apply active partial absences to instance", err)
	}
	return changed, nil
}

// plannedStudentIDsOn lists which of the students the plan lists on a day.
func plannedStudentIDsOn(ctx context.Context, plans timetable.InstanceStudentCapability, studentIDs []int64, date calendar.Date) ([]int64, error) {
	studentIDs = positiveInstanceStudentIDs(studentIDs)
	if len(studentIDs) == 0 {
		return []int64{}, nil
	}
	result, err := plans.ListPlannedStudentIDs(ctx, studentIDs, date.String())
	if err != nil {
		return nil, WrapDatabaseError("find planned student ids by date", err)
	}
	return result, nil
}

// listLegacyParticipants reads the participants with their attendance; a
// filter whose IDs are all <= 0 lists nothing.
func listLegacyParticipants(ctx context.Context, reads ports.LegacyParticipantReads, filter timetable.InstanceStudentFilter, operation string) ([]*scheduleModels.InstanceStudent, error) {
	requestedIDs, requestedInstances, requestedStudents := len(filter.IDs) > 0, len(filter.InstanceIDs) > 0, len(filter.StudentIDs) > 0
	filter.IDs = positiveInstanceStudentIDs(filter.IDs)
	filter.InstanceIDs = positiveInstanceStudentIDs(filter.InstanceIDs)
	filter.StudentIDs = positiveInstanceStudentIDs(filter.StudentIDs)
	if requestedIDs && len(filter.IDs) == 0 || requestedInstances && len(filter.InstanceIDs) == 0 || requestedStudents && len(filter.StudentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	rows, err := reads.ListLegacyParticipants(ctx, ports.LegacyParticipantFilter{
		IDs: filter.IDs, InstanceIDs: filter.InstanceIDs, StudentIDs: filter.StudentIDs,
		Date: filter.Date, FromDate: filter.FromDate, ToDate: filter.ToDate, CurrentTime: filter.CurrentTime,
		OrderByCreated: filter.OrderByCreated, OrderByInstanceStudent: filter.OrderByInstanceStudent,
		OrderByStudentActivityTime: filter.OrderByStudentActivityTime, OrderByActivityDateTime: filter.OrderByActivityDateTime,
		Limit: filter.Limit, Offset: filter.Offset,
	})
	if err != nil {
		return nil, WrapDatabaseError(operation, err)
	}
	return rows, nil
}

// attendanceByParticipant reads the stored attendance of the participants;
// a participant without a row is expected.
func attendanceByParticipant(ctx context.Context, presence studentpresence.SessionAttendanceQuery, participantIDs []int64) (map[int64]studentpresence.SessionAttendance, error) {
	result := make(map[int64]studentpresence.SessionAttendance, len(participantIDs))
	if len(participantIDs) == 0 {
		return result, nil
	}
	rows, err := presence.ListSessionAttendance(ctx, participantIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ParticipantID] = row
	}
	return result, nil
}

// participantIDsOf resolves (instance, student) pairs to participant ids; a
// pair the plan does not list resolves to nothing.
func participantIDsOf(ctx context.Context, reads ports.LegacyParticipantReads, keys []scheduleModels.InstanceStudentKey, excludeFinished bool) ([]int64, error) {
	instanceIDs, studentIDs := make([]int64, 0, len(keys)), make([]int64, 0, len(keys))
	wanted := make(map[scheduleModels.InstanceStudentKey]bool, len(keys))
	for _, key := range keys {
		if key.InstanceID <= 0 || key.StudentID <= 0 {
			continue
		}
		wanted[key] = true
		instanceIDs = append(instanceIDs, key.InstanceID)
		studentIDs = append(studentIDs, key.StudentID)
	}
	if len(wanted) == 0 {
		return nil, nil
	}
	participants, err := reads.ListLegacyParticipants(ctx, ports.LegacyParticipantFilter{InstanceIDs: uniqueInt64s(instanceIDs), StudentIDs: uniqueInt64s(studentIDs), ExcludeFinished: excludeFinished})
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		if wanted[scheduleModels.InstanceStudentKey{InstanceID: participant.InstanceID, StudentID: participant.StudentID}] {
			ids = append(ids, participant.ID)
		}
	}
	return ids, nil
}

func filterLegacyInstanceStudents(rows []*scheduleModels.InstanceStudent, keep func(*scheduleModels.InstanceStudent) bool) []*scheduleModels.InstanceStudent {
	result := make([]*scheduleModels.InstanceStudent, 0, len(rows))
	for _, row := range rows {
		if keep(row) {
			result = append(result, row)
		}
	}
	return result
}

func positiveInstanceStudentIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			result = append(result, id)
		}
	}
	return result
}

func uniqueInt64s(ids []int64) []int64 {
	result := slices.Clone(ids)
	slices.Sort(result)
	return slices.Compact(result)
}

func sortedKeys(values map[int64]string) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func legacyInstanceStudent(value timetable.InstanceStudent, attendance map[int64]studentpresence.SessionAttendance) *scheduleModels.InstanceStudent {
	result := &scheduleModels.InstanceStudent{}
	row, ok := attendance[value.ID]
	if !ok {
		row = studentpresence.ExpectedSessionAttendance(value.ID)
	}
	replaceLegacyInstanceStudent(result, value, row)
	return result
}

func replaceLegacyInstanceStudent(result *scheduleModels.InstanceStudent, value timetable.InstanceStudent, attendance studentpresence.SessionAttendance) {
	*result = scheduleModels.InstanceStudent{InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID,
		Status: attendance.Status, Substatus: attendance.Substatus, Note: attendance.Note, CheckedInAt: attendance.CheckedInAt,
		CheckedOutAt: attendance.CheckedOutAt, IsUnplanned: attendance.IsUnplanned, NotScheduled: attendance.NotScheduled,
		ManualStatusAt: attendance.ManualStatusAt, StudentStatusDayID: attendance.StudentStatusDayID, PickupExceptionID: attendance.PickupExceptionID}
	result.ID, result.CreatedAt, result.UpdatedAt = value.ID, value.CreatedAt, value.UpdatedAt
	if attendance.UpdatedAt.After(result.UpdatedAt) {
		result.UpdatedAt = attendance.UpdatedAt
	}
	result.SetTenantID(value.TenantID)
}

func publicInstanceStudentInput(value *scheduleModels.InstanceStudent) timetable.InstanceStudentInput {
	return timetable.InstanceStudentInput{InstanceID: value.InstanceID, StudentID: value.StudentID, RoomID: value.RoomID}
}

// legacyAttendanceDiffers reports whether the DTO carries anything beyond
// expected attendance.
func legacyAttendanceDiffers(value *scheduleModels.InstanceStudent) bool {
	return (value.Status != "" && value.Status != scheduleModels.AttendanceStatusExpected) || value.Substatus != nil || value.Note != nil ||
		value.CheckedInAt != nil || value.CheckedOutAt != nil || value.IsUnplanned || value.NotScheduled ||
		value.ManualStatusAt != nil || value.StudentStatusDayID != nil || value.PickupExceptionID != nil
}

func legacyAttendanceRestore(participantID int64, value *scheduleModels.InstanceStudent) studentpresence.SessionAttendanceRestore {
	status := value.Status
	if status == "" {
		status = scheduleModels.AttendanceStatusExpected
	}
	return studentpresence.SessionAttendanceRestore{
		ParticipantID: participantID, Status: status, Substatus: value.Substatus, Note: value.Note,
		CheckedInAt: value.CheckedInAt, CheckedOutAt: value.CheckedOutAt, IsUnplanned: value.IsUnplanned,
		NotScheduled: value.NotScheduled, ManualStatusAt: value.ManualStatusAt,
		StudentStatusDayID: value.StudentStatusDayID, PickupExceptionID: value.PickupExceptionID,
	}
}

func noParticipantRows(rows ports.ParticipantRows) bool {
	return rows.Plans == nil && rows.Presence == nil && rows.Reads == nil
}

// scheduledInstancesForStudent lists the blocks of a student's date range
// with the student's attendance; a student ID <= 0 lists nothing.
func scheduledInstancesForStudent(ctx context.Context, reads ports.LegacyParticipantReads, studentID int64, from, to calendar.Date) ([]*scheduleModels.ScheduledInstanceRow, error) {
	if studentID <= 0 {
		return []*scheduleModels.ScheduledInstanceRow{}, nil
	}
	result, err := reads.ListScheduledInstancesForStudent(ctx, studentID, from.String(), to.String())
	if err != nil {
		return nil, WrapDatabaseError("find instances with attendance by student and date range", err)
	}
	return result, nil
}

// ParticipantCounts count the children per block for the Betreuungsplan.
type ParticipantCounts struct {
	reads ports.LegacyParticipantReads
}

// NewParticipantCounts binds the counts to the joined participant read.
func NewParticipantCounts(reads ports.LegacyParticipantReads) ParticipantCounts {
	return ParticipantCounts{reads: reads}
}

// CountNonAbsentByInstanceIDs counts the participants per block that are not
// absent; a block without one is missing from the map.
func (c ParticipantCounts) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	result, err := c.reads.CountNonAbsentParticipants(ctx, positiveInstanceStudentIDs(instanceIDs))
	if err != nil {
		return nil, WrapDatabaseError("count non-absent by instance ids", err)
	}
	return result, nil
}

// ParticipantCheckouts close the open presence of the sessions a cleanup
// ends.
type ParticipantCheckouts struct {
	presence ports.ParticipantPresence
}

// NewParticipantCheckouts binds the checkouts to the participants'
// attendance.
func NewParticipantCheckouts(rows ports.ParticipantRows) ParticipantCheckouts {
	return ParticipantCheckouts{presence: rows.Presence}
}

// CloseOpenCheckoutsByActiveGroupIDs stamps the checkout on the open
// presence of the sessions; no sessions close nothing.
func (c ParticipantCheckouts) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}
	closed, err := c.presence.CloseOpenCheckoutsByActiveGroupIDs(ctx, positiveInstanceStudentIDs(activeGroupIDs), checkedOutAt)
	if err != nil {
		return 0, WrapDatabaseError("close open checkouts by active group ids", err)
	}
	return closed, nil
}
