package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
)

// The row adapters in this file serve the time-tracking staff absence and
// absence audit contracts from the Workforce capability, like
// worksession_rows.go does for the work-session rows (#3996). They perform no
// persistence of their own: they map the rows onto the public capability
// types and preserve the error shapes their callers still classify on. The
// root wraps the absence rows with the School Membership and People
// Directory lookups of the request listing and names them through the
// aliases below.

// The staff absence row contracts the root's decorators and their suites name.
type (
	StaffAbsence                = timetracking.StaffAbsence
	StaffAbsenceRepository      = timetracking.StaffAbsenceRepository
	StaffAbsenceAuditRepository = timetracking.StaffAbsenceAuditRepository
	AbsenceRequestFilter        = timetracking.AbsenceRequestFilter
	AbsenceRequestRow           = timetracking.AbsenceRequestRow
)

// staffAbsenceRepository serves StaffAbsenceRepository from the Workforce
// capability.
type staffAbsenceRepository struct{ workforce workforce.Capability }

func NewStaffAbsenceRepository(capability workforce.Capability) StaffAbsenceRepository {
	if capability == nil {
		panic("staff absence repository adapter: Workforce capability is required")
	}
	return staffAbsenceRepository{workforce: capability}
}

func (r staffAbsenceRepository) Create(ctx context.Context, entity *StaffAbsence) error {
	if entity == nil {
		return errors.New("StaffAbsence cannot be nil or zero value")
	}
	created, err := r.workforce.CreateStaffAbsence(ctx, absenceToWorkforce(entity))
	if err != nil {
		return absenceWriteError("create", err)
	}
	applyAbsenceToLegacy(entity, created)
	return nil
}

func (r staffAbsenceRepository) FindByID(ctx context.Context, id any) (*StaffAbsence, error) {
	absenceID, err := legacyID(id)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find by id", Err: err}
	}
	value, err := r.workforce.FindStaffAbsence(ctx, absenceID)
	if err != nil {
		return nil, rowReadError("find by id", err, workforce.ErrStaffAbsenceNotFound)
	}
	return absenceToLegacy(value), nil
}

func (r staffAbsenceRepository) Update(ctx context.Context, entity *StaffAbsence) error {
	if entity == nil {
		return errors.New("StaffAbsence cannot be nil or zero value")
	}
	updated, err := r.workforce.UpdateStaffAbsence(ctx, absenceToWorkforce(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffAbsenceNotFound) {
			return rowsAffectedError("update StaffAbsence")
		}
		return absenceWriteError("update", err)
	}
	applyAbsenceToLegacy(entity, updated)
	return nil
}

func (r staffAbsenceRepository) Delete(ctx context.Context, id any) error {
	absenceID, err := legacyID(id)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete", Err: err}
	}
	if err := r.workforce.DeleteStaffAbsence(ctx, absenceID); err != nil {
		return absenceWriteError("delete", err)
	}
	return nil
}

func (r staffAbsenceRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*StaffAbsence, error) {
	filter, err := staffAbsenceFilterFromOptions(options)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list with options", Err: err}
	}
	return r.list(ctx, "list with options", filter)
}

func (r staffAbsenceRepository) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	filter, err := staffAbsenceFilterFromOptions(options)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count with options", Err: err}
	}
	count, err := r.workforce.CountStaffAbsences(ctx, filter)
	if err != nil {
		return 0, rowReadError("count with options", err, nil)
	}
	return count, nil
}

func (r staffAbsenceRepository) OldestBefore(ctx context.Context, column string, cutoff *timezone.Date) (*timezone.Date, error) {
	before := ""
	if cutoff != nil {
		before = cutoff.String()
	}
	oldest, err := r.workforce.OldestStaffAbsenceDate(ctx, column, before)
	if err != nil {
		return nil, rowReadError("oldest before", err, nil)
	}
	if oldest == "" {
		return nil, nil
	}
	date, err := timezone.ParseDate(oldest)
	if err != nil {
		return nil, err
	}
	return &date, nil
}

func (r staffAbsenceRepository) DeleteOlderThan(ctx context.Context, column string, cutoff timezone.Date) (int64, error) {
	deleted, err := r.workforce.DeleteStaffAbsencesOlderThan(ctx, column, cutoff.String())
	if err != nil {
		return 0, absenceWriteError("delete older than", err)
	}
	return deleted, nil
}

