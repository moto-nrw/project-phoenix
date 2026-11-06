package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Amendments serves carecontract.Amendments (#4681). The parents request the
// change and the school approves it in Care Plan's existing change request;
// the approval changes the bookings in the school's tenant. The contract
// lives at the Träger, and no transaction holds both (ADR 0105), so the
// contract follows the committed bookings in its own Träger transaction,
// like the conclusion at acceptance (#4680). A run that fails or is cut
// short changes nothing; the next run finds the same difference.
type Amendments struct {
	intake *Intake
}

var _ carecontract.Amendments = (*Amendments)(nil)

// NewAmendments builds the amendment run over the intake, which reads the
// schools.
func NewAmendments(intake *Intake) *Amendments {
	if intake == nil {
		panic("care contract: the intake is required")
	}
	return &Amendments{intake: intake}
}

// AmendFromBookings makes the next version of every active contract whose
// booked care changed after its newest version.
//
// The run acts for no account and reads the bookings for the contract, not
// for a person, so it writes no entry into the school's access log (as the
// conclusion at acceptance); every new version lands in the trail.
func (a *Amendments) AmendFromBookings(ctx context.Context, organizationID int64) (carecontract.AmendmentRun, error) {
	run := carecontract.AmendmentRun{Amended: []carecontract.AmendedContract{}, Skipped: []carecontract.AmendmentSkip{}}
	if organizationID <= 0 {
		return run, carecontract.ErrSchoolOutsideTraeger
	}
	var contracts []AmendableContract
	err := a.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		contracts, err = a.intake.store.AmendableContracts(txCtx, organizationID)
		return err
	})
	if err != nil || len(contracts) == 0 {
		return run, err
	}
	schools, err := a.intake.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return run, fmt.Errorf("care contract: schools: %w", err)
	}
	today := a.intake.today()
	for _, school := range schools {
		if err := a.amendSchool(ctx, organizationID, school, contracts, today, &run); err != nil {
			return run, err
		}
	}
	return run, nil
}

// amendSchool reads the bookings of the school's contracted children in its
// tenant transaction and amends each contract whose care changed.
func (a *Amendments) amendSchool(ctx context.Context, organizationID int64, school carecontract.School,
	contracts []AmendableContract, today calendar.Date, run *carecontract.AmendmentRun) error {
	atSchool := []AmendableContract{}
	children := []int64{}
	for _, contract := range contracts {
		if contract.Key.SchoolID == school.ID {
			atSchool = append(atSchool, contract)
			children = append(children, contract.Key.RequestChildID)
		}
	}
	if len(atSchool) == 0 {
		return nil
	}
	read := schoolRead{school: school, offerings: map[int64]carecontract.Offering{}}
	if err := a.intake.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
		return a.intake.readSchool(txCtx, &read, children)
	}); err != nil {
		return fmt.Errorf("care contract: read school %d: %w", school.ID, err)
	}
	for _, contract := range atSchool {
		application, ok := findApplication(read.applications, contract.Key.RequestChildID)
		if !ok {
			run.Skipped = append(run.Skipped, carecontract.AmendmentSkip{ContractID: contract.ID, Reason: carecontract.SkipApplicationGone})
			continue
		}
		version, reason := nextAmendment(read, application, contract, today)
		switch {
		case reason != "":
			run.Skipped = append(run.Skipped, carecontract.AmendmentSkip{ContractID: contract.ID, Reason: reason})
		case version != nil:
			amended, err := a.amend(ctx, organizationID, contract, *version)
			if err != nil {
				return err
			}
			if amended {
				run.Amended = append(run.Amended, carecontract.AmendedContract{
					ContractID: contract.ID, VersionNumber: version.Number, EffectiveFrom: version.EffectiveFrom,
				})
			}
		}
	}
	return nil
}

// amend stores the new version in the contract's own Träger transaction,
// under the row lock of the contract. A contract that changed since the
// read (another run, a termination) is left for the next run.
func (a *Amendments) amend(ctx context.Context, organizationID int64, contract AmendableContract, version carecontract.Version) (bool, error) {
	amended := false
	err := a.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		record, err := a.intake.store.Contract(txCtx, organizationID, contract.ID, true)
		if err != nil {
			return err
		}
		if record.Summary.Status != carecontract.StatusActive {
			return nil
		}
		versionID, err := a.intake.store.InsertVersion(txCtx, organizationID, contract.ID, contract.Latest.ID, version)
		if errors.Is(err, ports.ErrVersionMoved) {
			return nil
		}
		if err != nil {
			return err
		}
		contractID, schoolID := contract.ID, contract.Key.SchoolID
		if err := a.intake.store.InsertEvents(txCtx, organizationID, []Event{{
			ContractID: &contractID, SchoolID: &schoolID, Action: ActionCreate,
			Details: map[string]any{
				"version_id": versionID, "version_number": version.Number, "source": version.Source,
				"effective_from": version.EffectiveFrom.String(), "previous_version_id": contract.Latest.ID,
			},
		}}); err != nil {
			return fmt.Errorf("care contract: record amendment: %w", err)
		}
		amended = true
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("care contract: amend contract %d: %w", contract.ID, err)
	}
	return amended, nil
}

func findApplication(applications []carecontract.Application, requestChildID int64) (carecontract.Application, bool) {
	for _, application := range applications {
		if application.RequestChildID == requestChildID {
			return application, true
		}
	}
	return carecontract.Application{}, false
}
