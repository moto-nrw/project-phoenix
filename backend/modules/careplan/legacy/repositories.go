package legacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// NewParentRuntime preserves ambient transaction joining for the legacy
// parent repositories while keeping transaction-runtime details at this seam.
func NewParentRuntime(db *bun.DB) parentRepo.Runtime {
	return parentRepo.RuntimeFunc(func(ctx context.Context) bun.IDB { return base.GetDB(ctx, db) })
}

// NewAmbientDatabase resolves the caller's legacy or tenant transaction.
func NewAmbientDatabase(db *bun.DB) carePlanCompose.AmbientDatabase {
	if db == nil {
		panic("care plan legacy: database is required")
	}
	return func(ctx context.Context) bun.IDB { return base.GetDB(ctx, db) }
}

// careOfferingCarePlanRepository preserves the enrollment model contract over
// the Care Plan owner capability. It contains no persistence of its own.
type careOfferingCarePlanRepository struct{ carePlan careplan.Capability }

var _ enrollmentModels.CareOfferingRepository = careOfferingCarePlanRepository{}

// NewCareOfferingRepository adapts the owner capability to the legacy model contract.
func NewCareOfferingRepository(capability careplan.Capability) enrollmentModels.CareOfferingRepository {
	return careOfferingCarePlanRepository{carePlan: capability}
}

func (r careOfferingCarePlanRepository) Create(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil {
		return errors.New("CareOffering cannot be nil or zero value")
	}
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fields, err := careOfferingFieldsFromLegacy(offering)
	if err != nil {
		return fmt.Errorf("encode care offering: %w", err)
	}
	created, err := r.carePlan.CreateCareOffering(ctx, careplan.CreateCareOffering{CareOfferingFields: fields})
	if err != nil {
		return fmt.Errorf("failed to create care offering: %w", err)
	}
	return applyCareOfferingToLegacy(offering, created)
}

func (r careOfferingCarePlanRepository) FindByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	value, err := r.carePlan.FindCareOffering(ctx, id)
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return nil, fmt.Errorf("care offering %d not found: %w", id, usersRepo.NotFoundError("find care offering"))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to find care offering: %w", err)
	}
	return careOfferingToLegacy(value)
}

func (r careOfferingCarePlanRepository) Update(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil {
		return errors.New("CareOffering cannot be nil or zero value")
	}
	if err := offering.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	fields, err := careOfferingFieldsFromLegacy(offering)
	if err != nil {
		return fmt.Errorf("encode care offering: %w", err)
	}
	updated, err := r.carePlan.UpdateCareOffering(ctx, careplan.UpdateCareOffering{
		ID: offering.ID, CareOfferingFields: fields,
	})
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return fmt.Errorf("care offering %d not found", offering.ID)
	}
	if err != nil {
		return fmt.Errorf("failed to update care offering: %w", err)
	}
	return applyCareOfferingToLegacy(offering, updated)
}

func (r careOfferingCarePlanRepository) Delete(ctx context.Context, id int64) error {
	err := r.carePlan.DeleteCareOffering(ctx, id)
	if errors.Is(err, careplan.ErrCareOfferingNotFound) {
		return fmt.Errorf("care offering %d not found", id)
	}
	if err != nil {
		return fmt.Errorf("failed to delete care offering: %w", err)
	}
	return nil
}

func (r careOfferingCarePlanRepository) ReplaceAutoAddTriggers(ctx context.Context, id int64, triggers []int64) error {
	if err := r.carePlan.ReplaceAutoAddTriggers(ctx, id, triggers); err != nil {
		return fmt.Errorf("failed to replace care offering auto triggers: %w", err)
	}
	return nil
}

func (r careOfferingCarePlanRepository) ListByTenant(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	return r.list(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderCatalog}, "failed to list care offerings")
}

func (r careOfferingCarePlanRepository) ListByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return r.list(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, Order: careplan.OfferingOrderCatalog}, "failed to list care offerings by phase")
}

func (r careOfferingCarePlanRepository) ListByIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollmentModels.CareOffering{}, nil
	}
	return r.list(ctx, careplan.CareOfferingFilter{IDs: ids, Order: careplan.OfferingOrderCatalog}, "failed to list care offerings by ids")
}

