//go:build testhooks

package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// nativeFixtureParticipants serve the integration fixtures' legacy
// participant row reads and writes over the owner's participant rows.
type nativeFixtureParticipants struct {
	rows    ports.ParticipantRows
	preview PartialAbsences
}

// NativeParticipantRecords binds the fixtures' participant rows; preview
// lists the blocks an earlier pickup excuses.
func NativeParticipantRecords(rows ports.ParticipantRows, preview PartialAbsences) ports.NativeParticipantRecords {
	if noParticipantRows(rows) || preview == nil {
		panic("timetable instance student repository: database, timetable, student presence and partial absence preview are required")
	}
	return nativeFixtureParticipants{rows: rows, preview: preview}
}

func (f nativeFixtureParticipants) Create(ctx context.Context, value *scheduleModels.InstanceStudent) error {
	return createLegacyParticipant(ctx, f.rows, value)
}

func (f nativeFixtureParticipants) FindByID(ctx context.Context, id int64) (*scheduleModels.InstanceStudent, error) {
	return findLegacyParticipant(ctx, f.rows.Reads, id)
}

func (f nativeFixtureParticipants) Update(ctx context.Context, value *scheduleModels.InstanceStudent) error {
	return updateLegacyParticipant(ctx, f.rows, value)
}

func (f nativeFixtureParticipants) Delete(ctx context.Context, id int64) error {
	return deleteLegacyParticipant(ctx, f.rows.Plans, id)
}

func (f nativeFixtureParticipants) FindPresentInOtherActiveInstances(ctx context.Context, excludedID int64, date calendar.Date, studentIDs []int64) ([]scheduleModels.ParallelPresence, error) {
	return parallelPresence(ctx, f.rows.Reads, excludedID, date, studentIDs)
}

func (f nativeFixtureParticipants) FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error) {
	return participantsOfInstance(ctx, f.rows.Reads, instanceID)
}

func (f nativeFixtureParticipants) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	return participantsOfInstances(ctx, f.rows.Reads, instanceIDs)
}

func (f nativeFixtureParticipants) FindExpectedByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	return expectedParticipantsOf(ctx, f.rows.Reads, instanceIDs)
}

func (f nativeFixtureParticipants) FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error) {
	return notScheduledCandidatesOf(ctx, f.rows.Reads, instanceIDs)
}

func (f nativeFixtureParticipants) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	return NewParticipantCounts(f.rows.Reads).CountNonAbsentByInstanceIDs(ctx, instanceIDs)
}

func (f nativeFixtureParticipants) FindByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.InstanceStudent, error) {
	return participantsOfStudentBetween(ctx, f.rows.Reads, studentID, from, to)
}

func (f nativeFixtureParticipants) FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*scheduleModels.InstanceStudent, error) {
	return participantsOfStudentsOn(ctx, f.rows.Reads, studentIDs, date)
}

func (f nativeFixtureParticipants) FindCurrentCandidatesByStudentIDs(ctx context.Context, studentIDs []int64, date calendar.Date, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	return currentCandidatesOfStudents(ctx, f.rows.Reads, studentIDs, date, at)
}

func (f nativeFixtureParticipants) UpdateAttendanceFromCheckinBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, checkedInAt time.Time) error {
	return checkInParticipants(ctx, f.rows, keys, checkedInAt)
}

func (f nativeFixtureParticipants) UpdateAttendanceCheckoutBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, checkedOutAt time.Time) error {
	return checkOutParticipants(ctx, f.rows, keys, checkedOutAt)
}

func (f nativeFixtureParticipants) FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error) {
	return participantOf(ctx, f.rows.Reads, instanceID, studentID)
}

func (f nativeFixtureParticipants) DeleteByInstanceID(ctx context.Context, instanceID int64) error {
	return deleteParticipantsOfInstance(ctx, f.rows.Plans, instanceID)
}

func (f nativeFixtureParticipants) ArchivePlannedByStudentIDsFrom(ctx context.Context, transitionID int64, studentIDs []int64, from calendar.Date, at time.Time) (int, error) {
	return archivePlannedParticipants(ctx, f.rows, transitionID, studentIDs, from, at)
}

func (f nativeFixtureParticipants) RestoreArchivedByTransition(ctx context.Context, transitionID int64, studentIDs []int64, from calendar.Date) (int, error) {
	return restoreArchivedParticipants(ctx, f.rows, transitionID, studentIDs, from)
}

