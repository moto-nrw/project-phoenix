package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Prefill of the holiday registration (#4662, spec #4611 user story 27): a
// parent with an account opens the link of a holiday period and picks the
// children the schools of its Träger already know. A child is offered only
// where the calling account's relationship grants
// parent_portal.enrollment.submit, the permission of a registration tied to
// an existing child; membership or a bare guardian link is not enough. The
// flow only reads: the parent corrects the prefilled values in the
// registration, which keeps its own copy (ADR 0110), so no school record
// changes. Every child whose data the flow reads gets an entry in the
// school's data access log.

// ChildDataAccess is one read of a child's data for the holiday
// registration of a Träger, recorded in the school's tenant transaction.
type ChildDataAccess struct {
	StudentID      int64
	ActorAccountID int64
	OrganizationID int64
	AccessedAt     time.Time
}

// ChildDataAccessLog is the consumer-owned port over the Audit Platform's
// data access log. It appends in the caller's tenant transaction.
type ChildDataAccessLog interface {
	RecordHolidayRegistrationPrefill(ctx context.Context, access ChildDataAccess) error
}

// HolidayRegistrationChild is a child the parent may register, with the
// values the school knows.
type HolidayRegistrationChild struct {
	StudentID int64
	SchoolID  int64
	FirstName string
	LastName  string
	BirthDate *calendar.Date
}

// HolidayRegistrationGuardian is the contact data of the calling guardian at
// the school of the first offered child.
type HolidayRegistrationGuardian struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

// HolidayRegistrationFamily is the prefill of a holiday registration.
// Guardian is nil when no child is offered.
type HolidayRegistrationFamily struct {
	Guardian *HolidayRegistrationGuardian
	Children []HolidayRegistrationChild
}

// HolidayRegistrationFamily returns the children of the account at the
// schools of the Träger organizationID (schoolIDs, resolved from the link by
// the caller) that the account may register, with the guardian's contact
// data, and records each child's read in its school's data access log.
func (s *Service) HolidayRegistrationFamily(ctx context.Context, accountID, organizationID int64, schoolIDs []int64) (*HolidayRegistrationFamily, error) {
	if s.ChildDataAccess == nil {
		return nil, errors.New("parent: child data access log is not configured")
	}
	if organizationID <= 0 {
		return nil, errors.New("parent: organization_id must be positive")
	}
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	tenantIDs, byTenant := s.holidayRegistrationChildren(children, schoolIDs)
	family := &HolidayRegistrationFamily{Children: []HolidayRegistrationChild{}}
	for _, tenantID := range tenantIDs {
		err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			return s.loadHolidayRegistrationFamily(txCtx, accountID, organizationID, tenantID, byTenant[tenantID], family)
		})
		if err != nil {
			return nil, fmt.Errorf("parent: holiday registration family: %w", err)
		}
	}
	s.Logger.Debug("parent: holiday registration family",
		slog.Int64("account_id", accountID),
		slog.Int64("organization_id", organizationID),
		slog.Int("count", len(family.Children)),
	)
	return family, nil
}

// holidayRegistrationChildren keeps the children at a school of the Träger
// whose relationship grants the registration and whose care has not ended,
// grouped by school in the order of children.
func (s *Service) holidayRegistrationChildren(children []*ChildSummary, schoolIDs []int64) ([]int64, map[int64][]*ChildSummary) {
	traegerSchools := make(map[int64]bool, len(schoolIDs))
	for _, id := range schoolIDs {
		traegerSchools[id] = true
	}
	today := s.todayDate()
	var tenantIDs []int64
	byTenant := map[int64][]*ChildSummary{}
	for _, child := range children {
		if child == nil || !traegerSchools[child.TenantID] || child.CareEnded(today) ||
			!childHasPermission(child, GuardianPermissionEnrollmentSubmit) {
			continue
		}
		if _, seen := byTenant[child.TenantID]; !seen {
			tenantIDs = append(tenantIDs, child.TenantID)
		}
		byTenant[child.TenantID] = append(byTenant[child.TenantID], child)
	}
	return tenantIDs, byTenant
}

// loadHolidayRegistrationFamily reads the children of one school inside its
// tenant transaction and records each read there.
func (s *Service) loadHolidayRegistrationFamily(ctx context.Context, accountID, organizationID, tenantID int64, children []*ChildSummary, family *HolidayRegistrationFamily) error {
	for _, child := range children {
		person, err := s.PersonRepo.FindByID(ctx, child.PersonID)
		if err != nil {
			return err
		}
		family.Children = append(family.Children, HolidayRegistrationChild{
			StudentID: child.StudentID, SchoolID: tenantID,
			FirstName: person.FirstName, LastName: person.LastName, BirthDate: person.Birthday,
		})
		if err := s.ChildDataAccess.RecordHolidayRegistrationPrefill(ctx, ChildDataAccess{
			StudentID: child.StudentID, ActorAccountID: accountID, OrganizationID: organizationID, AccessedAt: s.now(),
		}); err != nil {
			return err
		}
		if family.Guardian == nil {
			guardian, err := s.holidayRegistrationGuardian(ctx, child.GuardianProfileID)
			if err != nil {
				return err
			}
			family.Guardian = guardian
		}
	}
	return nil
}

// holidayRegistrationGuardian reads the calling guardian's own profile and
// primary phone at the school in context.
func (s *Service) holidayRegistrationGuardian(ctx context.Context, guardianProfileID int64) (*HolidayRegistrationGuardian, error) {
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	guardian := &HolidayRegistrationGuardian{FirstName: profile.FirstName, LastName: profile.LastName, Email: deref(profile.Email)}
	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if primary := pickPrimaryPhone(phones, func(phone *GuardianPhoneNumber) bool { return phone.IsPrimary }); primary != nil {
		guardian.Phone = primary.PhoneNumber
	}
	return guardian, nil
}