func (r careOfferingCarePlanRepository) ListByIDsForUpdate(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollmentModels.CareOffering{}, nil
	}
	return r.list(ctx, careplan.CareOfferingFilter{IDs: ids, LockForUpdate: true, Order: careplan.OfferingOrderID}, "failed to lock care offerings by ids")
}

func (r careOfferingCarePlanRepository) ListByActivityGroupIDs(ctx context.Context, ids []int64) ([]*enrollmentModels.CareOffering, error) {
	if len(ids) == 0 {
		return []*enrollmentModels.CareOffering{}, nil
	}
	return r.list(ctx, careplan.CareOfferingFilter{ActivityGroupIDs: ids, Order: careplan.OfferingOrderID}, "failed to list care offerings by activity groups")
}

func (r careOfferingCarePlanRepository) CountByPhaseID(ctx context.Context, phaseID int64) (int, error) {
	count, err := r.carePlan.CountCareOfferingsByPhase(ctx, phaseID)
	if err != nil {
		return 0, fmt.Errorf("failed to count care offerings by phase: %w", err)
	}
	return count, nil
}

func (r careOfferingCarePlanRepository) ListActiveByPhase(ctx context.Context, phaseID int64) ([]*enrollmentModels.CareOffering, error) {
	return r.list(ctx, careplan.CareOfferingFilter{PhaseIDs: []int64{phaseID}, ActiveOnly: true, Order: careplan.OfferingOrderCatalog}, "failed to list active offerings by phase")
}

func (r careOfferingCarePlanRepository) ListActiveByPhaseIDs(ctx context.Context, phaseIDs []int64) ([]*enrollmentModels.CareOffering, error) {
	if len(phaseIDs) == 0 {
		return []*enrollmentModels.CareOffering{}, nil
	}
	return r.list(ctx, careplan.CareOfferingFilter{PhaseIDs: phaseIDs, ActiveOnly: true, Order: careplan.OfferingOrderPhaseCatalog}, "failed to list active care offerings by phase ids")
}

func (r careOfferingCarePlanRepository) list(ctx context.Context, filter careplan.CareOfferingFilter, message string) ([]*enrollmentModels.CareOffering, error) {
	values, err := r.carePlan.ListCareOfferings(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", message, err)
	}
	result := make([]*enrollmentModels.CareOffering, 0, len(values))
	for _, value := range values {
		offering, convertErr := careOfferingToLegacy(value)
		if convertErr != nil {
			return nil, fmt.Errorf("%s: %w", message, convertErr)
		}
		result = append(result, offering)
	}
	return result, nil
}

func careOfferingFieldsFromLegacy(offering *enrollmentModels.CareOffering) (careplan.CareOfferingFields, error) {
	availabilityRule, err := marshalJSON(offering.AvailabilityRule)
	if err != nil {
		return careplan.CareOfferingFields{}, err
	}
	return careplan.CareOfferingFields{
		PhaseID: offering.PhaseID, ActivityGroupID: offering.ActivityGroupID, Name: offering.Name, Description: offering.Description,
		DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays,
		IncludesHolidayCare: offering.IncludesHolidayCare, IncludesLunch: offering.IncludesLunch,
		Capacity: offering.Capacity, PriceCents: offering.PriceCents, IsActive: offering.IsActive, IsRequired: offering.IsRequired,
		CountsAsCare: offering.CountsAsCare, AutoAddGradeLevels: offering.AutoAddGradeLevels,
		AvailabilityRule: availabilityRule, SortOrder: offering.SortOrder,
		SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule, PickupTimes: offering.PickupTimes,
		AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs,
	}, nil
}

func careOfferingToLegacy(value careplan.CareOffering) (*enrollmentModels.CareOffering, error) {
	offering := new(enrollmentModels.CareOffering)
	return offering, applyCareOfferingToLegacy(offering, value)
}

