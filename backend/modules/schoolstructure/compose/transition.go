package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The grade-transition commands run inside the workflow's tenant
// transaction; they take the tenant from context like the ledger commands
// and never open a transaction of their own.

func (e engine) FindTransition(ctx context.Context, id int64, lock string) (schoolstructure.Transition, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	value, err := e.service.FindTransition(ctx, tenantID.Int64(), id, lock)
	return transitionToPublic(value), mapTransitionError(err)
}

func (e engine) ListTransitions(ctx context.Context, filter schoolstructure.TransitionFilter) ([]schoolstructure.Transition, int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	values, total, err := e.service.ListTransitions(ctx, tenantID.Int64(), domain.TransitionFilter(filter))
	if err != nil {
		return nil, 0, mapTransitionError(err)
	}
	result := make([]schoolstructure.Transition, 0, len(values))
	for _, value := range values {
		result = append(result, transitionToPublic(value))
	}
	return result, total, nil
}

func (e engine) ListTransitionHistory(ctx context.Context, transitionID int64) ([]schoolstructure.TransitionHistoryEntry, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	values, err := e.service.ListHistory(ctx, tenantID.Int64(), transitionID)
	if err != nil {
		return nil, mapTransitionError(err)
	}
	result := make([]schoolstructure.TransitionHistoryEntry, 0, len(values))
	for _, value := range values {
		result = append(result, schoolstructure.TransitionHistoryEntry(value))
	}
	return result, nil
}

func (e engine) ListTransitionClassTeacherLedger(ctx context.Context, transitionID int64) ([]schoolstructure.TransitionClassTeacherEntry, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	values, err := e.service.ListClassTeacherLedger(ctx, tenantID.Int64(), transitionID)
	if err != nil {
		return nil, mapTransitionError(err)
	}
	result := make([]schoolstructure.TransitionClassTeacherEntry, 0, len(values))
	for _, value := range values {
		result = append(result, schoolstructure.TransitionClassTeacherEntry(value))
	}
	return result, nil
}

func (e engine) ListTransitionClassListLedger(ctx context.Context, transitionID int64) ([]schoolstructure.TransitionClassListEntry, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	values, err := e.service.ListClassListLedger(ctx, tenantID.Int64(), transitionID)
	if err != nil {
		return nil, mapTransitionError(err)
	}
	result := make([]schoolstructure.TransitionClassListEntry, 0, len(values))
	for _, value := range values {
		result = append(result, schoolstructure.TransitionClassListEntry(value))
	}
	return result, nil
}

func (e engine) CreateTransition(ctx context.Context, draft schoolstructure.TransitionDraft) (schoolstructure.Transition, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	value, err := e.service.CreateTransition(ctx, tenantID.Int64(), domain.TransitionDraft{
		AcademicYear: draft.AcademicYear, Notes: draft.Notes, CreatedBy: draft.CreatedBy, Mappings: mappingInputs(draft.Mappings),
	})
	return transitionToPublic(value), mapTransitionError(err)
}

func (e engine) UpdateTransition(ctx context.Context, update schoolstructure.TransitionUpdate) (schoolstructure.Transition, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, err
	}
	patch := domain.TransitionUpdate{ID: update.ID, AcademicYear: update.AcademicYear, Notes: update.Notes}
	if update.Mappings != nil {
		patch.Mappings = mappingInputs(update.Mappings)
		if patch.Mappings == nil {
			patch.Mappings = []domain.TransitionMappingInput{}
		}
	}
	value, err := e.service.UpdateTransition(ctx, tenantID.Int64(), patch)
	return transitionToPublic(value), mapTransitionError(err)
}

func (e engine) DeleteTransition(ctx context.Context, id int64) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return mapTransitionError(e.service.DeleteTransition(ctx, tenantID.Int64(), id))
}

func (e engine) LockTransitions(ctx context.Context) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("school structure: the transition gate requires a tenant transaction")
	}
	return mapTransitionError(e.service.LockTransitions(ctx, tenantID.Int64()))
}

func (e engine) LockLatestAppliedTransition(ctx context.Context) (schoolstructure.Transition, bool, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return schoolstructure.Transition{}, false, err
	}
	value, found, err := e.service.LockLatestApplied(ctx, tenantID.Int64())
	return transitionToPublic(value), found, mapTransitionError(err)
}

func (e engine) MarkTransitionApplied(ctx context.Context, id, accountID int64, at time.Time, rosterBaselineInstanceID *int64) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return mapTransitionError(e.service.MarkApplied(ctx, tenantID.Int64(), id, accountID, at, rosterBaselineInstanceID))
}

func (e engine) MarkTransitionReverted(ctx context.Context, id, accountID int64, at time.Time) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	return mapTransitionError(e.service.MarkReverted(ctx, tenantID.Int64(), id, accountID, at))
}

func (e engine) AppendTransitionHistory(ctx context.Context, entries []schoolstructure.TransitionHistoryEntry) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	values := make([]domain.TransitionHistoryEntry, 0, len(entries))
	for _, entry := range entries {
		values = append(values, domain.TransitionHistoryEntry(entry))
	}
	return mapTransitionError(e.service.AppendHistory(ctx, tenantID.Int64(), values))
}

func (e engine) AppendTransitionClassTeacherLedger(ctx context.Context, entries []schoolstructure.TransitionClassTeacherEntry) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	values := make([]domain.TransitionClassTeacherEntry, 0, len(entries))
	for _, entry := range entries {
		values = append(values, domain.TransitionClassTeacherEntry(entry))
	}
	return mapTransitionError(e.service.AppendClassTeacherLedger(ctx, tenantID.Int64(), values))
}

func (e engine) AppendTransitionClassListLedger(ctx context.Context, entries []schoolstructure.TransitionClassListEntry) error {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return err
	}
	values := make([]domain.TransitionClassListEntry, 0, len(entries))
	for _, entry := range entries {
		values = append(values, domain.TransitionClassListEntry(entry))
	}
	return mapTransitionError(e.service.AppendClassListLedger(ctx, tenantID.Int64(), values))
}

func mappingInputs(inputs []schoolstructure.TransitionMappingInput) []domain.TransitionMappingInput {
	if inputs == nil {
		return nil
	}
	result := make([]domain.TransitionMappingInput, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, domain.TransitionMappingInput(input))
	}
	return result
}

func transitionToPublic(value domain.Transition) schoolstructure.Transition {
	result := schoolstructure.Transition{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		AcademicYear: value.AcademicYear, Status: value.Status, AppliedAt: value.AppliedAt, AppliedBy: value.AppliedBy,
		RevertedAt: value.RevertedAt, RevertedBy: value.RevertedBy, CreatedBy: value.CreatedBy, Notes: value.Notes,
		RosterBaselineInstanceID: value.RosterBaselineInstanceID,
	}
	if value.Mappings != nil {
		result.Mappings = make([]schoolstructure.TransitionMapping, 0, len(value.Mappings))
		for _, mapping := range value.Mappings {
			result.Mappings = append(result.Mappings, schoolstructure.TransitionMapping(mapping))
		}
	}
	return result
}

func mapTransitionError(err error) error {
	var invalid *domain.InvalidTransitionError
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrTransitionNotFound):
		return schoolstructure.ErrTransitionNotFound
	case errors.Is(err, domain.ErrTransitionStateConflict):
		return schoolstructure.ErrTransitionStateConflict
	case errors.As(err, &invalid):
		return &schoolstructure.InvalidTransitionError{Reason: invalid.Reason}
	}
	return err
}
