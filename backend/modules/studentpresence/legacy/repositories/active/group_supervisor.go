// backend/modules/studentpresence/legacy/repositories/active/group_supervisor.go
package active

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// GroupSupervisorRepository implements active.GroupSupervisorRepository interface
type GroupSupervisorRepository struct {
	records SupervisionRecords
	today   func() active.Date
}

type SupervisionFilter struct {
	StaffIDs      []int64
	EndedBy       *string
	Limit, Offset int
	IDs           []int64
	OpenOnly      bool
	StartedBefore *string
	ForUpdate     bool
	GroupIDs      []int64
	StaffID       *int64
	ActiveOn      *string
}

type StaffRoomSupervision struct{ StaffID, RoomID int64 }

type SupervisionRecords interface {
	RecordErrors
	SetSupervisionEnd(context.Context, int64, string, time.Time) (int64, error)
	EndOpenGroupSupervisions(context.Context, int64, int64, string) (int, error)
	CreateSupervisionRecord(context.Context, *active.GroupSupervisor) error
	UpdateSupervisionRecord(context.Context, *active.GroupSupervisor) (bool, error)
	DeleteSupervisionRecord(context.Context, int64) error
	EndStaffSupervisionsOn(context.Context, int64, string) (int, error)
	EndSupervisionOn(context.Context, int64, string) (int, error)
	SupervisedRoomsOn(context.Context, string) ([]StaffRoomSupervision, error)
	StaffIDsWithSupervisionOn(context.Context, string) ([]int64, error)
	QueryGroupSupervisions(context.Context, SupervisionFilter) ([]GroupSupervisionRecord, error)
}

// NewGroupSupervisorRepository creates a new GroupSupervisorRepository
func NewGroupSupervisorRepository(records SupervisionRecords, clocks ...func() time.Time) active.GroupSupervisorRepository {
	return &GroupSupervisorRepository{records: records, today: active.CalendarDateClock(clocks...)}
}

// FindActiveByStaffID finds all active supervisions for a specific staff member
func (r *GroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	day := r.today().String()
	return r.querySupervisions(ctx, SupervisionFilter{StaffID: &staffID, ActiveOn: &day})
}

// FindActiveByStaffIDForUpdate locks current supervision rows so a caller can
// make an authorization decision that remains valid through its write.
func (r *GroupSupervisorRepository) FindActiveByStaffIDForUpdate(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	date := r.today().String()
	return r.querySupervisions(ctx, SupervisionFilter{StaffID: &staffID, ActiveOn: &date, ForUpdate: true})
}

// ListActiveSupervisedRooms returns distinct staff/room pairs in open sessions.
func (r *GroupSupervisorRepository) ListActiveSupervisedRooms(ctx context.Context) ([]active.StaffRoomSupervision, error) {
	rows, err := r.records.SupervisedRoomsOn(ctx, r.today().String())
	if err != nil {
		return nil, err
	}
	result := make([]active.StaffRoomSupervision, 0, len(rows))
	for _, row := range rows {
		result = append(result, active.StaffRoomSupervision{StaffID: row.StaffID, RoomID: row.RoomID})
	}
	return result, nil
}

// FindStaleOpen returns supervisor rows started before the given day that
// still lack an end_date. Feeds the nightly stale-supervisor cleanup and its
// preview (modules/studentpresence/legacy/services/active/cleanup_service.go, session_service.go).
func (r *GroupSupervisorRepository) FindStaleOpen(ctx context.Context, before active.Date) ([]*active.GroupSupervisor, error) {
	date := before.String()
	return r.querySupervisions(ctx, SupervisionFilter{OpenOnly: true, StartedBefore: &date})
}

// FindByActiveGroupID finds supervisors for a specific active group
// If activeOnly is true, returns supervisors whose start date has been reached
// and whose end date has not been reached.
// Includes the Staff relation; the composition layer attaches Staff.Person
// through the People Directory for name display.
func (r *GroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	return r.FindByActiveGroupIDs(ctx, []int64{activeGroupID}, activeOnly)
}

// FindByActiveGroupIDForUpdate locks current supervision rows so a caller can
// rely on target coverage through its write.
func (r *GroupSupervisorRepository) FindByActiveGroupIDForUpdate(ctx context.Context, groupID int64) ([]*active.GroupSupervisor, error) {
	date := r.today().String()
	return r.querySupervisions(ctx, SupervisionFilter{GroupIDs: []int64{groupID}, ActiveOn: &date, ForUpdate: true})
}

