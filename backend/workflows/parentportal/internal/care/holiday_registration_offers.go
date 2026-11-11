package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Holiday registrations in the parents app (#6671, ADR 0207): the start page
// names the published holiday periods of the Träger of the account's
// children, so a parent starts the registration there instead of from the
// link of a letter. A period is offered only for a child the account may
// register: the same relationship right and the same care check as the
// prefill of the form (#4662). The flow reads no record of a school; it
// names the children by what the account's own child list already shows.

// HolidayPeriodAnnouncement is a published holiday period of a Träger whose
// registration has not closed. FirstDay and LastDay are zero without weeks.
type HolidayPeriodAnnouncement struct {
	OrganizationID       int64
	Token                string
	Name                 string
	RegistrationOpensOn  calendar.Date
	RegistrationClosesOn calendar.Date
	FirstDay             calendar.Date
	LastDay              calendar.Date
}

// HolidayRegistrationAnnouncements is the consumer-owned port over
// Enrollment's published holiday periods. It reads in the caller's
// administrative transaction; the root binds it.
type HolidayRegistrationAnnouncements interface {
	AnnouncedHolidayRegistrations(ctx context.Context, organizationIDs []int64, on calendar.Date) ([]HolidayPeriodAnnouncement, error)
}

// HolidayOfferChild is a child of the account the offer is for.
type HolidayOfferChild struct {
	StudentID int64
	SchoolID  int64
	FirstName string
}

// HolidayRegistrationOffer is a holiday period a parent can register
// children for. Open reports whether today lies in its registration
// window; before it the registration starts on RegistrationOpensOn.
type HolidayRegistrationOffer struct {
	Token                string
	Name                 string
	TraegerName          string
	RegistrationOpensOn  calendar.Date
	RegistrationClosesOn calendar.Date
	FirstDay             calendar.Date
	LastDay              calendar.Date
	Open                 bool
	Children             []HolidayOfferChild
}

// HolidayRegistrationOffers returns the published holiday periods of the
// Träger of the account's children whose registration has not closed, each
// with the children the account may register (#6671). An account without
// such a child gets none.
func (s *Service) HolidayRegistrationOffers(ctx context.Context, accountID int64) ([]HolidayRegistrationOffer, error) {
	if s.HolidayAnnouncements == nil || s.SchoolTraeger == nil {
		return nil, errors.New("parent: holiday registration announcements are not configured")
	}
	offers := []HolidayRegistrationOffer{}
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		children, err := s.ListChildrenForAccount(adminCtx, accountID)
		if err != nil {
			return err
		}
		offers, err = s.holidayRegistrationOffers(adminCtx, children)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("parent: holiday registration offers: %w", err)
	}
	s.Logger.Debug("parent: holiday registration offers",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(offers)),
	)
	return offers, nil
}

// holidayRegistrationOffers resolves the Träger of the registrable
// children's schools and their announced periods, set-based.
func (s *Service) holidayRegistrationOffers(ctx context.Context, children []*ChildSummary) ([]HolidayRegistrationOffer, error) {
	today := s.todayDate()
	schoolIDs, bySchool := registrableChildrenBySchool(children, today)
	if len(schoolIDs) == 0 {
		return []HolidayRegistrationOffer{}, nil
	}
	traegerOf, err := s.SchoolTraeger.TraegerOfSchools(ctx, schoolIDs)
	if err != nil {
		return nil, err
	}
	var organizationIDs []int64
	names := map[int64]string{}
	offered := map[int64][]HolidayOfferChild{}
	for _, schoolID := range schoolIDs {
		traeger, ok := traegerOf[schoolID]
		if !ok {
			continue
		}
		if _, seen := names[traeger.ID]; !seen {
			names[traeger.ID] = traeger.Name
			organizationIDs = append(organizationIDs, traeger.ID)
		}
		for _, child := range bySchool[schoolID] {
			offered[traeger.ID] = append(offered[traeger.ID], HolidayOfferChild{StudentID: child.StudentID, SchoolID: schoolID, FirstName: child.FirstName})
		}
	}
	if len(organizationIDs) == 0 {
		return []HolidayRegistrationOffer{}, nil
	}
	announcements, err := s.HolidayAnnouncements.AnnouncedHolidayRegistrations(ctx, organizationIDs, today)
	if err != nil {
		return nil, err
	}
	offers := make([]HolidayRegistrationOffer, 0, len(announcements))
	for _, period := range announcements {
		// The port names the Träger; a period of any other one is not the
		// account's and a period past its window is no offer.
		if len(offered[period.OrganizationID]) == 0 || today.After(period.RegistrationClosesOn) {
			continue
		}
		offers = append(offers, HolidayRegistrationOffer{
			Token: period.Token, Name: period.Name, TraegerName: names[period.OrganizationID],
			RegistrationOpensOn: period.RegistrationOpensOn, RegistrationClosesOn: period.RegistrationClosesOn,
			FirstDay: period.FirstDay, LastDay: period.LastDay,
			Open:     !today.Before(period.RegistrationOpensOn),
			Children: offered[period.OrganizationID],
		})
	}
	return offers, nil
}

// registrableChild reports whether the account may register the child for
// a holiday period: its relationship grants the registration tied to an
// existing child and the child's care has not ended (#4662).
func registrableChild(child *ChildSummary, today calendar.Date) bool {
	return child != nil && !child.CareEnded(today) && childHasPermission(child, GuardianPermissionEnrollmentSubmit)
}

// registrableChildrenBySchool groups the registrable children by school, in
// the order of children.
func registrableChildrenBySchool(children []*ChildSummary, today calendar.Date) ([]int64, map[int64][]*ChildSummary) {
	var schoolIDs []int64
	bySchool := map[int64][]*ChildSummary{}
	for _, child := range children {
		if !registrableChild(child, today) {
			continue
		}
		if _, seen := bySchool[child.TenantID]; !seen {
			schoolIDs = append(schoolIDs, child.TenantID)
		}
		bySchool[child.TenantID] = append(bySchool[child.TenantID], child)
	}
	return schoolIDs, bySchool
}
