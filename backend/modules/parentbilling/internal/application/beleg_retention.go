package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BelegDeletionLogRetentionYears keeps an entry of the deletion log as long
// as the deletion log of a school.
const BelegDeletionLogRetentionYears = 3

// BelegRetentionStore persists the retention. Every method runs in the
// caller's administrative transaction; organizationID 0 means every Träger.
type BelegRetentionStore interface {
	BelegFacts(ctx context.Context, organizationID int64) ([]parentbilling.BelegFact, error)
	BelegEvaluationDue(ctx context.Context, today calendar.Date) (bool, error)
	ActiveInstructions(ctx context.Context, organizationID int64) ([]parentbilling.RetentionInstruction, error)
	ActiveSuspensions(ctx context.Context, organizationID int64) ([]parentbilling.RetentionSuspension, error)
	InsertInstruction(ctx context.Context, organizationID, operatorID int64, instruction parentbilling.NewRetentionInstruction) (parentbilling.RetentionInstruction, error)
	InsertSuspension(ctx context.Context, organizationID, operatorID int64, suspension parentbilling.NewRetentionSuspension) (parentbilling.RetentionSuspension, error)
	LiftSuspension(ctx context.Context, organizationID, suspensionID, operatorID int64) error
	SaveBelegRecords(ctx context.Context, organizationID int64, today calendar.Date, records []parentbilling.BelegRecord) error
	// DeleteDueBelege deletes the due Belege of parent-billing in the order
	// their cascades need: debits first, mandates last, then the runs left
	// without a Beleg.
	DeleteDueBelege(ctx context.Context, today calendar.Date) (int, error)
	DueContracts(ctx context.Context, today calendar.Date) ([]parentbilling.BelegRef, error)
	LogContractDeletions(ctx context.Context, organizationID int64, today calendar.Date, contractIDs []int64) error
	DeleteExpiredBelegDeletions(ctx context.Context, years int) (int, error)
	BelegRetentionOverview(ctx context.Context, organizationID int64, today calendar.Date) (parentbilling.BelegRetentionOverview, error)
}

// BelegRetention serves parentbilling.BelegRetention (#4720, ADR 0184).
type BelegRetention struct {
	store     BelegRetentionStore
	contracts parentbilling.RetainedContracts
	today     func() calendar.Date
}