// FindByActiveGroupIDs finds supervisors for multiple active groups in a single query
// If activeOnly is true, returns supervisors whose start date has been reached
// and whose end date has not been reached.
// Includes the Staff relation; the composition layer attaches Staff.Person
// through the People Directory for name display.
func (r *GroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, ids []int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	if len(ids) == 0 {
		return []*active.GroupSupervisor{}, nil
	}
	filter := SupervisionFilter{GroupIDs: ids}
	if activeOnly {
		date := r.today().String()
		filter.ActiveOn = &date
	}
	return r.querySupervisions(ctx, filter)
}

func (r *GroupSupervisorRepository) querySupervisions(ctx context.Context, filter SupervisionFilter) ([]*active.GroupSupervisor, error) {
	rows, err := r.records.QueryGroupSupervisions(ctx, filter)
	if err != nil {
		return nil, err
	}
	return legacyGroupSupervisions(rows)
}

// EndSupervision marks a supervision as ended at the current date
func (r *GroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	_, err := r.records.EndSupervisionOn(ctx, id, r.today().String())
	return err
}

func (r *GroupSupervisorRepository) Create(ctx context.Context, row *active.GroupSupervisor) error {
	if row == nil {
		return fmt.Errorf("group supervisor cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	if err := r.records.CreateSupervisionRecord(ctx, row); err != nil {
		return r.records.WrapRecordError("create", err)
	}
	return nil
}

func (r *GroupSupervisorRepository) Update(ctx context.Context, row *active.GroupSupervisor) error {
	if row == nil {
		return fmt.Errorf("group supervisor cannot be nil")
	}
	if err := row.Validate(); err != nil {
		return err
	}
	found, err := r.records.UpdateSupervisionRecord(ctx, row)
	if err != nil {
		return r.records.WrapRecordError("update", err)
	}
	if !found {
		return r.records.MissingRecordError("update")
	}
	return nil
}

func (r *GroupSupervisorRepository) Delete(ctx context.Context, id int64) error {
	return r.records.DeleteSupervisionRecord(ctx, id)
}

func (r *GroupSupervisorRepository) FindByID(ctx context.Context, id int64) (*active.GroupSupervisor, error) {
	rows, err := r.querySupervisions(ctx, SupervisionFilter{IDs: []int64{id}})
	if err != nil {
		return nil, r.records.WrapRecordError("find by id", err)
	}
	if len(rows) == 0 {
		return nil, r.records.MissingRecordError("find by id")
	}
	return rows[0], nil
}

// EndAllActiveByStaffID ends all active supervisions for a staff member.
// Sets end_date = CURRENT_DATE for every supervision active today.
// Returns the number of supervisions that were ended.
func (r *GroupSupervisorRepository) EndAllActiveByStaffID(ctx context.Context, staffID int64) (int, error) {
	return r.records.EndStaffSupervisionsOn(ctx, staffID, r.today().String())
}

// EndByActiveGroupAndStaffID ends active supervisions matching both the
// active_group_id and staff_id. Sets end_date=now() on all rows with
// end_date IS NULL. Idempotent — zero matches is not an error. Tenant-scoped.
func (r *GroupSupervisorRepository) EndByActiveGroupAndStaffID(ctx context.Context, activeGroupID, staffID int64) (int, error) {
	return r.records.EndOpenGroupSupervisions(ctx, activeGroupID, staffID, r.today().String())
}

func (r *GroupSupervisorRepository) SetEndDate(ctx context.Context, row *active.GroupSupervisor) (int64, error) {
	if row == nil || row.EndDate == nil {
		return 0, fmt.Errorf("supervision end date is required")
	}
	return r.records.SetSupervisionEnd(ctx, row.ID, row.EndDate.String(), row.UpdatedAt)
}

// GetStaffIDsWithSupervisionToday returns staff IDs who had any supervision activity today.
// This is used to determine "Anwesend" status - staff who were physically present via PyrePortal.
// A staff member is considered present today if:
// - Their supervision started today, OR
// - Their supervision ended today, OR
// - Their supervision spans today (started before and still ongoing or ends after today)
func (r *GroupSupervisorRepository) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	return r.records.StaffIDsWithSupervisionOn(ctx, r.today().String())
}

// ListActiveSupervisionBlockers projects tenant-owned supervision rows for caregiver checks.
func (r *GroupSupervisorRepository) ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]active.SupervisionBlocker, error) {
	date := r.today().String()
	rows, err := r.records.QueryGroupSupervisions(ctx, SupervisionFilter{StaffID: &staffID, ActiveOn: &date})
	if err != nil {
		return nil, err
	}
	result := make([]active.SupervisionBlocker, 0, len(rows))
	for _, row := range rows {
		result = append(result, active.SupervisionBlocker{ID: row.ID, GroupID: row.GroupID, StartDate: row.StartDate})
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].StartDate > result[j].StartDate })
	return result, nil
}
