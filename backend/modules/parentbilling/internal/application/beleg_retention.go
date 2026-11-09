package application

import (
	"context"
	"fmt"
	"slices"
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
	// LockTraeger takes the lock of each Träger that its instructions,
	// suspensions, reviews, evaluation and deletion share, until the
	// transaction ends.
	LockTraeger(ctx context.Context, organizationIDs []int64) error
	ActiveInstructions(ctx context.Context, organizationID int64) ([]parentbilling.RetentionInstruction, error)
	ActiveSuspensions(ctx context.Context, organizationID int64) ([]parentbilling.RetentionSuspension, error)
	InsertInstruction(ctx context.Context, organizationID, operatorID int64, instruction parentbilling.NewRetentionInstruction) (parentbilling.RetentionInstruction, error)
	InsertSuspension(ctx context.Context, organizationID, operatorID int64, suspension parentbilling.NewRetentionSuspension) (parentbilling.RetentionSuspension, error)
	LiftSuspension(ctx context.Context, organizationID, suspensionID, operatorID int64) error
	// ClarificationReviewOn answers the review date of the clarification
	// entry of a type and reason, the earliest of its Belege; an entry
	// without a listed Beleg is ErrBelegClarificationNotFound.
	ClarificationReviewOn(ctx context.Context, organizationID int64, belegType, reason string) (calendar.Date, error)
	InsertClarificationReview(ctx context.Context, organizationID, operatorID int64, today calendar.Date, review parentbilling.NewBelegClarificationReview) (parentbilling.BelegClarificationReview, error)
	SaveBelegRecords(ctx context.Context, organizationID int64, today calendar.Date, records []parentbilling.BelegRecord) error
	// DeleteDueBelege deletes the due Belege of parent-billing in the order
	// their cascades need: debits first, mandates last, then the runs left
	// without a Beleg.
	DeleteDueBelege(ctx context.Context, today calendar.Date) (int, error)
	// DueContracts locks the due contracts against new mandates and debits
	// and lists those to which none hangs and no suspension applies.
	DueContracts(ctx context.Context, today calendar.Date) ([]parentbilling.DueContract, error)
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
	input.BasisJustification = strings.TrimSpace(input.BasisJustification)
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

// RecordClarificationReview records what the moto team clarified with the
// Träger for an entry of the clarification list, an escalation to Florian
// with its reason, and moves the next review of the entry (Löschkonzept
// section 3). Under the lock of the Träger it reads the entry's review date:
// moving it past that date needs the escalation, so an overdue entry is
// escalated, not just rescheduled. It deletes nothing.
func (r *BelegRetention) RecordClarificationReview(ctx context.Context, organizationID, operatorID int64, input parentbilling.NewBelegClarificationReview) (parentbilling.BelegClarificationReview, error) {
	input.Note, input.EscalationReason = strings.TrimSpace(input.Note), strings.TrimSpace(input.EscalationReason)
	today := r.today()
	if err := ValidateClarificationReview(input, today); err != nil {
		return parentbilling.BelegClarificationReview{}, err
	}
	if err := r.store.LockTraeger(ctx, []int64{organizationID}); err != nil {
		return parentbilling.BelegClarificationReview{}, err
	}
	reviewOn, err := r.store.ClarificationReviewOn(ctx, organizationID, input.BelegType, input.Reason)
	if err != nil {
		return parentbilling.BelegClarificationReview{}, err
	}
	if err := ValidateClarificationEscalation(input, reviewOn); err != nil {
		return parentbilling.BelegClarificationReview{}, err
	}
	return r.store.InsertClarificationReview(ctx, organizationID, operatorID, today, input)
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
// It holds the lock of each Träger before it reads the instructions and
// suspensions, so a change an operator is recording is either in or waits.
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
	if err := r.store.LockTraeger(ctx, traegerOf(organizationID, facts)); err != nil {
		return err
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

// traegerOf lists the Träger of the facts, or the one Träger evaluated.
func traegerOf(organizationID int64, facts []parentbilling.BelegFact) []int64 {
	if organizationID > 0 {
		return []int64{organizationID}
	}
	seen := map[int64]bool{}
	traeger := []int64{}
	for _, fact := range facts {
		if !seen[fact.OrganizationID] {
			seen[fact.OrganizationID] = true
			traeger = append(traeger, fact.OrganizationID)
		}
	}
	slices.Sort(traeger)
	return traeger
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
			InstructionID: decision.InstructionID, SuspensionID: decision.SuspensionID, CollectionTrigger: fact.CollectionTrigger,
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
	// One Weisung per Träger names the event and the last day it may lie on.
	byTraeger := map[int64][]int64{}
	cutoff := map[int64]parentbilling.DueContract{}
	order := []int64{}
	for _, contract := range due {
		if _, seen := byTraeger[contract.OrganizationID]; !seen {
			order = append(order, contract.OrganizationID)
			cutoff[contract.OrganizationID] = contract
		}
		byTraeger[contract.OrganizationID] = append(byTraeger[contract.OrganizationID], contract.ContractID)
	}
	total := 0
	for _, organizationID := range order {
		ids, rule := byTraeger[organizationID], cutoff[organizationID]
		deleted, err := r.contracts.DeleteEndedContracts(ctx, organizationID, ids, today, rule.TriggerEvent, rule.LastEventOn)
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