func (f nativeFixtureParticipants) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (bool, error) {
	return checkInParticipant(ctx, f.rows, instanceID, studentID, checkedInAt)
}

func (f nativeFixtureParticipants) CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, checkedInAt time.Time) (*scheduleModels.InstanceStudent, error) {
	return createUnplannedPresentIfAbsent(ctx, f.rows, instanceID, studentID, checkedInAt)
}

func (f nativeFixtureParticipants) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, checkedOutAt time.Time) error {
	return checkOutParticipant(ctx, f.rows, instanceID, studentID, checkedOutAt)
}

func (f nativeFixtureParticipants) ReconcileAttendanceInterval(ctx context.Context, instanceID, studentID int64, previousCheckIn time.Time, previousCheckOut *time.Time, updatedCheckIn time.Time, updatedCheckOut *time.Time) (bool, error) {
	return reconcileAttendanceInterval(ctx, f.rows, instanceID, studentID, previousCheckIn, previousCheckOut, updatedCheckIn, updatedCheckOut)
}

func (f nativeFixtureParticipants) FindCurrentCandidates(ctx context.Context, studentID int64, date calendar.Date, at time.Time) ([]*scheduleModels.InstanceStudent, error) {
	return currentCandidates(ctx, f.rows.Reads, studentID, date, at)
}

func (f nativeFixtureParticipants) ApplyStatusDay(ctx context.Context, studentID int64, date calendar.Date, statusDayID int64, substatus string) (int, error) {
	return NewInstanceStatusSlots(f.rows).ApplyStatusDay(ctx, studentID, date.String(), statusDayID, substatus)
}

func (f nativeFixtureParticipants) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	return NewInstanceStatusSlots(f.rows).ReleaseStatusDay(ctx, statusDayID)
}

func (f nativeFixtureParticipants) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date calendar.Date) (int, error) {
	return applyActiveStatusDaysToInstance(ctx, f.rows.Presence, instanceID, date)
}

func (f nativeFixtureParticipants) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	return applyPartialAbsence(ctx, f.rows.Presence, pickupExceptionID)
}

func (f nativeFixtureParticipants) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	return releasePartialAbsence(ctx, f.rows.Presence, pickupExceptionID)
}

func (f nativeFixtureParticipants) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date calendar.Date) (int, error) {
	return applyActivePartialAbsencesToInstance(ctx, f.rows.Presence, instanceID, date)
}

func (f nativeFixtureParticipants) UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error {
	return patchAttendanceFields(ctx, f.rows.Presence, id, patch)
}

func (f nativeFixtureParticipants) BulkUpdateStatus(ctx context.Context, instanceID int64, fromStatus, toStatus string, excludedStudentIDs []int64) (int, error) {
	return bulkUpdateParticipantStatus(ctx, f.rows, instanceID, fromStatus, toStatus, excludedStudentIDs)
}

func (f nativeFixtureParticipants) MarkNotScheduled(ctx context.Context, refs []scheduleModels.StudentInstanceRef) error {
	return markParticipantsNotScheduled(ctx, f.rows, refs)
}

func (f nativeFixtureParticipants) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []scheduleModels.StudentInstanceRef) error {
	return markExpectedAbsentBySessions(ctx, f.rows.Presence, activeGroupIDs, updatedAt, exclusions)
}

func (f nativeFixtureParticipants) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, checkedOutAt time.Time) (int, error) {
	return NewParticipantCheckouts(f.rows).CloseOpenCheckoutsByActiveGroupIDs(ctx, activeGroupIDs, checkedOutAt)
}

func (f nativeFixtureParticipants) ListStudentInstanceRefsBefore(ctx context.Context, cutoff calendar.Date) ([]scheduleModels.StudentInstanceRef, error) {
	return studentInstanceRefsBefore(ctx, f.rows.Plans, cutoff)
}

func (f nativeFixtureParticipants) FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.ScheduledInstanceRow, error) {
	return scheduledInstancesForStudent(ctx, f.rows.Reads, studentID, from, to)
}

func (f nativeFixtureParticipants) HasPlannedSlotsInRange(ctx context.Context, from, to calendar.Date) (bool, error) {
	return NewAttendanceHistory(f.rows.Reads).HasPlannedSlots(ctx, from.String(), to.String())
}