func applyCareOfferingToLegacy(target *enrollmentModels.CareOffering, value careplan.CareOffering) error {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.TenantID = value.TenantID
	target.PhaseID = value.PhaseID
	target.ActivityGroupID = value.ActivityGroupID
	target.Name = value.Name
	target.Description = value.Description
	target.DaysOfWeekMode = value.DaysOfWeekMode
	target.AvailableDays = value.AvailableDays
	target.IncludesHolidayCare = value.IncludesHolidayCare
	target.IncludesLunch = value.IncludesLunch
	target.Capacity = value.Capacity
	target.PriceCents = value.PriceCents
	target.IsActive = value.IsActive
	target.IsRequired = value.IsRequired
	target.CountsAsCare = value.CountsAsCare
	target.CountsAsCareSet = true
	target.AutoAddGradeLevels = value.AutoAddGradeLevels
	if err := unmarshalOptionalJSON(value.AvailabilityRule, &target.AvailabilityRule); err != nil {
		return fmt.Errorf("decode care offering availability rule: %w", err)
	}
	target.SortOrder = value.SortOrder
	target.SelectionGroup = value.SelectionGroup
	target.SelectionRule = value.SelectionRule
	target.PickupTimes = value.PickupTimes
	target.AutoAddTriggerOfferingIDs = value.AutoAddTriggerOfferingIDs
	return nil
}

type offeringChangeCarePlanRepository struct {
	carePlan careplan.Capability
	people   peopledirectory.Query
}

var _ enrollmentModels.OfferingChangeRequestRepository = (*offeringChangeCarePlanRepository)(nil)

// NewOfferingChangeRepository adapts the owner capability and person search
// to the legacy request repository contract.
func NewOfferingChangeRepository(capability careplan.Capability, people peopledirectory.Query) enrollmentModels.OfferingChangeRequestRepository {
	return &offeringChangeCarePlanRepository{carePlan: capability, people: people}
}

func (r *offeringChangeCarePlanRepository) Create(ctx context.Context, row *enrollmentModels.OfferingChangeRequest) error {
	if row == nil {
		return errors.New("OfferingChangeRequest cannot be nil or zero value")
	}
	input, err := offeringChangeToPublic(row)
	if err != nil {
		return usersRepo.WrapError("create", err)
	}
	created, err := r.carePlan.CreateOfferingChange(ctx, input)
	if err != nil {
		return usersRepo.WrapError("create", err)
	}
	return applyOfferingChangeToLegacy(row, created)
}

func (r *offeringChangeCarePlanRepository) FindByID(ctx context.Context, rawID any) (*enrollmentModels.OfferingChangeRequest, error) {
	id, err := membershipID(rawID)
	if err != nil {
		return nil, usersRepo.WrapError("find by id", err)
	}
	return r.find(ctx, id, false, "find by id")
}

func (r *offeringChangeCarePlanRepository) GetPendingForStudent(ctx context.Context, studentID int64) (*enrollmentModels.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, enrollmentModels.OfferingChangeQueueFilters{StudentID: studentID}, []string{enrollmentModels.OfferingChangeStatusPending}, careplan.ChangeOrderCreated)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "get pending offering change request", Err: err}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (r *offeringChangeCarePlanRepository) ListByStudent(ctx context.Context, studentID int64) ([]*enrollmentModels.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, enrollmentModels.OfferingChangeQueueFilters{StudentID: studentID}, nil, careplan.ChangeOrderReviewed)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list offering change requests by student", Err: err}
	}
	return rows, nil
}

func (r *offeringChangeCarePlanRepository) ListPendingForTenant(ctx context.Context, filters enrollmentModels.OfferingChangeQueueFilters) ([]*enrollmentModels.OfferingChangeRequest, error) {
	rows, err := r.list(ctx, filters, []string{enrollmentModels.OfferingChangeStatusPending}, careplan.ChangeOrderCreated)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list pending offering change requests", Err: err}
	}
	return rows, nil
}

func (r *offeringChangeCarePlanRepository) ListDecidedForTenant(ctx context.Context, filters enrollmentModels.OfferingChangeQueueFilters) ([]*enrollmentModels.OfferingChangeRequest, error) {
	statuses := []string{enrollmentModels.OfferingChangeStatusApproved, enrollmentModels.OfferingChangeStatusRejected, enrollmentModels.OfferingChangeStatusWithdrawn}
	rows, err := r.list(ctx, filters, statuses, careplan.ChangeOrderUpdated)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list decided offering change requests", Err: err}
	}
	return rows, nil
}

