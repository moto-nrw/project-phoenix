package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// ParticipantPresence is the Student Presence surface the legacy participant
// adapter composes: the sessions of the blocks and the attendance of their
// participants (#2762).
type ParticipantPresence interface {
	studentpresence.ActivitySessionQuery
	studentpresence.SessionAttendanceQuery
	studentpresence.SessionAttendanceCommand
	studentpresence.SessionAttendanceRules
}

// timetableInstanceStudentRepository serves the legacy instance_students DTO,
// which used to be one row, from its two owners: the participant is
// Timetable's, the attendance is Student Presence's, and a participant
// without an attendance row is expected. Reads that join both halves with the
// block go through the tenant-safe projection.
type timetableInstanceStudentRepository struct {
	reads     PresenceReads
	preview   PartialAbsences
	timetable timetable.InstanceStudentCapability
	presence  ParticipantPresence
}

// NewInstanceStudentRepository serves the retained participant repository
// over the Timetable owner and Student Presence; reads are the joined
// presence reads over the database.
func NewInstanceStudentRepository(reads PresenceReads, capability timetable.InstanceStudentCapability, presence ParticipantPresence, preview PartialAbsences) ports.InstanceStudentRecords {
	return newTimetableInstanceStudentRepository(reads, capability, presence, preview)
}

func newTimetableInstanceStudentRepository(reads PresenceReads, capability timetable.InstanceStudentCapability, presence ParticipantPresence, preview PartialAbsences) timetableInstanceStudentRepository {
	if reads == nil || capability == nil || presence == nil || preview == nil {
		panic("timetable instance student repository: database, timetable, student presence and partial absence preview are required")
	}
	return timetableInstanceStudentRepository{reads: reads, preview: preview, timetable: capability, presence: presence}
}

