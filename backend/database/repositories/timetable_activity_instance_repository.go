package repositories

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

const legacyActivityInstanceDateColumn = "date"

// activityInstanceSessions is the Student Presence surface the legacy
// instance adapter composes: the execution of a block lives there since the
// presence cutover (#2762).
type activityInstanceSessions interface {
	studentpresence.ActivitySessionQuery
	studentpresence.ActivitySessionCommand
}

// timetableActivityInstanceRepository serves the legacy activity instance
// DTO from two owners: Timetable & Activities plans the block, Student
// Presence runs it. Reads come from the tenant-safe projection that joins
// the session onto the plan in one statement; the lifecycle column sets the
// retained service writes are routed to the owner command that owns them.
type timetableActivityInstanceRepository struct {
	timetable timetable.ActivityInstanceCapability
	presence  activityInstanceSessions
	reads     timetableCompose.PresenceReads
}

func newTimetableActivityInstanceRepository(db *bun.DB, capability timetable.ActivityInstanceCapability, presence activityInstanceSessions) timetableActivityInstanceRepository {
	if db == nil || capability == nil || presence == nil {
		panic("timetable activity instance repository: database, timetable and student presence are required")
	}
	return timetableActivityInstanceRepository{timetable: capability, presence: presence, reads: mustPresenceReads(db)}
}

func (r timetableActivityInstanceRepository) Create(ctx context.Context, value *scheduleModels.ActivityInstance) error {
	if value == nil {
		return errors.New("activity instance cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	created, err := r.timetable.CreateActivityInstance(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return timetableCompose.WrapDatabaseError("create", err)
	}
	if err := r.createSession(ctx, created.ID, value); err != nil {
		return timetableCompose.WrapDatabaseError("create", err)
	}
	return r.reload(ctx, value, created.ID, "create")
}

// createSession opens, and if asked ends, the session of a DTO created in
// an execution state: the retained services and fixtures still create
// running blocks in one call.
func (r timetableActivityInstanceRepository) createSession(ctx context.Context, instanceID int64, value *scheduleModels.ActivityInstance) error {
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
	if _, err := r.presence.StartActivitySession(ctx, studentpresence.ActivitySessionStart{
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
	_, err := r.presence.CompleteActivitySession(ctx, studentpresence.ActivitySessionCompletion{
		InstanceID: instanceID, CompletedAt: *completedAt, CompletedBy: value.CompletedBy,
		ReopenUntil: value.ReopenUntil, CompletionSnapshot: []byte(value.CompletionSnapshot),
	})
	return err
}

// reload replaces the DTO with the stored plan and execution.
func (r timetableActivityInstanceRepository) reload(ctx context.Context, value *scheduleModels.ActivityInstance, id int64, operation string) error {
	rows, err := r.reads.ListLegacyInstances(ctx, timetableCompose.LegacyInstanceFilter{IDs: []int64{id}})
	if err != nil {
		return timetableCompose.WrapDatabaseError(operation, err)
	}
	if len(rows) == 0 {
		return timetableCompose.WrapNotFoundDatabaseError(operation)
	}
	*value = *rows[0]
	return nil
}

func (r timetableActivityInstanceRepository) CreateTemplateBackedIfAbsent(ctx context.Context, value *scheduleModels.ActivityInstance) (bool, error) {
	if value == nil {
		return false, errors.New("activity instance cannot be nil")
	}
	if value.ActivityGroupID == nil {
		return false, errors.New("activity_group_id is required for template-backed insert")
	}
	if err := value.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.timetable.CreateTemplateBackedActivityInstanceIfAbsent(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return false, timetableCompose.WrapDatabaseError("create template-backed if absent", err)
	}
	if inserted {
		return true, r.reload(ctx, value, created.ID, "create template-backed if absent")
	}
	return false, nil
}

func (r timetableActivityInstanceRepository) CreateIdempotent(ctx context.Context, value *scheduleModels.ActivityInstance) (bool, error) {
	if value == nil {
		return false, errors.New("activity instance cannot be nil")
	}
	if value.IdempotencyKey == nil {
		return false, errors.New("idempotency_key is required for idempotent insert")
	}
	if err := value.Validate(); err != nil {
		return false, err
	}
	created, inserted, err := r.timetable.CreateIdempotentActivityInstance(ctx, publicActivityInstanceInput(value))
	if err != nil {
		return false, timetableCompose.WrapDatabaseError("create idempotent", err)
	}
	if inserted {
		return true, r.reload(ctx, value, created.ID, "create idempotent")
	}
	return false, nil
}

func (r timetableActivityInstanceRepository) FindByID(ctx context.Context, id any) (*scheduleModels.ActivityInstance, error) {
	instanceID, ok := legacyGroupID(id)
	if !ok {
		return nil, timetableCompose.WrapDatabaseError("find by id", fmt.Errorf("invalid activity instance id %T", id))
	}
	if instanceID <= 0 {
		return nil, timetableCompose.WrapNotFoundDatabaseError("find by id")
	}
	rows, err := r.reads.ListLegacyInstances(ctx, timetableCompose.LegacyInstanceFilter{IDs: []int64{instanceID}})
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("find by id", err)
	}
	if len(rows) == 0 {
		return nil, timetableCompose.WrapNotFoundDatabaseError("find by id")
	}
	return rows[0], nil
}

func (r timetableActivityInstanceRepository) Update(ctx context.Context, value *scheduleModels.ActivityInstance) error {
	if value == nil {
		return errors.New("activity instance cannot be nil or zero value")
	}
	if err := value.Validate(); err != nil {
		return err
	}
	updated, err := r.timetable.UpdateActivityInstance(ctx, value.ID, publicActivityInstanceInput(value))
	if err != nil {
		return legacyActivityInstanceError("update", err)
	}
	return r.reload(ctx, value, updated.ID, "update")
}

func (r timetableActivityInstanceRepository) Delete(ctx context.Context, id any) error {
	instanceID, ok := legacyGroupID(id)
	if !ok {
		return timetableCompose.WrapDatabaseError("delete", fmt.Errorf("invalid activity instance id %T", id))
	}
	if err := r.timetable.DeleteActivityInstance(ctx, instanceID); err != nil {
		return timetableCompose.WrapDatabaseError("delete", err)
	}
	return nil
}

func (r timetableActivityInstanceRepository) List(ctx context.Context, options *timetableCompose.ActivityInstanceQueryOptions) ([]*scheduleModels.ActivityInstance, error) {
	filter, err := timetableCompose.ActivityInstanceListOptions(options)
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("list with options", err)
	}
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{
		IDs: filter.IDs, Date: filter.Date, Dates: filter.Dates, ActivityGroupIDs: filter.ActivityGroupIDs,
		ActiveGroupIDs: filter.ActiveGroupIDs, Status: filter.Status, IsSpontaneous: filter.IsSpontaneous,
		IdempotencyKey: filter.IdempotencyKey, Limit: filter.Limit, Offset: filter.Offset,
	}, "list with options")
}

