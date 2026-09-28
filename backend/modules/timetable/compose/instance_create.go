package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/sliceutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CreateInstance inserts a block and optionally pre-assigns staff and children. The
// origin (#2299) defaults to a planned block; only ad-hoc start flows pass
// IsSpontaneous=true. Conflicts are not checked here: the planner's reads
// surface them, and admins may create overlapping blocks on purpose.
func (s *InstanceLifecycleService) CreateInstance(ctx context.Context, req timetable.CreateInstanceInput) (*timetable.LifecycleInstance, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, &ScheduleError{Op: "create instance", Err: errors.New("no tenant in context")}
	}
	if !s.hasTx(ctx) {
		var created *timetable.LifecycleInstance
		err := tenant.WithTenantTx(ctx, s.deps.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var err error
			created, err = s.CreateInstance(txCtx, req)
			return err
		})
		return created, err
	}
	created, err := s.createInTenantTransaction(ctx, tenantID, req)
	return LifecycleInstanceOf(created), err
}

func (s *InstanceLifecycleService) createInTenantTransaction(
	ctx context.Context, tenantID int64, req timetable.CreateInstanceInput,
) (*scheduleModel.ActivityInstance, error) {
	idempotencyFingerprint, err := s.createInstanceIdempotencyFingerprint(req)
	if err != nil {
		return nil, &ScheduleError{Op: "create instance: fingerprint idempotency key", Err: err}
	}
	if existing, replayed, err := s.replayIdempotentCreate(ctx, req, idempotencyFingerprint, "create instance: find idempotency key"); replayed || err != nil {
		return existing, err
	}
	if err := s.lockRecurrenceThenGradeTransitions(ctx, "create instance"); err != nil {
		return nil, err
	}
	if existing, replayed, err := s.replayIdempotentCreate(ctx, req, idempotencyFingerprint, "create instance: reload idempotency key"); replayed || err != nil {
		return existing, err
	}
	if req.IsSpontaneous == nil || !*req.IsSpontaneous {
		if err := s.validateInstanceDateInActiveCalendarPeriod(ctx, req.Date); err != nil {
			return nil, &ScheduleError{Op: "create instance: validate calendar period", Err: err}
		}
	}
	if err := s.validateInstanceReferences(ctx, req.Date, instanceReferences{
		roomID: req.RoomID, activityGroupID: req.ActivityGroupID, staffIDs: req.StaffIDs,
		studentIDs: req.StudentIDs, createdByStaffID: req.CreatedByStaffID,
	}); err != nil {
		return nil, &ScheduleError{Op: "create instance: validate references", Err: err}
	}
	inst := newActivityInstance(tenantID, req, idempotencyFingerprint)
	result, inserted, err := s.insertCreatedInstance(ctx, inst)
	if err != nil || !inserted {
		return result, err
	}
	if err := s.assignCreatedInstanceRoster(ctx, inst, req, tenantID); err != nil {
		return nil, err
	}
	s.logCreatedInstance(ctx, inst, req)
	return inst, nil
}

// replayIdempotentCreate returns the block an earlier request with the same
// idempotency key created; replayed is false without a key or a match.
func (s *InstanceLifecycleService) replayIdempotentCreate(
	ctx context.Context, req timetable.CreateInstanceInput, fingerprint *string, op string,
) (*scheduleModel.ActivityInstance, bool, error) {
	if req.IdempotencyKey == nil {
		return nil, false, nil
	}
	existing, err := s.findCreateByIdempotencyKey(ctx, *req.IdempotencyKey)
	if err != nil {
		return nil, false, &ScheduleError{Op: op, Err: err}
	}
	if existing == nil {
		return nil, false, nil
	}
	result, err := idempotentCreateResult(existing, fingerprint)
	return result, true, err
}

