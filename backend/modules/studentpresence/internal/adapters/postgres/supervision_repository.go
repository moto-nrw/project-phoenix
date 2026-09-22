package postgres

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

// SupervisionRepository reads and writes the supervision of room sessions
// through the owner's record store. "Today" is the school calendar date of
// the repository's clock.
type SupervisionRepository struct {
	records ports.SupervisionRecords
	errors  ports.RecordErrors
	staff   ports.SupervisionStaffDirectory
	today   func() ports.Date
}

var _ ports.GroupSupervisorRepository = (*SupervisionRepository)(nil)

// NewSupervisionRepository builds the supervision repository. staff resolves
// the staff members the group reads carry and may be nil; today is the school
// calendar date open supervisions are compared against.
func NewSupervisionRepository(records ports.SupervisionRecords, errs ports.RecordErrors, staff ports.SupervisionStaffDirectory, today func() ports.Date) *SupervisionRepository {
	return &SupervisionRepository{records: records, errors: errs, staff: staff, today: today}
}

// FindActiveByStaffID finds all active supervisions for a specific staff member
func (r *SupervisionRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*ports.GroupSupervisor, error) {
	day := r.today()
	return r.querySupervisions(ctx, ports.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day})
}

// FindActiveByStaffIDForUpdate locks current supervision rows so a caller can
// make an authorization decision that remains valid through its write.
func (r *SupervisionRepository) FindActiveByStaffIDForUpdate(ctx context.Context, staffID int64) ([]*ports.GroupSupervisor, error) {
	day := r.today()
	return r.querySupervisions(ctx, ports.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day, ForUpdate: true})
}

// ListActiveSupervisedRooms returns distinct staff/room pairs in open sessions.
func (r *SupervisionRepository) ListActiveSupervisedRooms(ctx context.Context) ([]ports.StaffRoomSupervision, error) {
	return r.records.SupervisedRoomsOn(ctx, r.today())
}

// FindStaleOpen returns supervisor rows started before the given day that
// still lack an end_date. Feeds the nightly stale-supervisor cleanup and its
// preview.
func (r *SupervisionRepository) FindStaleOpen(ctx context.Context, before ports.Date) ([]*ports.GroupSupervisor, error) {
	return r.querySupervisions(ctx, ports.GroupSupervisionFilter{OpenOnly: true, StartedBefore: &before})
}

// FindByActiveGroupID finds supervisors for a specific active group. If
// activeOnly is true, it returns supervisors whose start date has been
// reached and whose end date has not been reached. Each row carries its staff
// member when the staff directory can resolve it.
func (r *SupervisionRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*ports.GroupSupervisor, error) {
	return r.FindByActiveGroupIDs(ctx, []int64{activeGroupID}, activeOnly)
}

// FindByActiveGroupIDForUpdate locks current supervision rows so a caller can
// rely on target coverage through its write.
func (r *SupervisionRepository) FindByActiveGroupIDForUpdate(ctx context.Context, groupID int64) ([]*ports.GroupSupervisor, error) {
	day := r.today()
	return r.querySupervisions(ctx, ports.GroupSupervisionFilter{GroupIDs: []int64{groupID}, ActiveOn: &day, ForUpdate: true})
}

// FindByActiveGroupIDs finds supervisors for multiple active groups in a
// single query, with the same activeOnly rule and staff members as
// FindByActiveGroupID.
func (r *SupervisionRepository) FindByActiveGroupIDs(ctx context.Context, ids []int64, activeOnly bool) ([]*ports.GroupSupervisor, error) {
	if len(ids) == 0 {
		return []*ports.GroupSupervisor{}, nil
	}
	filter := ports.GroupSupervisionFilter{GroupIDs: ids}
	if activeOnly {
		day := r.today()
		filter.ActiveOn = &day
	}
	rows, err := r.querySupervisions(ctx, filter)
	if err != nil {
		return nil, err
	}
	return rows, r.attachStaff(ctx, rows)
}

// attachStaff resolves the staff members behind the rows through the bound
// directory; without one the rows stay as read.
func (r *SupervisionRepository) attachStaff(ctx context.Context, rows []*ports.GroupSupervisor) error {
	if r.staff == nil {
		return nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StaffID)
	}
	staff, err := r.staff.SupervisionStaff(ctx, ids)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if member, found := staff[row.StaffID]; found {
			row.Staff = member
		}
	}
	return nil
}

func (r *SupervisionRepository) querySupervisions(ctx context.Context, filter ports.GroupSupervisionFilter) ([]*ports.GroupSupervisor, error) {
	rows, err := r.records.QueryGroupSupervisions(ctx, filter)
	if err != nil {
		return nil, err
	}
	return groupSupervisors(rows), nil
}

