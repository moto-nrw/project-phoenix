package owners

import (
	"context"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentHolidayWishes binds the parents portal's prefill to Enrollment's
// statement of the holiday care wish (#4703). It reads in the caller's
// tenant transaction of the child's school; the workflow checked the
// relationship right before.
func NewParentHolidayWishes(proofs enrollmentOwner.HolidayWishProofs) parentportal.HolidayWishStatements {
	return parentHolidayWishes{proofs: proofs}
}

type parentHolidayWishes struct {
	proofs enrollmentOwner.HolidayWishProofs
}

func (w parentHolidayWishes) HolidayWishStatement(ctx context.Context, claim parentportal.HolidayWishClaim) (string, string, error) {
	statement, err := w.proofs.HolidayWishProof(ctx, enrollmentOwner.HolidayWishClaim(claim))
	return statement.Wish, statement.Proof, err
}

// NewParentHolidayAnnouncements binds the parents app's start page to
// Enrollment's published holiday periods (#6671, ADR 0207). It reads in the
// caller's administrative transaction, after the workflow resolved the
// Träger of the children the account may register.
func NewParentHolidayAnnouncements() parentportal.HolidayRegistrationAnnouncements {
	return parentHolidayAnnouncements{periods: enrollmentCompose.NewHolidayRegistrationAnnouncements()}
}

type parentHolidayAnnouncements struct {
	periods enrollmentOwner.HolidayRegistrationAnnouncements
}

func (a parentHolidayAnnouncements) AnnouncedHolidayRegistrations(ctx context.Context, organizationIDs []int64, on calendar.Date) ([]parentportal.HolidayPeriodAnnouncement, error) {
	periods, err := a.periods.AnnouncedHolidayRegistrations(ctx, organizationIDs, on)
	if err != nil {
		return nil, err
	}
	announcements := make([]parentportal.HolidayPeriodAnnouncement, len(periods))
	for i, period := range periods {
		announcements[i] = parentportal.HolidayPeriodAnnouncement{
			OrganizationID: period.OrganizationID, Token: period.Token, Name: period.Name,
			RegistrationOpensOn: period.RegistrationOpensOn, RegistrationClosesOn: period.RegistrationClosesOn,
			FirstDay: period.FirstDay, LastDay: period.LastDay,
		}
	}
	return announcements, nil
}