func newActivityInstance(tenantID int64, req timetable.CreateInstanceInput, idempotencyFingerprint *string) *scheduleModel.ActivityInstance {
	isSpontaneous := false
	if req.IsSpontaneous != nil {
		isSpontaneous = *req.IsSpontaneous
	}
	inst := &scheduleModel.ActivityInstance{
		Date:                   scheduleModel.Date(req.Date),
		StartTime:              req.StartTime,
		EndTime:                req.EndTime,
		Title:                  req.Title,
		Description:            req.Description,
		Notes:                  req.Notes,
		RoomID:                 req.RoomID,
		ActivityGroupID:        req.ActivityGroupID,
		RequiredStaff:          req.RequiredStaff,
		ListKind:               req.ListKind,
		Status:                 scheduleModel.InstanceStatusPlanned,
		IsSpontaneous:          isSpontaneous,
		CreatedBy:              req.CreatedByStaffID,
		IdempotencyKey:         req.IdempotencyKey,
		IdempotencyFingerprint: idempotencyFingerprint,
	}
	inst.SetTenantID(tenantID)
	return inst
}

// createInstanceIdempotencyFingerprint is the content fingerprint of the
// normalized request; a replay with different data is refused.
func (s *InstanceLifecycleService) createInstanceIdempotencyFingerprint(req timetable.CreateInstanceInput) (*string, error) {
	if req.IdempotencyKey == nil {
		return nil, nil
	}
	isSpontaneous := req.IsSpontaneous != nil && *req.IsSpontaneous
	staffIDs := sliceutil.UniquePositive(slices.Clone(req.StaffIDs))
	studentIDs := sliceutil.UniquePositive(slices.Clone(req.StudentIDs))
	slices.Sort(staffIDs)
	slices.Sort(studentIDs)
	payload, err := json.Marshal(struct {
		Date             string  `json:"date"`
		StartTime        string  `json:"start_time"`
		EndTime          string  `json:"end_time"`
		Title            string  `json:"title"`
		Description      *string `json:"description"`
		Notes            *string `json:"notes"`
		RoomID           int64   `json:"room_id"`
		ActivityGroupID  *int64  `json:"activity_group_id"`
		ListKind         *string `json:"list_kind"`
		IsSpontaneous    bool    `json:"is_spontaneous"`
		StaffIDs         []int64 `json:"staff_ids"`
		StudentIDs       []int64 `json:"student_ids"`
		CreatedByStaffID *int64  `json:"created_by_staff_id"`
		RequiredStaff    *int    `json:"required_staff"`
	}{
		Date: req.Date.String(), StartTime: req.StartTime.Format("15:04:05"), EndTime: req.EndTime.Format("15:04:05"),
		Title: req.Title, Description: req.Description, Notes: req.Notes, RoomID: req.RoomID,
		ActivityGroupID: req.ActivityGroupID, ListKind: req.ListKind, IsSpontaneous: isSpontaneous,
		StaffIDs: staffIDs, StudentIDs: studentIDs, CreatedByStaffID: req.CreatedByStaffID, RequiredStaff: req.RequiredStaff,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal create request: %w", err)
	}
	fingerprint := s.deps.ContentHash(payload)
	return &fingerprint, nil
}

func idempotentCreateResult(existing *scheduleModel.ActivityInstance, fingerprint *string) (*scheduleModel.ActivityInstance, error) {
	if fingerprint == nil || existing.IdempotencyFingerprint == nil ||
		*existing.IdempotencyFingerprint != *fingerprint {
		return nil, timetable.ErrIdempotencyKeyReuse
	}
	return existing, nil
}

func (s *InstanceLifecycleService) insertCreatedInstance(ctx context.Context, inst *scheduleModel.ActivityInstance) (*scheduleModel.ActivityInstance, bool, error) {
	if inst.IdempotencyKey == nil {
		if err := s.deps.InstanceRepo.Create(ctx, inst); err != nil {
			return nil, false, &ScheduleError{Op: "create instance: insert", Err: duplicateTemplateInstance(err)}
		}
		return inst, true, nil
	}
	inserted, err := s.deps.IdempotencyRepo.CreateIdempotent(ctx, inst)
	if err != nil {
		return nil, false, &ScheduleError{Op: "create instance: insert idempotent", Err: duplicateTemplateInstance(err)}
	}
	if inserted {
		return inst, true, nil
	}
	existing, err := s.findCreateByIdempotencyKey(ctx, *inst.IdempotencyKey)
	if err != nil {
		return nil, false, &ScheduleError{Op: "create instance: load idempotent result", Err: err}
	}
	if existing == nil {
		return nil, false, &ScheduleError{
			Op:  "create instance: load idempotent result",
			Err: errors.New("idempotent insert conflict has no matching row"),
		}
	}
	result, err := idempotentCreateResult(existing, inst.IdempotencyFingerprint)
	return result, false, err
}

func (s *InstanceLifecycleService) findCreateByIdempotencyKey(ctx context.Context, key string) (*scheduleModel.ActivityInstance, error) {
	options := modelBase.NewQueryOptions().WithPagination(1, 1)
	options.Filter.Equal("idempotency_key", key)
	instances, err := legacyList[*scheduleModel.ActivityInstance](ctx, s.deps.InstanceRepo, options)
	if err != nil || len(instances) == 0 {
		return nil, err
	}
	return instances[0], nil
}

func (s *InstanceLifecycleService) assignCreatedInstanceRoster(ctx context.Context, inst *scheduleModel.ActivityInstance, req timetable.CreateInstanceInput, tenantID int64) error {
	for _, staffID := range sliceutil.UniquePositive(req.StaffIDs) {
		// A nil RoomID falls back to the block's room at runtime.
		row := &scheduleModel.InstanceStaff{InstanceID: inst.ID, StaffID: staffID, IsPrimary: false}
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStaffRepo.Create(ctx, row); err != nil {
			return &ScheduleError{Op: "create instance: assign staff", Err: err}
		}
	}
	studentIDs := sliceutil.UniquePositive(req.StudentIDs)
	if err := s.lockCareExceptionDaysForStudents(ctx, studentIDs, timezone.Date(inst.Date)); err != nil {
		return err
	}
	return s.assignInstanceStudents(ctx, inst, studentIDs, tenantID, "create instance")
}

// assignInstanceStudents creates the expected attendance rows and applies
// the children's active status days and partial absences to them.
func (s *InstanceLifecycleService) assignInstanceStudents(ctx context.Context, inst *scheduleModel.ActivityInstance, studentIDs []int64, tenantID int64, op string) error {
	for _, studentID := range studentIDs {
		row := &scheduleModel.InstanceStudent{
			InstanceID: inst.ID,
			StudentID:  studentID,
			Status:     scheduleModel.AttendanceStatusExpected,
		}
		row.SetTenantID(tenantID)
		if err := s.deps.InstanceStudents.Create(ctx, row); err != nil {
			return &ScheduleError{Op: op + ": assign student", Err: err}
		}
	}
	if _, err := s.deps.InstanceStudents.ApplyActiveStatusDaysForInstance(ctx, inst.ID, inst.Date); err != nil {
		return &ScheduleError{Op: op + ": apply student status days", Err: err}
	}
	if _, err := s.deps.InstanceStudents.ApplyActivePartialAbsencesForInstance(ctx, inst.ID, inst.Date); err != nil {
		return &ScheduleError{Op: op + ": apply student partial absences", Err: err}
	}
	return nil
}

func (s *InstanceLifecycleService) logCreatedInstance(ctx context.Context, inst *scheduleModel.ActivityInstance, req timetable.CreateInstanceInput) {
	s.getLogger().Info("instance created",
		slog.Int64("tenant_id", inst.GetTenantID()),
		slog.Int64("instance_id", inst.ID),
		slog.String("date", inst.Date.String()),
		slog.Bool("spontaneous", inst.IsSpontaneous),
		slog.Int("staff_assigned", len(req.StaffIDs)),
	)
	s.broadcastPlannedInstanceChanged(ctx, "instance_create")
}

// templateInstanceUniqueIndex keeps one block per template, date and start
// time.
const templateInstanceUniqueIndex = "idx_activity_instances_template_unique"

// duplicateTemplateInstance classifies a write that collides with another
// block of the same template slot; the storage error stays in the chain.
func duplicateTemplateInstance(err error) error {
	if modelBase.IsUniqueViolationOn(err, templateInstanceUniqueIndex) {
		return fmt.Errorf("%w: %w", timetable.ErrDuplicateTemplateInstance, err)
	}
	return err
}
