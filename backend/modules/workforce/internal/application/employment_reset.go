package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ResetEmployment joins acceptance's tenant transaction. Audits and all three
// work-time changes roll back together when any owner command fails.
type EmploymentResetter struct {
	Capability workforce.Capability
	Employment workforce.StaffEmployments
	Audit      workforce.StaffAdminAudit
	Admin      workforce.StaffRecordAdmin
	WorkTime   func(context.Context, workforce.EmploymentReset, string) error
}

func (r EmploymentResetter) ResetEmployment(ctx context.Context, in workforce.EmploymentReset) error {
	if in.StaffID <= 0 || in.ActorAccountID <= 0 || in.InvitationID <= 0 {
		return errors.New("employment reset requires staff, account and invitation")
	}
	note := fmt.Sprintf("Wiederaufnahme gemäß Einladung %d", in.InvitationID)
	if err := requireEmploymentResetTransaction(ctx); err != nil {
		return err
	}

	if in.WorkTime {
		if err := r.WorkTime(ctx, in, note); err != nil {
			return err
		}
	}
	if in.MasterData {
		if err := r.resetMasterData(ctx, in, note); err != nil {
			return err
		}
	}
	if in.Qualifications {
		return r.Admin.ReplaceStaffQualificationList(ctx, in.StaffID, nil, in.StaffID, note)
	}
	return nil
}

func (r EmploymentResetter) resetMasterData(ctx context.Context, in workforce.EmploymentReset, note string) error {
	a := r.Admin
	// Names and birthday belong to the shared directory identity. The former
	// employment's gender, contact and contract fields belong to this reset.
	data, err := a.StaffStammdaten(ctx, in.StaffID)
	if err != nil {
		return err
	}
	subject := data.Staff
	if err := a.UpdateStaffStammdatenPerson(ctx, in.StaffID, workforce.StammdatenPersonInput{FirstName: subject.FirstName, LastName: subject.LastName, Birthday: optionalString(subject.Birthday)}, in.StaffID, note); err != nil {
		return err
	}
	if err := a.UpdateStaffStammdatenKontakt(ctx, in.StaffID, workforce.StammdatenKontaktInput{}, in.StaffID, note); err != nil {
		return err
	}
	if err := a.UpdateStaffStammdatenArbeitsvertrag(ctx, in.StaffID, workforce.StammdatenArbeitsvertragInput{}, in.StaffID, note); err != nil {
		return err
	}
	if subject.PersonnelNumber == nil && in.PreviousPersonnelNumber != nil {
		if err := r.Audit.RecordPersonnelNumberChange(ctx, workforce.PersonnelNumberChange{StaffID: in.StaffID, ChangedBy: in.StaffID, OldValue: *in.PreviousPersonnelNumber, NewValue: "", Note: note}); err != nil {
			return err
		}
	}
	_, err = a.UpdatePersonnelNumber(ctx, in.StaffID, nil, in.StaffID, note)
	return err
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// NewEmploymentReset composes the personnel and time-account commands.
func NewEmploymentReset(admin workforce.StaffRecordAdmin, capability workforce.Capability, employment workforce.StaffEmployments, audit workforce.StaffAdminAudit, resetTime func(context.Context, workforce.EmploymentReset, string) error) workforce.EmploymentResetter {
	return EmploymentResetter{Capability: capability, Employment: employment, Audit: audit, Admin: admin, WorkTime: resetTime}
}

// PrepareEmploymentReset takes the account locks before Membership's row
// lock, matching offboarding. Retired memberships cannot edit their profile.
func (r EmploymentResetter) PrepareEmploymentReset(ctx context.Context, in workforce.EmploymentReset) (*string, error) {
	if err := requireEmploymentResetTransaction(ctx); err != nil {
		return nil, err
	}
	if in.WorkTime {
		if err := r.Capability.LockStaffBalanceWrites(ctx, in.StaffID); err != nil {
			return nil, err
		}
		if err := r.Capability.LockStaffAbsenceWrites(ctx, in.StaffID); err != nil {
			return nil, err
		}
	}
	profiles, err := r.Employment.StaffEmployments(ctx, []int64{in.StaffID})
	if err != nil {
		return nil, err
	}
	profile, found := profiles[in.StaffID]
	if !found {
		return nil, workforce.ErrStaffEmploymentNotFound
	}
	return profile.PersonnelNumber, nil
}

// The invitation already owns the outer unit of work. Refuse an unscoped
// call so prepare's locks and every reset share that unit's lifetime.
func requireEmploymentResetTransaction(ctx context.Context) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("employment reset requires an invitation transaction")
	}
	return nil
}