func (r *offeringChangeCarePlanRepository) FindByIDForUpdate(ctx context.Context, id int64) (*enrollmentModels.OfferingChangeRequest, error) {
	row, err := r.find(ctx, id, true, "find offering change request for update")
	if errors.Is(err, careplan.ErrOfferingChangeNotFound) || modelBase.IsNoRows(err) {
		return nil, enrollmentModels.ErrOfferingChangeNotFound
	}
	return row, err
}

func (r *offeringChangeCarePlanRepository) UpdateEffectiveFrom(ctx context.Context, id int64, date enrollmentModels.OfferingChangeDate) error {
	return r.pendingError("update offering change effective date", r.carePlan.UpdateOfferingChangeEffectiveFrom(ctx, id, string(date)))
}

func (r *offeringChangeCarePlanRepository) UpdateApprovedCompleteWithdrawal(ctx context.Context, id int64, complete bool) error {
	return r.pendingError("update approved complete-withdrawal result", r.carePlan.UpdateApprovedCompleteWithdrawal(ctx, id, complete))
}

func (r *offeringChangeCarePlanRepository) UpdatePending(ctx context.Context, id int64, payload map[string]any, date enrollmentModels.OfferingChangeDate, note *string) error {
	encoded, err := marshalJSON(payload)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update pending offering change request", Err: err}
	}
	err = r.carePlan.UpdatePendingOfferingChange(ctx, careplan.UpdatePendingOfferingChange{ID: id, Payload: encoded, EffectiveFrom: string(date), ParentNote: note})
	return r.pendingError("update pending offering change request", err)
}

func (r *offeringChangeCarePlanRepository) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	err := r.carePlan.DecideOfferingChange(ctx, careplan.DecideOfferingChange{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied})
	return r.pendingError("decide offering change request", err)
}

func (r *offeringChangeCarePlanRepository) UpdateDecisionSnapshot(ctx context.Context, id int64, snapshot *enrollmentModels.OfferingChangeDecisionSnapshot) error {
	encoded, err := marshalJSON(snapshot)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update offering change decision snapshot", Err: err}
	}
	err = r.carePlan.UpdateOfferingChangeSnapshot(ctx, id, encoded)
	if err != nil {
		return &modelBase.DatabaseError{Op: "update offering change decision snapshot", Err: err}
	}
	return nil
}

func (r *offeringChangeCarePlanRepository) find(ctx context.Context, id int64, lock bool, op string) (*enrollmentModels.OfferingChangeRequest, error) {
	value, err := r.carePlan.FindOfferingChange(ctx, id, lock)
	if errors.Is(err, careplan.ErrOfferingChangeNotFound) {
		return nil, usersRepo.NotFoundError(op)
	}
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	row := new(enrollmentModels.OfferingChangeRequest)
	if err := applyOfferingChangeToLegacy(row, value); err != nil {
		return nil, &modelBase.DatabaseError{Op: op, Err: err}
	}
	return row, nil
}

