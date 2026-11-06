package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
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
	existing, templates, err := c.readContracts(ctx, organizationID, reads)
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
			draft, reason := c.intake.draft(key, reads, templates, existing, today)
			if err := c.concludeChild(ctx, organizationID, key, draft, reason, &run); err != nil {
				return run, err
			}
		}
	}
	return run, nil
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

// readContracts reads the keys of the Träger's contracts at the schools
// and its templates in the Träger transaction.
func (c *Conclusions) readContracts(ctx context.Context, organizationID int64, reads []schoolRead) (map[ContractKey]bool, templateIndex, error) {
	var existing map[ContractKey]bool
	var templates templateIndex
	err := c.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		if templates, err = c.intake.templateIndex(txCtx, organizationID); err != nil {
			return err
		}
		existing, err = c.intake.store.ContractKeys(txCtx, organizationID, readSchoolIDs(reads))
		return err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("care contract: read contracts: %w", err)
	}
	return existing, templates, nil
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
	system := carecontract.Reviewer{OrganizationID: organizationID}
	var id int64
	err := c.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		if id, err = c.intake.store.InsertContract(txCtx, organizationID, 0, draft); err != nil {
			return err
		}
		schoolID := draft.Key.SchoolID
		if err := c.intake.store.InsertEvents(txCtx, organizationID, []Event{{
			ContractID: &id, SchoolID: &schoolID, Action: ActionCreate,
			Details: map[string]any{"request_child_id": draft.Key.RequestChildID, "care_offering_ids": draftOfferings(draft),
				"conclusion": carecontract.MethodWithApplication},
		}}); err != nil {
			return err
		}
		conclusion := carecontract.Concluded{Method: carecontract.MethodWithApplication, At: c.clock.now()}
		record, err := c.intake.store.Contract(txCtx, organizationID, id, true)
		if err != nil {
			return err
		}
		record.Concluded = &conclusion
		if err := c.contracts.deliver(txCtx, system, record, ""); err != nil {
			return sendHoldBack(err)
		}
		if record, err = c.intake.store.Contract(txCtx, organizationID, id, true); err != nil {
			return err
		}
		return concludeContract(txCtx, c.intake.store, organizationID, 0, record, conclusion)
	})
	return id, err
}

// sendHoldBack names a send failure that the Träger-Büro or the parents can
// fix; any other error stays an error of the run.
func sendHoldBack(err error) error {
	switch {
	case missingContractPartner(err):
		return errHoldBack{reason: carecontract.SkipPartnerMissing}
	case errors.Is(err, carecontract.ErrRecipientMissing):
		return errHoldBack{reason: carecontract.SkipRecipientMissing}
	}
	return err
}
