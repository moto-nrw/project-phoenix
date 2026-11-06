package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// AmendFromBookings makes the next version of every active contract whose
// booked care changed after its newest version (#4681). The parents request
// the change and the school approves it in Care Plan's existing change
// request; the approval changes the bookings in the school's tenant. The
// contract lives at the Träger, and no transaction holds both (ADR 0105), so
// the contract follows the committed bookings in its own Träger transaction,
// like the conclusion at acceptance (#4680). A run that fails or is cut
// short changes nothing; the next run finds the same difference.
//
// The run acts for no account and reads the bookings for the contract, not
// for a person, so it writes no entry into the school's access log (as the
// conclusion at acceptance); every new version lands in the trail.
func (c *Conclusions) AmendFromBookings(ctx context.Context, organizationID int64) (carecontract.AmendmentRun, error) {
	run := carecontract.AmendmentRun{Amended: []carecontract.AmendedContract{}, Skipped: []carecontract.AmendmentSkip{}}
	if organizationID <= 0 {
		return run, carecontract.ErrSchoolOutsideTraeger
	}
	var contracts []AmendableContract
	err := c.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		var err error
		contracts, err = c.intake.store.AmendableContracts(txCtx, organizationID)
		return err
	})
	if err != nil || len(contracts) == 0 {
		return run, err
	}
	schools, err := c.intake.schools.TraegerSchools(ctx, organizationID)
	if err != nil {
		return run, fmt.Errorf("care contract: schools: %w", err)
	}
	today := c.intake.today()
	for _, school := range schools {
		if err := c.amendSchool(ctx, organizationID, school, contracts, today, &run); err != nil {
			return run, err
		}
	}
	return run, nil
}

// amendSchool reads the bookings of the school's contracted children in its
// tenant transaction and amends each contract whose care changed.
func (c *Conclusions) amendSchool(ctx context.Context, organizationID int64, school carecontract.School,
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
	if err := c.intake.runtime.WithinSchool(ctx, school.ID, func(txCtx context.Context) error {
		return c.intake.readSchool(txCtx, &read, children)
	}); err != nil {
		return fmt.Errorf("care contract: read school %d: %w", school.ID, err)
	}
	for _, contract := range atSchool {
		if err := c.amendContract(ctx, organizationID, read, contract, today, run); err != nil {
			return err
		}
	}
	return nil
}

// amendContract makes the next version of one contract, or records in run
// why its changed care makes none.
func (c *Conclusions) amendContract(ctx context.Context, organizationID int64, read schoolRead, contract AmendableContract,
	today calendar.Date, run *carecontract.AmendmentRun) error {
	application, ok := findApplication(read.applications, contract.Key.RequestChildID)
	if !ok {
		run.Skipped = append(run.Skipped, carecontract.AmendmentSkip{ContractID: contract.ID, Reason: carecontract.SkipApplicationGone})
		return nil
	}
	version, reason := nextAmendment(read, application, contract, today)
	if reason != "" {
		run.Skipped = append(run.Skipped, carecontract.AmendmentSkip{ContractID: contract.ID, Reason: reason})
		return nil
	}
	if version == nil {
		return nil
	}
	amended, err := c.amend(ctx, organizationID, contract, *version)
	if amended {
		run.Amended = append(run.Amended, carecontract.AmendedContract{
			ContractID: contract.ID, VersionNumber: version.Number, EffectiveFrom: version.EffectiveFrom,
		})
	}
	return err
}

// amend stores the new version in the contract's own Träger transaction,
// under the row lock of the contract. A contract that changed since the
// read (another run, a termination) is left for the next run.
func (c *Conclusions) amend(ctx context.Context, organizationID int64, contract AmendableContract, version carecontract.Version) (bool, error) {
	amended := false
	err := c.intake.runtime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		unchanged, err := c.lockedTimeline(txCtx, organizationID, contract)
		if err != nil || !unchanged {
			return err
		}
		// Only the newest version supersedes the wirksam one; a version that
		// goes before a planned one supersedes nothing.
		previousID := int64(0)
		if version.Status == carecontract.VersionEffective {
			previousID = contract.Latest.ID
		}
		versionID, err := c.intake.store.InsertVersion(txCtx, organizationID, contract.ID, previousID, version)
		if errors.Is(err, ports.ErrVersionMoved) {
			return nil
		}
		if err != nil {
			return err
		}
		contractID, schoolID := contract.ID, contract.Key.SchoolID
		if err := c.intake.store.InsertEvents(txCtx, organizationID, []Event{{
			ContractID: &contractID, SchoolID: &schoolID, Action: ActionCreate,
			Details: map[string]any{
				"version_id": versionID, "version_number": version.Number, "source": version.Source,
				"effective_from": version.EffectiveFrom.String(), "status": version.Status,
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

// lockedTimeline takes the row lock of the contract, the same lock the
// conclusion takes (#4680), and tells whether the contract is still active
// with the timeline the new version was planned on. A contract that ended
// or got another version meanwhile is left for the next run.
func (c *Conclusions) lockedTimeline(ctx context.Context, organizationID int64, contract AmendableContract) (bool, error) {
	record, err := c.intake.store.Contract(ctx, organizationID, contract.ID, true)
	if err != nil || record.Summary.Status != carecontract.StatusActive {
		return false, err
	}
	versions, err := c.intake.store.Versions(ctx, organizationID, contract.ID)
	if err != nil {
		return false, err
	}
	return sameTimeline(versions, contract.Versions), nil
}

// sameTimeline compares the effective versions of all versions with the
// planned timeline by ID and status.
func sameTimeline(all, planned []carecontract.Version) bool {
	effective := []string{}
	for _, version := range all {
		if version.Status == carecontract.VersionEffective || version.Status == carecontract.VersionSuperseded {
			effective = append(effective, fmt.Sprint(version.ID, version.Status))
		}
	}
	want := make([]string, len(planned))
	for i, version := range planned {
		want[i] = fmt.Sprint(version.ID, version.Status)
	}
	slices.Sort(effective)
	slices.Sort(want)
	return slices.Equal(effective, want)
}

func findApplication(applications []carecontract.Application, requestChildID int64) (carecontract.Application, bool) {
	for _, application := range applications {
		if application.RequestChildID == requestChildID {
			return application, true
		}
	}
	return carecontract.Application{}, false
}
