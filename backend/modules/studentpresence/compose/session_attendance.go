package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func sessionAttendanceToPublic(row ports.SessionAttendance) studentpresence.SessionAttendance {
	return studentpresence.SessionAttendance{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		ParticipantID: row.ParticipantID, Status: row.Status, Substatus: row.Substatus, Note: row.Note,
		CheckedInAt: row.CheckedInAt, CheckedOutAt: row.CheckedOutAt, IsUnplanned: row.IsUnplanned,
		NotScheduled: row.NotScheduled, ManualStatusAt: row.ManualStatusAt,
		StudentStatusDayID: row.StudentStatusDayID, PickupExceptionID: row.PickupExceptionID,
	}
}

func sessionAttendanceRowsToPublic(rows []ports.SessionAttendance, err error) ([]studentpresence.SessionAttendance, error) {
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.SessionAttendance, 0, len(rows))
	for _, row := range rows {
		result = append(result, sessionAttendanceToPublic(row))
	}
	return result, nil
}

func sessionAttendanceError(err error) error {
	switch {
	case errors.Is(err, ports.ErrSessionAttendanceNotFound):
		return studentpresence.ErrSessionAttendanceNotFound
	case errors.Is(err, application.ErrPlannedRosterUnbound):
		return studentpresence.ErrSessionAttendanceRoster
	case errors.Is(err, application.ErrCarePlanDirectoryUnbound):
		return studentpresence.ErrSessionAttendanceCarePlan
	}
	return err
}

func (e engine) ListSessionAttendance(ctx context.Context, participantIDs []int64) ([]studentpresence.SessionAttendance, error) {
	return sessionAttendanceRowsToPublic(e.Service.ListSessionAttendance(ctx, participantIDs))
}

func (e engine) CheckInParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return e.Service.CheckInParticipants(ctx, participantIDs, at)
}

func (e engine) CheckInWalkIn(ctx context.Context, participantID int64, at time.Time) (studentpresence.SessionAttendance, error) {
	row, err := e.Service.CheckInWalkIn(ctx, participantID, at)
	if err != nil {
		return studentpresence.SessionAttendance{}, sessionAttendanceError(err)
	}
	return sessionAttendanceToPublic(row), nil
}

func (e engine) CheckOutParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return e.Service.CheckOutParticipants(ctx, participantIDs, at)
}

func (e engine) CloseOpenParticipants(ctx context.Context, participantIDs []int64, at time.Time) (int64, error) {
	return e.Service.CloseOpenParticipants(ctx, participantIDs, at)
}

func (e engine) ReconcileParticipantInterval(ctx context.Context, participantID int64, previousCheckIn time.Time, previousCheckOut *time.Time, checkIn time.Time, checkOut *time.Time) (bool, error) {
	return e.Service.ReconcileParticipantInterval(ctx, participantID, previousCheckIn, previousCheckOut, checkIn, checkOut)
}

func (e engine) PatchSessionAttendance(ctx context.Context, participantID int64, patch studentpresence.SessionAttendancePatch) error {
	return e.Service.PatchSessionAttendance(ctx, participantID, ports.SessionAttendancePatch(patch))
}

func (e engine) TransitionParticipants(ctx context.Context, participantIDs []int64, from, to string, at time.Time) (int64, error) {
	return e.Service.TransitionParticipants(ctx, participantIDs, from, to, at)
}

func (e engine) MarkParticipantsNotScheduled(ctx context.Context, participantIDs []int64) (int64, error) {
	return e.Service.MarkParticipantsNotScheduled(ctx, participantIDs)
}

func (e engine) RestoreSessionAttendance(ctx context.Context, rows []studentpresence.SessionAttendanceRestore) error {
	values := make([]ports.SessionAttendanceRestore, 0, len(rows))
	for _, row := range rows {
		values = append(values, ports.SessionAttendanceRestore(row))
	}
	return e.Service.RestoreSessionAttendance(ctx, values)
}

func (e engine) ReconnectParticipantPickupExceptions(ctx context.Context, links []studentpresence.ParticipantPickupException) error {
	values := make([]ports.ParticipantPickupException, 0, len(links))
	for _, link := range links {
		values = append(values, ports.ParticipantPickupException(link))
	}
	return e.Service.ReconnectParticipantPickupExceptions(ctx, values)
}