func (r timetableInstanceStudentRepository) Create(ctx context.Context, value *scheduleModels.InstanceStudent) error {
	if value == nil {
		return errors.New("instance student cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateInstanceStudent(ctx, publicInstanceStudentInput(value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	attendance := studentpresence.ExpectedSessionAttendance(created.ID)
	if legacyAttendanceDiffers(value) {
		if err := r.presence.RestoreSessionAttendance(ctx, []studentpresence.SessionAttendanceRestore{legacyAttendanceRestore(created.ID, value)}); err != nil {
			return WrapDatabaseError("create", err)
		}
		rows, err := r.presence.ListSessionAttendance(ctx, []int64{created.ID})
		if err != nil {
			return WrapDatabaseError("create", err)
		}
		if len(rows) > 0 {
			attendance = rows[0]
		}
	}
	replaceLegacyInstanceStudent(value, created, attendance)
	return nil
}

func (r timetableInstanceStudentRepository) FindByID(ctx context.Context, id int64) (*scheduleModels.InstanceStudent, error) {
	rowID, ok := LegacyGroupID(id)
	if !ok {
		return nil, WrapDatabaseError("find by id", fmt.Errorf("invalid instance student id %T", id))
	}
	if rowID <= 0 {
		return nil, WrapNotFoundDatabaseError("find by id")
	}
	rows, err := r.list(ctx, timetable.InstanceStudentFilter{IDs: []int64{rowID}}, "find by id")
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, WrapNotFoundDatabaseError("find by id")
	}
	return rows[0], nil
}

func (r timetableInstanceStudentRepository) Update(ctx context.Context, value *scheduleModels.InstanceStudent) error {
	if value == nil {
		return errors.New("instance student cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateInstanceStudent(ctx, value.ID, publicInstanceStudentInput(value))
	if err != nil {
		return legacyInstanceStudentError("update", err)
	}
	current, err := r.attendanceByParticipant(ctx, []int64{updated.ID})
	if err != nil {
		return WrapDatabaseError("update", err)
	}
	attendance, ok := current[updated.ID]
	if !ok {
		attendance = studentpresence.ExpectedSessionAttendance(updated.ID)
	}
	if wanted := legacyAttendanceRestore(updated.ID, value); !sameLegacyAttendanceRestore(wanted, legacyAttendanceRestoreOf(attendance)) {
		if err := r.presence.RestoreSessionAttendance(ctx, []studentpresence.SessionAttendanceRestore{wanted}); err != nil {
			return WrapDatabaseError("update", err)
		}
		current, err = r.attendanceByParticipant(ctx, []int64{updated.ID})
		if err != nil {
			return WrapDatabaseError("update", err)
		}
		attendance = current[updated.ID]
	}
	replaceLegacyInstanceStudent(value, updated, attendance)
	return nil
}

// legacyAttendanceRestoreOf is the stored attendance in restore shape, so an
// unchanged row is recognised without a write.
func legacyAttendanceRestoreOf(row studentpresence.SessionAttendance) studentpresence.SessionAttendanceRestore {
	status := row.Status
	if status == "" {
		status = studentpresence.SessionAttendanceExpected
	}
	return studentpresence.SessionAttendanceRestore{
		ParticipantID: row.ParticipantID, Status: status, Substatus: row.Substatus, Note: row.Note,
		CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt, IsUnplanned: row.IsUnplanned,
		NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
}

// sameLegacyAttendanceRestore holds the wanted attendance against the stored
// one by value. The restore shape carries pointers, so Go's == would compare
// their addresses and report a change on every call.
func sameLegacyAttendanceRestore(wanted, stored studentpresence.SessionAttendanceRestore) bool {
	return wanted.ParticipantID == stored.ParticipantID && wanted.Status == stored.Status &&
		sameLegacyAttendanceValue(wanted.Substatus, stored.Substatus) &&
		sameLegacyAttendanceValue(wanted.Note, stored.Note) &&
		sameLegacyAttendanceInstant(wanted.CheckedInAt, stored.CheckedInAt) &&
		sameLegacyAttendanceInstant(wanted.CheckedOutAt, stored.CheckedOutAt) &&
		wanted.IsUnplanned == stored.IsUnplanned && wanted.NotScheduled == stored.NotScheduled &&
		sameLegacyAttendanceInstant(wanted.ManualStatusAt, stored.ManualStatusAt) &&
		sameLegacyAttendanceValue(wanted.StudentStatusDayID, stored.StudentStatusDayID) &&
		sameLegacyAttendanceValue(wanted.PickupExceptionID, stored.PickupExceptionID)
}

func sameLegacyAttendanceValue[T comparable](wanted, stored *T) bool {
	if wanted == nil || stored == nil {
		return wanted == nil && stored == nil
	}
	return *wanted == *stored
}

// sameLegacyAttendanceInstant compares the instant, not the location the
// stored row was decoded in.
func sameLegacyAttendanceInstant(wanted, stored *time.Time) bool {
	if wanted == nil || stored == nil {
		return wanted == nil && stored == nil
	}
	return wanted.Equal(*stored)
}

func (r timetableInstanceStudentRepository) Delete(ctx context.Context, id int64) error {
	rowID, ok := LegacyGroupID(id)
	if !ok {
		return WrapDatabaseError("delete", fmt.Errorf("invalid instance student id %T", id))
	}
	if err := r.timetable.DeleteInstanceStudent(ctx, rowID); err != nil {
		return WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error) {
	return r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}, OrderByCreated: true}, "find by instance id")
}

func (r timetableInstanceStudentRepository) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	if len(instanceIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: instanceIDs, OrderByInstanceStudent: true}, "find by instance ids")
}

func (r timetableInstanceStudentRepository) FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := r.FindByInstanceIDs(ctx, instanceIDs)
	if err != nil {
		return nil, err
	}
	return filterLegacyInstanceStudents(rows, func(row *scheduleModels.InstanceStudent) bool {
		return row.Status == scheduleModels.AttendanceStatusExpected
	}), nil
}

// FindNotScheduledCandidatesByInstanceIDs mirrors the rows Student Presence's
// non-booking marker can still change: expected rows and absences a day
// status or an excusal owns, never a decision somebody made by hand.
func (r timetableInstanceStudentRepository) FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := r.FindByInstanceIDs(ctx, instanceIDs)
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