// NewBelegRetention builds the retention; a nil today uses the Berlin day.
func NewBelegRetention(store BelegRetentionStore, contracts parentbilling.RetainedContracts, today func() calendar.Date) *BelegRetention {
	if store == nil || contracts == nil {
		panic("parent billing: the beleg retention needs its store and the contracts")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &BelegRetention{store: store, contracts: contracts, today: today}
}

// Overview answers the instructions, suspensions, counts, clarification
// list and deletions of one Träger.
func (r *BelegRetention) Overview(ctx context.Context, organizationID int64) (parentbilling.BelegRetentionOverview, error) {
	overview, err := r.store.BelegRetentionOverview(ctx, organizationID, r.today())
	if err != nil {
		return parentbilling.BelegRetentionOverview{}, err
	}
	if overview.Instructions, err = r.store.ActiveInstructions(ctx, organizationID); err != nil {
		return parentbilling.BelegRetentionOverview{}, err
	}
	if overview.Suspensions, err = r.store.ActiveSuspensions(ctx, organizationID); err != nil {
		return parentbilling.BelegRetentionOverview{}, err
	}
	return overview, nil
}

// RecordInstruction records the instruction of a Träger for a type and
// evaluates its Belege at once, so the overview shows the effect. It never
// deletes: that is the hourly job's.
func (r *BelegRetention) RecordInstruction(ctx context.Context, organizationID, operatorID int64, input parentbilling.NewRetentionInstruction) (parentbilling.RetentionInstruction, error) {
	input.LegalBasis, input.InstructedBy = strings.TrimSpace(input.LegalBasis), strings.TrimSpace(input.InstructedBy)
	input.InstructionReference = strings.TrimSpace(input.InstructionReference)
	if err := ValidateRetentionInstruction(input); err != nil {
		return parentbilling.RetentionInstruction{}, err
	}
	if input.InstructedOn.After(r.today()) {
		return parentbilling.RetentionInstruction{}, fmt.Errorf("%w: the instruction cannot lie in the future", parentbilling.ErrBelegRetentionInvalid)
	}
	instruction, err := r.store.InsertInstruction(ctx, organizationID, operatorID, input)
	if err != nil {
		return parentbilling.RetentionInstruction{}, err
	}
	return instruction, r.evaluate(ctx, organizationID, r.today())
}

// RecordSuspension records a suspension and evaluates the Träger again.
func (r *BelegRetention) RecordSuspension(ctx context.Context, organizationID, operatorID int64, input parentbilling.NewRetentionSuspension) (parentbilling.RetentionSuspension, error) {
	input.Reason, input.Scope, input.Responsible = strings.TrimSpace(input.Reason), strings.TrimSpace(input.Scope), strings.TrimSpace(input.Responsible)
	if err := ValidateRetentionSuspension(input, r.today()); err != nil {
		return parentbilling.RetentionSuspension{}, err
	}
	suspension, err := r.store.InsertSuspension(ctx, organizationID, operatorID, input)
	if err != nil {
		return parentbilling.RetentionSuspension{}, err
	}
	return suspension, r.evaluate(ctx, organizationID, r.today())
}

// LiftSuspension ends a suspension and evaluates the Träger again.
func (r *BelegRetention) LiftSuspension(ctx context.Context, organizationID, suspensionID, operatorID int64) error {
	if err := r.store.LiftSuspension(ctx, organizationID, suspensionID, operatorID); err != nil {
		return err
	}
	return r.evaluate(ctx, organizationID, r.today())
}

// ApplyBelegRetention evaluates every Beleg once per day, then deletes the
// Belege whose instructed period ended and nothing of which is still kept.
// Without an instruction nothing is due, so the job deletes nothing.
func (r *BelegRetention) ApplyBelegRetention(ctx context.Context) (int, error) {
	today := r.today()
	due, err := r.store.BelegEvaluationDue(ctx, today)
	if err != nil || !due {
		return 0, err
	}
	if err := r.evaluate(ctx, 0, today); err != nil {
		return 0, err
	}
	deleted, err := r.store.DeleteDueBelege(ctx, today)
	if err != nil {
		return 0, err
	}
	// Contracts go last: their deletion would take their mandates and
	// debits, so only contracts without either are due.
	contracts, err := r.deleteDueContracts(ctx, today)
	if err != nil {
		return 0, err
	}
	logged, err := r.store.DeleteExpiredBelegDeletions(ctx, BelegDeletionLogRetentionYears)
	if err != nil {
		return 0, err
	}
	return deleted + contracts + logged, nil
}

// evaluate writes today's record of every Beleg of one Träger or of all.
func (r *BelegRetention) evaluate(ctx context.Context, organizationID int64, today calendar.Date) error {
	facts, err := r.store.BelegFacts(ctx, organizationID)
	if err != nil {
		return err
	}
	contracts, err := r.contracts.ContractRetentionFacts(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("parent billing: read contract retention facts: %w", err)
	}
	for _, contract := range contracts {
		facts = append(facts, parentbilling.BelegFact{
			OrganizationID: contract.OrganizationID, BelegType: parentbilling.BelegCareContract, BelegID: contract.ContractID,
			ConcludedOn: contract.ConcludedOn, Ended: contract.Ended, EndedOn: contract.EndedOn,
		})
	}
	instructions, err := r.store.ActiveInstructions(ctx, organizationID)
	if err != nil {
		return err
	}
	suspensions, err := r.store.ActiveSuspensions(ctx, organizationID)
	if err != nil {
		return err
	}
	records := decideBelegRecords(facts, instructions, suspensions, today)
	return r.store.SaveBelegRecords(ctx, organizationID, today, records)
}

type belegKey struct {
	organizationID int64
	belegType      string
}

// decideBelegRecords pairs each Beleg with the instruction of its type and
// the suspension that covers it: one for the Beleg itself before one for
// its whole type.
func decideBelegRecords(facts []parentbilling.BelegFact, instructions []parentbilling.RetentionInstruction,
	suspensions []parentbilling.RetentionSuspension, today calendar.Date,
) []parentbilling.BelegRecord {
	byType := make(map[belegKey]*parentbilling.RetentionInstruction, len(instructions))
	for i := range instructions {
		byType[belegKey{instructions[i].OrganizationID, instructions[i].BelegType}] = &instructions[i]
	}
	typeHolds := map[belegKey]*parentbilling.RetentionSuspension{}
	belegHolds := map[belegKey]map[int64]*parentbilling.RetentionSuspension{}
	for i := range suspensions {
		key := belegKey{suspensions[i].OrganizationID, suspensions[i].BelegType}
		if suspensions[i].BelegID == 0 {
			if typeHolds[key] == nil {
				typeHolds[key] = &suspensions[i]
			}
			continue
		}
		if belegHolds[key] == nil {
			belegHolds[key] = map[int64]*parentbilling.RetentionSuspension{}
		}
		belegHolds[key][suspensions[i].BelegID] = &suspensions[i]
	}
	records := make([]parentbilling.BelegRecord, 0, len(facts))
	for _, fact := range facts {
		key := belegKey{fact.OrganizationID, fact.BelegType}
		hold := belegHolds[key][fact.BelegID]
		if hold == nil {
			hold = typeHolds[key]
		}
		decision := DecideBelegRetention(fact, byType[key], hold, today)
		records = append(records, parentbilling.BelegRecord{
			OrganizationID: fact.OrganizationID, BelegType: fact.BelegType, BelegID: fact.BelegID,
			State: decision.State, Reason: decision.Reason, TriggerEvent: decision.TriggerEvent, TriggerDate: decision.TriggerDate,
			DeletableOn: decision.DeletableOn, ProposedOn: decision.ProposedOn,
			InstructionID: decision.InstructionID, SuspensionID: decision.SuspensionID,
		})
	}
	return records
}

// deleteDueContracts asks Care Contract to delete the due contracts per
// Träger and logs what went.
func (r *BelegRetention) deleteDueContracts(ctx context.Context, today calendar.Date) (int, error) {
	due, err := r.store.DueContracts(ctx, today)
	if err != nil {
		return 0, err
	}
	byTraeger := map[int64][]int64{}
	order := []int64{}
	for _, ref := range due {
		if _, seen := byTraeger[ref.OrganizationID]; !seen {
			order = append(order, ref.OrganizationID)
		}
		byTraeger[ref.OrganizationID] = append(byTraeger[ref.OrganizationID], ref.BelegID)
	}
	total := 0
	for _, organizationID := range order {
		ids := byTraeger[organizationID]
		deleted, err := r.contracts.DeleteEndedContracts(ctx, organizationID, ids, today)
		if err != nil {
			return 0, fmt.Errorf("parent billing: delete retained contracts: %w", err)
		}
		if err := r.store.LogContractDeletions(ctx, organizationID, today, deleted); err != nil {
			return 0, err
		}
		total += len(deleted)
	}
	return total, nil
}
