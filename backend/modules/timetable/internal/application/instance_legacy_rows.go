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

// The instance lifecycle, the materializer, the template writes, the
// operational day and the conflict, deviation and coverage reads still
// compute on the legacy activity instance row, which joins the owner's plan
// and the Student Presence session (#2762). They read it through the joined
// presence read, write the plan through the owner's instance capability and
// start, complete and discard the session through Student Presence. These
// functions keep the routing of the lifecycle columns, the reload after a
// write and the operation names the retained repository reported.

// findLegacyInstance reads one instance with its session; an ID <= 0 is
// not found.
func findLegacyInstance(ctx context.Context, reads ports.LegacyInstanceReads, id int64) (*scheduleModels.ActivityInstance, error) {
	if id <= 0 {
		return nil, WrapNotFoundDatabaseError("find by id")
	}
	rows, err := reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{IDs: []int64{id}})
	if err != nil {
		return nil, WrapDatabaseError("find by id", err)
	}
	if len(rows) == 0 {
		return nil, WrapNotFoundDatabaseError("find by id")
	}
	return rows[0], nil
}

func listLegacyInstances(ctx context.Context, reads ports.LegacyInstanceReads, filter ports.LegacyInstanceFilter, operation string) ([]*scheduleModels.ActivityInstance, error) {
	rows, err := reads.ListLegacyInstances(ctx, filter)
	if err != nil {
		return nil, WrapDatabaseError(operation, err)
	}
	return rows, nil
}

// instancesOn lists the tenant's instances of a date by time.
func instancesOn(ctx context.Context, reads ports.LegacyInstanceReads, date calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	text := date.String()
	return listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{Date: &text, OrderByDateAndTime: true}, "find by tenant and date")
}

// instancesBetween lists the tenant's instances of an inclusive date range.
func instancesBetween(ctx context.Context, reads ports.LegacyInstanceReads, from, to calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	fromText, toText := from.String(), to.String()
	return listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{
		FromDate: &fromText, ToDate: &toText, OrderByDateAndTime: true,
	}, "find by tenant and date range")
}

// plannedTemplateBackedInstancesFrom lists the materialized planned
// instances from a date on.
func plannedTemplateBackedInstancesFrom(ctx context.Context, reads ports.LegacyInstanceReads, from calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	text := from.String()
	return listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{
		FromDate: &text, MaterializedPlanned: true, OrderByDateAndTime: true,
	}, "find planned template-backed instances from date")
}

// instancesByID lists the instances with the IDs; no IDs list nothing.
func instancesByID(ctx context.Context, reads ports.LegacyInstanceReads, ids []int64) ([]*scheduleModels.ActivityInstance, error) {
	if len(ids) == 0 {
		return []*scheduleModels.ActivityInstance{}, nil
	}
	return listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{IDs: ids, OrderByDateAndTime: true}, "find by ids")
}

// groupInstancesOn lists an activity group's instances of a date.
func groupInstancesOn(ctx context.Context, reads ports.LegacyInstanceReads, groupID int64, date calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	text := date.String()
	return listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{
		ActivityGroupID: &groupID, Date: &text, OrderByDateAndTime: true,
	}, "find by activity group and date")
}

// groupInstancesBetween lists an activity group's instances of an inclusive
// date range.
func groupInstancesBetween(ctx context.Context, reads ports.LegacyInstanceReads, groupID int64, from, to calendar.Date) ([]*scheduleModels.ActivityInstance, error) {
	fromText, toText := from.String(), to.String()
	return listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{
		ActivityGroupID: &groupID, FromDate: &fromText, ToDate: &toText, OrderByDateAndTime: true,
	}, "find by activity group and date range")
}