func (r timetableInstanceStudentRepository) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	result, err := r.reads.CountNonAbsentParticipants(ctx, positiveInstanceStudentIDs(instanceIDs))
	if err != nil {
		return nil, WrapDatabaseError("count non-absent by instance ids", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindPresentInOtherActiveInstances(ctx context.Context, excludedID int64, date calendar.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	result, err := r.reads.ListParallelPresence(ctx, excludedID, date.String(), positiveInstanceStudentIDs(studentIDs))
	if err != nil {
		return nil, WrapDatabaseError("find present in other active instances", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date calendar.Date, cutoff time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	result, err := r.preview.FindPartialAbsenceBlocks(ctx, studentID, date.String(), cutoff)
	if err != nil {
		return nil, WrapDatabaseError("find partial absence blocks", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.InstanceStudent, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetable.InstanceStudentFilter{StudentIDs: []int64{studentID}, FromDate: &fromText,
		ToDate: &toText, OrderByActivityDateTime: true}, "find by student and date range")
}

func (r timetableInstanceStudentRepository) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*scheduleModels.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	text := date.String()
	return r.list(ctx, timetable.InstanceStudentFilter{StudentIDs: studentIDs, Date: &text,
		OrderByStudentActivityTime: true}, "find by student ids and date")
}

func (r timetableInstanceStudentRepository) FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error) {
	values, err := r.list(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}, StudentIDs: []int64{studentID}}, "find by instance and student")
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}

func (r timetableInstanceStudentRepository) FindCurrentCandidates(ctx context.Context, studentID int64, date calendar.Date, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	return r.findCurrent(ctx, []int64{studentID}, date, at, "find current student slot candidates")
}

func (r timetableInstanceStudentRepository) FindCurrentCandidatesByStudentIDs(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	if len(studentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	return r.findCurrent(ctx, studentIDs, date, at, "find current student slot candidates batch")
}

// findCurrent lists the running blocks of the students; a block that ended
// is no candidate.
func (r timetableInstanceStudentRepository) findCurrent(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time, operation string) ([]*scheduleModels.InstanceStudent, error) {
	dateText, clock := date.String(), InstanceStudentWallClock(at)
	return r.list(ctx, timetable.InstanceStudentFilter{StudentIDs: studentIDs, Date: &dateText, CurrentTime: &clock,
		OrderByStudentActivityTime: true}, operation)
}

func (r timetableInstanceStudentRepository) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	if err := r.timetable.DeleteInstanceStudentsByInstance(ctx, instanceID); err != nil {
		return WrapDatabaseError("delete by instance id", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error) {
	ids, err := r.participantIDs(ctx, []scheduleModels.InstanceStudentKey{{InstanceID: instanceID, StudentID: studentID}}, false)
	if err != nil {
		return false, WrapDatabaseError("update attendance from checkin", err)
	}
	if len(ids) == 0 {
		return false, nil
	}
	rows, err := r.presence.CheckInParticipants(ctx, ids, checkedInAt)
	if err != nil {
		return false, WrapDatabaseError("update attendance from checkin", err)
	}
	return rows > 0, nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, checkedInAt time.Time) error {
	ids, err := r.participantIDs(ctx, keys, false)
	if err != nil {
		return WrapDatabaseError("update attendance from checkin batch", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.presence.CheckInParticipants(ctx, ids, checkedInAt); err != nil {
		return WrapDatabaseError("update attendance from checkin batch", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error {
	return r.UpdateAttendanceCheckoutBatch(ctx, []scheduleModels.InstanceStudentKey{{InstanceID: instanceID, StudentID: studentID}}, checkedOutAt)
}

func (r timetableInstanceStudentRepository) UpdateAttendanceCheckoutBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, checkedOutAt time.Time) error {
	ids, err := r.participantIDs(ctx, keys, false)
	if err != nil {
		return WrapDatabaseError("update slot attendance checkout batch", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.presence.CheckOutParticipants(ctx, ids, checkedOutAt); err != nil {
		return WrapDatabaseError("update slot attendance checkout batch", err)
	}
	return nil
}

// CreateUnplannedPresentIfAbsent lists the child on the block if the plan
// does not, then opens the presence: as a walk-in for a new participant, as a
// check-in for a planned one.
func (r timetableInstanceStudentRepository) CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (*scheduleModels.InstanceStudent, error) {
	participant, inserted, err := r.timetable.EnsureInstanceStudent(ctx, instanceID, studentID)
	if err != nil {
		return nil, WrapDatabaseError("create unplanned slot attendance", err)
	}
	if inserted {
		if _, err := r.presence.CheckInWalkIn(ctx, participant.ID, checkedInAt); err != nil {
			return nil, WrapDatabaseError("create unplanned slot attendance", err)
		}
	} else if _, err := r.presence.CheckInParticipants(ctx, []int64{participant.ID}, checkedInAt); err != nil {
		return nil, WrapDatabaseError("create unplanned slot attendance", err)
	}
	attendance, err := r.attendanceByParticipant(ctx, []int64{participant.ID})
	if err != nil {
		return nil, WrapDatabaseError("create unplanned slot attendance", err)
	}
	return legacyInstanceStudent(participant, attendance), nil
}

func (r timetableInstanceStudentRepository) ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error) {
	ids, err := r.participantIDs(ctx, []scheduleModels.InstanceStudentKey{{InstanceID: instanceID, StudentID: studentID}}, false)
	if err != nil {
		return false, WrapDatabaseError("reconcile slot attendance interval", err)
	}
	if len(ids) == 0 {
		return false, nil
	}
	updated, err := r.presence.ReconcileParticipantInterval(ctx, ids[0], previousCheckIn, previousCheckOut, updatedCheckIn, updatedCheckOut)
	if err != nil {
		return false, WrapDatabaseError("reconcile slot attendance interval", err)
	}
	return updated, nil
}

func (r timetableInstanceStudentRepository) UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error {
	if !patch.HasChanges() {
		return nil
	}
	if err := r.presence.PatchSessionAttendance(ctx, id, studentpresence.SessionAttendancePatch{Status: patch.Status, Substatus: patch.Substatus,
		SubstatusClear: patch.SubstatusClear, Note: patch.Note, NoteClear: patch.NoteClear}); err != nil {
		return WrapDatabaseError("update attendance fields", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludedStudentIDs []int64) (int, error) {
	participants, err := r.timetable.ListInstanceStudents(ctx, timetable.InstanceStudentFilter{InstanceIDs: []int64{instanceID}})
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
	rows, err := r.presence.TransitionParticipants(ctx, ids, fromStatus, toStatus, time.Time{})
	if err != nil {
		return 0, WrapDatabaseError("bulk update status", err)
	}
	return int(rows), nil
}

func (r timetableInstanceStudentRepository) MarkNotScheduled(ctx context.Context, refs []scheduleModels.StudentInstanceRef) error {
	keys := make([]scheduleModels.InstanceStudentKey, 0, len(refs))
	for _, ref := range refs {
		keys = append(keys, scheduleModels.InstanceStudentKey{InstanceID: ref.InstanceID, StudentID: ref.StudentID})
	}
	ids, err := r.participantIDs(ctx, keys, true)
	if err != nil {
		return WrapDatabaseError("mark attendance rows not scheduled", err)
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.presence.MarkParticipantsNotScheduled(ctx, ids); err != nil {
		return WrapDatabaseError("mark attendance rows not scheduled", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []scheduleModels.StudentInstanceRef) error {
	if len(activeGroupIDs) == 0 {
		return nil
	}
	refs := make([]studentpresence.PlannedParticipantRef, 0, len(exclusions))
	for _, ref := range exclusions {
		if ref.InstanceID > 0 && ref.StudentID > 0 {
			refs = append(refs, studentpresence.PlannedParticipantRef(ref))
		}
	}
	if err := r.presence.MarkExpectedAbsentByActiveGroupIDs(ctx, positiveInstanceStudentIDs(activeGroupIDs), updatedAt, refs); err != nil {
		return WrapDatabaseError("mark expected absent by active group ids", err)
	}
	return nil
}

func (r timetableInstanceStudentRepository) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error) {
	if len(activeGroupIDs) == 0 {
		return 0, nil
	}
	rows, err := r.presence.CloseOpenCheckoutsByActiveGroupIDs(ctx, positiveInstanceStudentIDs(activeGroupIDs), checkedOutAt)
	if err != nil {
		return 0, WrapDatabaseError("close open checkouts by active group ids", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ListStudentInstanceRefsBefore(ctx context.Context, cutoff calendar.Date) ([]scheduleModels.StudentInstanceRef, error) {
	values, err := r.timetable.ListStudentInstanceRefsBefore(ctx, cutoff.String())
	if err != nil {
		return nil, WrapDatabaseError("list student instance refs before", err)
	}
	result := make([]scheduleModels.StudentInstanceRef, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleModels.StudentInstanceRef(value))
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) ApplyStatusDay(ctx context.Context, studentID int64, date calendar.Date, statusDayID int64, substatus string) (int, error) {
	rows, err := r.presence.ApplyStatusDay(ctx, studentID, date.String(), statusDayID, substatus)
	if err != nil {
		return 0, WrapDatabaseError("apply student status day to slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	rows, err := r.presence.ReleaseStatusDay(ctx, statusDayID)
	if err != nil {
		return 0, WrapDatabaseError("release student status day from slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date calendar.Date) (int, error) {
	rows, err := r.presence.ApplyActiveStatusDaysForInstance(ctx, instanceID, date.String())
	if err != nil {
		return 0, WrapDatabaseError("apply active status days to instance", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	rows, err := r.presence.ApplyPartialAbsence(ctx, pickupExceptionID)
	if err != nil {
		return 0, WrapDatabaseError("apply partial absence to slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	rows, err := r.presence.ReleasePartialAbsence(ctx, pickupExceptionID)
	if err != nil {
		return 0, WrapDatabaseError("release partial absence from slots", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date calendar.Date) (int, error) {
	rows, err := r.presence.ApplyActivePartialAbsencesForInstance(ctx, instanceID, date.String())
	if err != nil {
		return 0, WrapDatabaseError("apply active partial absences to instance", err)
	}
	return rows, nil
}

func (r timetableInstanceStudentRepository) FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.ScheduledInstanceRow, error) {
	if studentID <= 0 {
		return []*scheduleModels.ScheduledInstanceRow{}, nil
	}
	result, err := r.reads.ListScheduledInstancesForStudent(ctx, studentID, from.String(), to.String())
	if err != nil {
		return nil, WrapDatabaseError("find instances with attendance by student and date range", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) HasPlannedSlotsInRange(ctx context.Context, from, to calendar.Date) (bool, error) {
	result, err := r.reads.HasPlannedSlotsInRange(ctx, from.String(), to.String())
	if err != nil {
		return false, WrapDatabaseError("check planned slots in range", err)
	}
	return result, nil
}

func (r timetableInstanceStudentRepository) FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error) {
	studentIDs = positiveInstanceStudentIDs(studentIDs)
	if len(studentIDs) == 0 {
		return []int64{}, nil
	}
	result, err := r.timetable.ListPlannedStudentIDs(ctx, studentIDs, date.String())
	if err != nil {
		return nil, WrapDatabaseError("find planned student ids by date", err)
	}
	return result, nil
}

// list reads the participants and joins their attendance.
func (r timetableInstanceStudentRepository) list(ctx context.Context, filter timetable.InstanceStudentFilter, operation string) ([]*scheduleModels.InstanceStudent, error) {
	requestedIDs, requestedInstances, requestedStudents := len(filter.IDs) > 0, len(filter.InstanceIDs) > 0, len(filter.StudentIDs) > 0
	filter.IDs = positiveInstanceStudentIDs(filter.IDs)
	filter.InstanceIDs = positiveInstanceStudentIDs(filter.InstanceIDs)
	filter.StudentIDs = positiveInstanceStudentIDs(filter.StudentIDs)
	if requestedIDs && len(filter.IDs) == 0 || requestedInstances && len(filter.InstanceIDs) == 0 || requestedStudents && len(filter.StudentIDs) == 0 {
		return []*scheduleModels.InstanceStudent{}, nil
	}
	rows, err := r.reads.ListLegacyParticipants(ctx, ports.LegacyParticipantFilter{
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
func (r timetableInstanceStudentRepository) attendanceByParticipant(ctx context.Context, participantIDs []int64) (map[int64]studentpresence.SessionAttendance, error) {
	result := make(map[int64]studentpresence.SessionAttendance, len(participantIDs))
	if len(participantIDs) == 0 {
		return result, nil
	}
	rows, err := r.presence.ListSessionAttendance(ctx, participantIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ParticipantID] = row
	}
	return result, nil
}

// participantIDs resolves (instance, student) pairs to participant ids; a
// pair the plan does not list resolves to nothing.
func (r timetableInstanceStudentRepository) participantIDs(ctx context.Context, keys []scheduleModels.InstanceStudentKey, excludeFinished bool) ([]int64, error) {
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
	participants, err := r.reads.ListLegacyParticipants(ctx, ports.LegacyParticipantFilter{InstanceIDs: uniqueInt64s(instanceIDs), StudentIDs: uniqueInt64s(studentIDs), ExcludeFinished: excludeFinished})
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

func legacyInstanceStudentError(operation string, err error) error {
	if errors.Is(err, timetable.ErrInstanceStudentNotFound) {
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}

func (r timetableInstanceStudentRepository) ListStudents(ctx context.Context, filter ports.ParticipantListingFilter) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := r.list(ctx, timetable.InstanceStudentFilter{IDs: filter.IDs, InstanceIDs: filter.InstanceIDs,
		StudentIDs: filter.StudentIDs, Limit: filter.Limit, Offset: filter.Offset}, "list with options")
	if err != nil || filter.Status == nil {
		return rows, err
	}
	return filterLegacyInstanceStudents(rows, func(row *scheduleModels.InstanceStudent) bool { return row.Status == *filter.Status }), nil
}