func (r staffAbsenceRepository) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffAbsenceWrites(ctx, staffID)
}

func (r staffAbsenceRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsence, error) {
	return r.list(ctx, "get absences by staff and date range", workforce.StaffAbsenceFilter{
		StaffID: staffID, OverlapFrom: from.String(), OverlapTo: to.String(),
		Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}},
	})
}

func (r staffAbsenceRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to timezone.Date) (map[int64][]*StaffAbsence, error) {
	result := make(map[int64][]*StaffAbsence, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	absences, err := r.list(ctx, "get absences by staff IDs and date range", workforce.StaffAbsenceFilter{
		StaffIDs: staffIDs, OverlapFrom: from.String(), OverlapTo: to.String(),
		Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderStaffID}, {Field: workforce.StaffAbsenceOrderDateStart}},
	})
	if err != nil {
		return nil, err
	}
	for _, absence := range absences {
		result[absence.StaffID] = append(result[absence.StaffID], absence)
	}
	return result, nil
}

func (r staffAbsenceRepository) GetByStaffAndDate(ctx context.Context, staffID int64, date timezone.Date) (*StaffAbsence, error) {
	absences, err := r.list(ctx, "get absence by staff and date", workforce.StaffAbsenceFilter{
		StaffID: staffID, OverlapFrom: date.String(), OverlapTo: date.String(),
		Statuses: []string{workforce.AbsenceStatusReported, workforce.AbsenceStatusApproved}, Limit: 1,
	})
	if err != nil || len(absences) == 0 {
		return nil, err
	}
	return absences[0], nil
}

func (r staffAbsenceRepository) GetAbsenceMapForDate(ctx context.Context, date timezone.Date) (map[int64]string, error) {
	result, err := r.workforce.StaffAbsenceMapForDate(ctx, date.String())
	if err != nil {
		return nil, rowReadError("get effective absences for date", err, nil)
	}
	return result, nil
}

func (r staffAbsenceRepository) GetAbsenceTypeIDMapForDate(ctx context.Context, date timezone.Date) (map[int64]int64, error) {
	result, err := r.workforce.StaffAbsenceTypeIDMapForDate(ctx, date.String())
	if err != nil {
		return nil, rowReadError("get effective absences for date", err, nil)
	}
	return result, nil
}

func (r staffAbsenceRepository) ListByStatuses(ctx context.Context, statuses []string) ([]*StaffAbsence, error) {
	return r.list(ctx, "list absences by statuses", workforce.StaffAbsenceFilter{
		Statuses: statuses, Order: []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderRequestedAt}},
	})
}

// ListRequests is only correct once the composition layer has wrapped this
// adapter: the subject and decider persons belong to School Membership, and
// so does resolving filter.SubjectPersonIDs.
func (r staffAbsenceRepository) ListRequests(ctx context.Context, filter AbsenceRequestFilter) ([]*AbsenceRequestRow, error) {
	if filter.SubjectPersonIDs != nil {
		return nil, errors.New("staff absence repository resolves subject persons through School Membership")
	}
	return r.ListRequestRows(ctx, filter, nil)
}

// ListRequestRows returns absence requests narrowed to the subject staff IDs
// (nil means no subject filter); names are attached by the composition layer.
func (r staffAbsenceRepository) ListRequestRows(ctx context.Context, filter AbsenceRequestFilter, subjectStaffIDs []int64) ([]*AbsenceRequestRow, error) {
	absences, err := r.workforce.ListStaffAbsenceRequests(ctx, workforce.StaffAbsenceRequestFilter{
		Statuses: filter.Statuses, Types: filter.Types, FilterSubjects: subjectStaffIDs != nil,
		SubjectStaffIDs: subjectStaffIDs, Limit: filter.Limit, Decided: filter.Decided,
	})
	if err != nil {
		if errors.Is(err, workforce.ErrInvalidStaffAbsence) {
			return nil, errors.New(err.Error())
		}
		return nil, rowReadError("list absence requests", err, nil)
	}
	rows := make([]*AbsenceRequestRow, 0, len(absences))
	for _, absence := range absences {
		rows = append(rows, &AbsenceRequestRow{StaffAbsence: absenceToLegacy(absence)})
	}
	return rows, nil
}