func (e engine) LockSessionAttendance(ctx context.Context, participantIDs []int64) error {
	return e.Service.LockSessionAttendance(ctx, participantIDs)
}

func (e engine) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (int, error) {
	rows, err := e.Service.ApplyStatusDay(ctx, studentID, date, statusDayID, substatus)
	return rows, sessionAttendanceError(err)
}

func (e engine) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	rows, err := e.Service.ReleaseStatusDay(ctx, statusDayID)
	return rows, sessionAttendanceError(err)
}

func (e engine) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date string) (int, error) {
	rows, err := e.Service.ApplyActiveStatusDaysForInstance(ctx, instanceID, date)
	return rows, sessionAttendanceError(err)
}

func (e engine) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	rows, err := e.Service.ApplyPartialAbsence(ctx, pickupExceptionID)
	return rows, sessionAttendanceError(err)
}

func (e engine) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (int, error) {
	rows, err := e.Service.ReleasePartialAbsence(ctx, pickupExceptionID)
	return rows, sessionAttendanceError(err)
}

func (e engine) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string) (int, error) {
	rows, err := e.Service.ApplyActivePartialAbsencesForInstance(ctx, instanceID, date)
	return rows, sessionAttendanceError(err)
}

func (e engine) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, at time.Time, exclusions []studentpresence.PlannedParticipantRef) error {
	values := make([]ports.PlannedParticipant, 0, len(exclusions))
	for _, exclusion := range exclusions {
		values = append(values, ports.PlannedParticipant{StudentID: exclusion.StudentID, InstanceID: exclusion.InstanceID})
	}
	return sessionAttendanceError(e.Service.MarkExpectedAbsentByActiveGroupIDs(ctx, activeGroupIDs, at, values))
}

func (e engine) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, at time.Time) (int, error) {
	rows, err := e.Service.CloseOpenCheckoutsByActiveGroupIDs(ctx, activeGroupIDs, at)
	return rows, sessionAttendanceError(err)
}

// plannedRoster adapts the public roster port to the application port.
type plannedRoster struct{ query studentpresence.PlannedRoster }

func (r plannedRoster) ListPlannedParticipants(ctx context.Context, filter ports.PlannedRosterFilter) ([]ports.PlannedParticipant, error) {
	values, err := r.query.ListPlannedParticipants(ctx, studentpresence.PlannedRosterFilter(filter))
	if err != nil {
		return nil, err
	}
	result := make([]ports.PlannedParticipant, 0, len(values))
	for _, value := range values {
		result = append(result, ports.PlannedParticipant(value))
	}
	return result, nil
}

type carePlanDirectory struct {
	query studentpresence.SessionCarePlanDirectory
}

func (d carePlanDirectory) FindPickupException(ctx context.Context, id int64) (*ports.PickupException, error) {
	value, err := d.query.FindPickupException(ctx, id)
	if value == nil || err != nil {
		return nil, err
	}
	result := ports.PickupException(*value)
	return &result, nil
}

func (d carePlanDirectory) ListPickupExceptions(ctx context.Context, filter ports.PickupExceptionFilter) ([]ports.PickupException, error) {
	values, err := d.query.ListPickupExceptions(ctx, studentpresence.SessionPickupExceptionFilter(filter))
	result := make([]ports.PickupException, 0, len(values))
	for _, value := range values {
		result = append(result, ports.PickupException(value))
	}
	return result, err
}

func (d carePlanDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*ports.StudentStatusDay, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if value == nil || err != nil {
		return nil, err
	}
	result := ports.StudentStatusDay(*value)
	return &result, nil
}

func (d carePlanDirectory) ListStudentStatusDays(ctx context.Context, filter ports.StudentStatusDayFilter) ([]ports.StudentStatusDay, error) {
	values, err := d.query.ListStudentStatusDays(ctx, studentpresence.SessionStudentStatusDayFilter(filter))
	result := make([]ports.StudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, ports.StudentStatusDay(value))
	}
	return result, err
}