func (f nativeFixtureParticipants) FindPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error) {
	return plannedStudentIDsOn(ctx, f.rows.Plans, studentIDs, date)
}

func (f nativeFixtureParticipants) FindPartialAbsenceBlocks(ctx context.Context, studentID int64, date calendar.Date, cutoff time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	return partialAbsenceBlocks(ctx, f.preview, studentID, date, cutoff)
}

func (f nativeFixtureParticipants) ListStudents(ctx context.Context, filter ports.ParticipantListingFilter) ([]*scheduleModels.InstanceStudent, error) {
	rows, err := listLegacyParticipants(ctx, f.rows.Reads, timetable.InstanceStudentFilter{IDs: filter.IDs, InstanceIDs: filter.InstanceIDs,
		StudentIDs: filter.StudentIDs, Limit: filter.Limit, Offset: filter.Offset}, "list with options")
	if err != nil || filter.Status == nil {
		return rows, err
	}
	return filterLegacyInstanceStudents(rows, func(row *scheduleModels.InstanceStudent) bool { return row.Status == *filter.Status }), nil
}

func applyPartialAbsence(ctx context.Context, presence ports.ParticipantPresence, pickupExceptionID int64) (int, error) {
	changed, err := presence.ApplyPartialAbsence(ctx, pickupExceptionID)
	if err != nil {
		return 0, WrapDatabaseError("apply partial absence to slots", err)
	}
	return changed, nil
}

func releasePartialAbsence(ctx context.Context, presence ports.ParticipantPresence, pickupExceptionID int64) (int, error) {
	changed, err := presence.ReleasePartialAbsence(ctx, pickupExceptionID)
	if err != nil {
		return 0, WrapDatabaseError("release partial absence from slots", err)
	}
	return changed, nil
}

func studentInstanceRefsBefore(ctx context.Context, plans timetable.InstanceStudentCapability, cutoff calendar.Date) ([]scheduleModels.StudentInstanceRef, error) {
	values, err := plans.ListStudentInstanceRefsBefore(ctx, cutoff.String())
	if err != nil {
		return nil, WrapDatabaseError("list student instance refs before", err)
	}
	result := make([]scheduleModels.StudentInstanceRef, 0, len(values))
	for _, value := range values {
		result = append(result, scheduleModels.StudentInstanceRef(value))
	}
	return result, nil
}

func partialAbsenceBlocks(ctx context.Context, preview PartialAbsences, studentID int64, date calendar.Date, cutoff time.Time) ([]scheduleModels.PartialAbsenceBlock, error) {
	result, err := preview.FindPartialAbsenceBlocks(ctx, studentID, date.String(), cutoff)
	if err != nil {
		return nil, WrapDatabaseError("find partial absence blocks", err)
	}
	return result, nil
}

// updateLegacyParticipant writes the participant and, when the row carries
// a different attendance than the stored one, restores it.
func updateLegacyParticipant(ctx context.Context, rows ports.ParticipantRows, value *scheduleModels.InstanceStudent) error {
	if value == nil {
		return errors.New("instance student cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := rows.Plans.UpdateInstanceStudent(ctx, value.ID, publicInstanceStudentInput(value))
	if err != nil {
		return legacyInstanceStudentError("update", err)
	}
	current, err := attendanceByParticipant(ctx, rows.Presence, []int64{updated.ID})
	if err != nil {
		return WrapDatabaseError("update", err)
	}
	attendance, ok := current[updated.ID]
	if !ok {
		attendance = studentpresence.ExpectedSessionAttendance(updated.ID)
	}
	if wanted := legacyAttendanceRestore(updated.ID, value); !sameLegacyAttendanceRestore(wanted, legacyAttendanceRestoreOf(attendance)) {
		if err := rows.Presence.RestoreSessionAttendance(ctx, []studentpresence.SessionAttendanceRestore{wanted}); err != nil {
			return WrapDatabaseError("update", err)
		}
		current, err = attendanceByParticipant(ctx, rows.Presence, []int64{updated.ID})
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

func legacyInstanceStudentError(operation string, err error) error {
	if errors.Is(err, timetable.ErrInstanceStudentNotFound) {
		return WrapNotFoundDatabaseError(operation)
	}
	return WrapDatabaseError(operation, err)
}
