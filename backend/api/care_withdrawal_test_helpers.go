//go:build testhooks

package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// SeedPendingCareWithdrawal stores completion as the child's one pending
// withdrawal task through the Care Plan owner behind repo, and writes the
// stored id and timestamps back onto completion. It is the fixture the
// retained contract's UpsertPending served the suites with; production
// creates the task inside Care Plan's withdrawal flow (#4082).
func SeedPendingCareWithdrawal(
	ctx context.Context, repo CareWithdrawalRecords, completion *careplan.WithdrawalCompletion,
) error {
	adapter := repo
	if completion.StudentID == nil || *completion.StudentID <= 0 {
		return errors.New("care withdrawal completion requires a student")
	}
	carePlan, err := adapter.owner()
	if err != nil {
		return err
	}
	completion.State = careplan.WithdrawalStatePending
	if completion.SourceOfferings == nil {
		completion.SourceOfferings = []careplan.CareExitSourceOffering{}
	}
	stored, err := carePlan.UpsertPendingWithdrawal(ctx, careplan.WithdrawalCompletion{
		ID: completion.ID, TenantID: completion.TenantID, CreatedAt: completion.CreatedAt, UpdatedAt: completion.UpdatedAt,
		StudentID: completion.StudentID, FirstBookinglessDay: completion.FirstBookinglessDay,
		Trigger: completion.Trigger, SourceAdjustmentID: completion.SourceAdjustmentID,
		SourceRequestChildID: completion.SourceRequestChildID, WithdrawalConfirmedBy: completion.WithdrawalConfirmedBy,
		WithdrawalConfirmedRole: completion.WithdrawalConfirmedRole, WithdrawalConfirmedAt: completion.WithdrawalConfirmedAt,
		SourceOfferings: completion.SourceOfferings, State: completion.State, Outcome: completion.Outcome,
		ObsoleteReason: completion.ObsoleteReason, ResolvedBy: completion.ResolvedBy, ResolvedAt: completion.ResolvedAt,
	})
	if err != nil {
		return fmt.Errorf("upsert pending care withdrawal completion: %w", err)
	}
	completion.ID, completion.TenantID = stored.ID, stored.TenantID
	completion.CreatedAt, completion.UpdatedAt = stored.CreatedAt, stored.UpdatedAt
	return nil
}
