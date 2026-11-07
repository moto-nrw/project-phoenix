package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Conclusions serves carecontract.Conclusion: the conclusion at the
// acceptance (Weg b, #4680, ADR 0088 section 6). It is the reconciliation
// ADR 0088 names: Enrollment's decision stays unchanged (exception E1 of ADR
// 0067), and since no transaction holds a school and its Träger (ADR 0105),
// the contract follows the committed acceptance in its own Träger
// transaction. A run that fails or is cut short leaves the child a
// candidate; the next run picks it up.
type Conclusions struct {
	intake    *Intake
	contracts *Contracts
	clock     Clock
}

var _ carecontract.Conclusion = (*Conclusions)(nil)

// NewConclusions builds the conclusion at acceptance over the intake, which
// reads the schools and builds the contract, and the contracts, which send
// it.
func NewConclusions(intake *Intake, contracts *Contracts, clock Clock) *Conclusions {
	if intake == nil || contracts == nil {
		panic("care contract: the intake and the contracts are required")
	}
	return &Conclusions{intake: intake, contracts: contracts, clock: clock}
}

// errHoldBack rolls back the Träger transaction of one contract that cannot
// be concluded yet, with the reason the run reports.
type errHoldBack struct{ reason string }

func (e errHoldBack) Error() string { return "care contract: conclusion held back: " + e.reason }

// ConcludeAccepted concludes the contract of every accepted application
// child with booked care and no contract at the schools of the Träger that
// conclude at the acceptance.
//
// The run acts for no account: the school accepted the application, and the
// trail of the contract records the conclusion without one. It reads the
// applications for the contract, not for a person, so it writes no entry
// into the access log of the school (ADR 0084 section 7 records reads of
// persons); every contract it writes lands in the trail.
func (c *Conclusions) ConcludeAccepted(ctx context.Context, organizationID int64) (carecontract.ConclusionRun, error) {
	run := carecontract.ConclusionRun{Concluded: []int64{}, Skipped: []carecontract.CreateSkip{}}
	if organizationID <= 0 {
		return run, carecontract.ErrSchoolOutsideTraeger
	}
	reads, err := c.readConcludingSchools(ctx, organizationID)
	if err != nil || len(reads) == 0 {
		return run, err
	}
	existing, templates, changed, err := c.readContracts(ctx, organizationID, reads)
	if err != nil {
		return run, err
	}
	today := c.intake.today()
	for _, read := range reads {
		for _, application := range read.applications {
			key := carecontract.CandidateKey{SchoolID: read.school.ID, RequestChildID: application.RequestChildID}
			if existing[ContractKey(key)] || len(bookedCare(read, application, today).services) == 0 {
				continue
			}
			draft, reason := c.conclusionDraft(key, reads, templates, existing, changed, today)
			if err := c.concludeChild(ctx, organizationID, key, draft, reason, &run); err != nil {
				return run, err
			}
		}
	}
	return run, nil
}

// conclusionDraft builds the contract of one candidate, or names why it
// cannot be concluded: the reasons of a draft, then the missing or outdated
// proof of the parents' agreement.
func (c *Conclusions) conclusionDraft(key carecontract.CandidateKey, reads []schoolRead, templates templateIndex,
	existing map[ContractKey]bool, changed templateChanges, today calendar.Date) (NewContract, string) {
	draft, reason := c.intake.draft(key, reads, templates, existing, today)
	if reason != "" {
		return draft, reason
	}
	draft.EarlyStart = earlyStartRequested(draft)
	return draft, acceptanceIssue(draft, changed.since(key.SchoolID, draftOfferings(draft)))
}

// concludeChild concludes the contract of one candidate, or records in run
// why it waits for the next run; only an unexpected error ends the run.
func (c *Conclusions) concludeChild(ctx context.Context, organizationID int64, key carecontract.CandidateKey,
	draft NewContract, reason string, run *carecontract.ConclusionRun) error {
	if reason == "" {
		id, err := c.concludeOne(ctx, organizationID, draft)
		if reason = holdBackReason(err); reason == "" {
			if err != nil {
				return err
			}
			run.Concluded = append(run.Concluded, id)
			return nil
		}
	}
	run.Skipped = append(run.Skipped, carecontract.CreateSkip{CandidateKey: key, Reason: reason})
	return nil
}