func (r *offeringChangeCarePlanRepository) list(ctx context.Context, filters enrollmentModels.OfferingChangeQueueFilters, statuses []string, order string) ([]*enrollmentModels.OfferingChangeRequest, error) {
	studentIDs, err := r.searchStudentIDs(ctx, filters)
	if err != nil {
		return nil, err
	}
	values, err := r.carePlan.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{
		StudentID: filters.StudentID, StudentIDs: studentIDs, Statuses: statuses,
		UrgentOnly: filters.UrgentOnly, UrgentDate: filters.UrgentDate,
		BeforeInstant: filters.BeforeInstant, BeforeID: filters.BeforeID, Limit: filters.Limit, Order: order,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]*enrollmentModels.OfferingChangeRequest, 0, len(values))
	for _, value := range values {
		row := new(enrollmentModels.OfferingChangeRequest)
		if err := applyOfferingChangeToLegacy(row, value); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (r *offeringChangeCarePlanRepository) searchStudentIDs(ctx context.Context, filters enrollmentModels.OfferingChangeQueueFilters) ([]int64, error) {
	if strings.TrimSpace(filters.Search) == "" {
		return filters.StudentIDs, nil
	}
	if r.people == nil {
		return nil, errors.New("offering change student search requires the People Directory")
	}
	personIDs, err := r.searchPersonIDs(ctx, filters.Search)
	if err != nil {
		return nil, err
	}
	students, err := r.people.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	allowed := make(map[int64]struct{}, len(filters.StudentIDs))
	for _, id := range filters.StudentIDs {
		allowed[id] = struct{}{}
	}
	result := make([]int64, 0)
	for _, student := range students {
		_, matches := personIDs[student.PersonID]
		_, included := allowed[student.ID]
		if matches && (len(allowed) == 0 || included) {
			result = append(result, student.ID)
		}
	}
	return result, nil
}

func (r *offeringChangeCarePlanRepository) searchPersonIDs(ctx context.Context, name string) (map[int64]struct{}, error) {
	result := map[int64]struct{}{}
	for page := 1; ; page++ {
		people, err := r.people.SearchPersons(ctx, peopledirectory.PersonFilter{FullNameContains: name, Page: page, PageSize: 100})
		if err != nil {
			return nil, err
		}
		for _, person := range people {
			result[person.ID] = struct{}{}
		}
		if len(people) < 100 {
			return result, nil
		}
	}
}

func (r *offeringChangeCarePlanRepository) pendingError(op string, err error) error {
	if errors.Is(err, careplan.ErrOfferingChangeNotPending) {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
	if err != nil {
		return &modelBase.DatabaseError{Op: op, Err: err}
	}
	return nil
}

func offeringChangeToPublic(row *enrollmentModels.OfferingChangeRequest) (careplan.OfferingChangeRequest, error) {
	payload, err := marshalJSON(row.Payload)
	if err != nil {
		return careplan.OfferingChangeRequest{}, fmt.Errorf("encode offering change payload: %w", err)
	}
	decisionSnapshot, err := marshalJSON(row.DecisionSnapshot)
	if err != nil {
		return careplan.OfferingChangeRequest{}, fmt.Errorf("encode offering change decision snapshot: %w", err)
	}
	return careplan.OfferingChangeRequest{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, RequestChildID: row.RequestChildID, SubmittedBy: row.SubmittedBy,
		CompleteWithdrawalConfirmed: row.CompleteWithdrawalConfirmed,
		WithdrawalConfirmedBy:       row.WithdrawalConfirmedBy, WithdrawalConfirmedAt: row.WithdrawalConfirmedAt,
		ApprovedCompleteWithdrawal: row.ApprovedCompleteWithdrawal, Payload: payload,
		EffectiveFrom: string(row.EffectiveFrom), ParentNote: row.ParentNote, Status: row.Status,
		DecisionReason: row.DecisionReason, DecisionSnapshot: decisionSnapshot,
		ReviewedBy: row.ReviewedBy, ReviewedAt: row.ReviewedAt, AppliedAt: row.AppliedAt,
	}, nil
}

func applyOfferingChangeToLegacy(target *enrollmentModels.OfferingChangeRequest, value careplan.OfferingChangeRequest) error {
	target.ID = value.ID
	target.TenantID = value.TenantID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.StudentID = value.StudentID
	target.RequestChildID = value.RequestChildID
	target.SubmittedBy = value.SubmittedBy
	target.CompleteWithdrawalConfirmed = value.CompleteWithdrawalConfirmed
	target.WithdrawalConfirmedBy = value.WithdrawalConfirmedBy
	target.WithdrawalConfirmedAt = value.WithdrawalConfirmedAt
	target.ApprovedCompleteWithdrawal = value.ApprovedCompleteWithdrawal
	if err := unmarshalOptionalJSON(value.Payload, &target.Payload); err != nil {
		return fmt.Errorf("decode offering change payload: %w", err)
	}
	target.EffectiveFrom = enrollmentModels.OfferingChangeDate(value.EffectiveFrom)
	target.ParentNote = value.ParentNote
	target.Status = value.Status
	target.DecisionReason = value.DecisionReason
	if err := unmarshalOptionalJSON(value.DecisionSnapshot, &target.DecisionSnapshot); err != nil {
		return fmt.Errorf("decode offering change decision snapshot: %w", err)
	}
	target.ReviewedBy = value.ReviewedBy
	target.ReviewedAt = value.ReviewedAt
	target.AppliedAt = value.AppliedAt
	return nil
}

func marshalJSON(value any) (json.RawMessage, error) {
	if value == nil {
		return nil, nil
	}
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func unmarshalOptionalJSON(data json.RawMessage, target any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, target)
}

func membershipID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", id)
	}
}