func (r timetableActivityInstanceRepository) FindByTenantAndDate(ctx context.Context, date timetableCompose.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	text := date.String()
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{Date: &text, OrderByDateAndTime: true}, "find by tenant and date")
}

func (r timetableActivityInstanceRepository) FindPlannedTemplateBackedFrom(ctx context.Context, from timetableCompose.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	text := from.String()
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{
		FromDate: &text, MaterializedPlanned: true, OrderByDateAndTime: true,
	}, "find planned template-backed instances from date")
}

func (r timetableActivityInstanceRepository) MaxID(ctx context.Context) (int64, error) {
	value, err := r.timetable.MaxActivityInstanceID(ctx)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("get max activity instance id", err)
	}
	return value, nil
}

func (r timetableActivityInstanceRepository) FindByTenantAndDateRange(ctx context.Context, from, to timetableCompose.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{
		FromDate: &fromText, ToDate: &toText, OrderByDateAndTime: true,
	}, "find by tenant and date range")
}

func (r timetableActivityInstanceRepository) FindByIDs(ctx context.Context, ids []int64) ([]*scheduleModels.ActivityInstance, error) {
	if len(ids) == 0 {
		return []*scheduleModels.ActivityInstance{}, nil
	}
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{IDs: ids, OrderByDateAndTime: true}, "find by ids")
}

func (r timetableActivityInstanceRepository) FindByActivityGroupAndDate(ctx context.Context, groupID int64, date timetableCompose.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	text := date.String()
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{
		ActivityGroupID: &groupID, Date: &text, OrderByDateAndTime: true,
	}, "find by activity group and date")
}

func (r timetableActivityInstanceRepository) FindByActivityGroupAndDateRange(ctx context.Context, groupID int64, from, to timetableCompose.ActivityInstanceDate) ([]*scheduleModels.ActivityInstance, error) {
	fromText, toText := from.String(), to.String()
	return r.list(ctx, timetableCompose.LegacyInstanceFilter{
		ActivityGroupID: &groupID, FromDate: &fromText, ToDate: &toText, OrderByDateAndTime: true,
	}, "find by activity group and date range")
}