// sessionInstance reads the instance a live session runs; nil when none.
func sessionInstance(ctx context.Context, reads ports.LegacyInstanceReads, activeGroupID int64) (*scheduleModels.ActivityInstance, error) {
	rows, err := listLegacyInstances(ctx, reads, ports.LegacyInstanceFilter{ActiveGroupIDs: []int64{activeGroupID}, Limit: 1}, "find by active group id")
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

// createLegacyInstance creates the plan, opens and if asked ends the session
// of an instance created in an execution state, and reloads the stored row.
func createLegacyInstance(ctx context.Context, rows ports.InstanceRows, value *scheduleModels.ActivityInstance) error {
	if value == nil {
		return errors.New("activity instance cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := rows.Plans.CreateActivityInstance(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return WrapDatabaseError("create", err)
	}
	if err := createInstanceSession(ctx, rows.Sessions, created.ID, value); err != nil {
		return WrapDatabaseError("create", err)
	}
	return reloadLegacyInstance(ctx, rows.Reads, value, created.ID, "create")
}

// createInstanceSession opens, and if asked ends, the session of a row
// created in an execution state: the retained services and fixtures still
// create running blocks in one call.
func createInstanceSession(ctx context.Context, sessions ports.InstanceSessions, instanceID int64, value *scheduleModels.ActivityInstance) error {
	if value.Status != scheduleModels.InstanceStatusActive && value.Status != scheduleModels.InstanceStatusCompleted {
		return nil
	}
	if value.ActiveGroupID == nil {
		return errors.New("an instance created in an execution state needs its active group")
	}
	startedAt := value.StartedAt
	if startedAt == nil {
		now := time.Now()
		startedAt = &now
	}
	if _, err := sessions.StartActivitySession(ctx, studentpresence.ActivitySessionStart{
		InstanceID: instanceID, ActiveGroupID: *value.ActiveGroupID, StartedBy: value.StartedBy, StartedAt: *startedAt,
	}); err != nil {
		return err
	}
	if value.Status != scheduleModels.InstanceStatusCompleted {
		return nil
	}
	completedAt := value.CompletedAt
	if completedAt == nil {
		now := time.Now()
		completedAt = &now
	}
	_, err := sessions.CompleteActivitySession(ctx, studentpresence.ActivitySessionCompletion{
		InstanceID: instanceID, CompletedAt: *completedAt, CompletedBy: value.CompletedBy,
		ReopenUntil: value.ReopenUntil, CompletionSnapshot: []byte(value.CompletionSnapshot),
	})
	return err
}

// reloadLegacyInstance replaces the row with the stored plan and execution.
func reloadLegacyInstance(ctx context.Context, reads ports.LegacyInstanceReads, value *scheduleModels.ActivityInstance, id int64, operation string) error {
	rows, err := reads.ListLegacyInstances(ctx, ports.LegacyInstanceFilter{IDs: []int64{id}})
	if err != nil {
		return WrapDatabaseError(operation, err)
	}
	if len(rows) == 0 {
		return WrapNotFoundDatabaseError(operation)
	}
	*value = *rows[0]
	return nil
}

// createTemplateBackedInstanceIfAbsent inserts a template-backed occurrence
// unless its key exists and reloads the stored row after an insert.
func createTemplateBackedInstanceIfAbsent(ctx context.Context, rows ports.InstanceRows, value *scheduleModels.ActivityInstance) (bool, error) {
	const operation = "create template-backed if absent"
	if value == nil {
		return false, errors.New("activity instance cannot be nil")
	}
	if value.ActivityGroupID == nil {
		return false, errors.New("activity_group_id is required for template-backed insert")
	}
	if err := value.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := rows.Plans.CreateTemplateBackedActivityInstanceIfAbsent(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return false, WrapDatabaseError(operation, err)
	}
	if inserted {
		return true, reloadLegacyInstance(ctx, rows.Reads, value, created.ID, operation)
	}
	return false, nil
}

func deleteLegacyInstance(ctx context.Context, plans timetable.ActivityInstanceCommand, id int64) error {
	if err := plans.DeleteActivityInstance(ctx, id); err != nil {
		return WrapDatabaseError("delete", err)
	}
	return nil
}

// completeSessionInstances completes the running blocks of the sessions.
func completeSessionInstances(ctx context.Context, sessions ports.InstanceSessions, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	rows, err := sessions.CompleteActivitySessionsByGroups(ctx, activeGroupIDs, completedAt)
	if err != nil {
		return 0, WrapDatabaseError("complete active instances by active group ids", err)
	}
	return rows, nil
}

// deletePlannedInstancesInWindow removes the template-backed occurrences of
// the window that have not started.
func deletePlannedInstancesInWindow(ctx context.Context, plans timetable.ActivityInstanceCommand, from calendar.Date, to *calendar.Date, groupID *int64, preserveDeviations bool) (int64, error) {
	rows, err := plans.DeletePlannedActivityInstances(ctx, from.String(), publicOptionalInstanceDate(to), groupID, preserveDeviations)
	if err != nil {
		return 0, WrapDatabaseError("delete planned non-spontaneous in window", err)
	}
	return rows, nil
}

func deleteRemovedWeekendInstances(ctx context.Context, plans timetable.ActivityInstanceCommand, groupID int64, weekdays []int) (int64, error) {
	rows, err := plans.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
	if err != nil {
		return 0, WrapDatabaseError("delete removed legacy weekend instances", err)
	}
	return rows, nil
}

func propagateInstanceListKind(ctx context.Context, plans timetable.ActivityInstanceCommand, groupID int64, previousKind, newKind *string, after calendar.Date) (int64, error) {
	rows, err := plans.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after.String())
	if err != nil {
		return 0, WrapDatabaseError("propagate list kind to future instances", err)
	}
	return rows, nil
}

// activityExecutionColumns are the legacy columns Student Presence owns since
// the cutover. A column set that names one of them is a lifecycle write.
var activityExecutionColumns = []string{"active_group_id", "started_at", "started_by", "completed_at", "completed_by", "reopen_until", "completion_snapshot"}

// updateLegacyInstanceColumns routes the retained lifecycle column sets to
// the owner that holds them: starting and completing a block are Student
// Presence session commands, cancelling discards the session and then
// patches the plan, and every other column set is a plan patch.
func updateLegacyInstanceColumns(ctx context.Context, rows ports.InstanceRows, value *scheduleModels.ActivityInstance, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("ActivityInstance cannot be nil or zero value")
	}
	if len(columns) == 0 {
		return 0, errors.New("update columns ActivityInstance: at least one column required")
	}
	if !slices.Contains(columns, "status") {
		if slices.ContainsFunc(columns, func(column string) bool { return slices.Contains(activityExecutionColumns, column) }) {
			return 0, WrapDatabaseError("update columns", errors.New("execution columns are written through a status transition"))
		}
		return patchLegacyInstance(ctx, rows.Plans, value, columns)
	}
	switch value.Status {
	case scheduleModels.InstanceStatusActive:
		return startInstanceSession(ctx, rows.Sessions, value)
	case scheduleModels.InstanceStatusCompleted:
		return completeInstanceSession(ctx, rows.Sessions, value)
	case scheduleModels.InstanceStatusCancelled:
		if err := rows.Sessions.DiscardActivitySession(ctx, value.ID); err != nil {
			return 0, WrapDatabaseError("update columns", err)
		}
		return patchLegacyInstance(ctx, rows.Plans, value, planningColumns(columns))
	default:
		return patchLegacyInstance(ctx, rows.Plans, value, planningColumns(columns))
	}
}

