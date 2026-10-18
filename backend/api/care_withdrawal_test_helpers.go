//go:build testhooks

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
)

// SeedPendingCareWithdrawal stores completion as the child's one pending
// withdrawal task through the Care Plan owner behind repo, and writes the
// stored id and timestamps back onto completion. It is the fixture the
// retained contract's UpsertPending served the suites with; production
// creates the task inside Care Plan's withdrawal flow (#4082).
func SeedPendingCareWithdrawal(
	ctx context.Context, repo userModels.CareWithdrawalCompletionRepository, completion *userModels.CareWithdrawalCompletion,
) error {
	adapter, ok := repo.(careWithdrawalCompletionRepository)
	if !ok {
		return fmt.Errorf("seed pending care withdrawal: %T is not the Care Plan adapter", repo)
	}
	if completion.StudentID == nil || *completion.StudentID <= 0 {
		return errors.New("care withdrawal completion requires a student")
	}
	carePlan, err := adapter.owner()
	if err != nil {
		return err
	}
	completion.State = userModels.CareWithdrawalStatePending
	if completion.SourceOfferings == nil {
		completion.SourceOfferings = []userModels.CareExitSourceOffering{}
	}
	offerings, err := json.Marshal(completion.SourceOfferings)
	if err != nil {
		return fmt.Errorf("encode care withdrawal source offerings: %w", err)
	}
	stored, err := carePlan.UpsertPendingWithdrawal(ctx, careplan.WithdrawalCompletion{
		ID: completion.ID, TenantID: completion.TenantID, CreatedAt: completion.CreatedAt, UpdatedAt: completion.UpdatedAt,
		StudentID: completion.StudentID, FirstBookinglessDay: careplan.Date(completion.FirstBookinglessDay),
		Trigger: completion.Trigger, SourceAdjustmentID: completion.SourceAdjustmentID,
		SourceRequestChildID: completion.SourceRequestChildID, WithdrawalConfirmedBy: completion.WithdrawalConfirmedBy,
		WithdrawalConfirmedRole: completion.WithdrawalConfirmedRole, WithdrawalConfirmedAt: completion.WithdrawalConfirmedAt,
		SourceOfferings: offerings, State: completion.State, Outcome: completion.Outcome,
		ObsoleteReason: completion.ObsoleteReason, ResolvedBy: completion.ResolvedBy, ResolvedAt: completion.ResolvedAt,
	})
	if err != nil {
		return fmt.Errorf("upsert pending care withdrawal completion: %w", err)
	}
	completion.ID, completion.TenantID = stored.ID, stored.TenantID
	completion.CreatedAt, completion.UpdatedAt = stored.CreatedAt, stored.UpdatedAt
	return nil
}