// templateChanges holds when the conditions of each offering last changed.
type templateChanges map[[2]int64]time.Time

// since is the latest change of the conditions of the offerings at the
// school.
func (t templateChanges) since(schoolID int64, offerings []int64) time.Time {
	var latest time.Time
	for _, offering := range offerings {
		if changed := t[[2]int64{schoolID, offering}]; changed.After(latest) {
			latest = changed
		}
	}
	return latest
}

// readContracts reads the templates of the Träger with their last change and
// the keys of its contracts at the schools in the Träger transaction.
func (c *Conclusions) readContracts(ctx context.Context, organizationID int64, reads []schoolRead) (map[ContractKey]bool, templateIndex, templateChanges, error) {
	var existing map[ContractKey]bool
	var rows []TemplateRow
	err := c.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		if rows, err = c.intake.store.Templates(txCtx, organizationID); err != nil {
			return err
		}
		existing, err = c.intake.store.ContractKeys(txCtx, organizationID, readSchoolIDs(reads))
		return err
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("care contract: read contracts: %w", err)
	}
	templates, changed := indexTemplates(rows)
	return existing, templates, changed, nil
}

func holdBackReason(err error) string {
	var held errHoldBack
	switch {
	case errors.As(err, &held):
		return held.reason
	case errors.Is(err, carecontract.ErrContractExists):
		return carecontract.SkipContractExists
	}
	return ""
}

// readConcludingSchools reads every school of the Träger in its own tenant
// transaction and keeps those that conclude at the acceptance, with their
// accepted applications.
func (c *Conclusions) readConcludingSchools(ctx context.Context, organizationID int64) ([]schoolRead, error) {
	schools, err := c.intake.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("care contract: schools: %w", err)
	}
	reads := []schoolRead{}
	for _, school := range schools {
		read := schoolRead{school: school, offerings: map[int64]carecontract.Offering{}}
		concludes := false
		err := c.intake.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
			mode, err := c.intake.modes.ConclusionMode(txCtx, school.ID)
			if err != nil || mode != carecontract.ModeOnAcceptance {
				return err
			}
			concludes = true
			return c.intake.readSchool(txCtx, &read, nil)
		})
		if err != nil {
			return nil, fmt.Errorf("care contract: read school %d: %w", school.ID, err)
		}
		if concludes && len(read.applications) > 0 {
			reads = append(reads, read)
		}
	}
	return reads, nil
}

// concludeOne writes, sends and concludes one contract in one Träger
// transaction: a contract that cannot be sent rolls back whole and stays a
// candidate, so no half-made draft blocks a later run or the Träger-Büro.
func (c *Conclusions) concludeOne(ctx context.Context, organizationID int64, draft NewContract) (int64, error) {
	var id int64
	err := c.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		if id, err = c.intake.store.InsertContract(txCtx, organizationID, 0, draft); err != nil {
			return err
		}
		if err := c.recordAcceptance(txCtx, organizationID, id, draft); err != nil {
			return err
		}
		return c.concludeRecord(txCtx, organizationID, id,
			carecontract.Concluded{Method: carecontract.MethodWithApplication, At: *draft.AcceptedAt})
	})
	return id, err
}

// concludeRecord sends and concludes one contract in the caller's Träger
// transaction. It reads the contract under its row lock first, so a
// concurrent send, discard or conclusion of the same contract either ran
// before (and is seen) or waits until this transaction ends: at most one
// transition per contract wins, and the loser gets a defined error
// (ErrContractNotFound, ErrContractNotConcludable, ErrMailPending) instead of
// working on a stale copy (conclusion_race_test.go).
func (c *Conclusions) concludeRecord(ctx context.Context, organizationID, contractID int64, conclusion carecontract.Concluded) error {
	record, err := c.intake.store.Contract(ctx, organizationID, contractID, true)
	if err != nil {
		return err
	}
	if record.Summary.Status != carecontract.StatusDraft && record.Summary.Status != carecontract.StatusSent {
		return carecontract.ErrContractNotConcludable
	}
	record.Concluded = &conclusion
	if err := c.contracts.deliver(ctx, carecontract.Reviewer{OrganizationID: organizationID}, record, ""); err != nil {
		return sendHoldBack(err)
	}
	if record, err = c.intake.store.Contract(ctx, organizationID, contractID, true); err != nil {
		return err
	}
	return concludeContract(ctx, c.intake.store, organizationID, 0, record, conclusion)
}