func patchLegacyInstance(ctx context.Context, plans timetable.ActivityInstanceCommand, value *scheduleModels.ActivityInstance, columns []string) (int64, error) {
	if len(columns) == 0 {
		return 1, nil
	}
	rows, err := plans.PatchActivityInstance(ctx, value.ID, publicActivityInstanceInput(value), columns)
	if err != nil {
		return 0, WrapDatabaseError("update columns", err)
	}
	return rows, nil
}

func startInstanceSession(ctx context.Context, sessions ports.InstanceSessions, value *scheduleModels.ActivityInstance) (int64, error) {
	if value.ActiveGroupID == nil || value.StartedAt == nil {
		return 0, WrapDatabaseError("update columns", errors.New("starting an instance requires active_group_id and started_at"))
	}
	if _, err := sessions.StartActivitySession(ctx, studentpresence.ActivitySessionStart{
		InstanceID: value.ID, ActiveGroupID: *value.ActiveGroupID, StartedBy: value.StartedBy, StartedAt: *value.StartedAt,
	}); err != nil {
		return 0, WrapDatabaseError("update columns", err)
	}
	return 1, nil
}

func completeInstanceSession(ctx context.Context, sessions ports.InstanceSessions, value *scheduleModels.ActivityInstance) (int64, error) {
	if value.CompletedAt == nil {
		return 0, WrapDatabaseError("update columns", errors.New("completing an instance requires completed_at"))
	}
	if _, err := sessions.CompleteActivitySession(ctx, studentpresence.ActivitySessionCompletion{
		InstanceID: value.ID, CompletedAt: *value.CompletedAt, CompletedBy: value.CompletedBy,
		ReopenUntil: value.ReopenUntil, CompletionSnapshot: []byte(value.CompletionSnapshot),
	}); err != nil {
		if errors.Is(err, studentpresence.ErrActivitySessionNotFound) {
			return 0, nil
		}
		return 0, WrapDatabaseError("update columns", err)
	}
	return 1, nil
}