func (r staffAbsenceRepository) list(ctx context.Context, op string, filter workforce.StaffAbsenceFilter) ([]*StaffAbsence, error) {
	values, err := r.workforce.ListStaffAbsences(ctx, filter)
	if err != nil {
		return nil, rowReadError(op, err, nil)
	}
	// Empty, not nil: callers serialize the result straight to JSON.
	result := make([]*StaffAbsence, 0, len(values))
	for _, value := range values {
		result = append(result, absenceToLegacy(value))
	}
	return result, nil
}

// staffAbsenceFilterFromOptions translates the generic query options the
// retained callers still build into the typed capability filter. Only the
// columns and operators those callers use are accepted; anything else is an
// error rather than a silently ignored predicate.
func staffAbsenceFilterFromOptions(options *modelBase.QueryOptions) (workforce.StaffAbsenceFilter, error) {
	filter := workforce.StaffAbsenceFilter{}
	if options == nil {
		return filter, nil
	}
	if options.Filter != nil {
		if len(options.Filter.OrFilters()) > 0 || len(options.Filter.AndFilters()) > 0 {
			return filter, errors.New("nested staff absence filters are not supported")
		}
		for _, condition := range options.Filter.Conditions() {
			if err := applyStaffAbsenceCondition(&filter, condition); err != nil {
				return filter, err
			}
		}
	}
	if options.Sorting != nil {
		for _, field := range options.Sorting.Fields {
			switch field.Field {
			case "id", "staff_id", "date_start", "date_end", "requested_at":
				filter.Order = append(filter.Order, workforce.StaffAbsenceOrder{
					Field: workforce.StaffAbsenceOrderField(field.Field), Descending: field.Direction == modelBase.SortDesc,
				})
			default:
				return filter, fmt.Errorf("unsupported staff absence sort field %q", field.Field)
			}
		}
	}
	if options.Pagination != nil && options.Pagination.PageSize > 0 {
		filter.Limit = options.Pagination.PageSize
		if options.Pagination.Page > 1 {
			filter.Offset = (options.Pagination.Page - 1) * options.Pagination.PageSize
		}
	}
	return filter, nil
}

func applyStaffAbsenceCondition(filter *workforce.StaffAbsenceFilter, condition modelBase.FilterCondition) error {
	switch {
	case condition.Field == "staff_id" && condition.Operator == modelBase.OpEqual:
		id, ok := conditionInt64(condition.Value)
		if !ok {
			return errors.New("staff_id filter must be an integer")
		}
		filter.StaffID = id
	case condition.Field == "staff_id" && condition.Operator == modelBase.OpIn:
		ids, ok := conditionInt64List(condition.Value)
		if !ok {
			return errors.New("staff_id filter must be a list of integers")
		}
		filter.StaffIDs = ids
	case condition.Field == "status" && condition.Operator == modelBase.OpEqual:
		status, ok := condition.Value.(string)
		if !ok {
			return errors.New("status filter must be a string")
		}
		filter.Statuses = []string{status}
	case condition.Field == "status" && condition.Operator == modelBase.OpIn:
		statuses, ok := conditionStringList(condition.Value)
		if !ok {
			return errors.New("status filter must be a list of strings")
		}
		filter.Statuses = statuses
	case condition.Field == "absence_type" && condition.Operator == modelBase.OpEqual:
		absenceType, ok := condition.Value.(string)
		if !ok {
			return errors.New("absence_type filter must be a string")
		}
		filter.Types = []string{absenceType}
	case condition.Field == "date_start" && condition.Operator == modelBase.OpLessThanOrEqual:
		date, ok := conditionDate(condition.Value)
		if !ok {
			return errors.New("date_start filter must be a calendar date")
		}
		filter.OverlapTo = date
	case condition.Field == "date_end" && condition.Operator == modelBase.OpGreaterThanOrEqual:
		date, ok := conditionDate(condition.Value)
		if !ok {
			return errors.New("date_end filter must be a calendar date")
		}
		filter.OverlapFrom = date
	case condition.Field == "date_end" && condition.Operator == modelBase.OpLessThan:
		date, ok := conditionDate(condition.Value)
		if !ok {
			return errors.New("date_end filter must be a calendar date")
		}
		filter.DateEndBefore = date
	case condition.Field == "date_start" && condition.Operator == modelBase.OpLessThan:
		date, ok := conditionDate(condition.Value)
		if !ok {
			return errors.New("date_start filter must be a calendar date")
		}
		filter.DateStartBefore = date
	default:
		return fmt.Errorf("unsupported staff absence filter %s %s", condition.Field, condition.Operator)
	}
	return nil
}