// sendHoldBack names a send failure that the Träger-Büro or the parents can
// fix; any other error stays an error of the run.
func sendHoldBack(err error) error {
	switch {
	case missingContractPartner(err):
		return errHoldBack{reason: carecontract.SkipPartnerMissing}
	case errors.Is(err, carecontract.ErrRecipientMissing):
		return errHoldBack{reason: carecontract.SkipRecipientMissing}
	case errors.Is(err, carecontract.ErrMailDelayed):
		return errHoldBack{reason: carecontract.SkipMailDelayed}
	case errors.Is(err, carecontract.ErrMailBlocked):
		return errHoldBack{reason: carecontract.SkipMailBlocked}
	case errors.Is(err, carecontract.ErrMailBlockUnreadable):
		return errHoldBack{reason: carecontract.SkipMailBlockUnreadable}
	}
	return err
}

// acceptanceIssue names why the application gives no proof that the parents
// agreed to the conditions of this contract (ADR 0088 section 3, Weg b): the
// school's acceptance needs its instant, and the submission a required
// conditions block the parents ticked, with the checksum taken when it was
// shown, before the acceptance. Its complete text must equal the applicable
// template conditions. A save timestamp alone neither proves nor invalidates
// agreement; the contract waits when its conditions have not been accepted.
func acceptanceIssue(draft NewContract, termsChanged time.Time) string {
	if _, ok := acceptanceProof(draft); ok {
		return ""
	}
	for _, proof := range draft.Acceptances {
		if proof.Required && submittedAcceptance(draft, proof) && strings.HasPrefix(proof.Text, "Vertragsbedingungen für die Betreuung\n") &&
			termsChanged.After(proof.SubmittedAt) {
			return carecontract.SkipAcceptanceChanged
		}
	}
	return carecontract.SkipAcceptanceMissing
}

// acceptanceProof is the latest ticked block covering exactly the conditions
// frozen into this contract, submitted before the school's acceptance.
func acceptanceProof(draft NewContract) (carecontract.AcceptedText, bool) {
	text := acceptanceText(draft.Version.Terms)
	if text == "" {
		return carecontract.AcceptedText{}, false
	}
	var found carecontract.AcceptedText
	for _, proof := range draft.Acceptances {
		if !proof.Required || !submittedAcceptance(draft, proof) || proof.Text != text {
			continue
		}
		if proof.SubmittedAt.After(found.SubmittedAt) {
			found = proof
		}
	}
	return found, !found.SubmittedAt.IsZero()
}

func submittedAcceptance(draft NewContract, proof carecontract.AcceptedText) bool {
	return draft.AcceptedAt != nil && !draft.AcceptedAt.IsZero() && proof.SHA256 != "" &&
		!proof.SubmittedAt.IsZero() && !proof.SubmittedAt.After(*draft.AcceptedAt)
}

// recordAcceptance writes the creation of the contract with the proof it
// rests on into the trail: which conditions block, its checksum, when it was
// submitted and accepted.
func (c *Conclusions) recordAcceptance(ctx context.Context, organizationID, contractID int64, draft NewContract) error {
	proof, ok := acceptanceProof(draft)
	if !ok {
		return errHoldBack{reason: carecontract.SkipAcceptanceMissing}
	}
	id, schoolID := contractID, draft.Key.SchoolID
	return c.intake.store.InsertEvents(ctx, organizationID, []Event{{
		ContractID: &id, SchoolID: &schoolID, Action: ActionCreate,
		Details: map[string]any{"request_child_id": draft.Key.RequestChildID, "care_offering_ids": draftOfferings(draft),
			"conclusion": carecontract.MethodWithApplication, "acceptance_key": proof.Key, "acceptance_sha256": proof.SHA256,
			"submitted_at": proof.SubmittedAt, "accepted_at": *draft.AcceptedAt, "processed_at": c.clock.now()},
	}})
}