func (r timetableActivityInstanceRepository) FindByActiveGroupID(ctx context.Context, groupID int64) (*scheduleModels.ActivityInstance, error) {
	rows, err := r.list(ctx, timetableCompose.LegacyInstanceFilter{ActiveGroupIDs: []int64{groupID}, Limit: 1}, "find by active group id")
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return rows[0], nil
}

func (r timetableActivityInstanceRepository) MarkCompleted(ctx context.Context, instanceID int64, completedAt time.Time) error {
	if err := r.presence.RecordActivitySessionCompleted(ctx, instanceID, completedAt); err != nil {
		if errors.Is(err, studentpresence.ErrActivitySessionNotFound) {
			return timetableCompose.WrapNotFoundDatabaseError("mark completed")
		}
		return timetableCompose.WrapDatabaseError("mark completed", err)
	}
	return nil
}

func (r timetableActivityInstanceRepository) CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error) {
	rows, err := r.presence.CompleteActivitySessionsByGroups(ctx, activeGroupIDs, completedAt)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("complete active instances by active group ids", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) DeletePlannedNonSpontaneousInWindow(ctx context.Context, from timetableCompose.ActivityInstanceDate, to *timetableCompose.ActivityInstanceDate, groupID *int64, preserveDeviations bool) (int64, error) {
	rows, err := r.timetable.DeletePlannedActivityInstances(ctx, from.String(), publicOptionalInstanceDate(to), groupID, preserveDeviations)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("delete planned non-spontaneous in window", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) DeletePlannedMaterializedWeekendInstances(ctx context.Context, groupID int64, weekdays []int) (int64, error) {
	rows, err := r.timetable.DeleteRemovedWeekendActivityInstances(ctx, groupID, weekdays)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("delete removed legacy weekend instances", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) PropagateListKindToFutureInstances(ctx context.Context, groupID int64, previousKind, newKind *string, after timetableCompose.ActivityInstanceDate) (int64, error) {
	rows, err := r.timetable.PropagateActivityInstanceListKind(ctx, groupID, previousKind, newKind, after.String())
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("propagate list kind to future instances", err)
	}
	return rows, nil
}

// activityExecutionColumns are the legacy columns Student Presence owns since
// the cutover. A column set that names one of them is a lifecycle write.
var activityExecutionColumns = []string{"active_group_id", "started_at", "started_by", "completed_at", "completed_by", "reopen_until", "completion_snapshot"}

// UpdateColumns routes the retained lifecycle column sets to the owner that
// holds them: starting and completing a block are Student Presence session
// commands, cancelling discards the session and then patches the plan, and
// every other column set is a plan patch.
func (r timetableActivityInstanceRepository) UpdateColumns(ctx context.Context, value *scheduleModels.ActivityInstance, columns ...string) (int64, error) {
	if value == nil {
		return 0, errors.New("ActivityInstance cannot be nil or zero value")
	}
	if len(columns) == 0 {
		return 0, errors.New("update columns ActivityInstance: at least one column required")
	}
	if !slices.Contains(columns, "status") {
		if slices.ContainsFunc(columns, func(column string) bool { return slices.Contains(activityExecutionColumns, column) }) {
			return 0, timetableCompose.WrapDatabaseError("update columns", errors.New("execution columns are written through a status transition"))
		}
		return r.patch(ctx, value, columns)
	}
	switch value.Status {
	case scheduleModels.InstanceStatusActive:
		return r.start(ctx, value)
	case scheduleModels.InstanceStatusCompleted:
		return r.complete(ctx, value)
	case scheduleModels.InstanceStatusCancelled:
		if err := r.presence.DiscardActivitySession(ctx, value.ID); err != nil {
			return 0, timetableCompose.WrapDatabaseError("update columns", err)
		}
		return r.patch(ctx, value, planningColumns(columns))
	default:
		return r.patch(ctx, value, planningColumns(columns))
	}
}

func (r timetableActivityInstanceRepository) patch(ctx context.Context, value *scheduleModels.ActivityInstance, columns []string) (int64, error) {
	if len(columns) == 0 {
		return 1, nil
	}
	rows, err := r.timetable.PatchActivityInstance(ctx, value.ID, publicActivityInstanceInput(value), columns)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("update columns", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) start(ctx context.Context, value *scheduleModels.ActivityInstance) (int64, error) {
	if value.ActiveGroupID == nil || value.StartedAt == nil {
		return 0, timetableCompose.WrapDatabaseError("update columns", errors.New("starting an instance requires active_group_id and started_at"))
	}
	if _, err := r.presence.StartActivitySession(ctx, studentpresence.ActivitySessionStart{
		InstanceID: value.ID, ActiveGroupID: *value.ActiveGroupID, StartedBy: value.StartedBy, StartedAt: *value.StartedAt,
	}); err != nil {
		return 0, timetableCompose.WrapDatabaseError("update columns", err)
	}
	return 1, nil
}

func (r timetableActivityInstanceRepository) complete(ctx context.Context, value *scheduleModels.ActivityInstance) (int64, error) {
	if value.CompletedAt == nil {
		return 0, timetableCompose.WrapDatabaseError("update columns", errors.New("completing an instance requires completed_at"))
	}
	if _, err := r.presence.CompleteActivitySession(ctx, studentpresence.ActivitySessionCompletion{
		InstanceID: value.ID, CompletedAt: *value.CompletedAt, CompletedBy: value.CompletedBy,
		ReopenUntil: value.ReopenUntil, CompletionSnapshot: []byte(value.CompletionSnapshot),
	}); err != nil {
		if errors.Is(err, studentpresence.ErrActivitySessionNotFound) {
			return 0, nil
		}
		return 0, timetableCompose.WrapDatabaseError("update columns", err)
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

func (r timetableActivityInstanceRepository) CountWithOptions(ctx context.Context, options *timetableCompose.ActivityInstanceQueryOptions) (int, error) {
	before, err := timetableCompose.ActivityInstanceBefore(options)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("count with options", err)
	}
	count, err := r.timetable.CountActivityInstances(ctx, before)
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("count with options", err)
	}
	return count, nil
}

func (r timetableActivityInstanceRepository) OldestBefore(ctx context.Context, column string, cutoff *timetableCompose.ActivityInstanceDate) (*timetableCompose.ActivityInstanceDate, error) {
	if column != legacyActivityInstanceDateColumn {
		return nil, timetableCompose.WrapDatabaseError("oldest before", fmt.Errorf("unsupported activity instance date column %q", column))
	}
	value, err := r.timetable.OldestActivityInstanceBefore(ctx, publicOptionalInstanceDate(cutoff))
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("oldest before", err)
	}
	if value == nil {
		return nil, nil
	}
	result, err := timetableCompose.ParseActivityInstanceDate(*value)
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError("oldest before", err)
	}
	return &result, nil
}

func (r timetableActivityInstanceRepository) DeleteOlderThan(ctx context.Context, column string, cutoff timetableCompose.ActivityInstanceDate) (int64, error) {
	if column != legacyActivityInstanceDateColumn {
		return 0, timetableCompose.WrapDatabaseError("delete older than", fmt.Errorf("unsupported activity instance date column %q", column))
	}
	rows, err := r.timetable.DeleteActivityInstancesBefore(ctx, cutoff.String())
	if err != nil {
		return 0, timetableCompose.WrapDatabaseError("delete older than", err)
	}
	return rows, nil
}

func (r timetableActivityInstanceRepository) list(ctx context.Context, filter timetableCompose.LegacyInstanceFilter, operation string) ([]*scheduleModels.ActivityInstance, error) {
	rows, err := r.reads.ListLegacyInstances(ctx, filter)
	if err != nil {
		return nil, timetableCompose.WrapDatabaseError(operation, err)
	}
	return rows, nil
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
		Title: value.Title, Description: value.Description, StartTime: publicClock(value.StartTime),
		EndTime: publicClock(value.EndTime), RoomID: value.RoomID, RequiredStaff: value.RequiredStaff,
		Status: status, ListKind: value.ListKind,
		IsSpontaneous: value.IsSpontaneous, UnderstaffedAck: value.UnderstaffedAck,
		UnderstaffedNote: value.UnderstaffedNote, CancelReason: value.CancelReason, Notes: value.Notes,
		IdempotencyKey: value.IdempotencyKey, IdempotencyFingerprint: value.IdempotencyFingerprint,
		CreatedBy: value.CreatedBy,
	}
}

func publicOptionalInstanceDate(value *timetableCompose.ActivityInstanceDate) *string {
	if value == nil {
		return nil
	}
	text := value.String()
	return &text
}

func legacyActivityInstanceError(operation string, err error) error {
	if errors.Is(err, timetable.ErrActivityInstanceNotFound) {
		return timetableCompose.WrapNotFoundDatabaseError(operation)
	}
	return timetableCompose.WrapDatabaseError(operation, err)
}