// EndSupervision marks a supervision as ended at the current date
func (r *SupervisionRepository) EndSupervision(ctx context.Context, id int64) error {
	_, err := r.records.EndSupervisionOn(ctx, id, r.today())
	return err
}

func (r *SupervisionRepository) Create(ctx context.Context, row *ports.GroupSupervisor) error {
	if row == nil {
		return fmt.Errorf("group supervisor cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	result, err := r.records.RecordSupervision(ctx, groupSupervision(row))
	if err != nil {
		return r.errors.WrapRecordError("create", err)
	}
	row.ID, row.CreatedAt, row.UpdatedAt, row.TenantID = result.ID, result.CreatedAt, result.UpdatedAt, result.TenantID
	return nil
}

func (r *SupervisionRepository) Update(ctx context.Context, row *ports.GroupSupervisor) error {
	if row == nil {
		return fmt.Errorf("group supervisor cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	result, err := r.records.ReviseSupervision(ctx, groupSupervision(row))
	if errors.Is(err, ports.ErrSupervisionNotFound) {
		return r.errors.MissingRecordError("update")
	}
	if err != nil {
		return r.errors.WrapRecordError("update", err)
	}
	row.CreatedAt, row.UpdatedAt, row.TenantID = result.CreatedAt, result.UpdatedAt, result.TenantID
	return nil
}

func (r *SupervisionRepository) Delete(ctx context.Context, id int64) error {
	return r.records.RemoveSupervision(ctx, id)
}

func (r *SupervisionRepository) FindByID(ctx context.Context, id int64) (*ports.GroupSupervisor, error) {
	rows, err := r.querySupervisions(ctx, ports.GroupSupervisionFilter{IDs: []int64{id}})
	if err != nil {
		return nil, r.errors.WrapRecordError("find by id", err)
	}
	if len(rows) == 0 {
		return nil, r.errors.MissingRecordError("find by id")
	}
	return rows[0], nil
}

// EndAllActiveByStaffID ends all active supervisions for a staff member.
// Sets end_date to today for every supervision active today and returns the
// number of supervisions that were ended.
func (r *SupervisionRepository) EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error) {
	return r.records.EndStaffSupervisionsOn(ctx, staffID, r.today())
}

// EndByActiveGroupAndStaffID ends active supervisions matching both the
// active_group_id and staff_id. Idempotent — zero matches is not an error.
// Tenant-scoped.
func (r *SupervisionRepository) EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return r.records.EndOpenGroupSupervisions(ctx, activeGroupID, staffID, r.today())
}

func (r *SupervisionRepository) SetEndDate(ctx context.Context, row *ports.GroupSupervisor) (int64, error) {
	if row == nil || row.EndDate == nil {
		return 0, fmt.Errorf("supervision end date is required")
	}
	return r.records.SetSupervisionEnd(ctx, row.ID, *row.EndDate, row.UpdatedAt)
}

// GetStaffIDsWithSupervisionToday returns staff IDs who had any supervision
// activity today: supervision that started today, ended today or spans today.
// This determines the "Anwesend" status of staff present via PyrePortal.
func (r *SupervisionRepository) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	return r.records.StaffIDsWithSupervisionOn(ctx, r.today())
}

// ListActiveSupervisionBlockers projects tenant-owned supervision rows for
// caregiver checks, latest start first.
func (r *SupervisionRepository) ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]ports.SupervisionBlocker, error) {
	day := r.today()
	rows, err := r.records.QueryGroupSupervisions(ctx, ports.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day})
	if err != nil {
		return nil, err
	}
	result := make([]ports.SupervisionBlocker, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.SupervisionBlocker{ID: row.ID, GroupID: row.GroupID, StartDate: row.StartDate.String()})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartDate > result[j].StartDate })
	return result, nil
}

func groupSupervisors(rows []ports.GroupSupervision) []*ports.GroupSupervisor {
	result := make([]*ports.GroupSupervisor, 0, len(rows))
	for _, row := range rows {
		result = append(result, &ports.GroupSupervisor{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			GroupID: row.GroupID, StaffID: row.StaffID, Role: row.Role, StartDate: row.StartDate, EndDate: row.EndDate,
		})
	}
	return result
}

func groupSupervision(row *ports.GroupSupervisor) ports.GroupSupervision {
	return ports.GroupSupervision{
		ID: row.ID, StaffID: row.StaffID, GroupID: row.GroupID, Role: row.Role,
		StartDate: row.StartDate, EndDate: row.EndDate, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