// planningColumns keeps the columns Timetable still owns.
func planningColumns(columns []string) []string {
	result := make([]string, 0, len(columns))
	for _, column := range columns {
		if !slices.Contains(activityExecutionColumns, column) {
			result = append(result, column)
		}
	}
	return result
}

// publicActivityInstanceInput carries the plan of the legacy DTO to the
// Timetable owner. An execution status is a plan that exists; the session
// itself travels through the Student Presence commands.
func publicActivityInstanceInput(value *scheduleModels.ActivityInstance) timetable.ActivityInstanceInput {
	status := value.Status
	if status == scheduleModels.InstanceStatusActive || status == scheduleModels.InstanceStatusCompleted {
		status = timetable.InstanceStatusPlanned
	}
	return timetable.ActivityInstanceInput{
		Date: value.Date.String(), ActivityGroupID: value.ActivityGroupID, CalendarPeriodID: value.CalendarPeriodID,
		Title: value.Title, Description: value.Description, StartTime: PublicClock(value.StartTime),
		EndTime: PublicClock(value.EndTime), RoomID: value.RoomID, RequiredStaff: value.RequiredStaff,
		Status: status, ListKind: value.ListKind,
		IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck,
		UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes,
		IdempotencyKey: value.IdempotencyKey, IdempotencyFingerprint: value.IdempotencyFingerprint,
		CreatedBy: value.CreatedBy,
	}
}

func publicOptionalInstanceDate(value *calendar.Date) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

// noInstanceRows reports instance rows nobody wired.
func noInstanceRows(rows ports.InstanceRows) bool {
	return rows.Plans == nil && rows.Sessions == nil && rows.Reads == nil
}

// SessionInstanceCompletion completes the running blocks of the sessions a
// cleanup ends.
type SessionInstanceCompletion struct {
	sessions ports.InstanceSessions
}

// NewSessionInstanceCompletion binds the completion to Student Presence.
func NewSessionInstanceCompletion(sessions ports.InstanceSessions) SessionInstanceCompletion {
	return SessionInstanceCompletion{sessions: sessions}
}

// CompleteActiveByActiveGroupIDs completes the running blocks of the
// sessions at the instant.
func (c SessionInstanceCompletion) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	rows, err := c.sessions.CompleteActivitySessionsByGroups(ctx, activeGroupIDs, completedAt)
	if err != nil {
		return 0, WrapDatabaseError("complete active instances by active group ids", err)
	}
	return rows, nil
}
