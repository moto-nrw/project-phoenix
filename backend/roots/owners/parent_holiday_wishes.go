package owners

import (
	"context"
	"slices"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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

// NewParentHolidayAccountProofs binds the prefill's proof of the signed-in
// account (ADR 0216 point 2) to Enrollment, which seals it with the field
// cipher of the holiday registration.
func NewParentHolidayAccountProofs(health enrollmentOwner.HolidayHealthCipher) parentportal.HolidayAccountProofs {
	return enrollmentCompose.NewHolidayParentAccountProofs(health)
}

// HolidayRegistrationOrganizations names Träger for the holiday
// registrations of a parent account, in the caller's administrative
// transaction.
type HolidayRegistrationOrganizations interface {
	ListOrganizationsByID(ctx context.Context, ids []int64) ([]organizationtenancy.Organization, error)
}

// NewParentHolidayRegistrations binds the parents portal to Enrollment's
// registrations of a parent account (ADR 0216 points 3 and 9) and names
// their Träger through Organisation & Tenancy in one read. It runs in the
// caller's administrative transaction.
func NewParentHolidayRegistrations(organizations HolidayRegistrationOrganizations) parentportal.HolidayParentRegistrations {
	return parentHolidayRegistrations{HolidayParentRegistrations: enrollmentCompose.NewHolidayParentRegistrations(), organizations: organizations}
}

// parentHolidayRegistrations claims through Enrollment's capability as it
// is; it names the Träger of the reads.
type parentHolidayRegistrations struct {
	enrollmentOwner.HolidayParentRegistrations
	organizations HolidayRegistrationOrganizations
}

func (r parentHolidayRegistrations) HolidayRegistrationEntries(ctx context.Context, accountID int64) ([]parentportal.HolidayRegistrationEntry, error) {
	registrations, err := r.HolidayParentRegistrations.HolidayParentRegistrations(ctx, accountID)
	if err != nil || len(registrations) == 0 {
		return []parentportal.HolidayRegistrationEntry{}, err
	}
	ids := make([]int64, 0, len(registrations))
	for _, registration := range registrations {
		ids = append(ids, registration.OrganizationID)
	}
	names, err := r.traegerNames(ctx, ids, false)
	if err != nil {
		return nil, err
	}
	entries := make([]parentportal.HolidayRegistrationEntry, len(registrations))
	for i, registration := range registrations {
		children := make([]parentportal.HolidayRegistrationEntryChild, len(registration.Children))
		for j, child := range registration.Children {
			weeks := make([]parentportal.HolidayRegistrationEntryWeek, len(child.Weeks))
			for k, week := range child.Weeks {
				weeks[k] = parentportal.HolidayRegistrationEntryWeek(week)
			}
			children[j] = parentportal.HolidayRegistrationEntryChild{FirstName: child.FirstName, Weeks: weeks}
		}
		entries[i] = parentportal.HolidayRegistrationEntry{
			ID: registration.ID, TraegerName: names[registration.OrganizationID], PeriodName: registration.PeriodName,
			FirstDay: registration.FirstDay, LastDay: registration.LastDay, SubmittedAt: registration.SubmittedAt,
			Status: registration.Status, Children: children,
		}
	}
	return entries, nil
}

func (r parentHolidayRegistrations) HolidayRegistrationTraeger(ctx context.Context, accountID int64) ([]parentportal.Traeger, error) {
	ids, err := r.HolidayParentRegistrationTraeger(ctx, accountID)
	if err != nil || len(ids) == 0 {
		return nil, err
	}
	names, err := r.traegerNames(ctx, ids, true)
	if err != nil {
		return nil, err
	}
	traeger := make([]parentportal.Traeger, 0, len(ids))
	for _, id := range ids {
		if name, ok := names[id]; ok {
			traeger = append(traeger, parentportal.Traeger{ID: id, Name: name})
		}
	}
	return traeger, nil
}

// traegerNames reads the names of the Träger in one call; activeOnly keeps
// only an active, undeleted Träger, whose documents the inbox may read.
func (r parentHolidayRegistrations) traegerNames(ctx context.Context, ids []int64, activeOnly bool) (map[int64]string, error) {
	organizations, err := r.organizations.ListOrganizationsByID(ctx, slices.Compact(slices.Sorted(slices.Values(ids))))
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(organizations))
	for _, organization := range organizations {
		if !activeOnly || (organization.Active && !organization.IsDeleted()) {
			names[organization.ID] = organization.Name
		}
	}
	return names, nil
}