// staffAbsenceAuditRepository serves StaffAbsenceAuditRepository from the
// Workforce capability.
type staffAbsenceAuditRepository struct{ workforce workforce.Capability }

func NewStaffAbsenceAuditRepository(capability workforce.Capability) StaffAbsenceAuditRepository {
	if capability == nil {
		panic("staff absence audit repository adapter: Workforce capability is required")
	}
	return staffAbsenceAuditRepository{workforce: capability}
}

func (r staffAbsenceAuditRepository) Create(ctx context.Context, audit *timetracking.StaffAbsenceAudit) error {
	if audit == nil {
		return errors.New("staff absence audit cannot be nil")
	}
	if err := audit.Validate(); err != nil {
		return err
	}
	recorded, err := r.workforce.RecordStaffAbsenceAudit(ctx, workforce.StaffAbsenceAudit{
		TenantID: audit.TenantID, AbsenceID: audit.AbsenceID, FromStatus: audit.FromStatus, ToStatus: audit.ToStatus,
		ActorID: audit.ActorID, Note: audit.Note, ChangedAt: audit.ChangedAt,
		TypeChange: (*workforce.AbsenceTypeChange)(audit.TypeChange),
	})
	if err != nil {
		return absenceWriteError("create staff absence audit", err)
	}
	audit.ID = recorded.ID
	audit.TenantID = recorded.TenantID
	audit.ChangedAt = recorded.ChangedAt
	return nil
}

// --- mapping ---

func absenceToWorkforce(entity *StaffAbsence) workforce.StaffAbsence {
	return workforce.StaffAbsence{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, AbsenceType: entity.AbsenceType,
		AbsenceTypeID: entity.AbsenceTypeID, DateStart: entity.DateStart.String(), DateEnd: entity.DateEnd.String(),
		HalfDay: entity.HalfDay, StartHalfDay: entity.StartHalfDay, EndHalfDay: entity.EndHalfDay, Note: entity.Note,
		Status: entity.Status, ApprovedBy: entity.ApprovedBy, ApprovedAt: entity.ApprovedAt, CreatedBy: entity.CreatedBy,
		WorkingDays: entity.WorkingDays, DecisionNote: entity.DecisionNote, RequestedAt: entity.RequestedAt,
		SubstituteStaffID: entity.SubstituteStaffID, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func absenceToLegacy(value workforce.StaffAbsence) *StaffAbsence {
	entity := &StaffAbsence{}
	applyAbsenceToLegacy(entity, value)
	return entity
}

func applyAbsenceToLegacy(entity *StaffAbsence, value workforce.StaffAbsence) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.AbsenceType = value.AbsenceType
	entity.AbsenceTypeID = value.AbsenceTypeID
	entity.DateStart = timezone.Date(value.DateStart)
	entity.DateEnd = timezone.Date(value.DateEnd)
	entity.HalfDay = value.HalfDay
	entity.StartHalfDay = value.StartHalfDay
	entity.EndHalfDay = value.EndHalfDay
	entity.Note = value.Note
	entity.Status = value.Status
	entity.ApprovedBy = value.ApprovedBy
	entity.ApprovedAt = value.ApprovedAt
	entity.CreatedBy = value.CreatedBy
	entity.WorkingDays = value.WorkingDays
	entity.DecisionNote = value.DecisionNote
	entity.RequestedAt = value.RequestedAt
	entity.SubstituteStaffID = value.SubstituteStaffID
}

// absenceWriteError keeps the legacy write error shape. Validation failures
// surface bare, as the row's own Validate() did, so callers that compare the
// reason text keep seeing it; everything else is a DatabaseError whose chain
// still reaches the driver error of a rejected duplicate.
func absenceWriteError(op string, err error) error {
	if errors.Is(err, workforce.ErrInvalidStaffAbsence) || errors.Is(err, workforce.ErrAbsenceTypeInvalid) {
		return errors.New(err.Error())
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}
