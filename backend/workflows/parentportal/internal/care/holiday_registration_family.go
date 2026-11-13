package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
// HolidayWish says that the read included the child's holiday care wish
// (#4703).
type ChildDataAccess struct {
	StudentID      int64
	ActorAccountID int64
	OrganizationID int64
	AccessedAt     time.Time
	HolidayWish    bool
}

// ChildDataAccessLog is the consumer-owned port over the Audit Platform's
// data access log. It appends in the caller's tenant transaction.
type ChildDataAccessLog interface {
	RecordHolidayRegistrationPrefill(ctx context.Context, access ChildDataAccess) error
}

// HolidayRegistrationChild is a child the parent may register, with the
// values the school knows. HolidayWish and HolidayWishProof state the
// child's holiday care wish when the period takes it into account (#4703):
// "yes", "no" or empty when unknown, and the sealed statement the form
// sends back.
type HolidayRegistrationChild struct {
	StudentID        int64
	SchoolID         int64
	FirstName        string
	LastName         string
	BirthDate        *calendar.Date
	HolidayWish      string
	HolidayWishProof string
	// Contacts are the child's other guardians the form may offer (#6682).
	Contacts []HolidayRegistrationContact
}

// HolidayRegistrationScope is what the link of a holiday period names: the
// Träger, the period, the Träger's schools and whether and from which day
// the period takes the holiday care wish into account (#4703).
type HolidayRegistrationScope struct {
	OrganizationID  int64
	PeriodID        int64
	SchoolIDs       []int64
	HolidayWishRule bool
	FirstDay        calendar.Date
}

// HolidayWishClaim names a prefilled child for the statement of its
// holiday care wish (#4703).
type HolidayWishClaim struct {
	OrganizationID int64
	PeriodID       int64
	SchoolID       int64
	StudentID      int64
	FirstName      string
	LastName       string
	BirthDate      calendar.Date
	On             calendar.Date
}

// HolidayWishStatements is the consumer-owned port over Enrollment's
// statement of the holiday care wish. It reads in the caller's tenant
// transaction of the child's school; the root binds it.
type HolidayWishStatements interface {
	HolidayWishStatement(ctx context.Context, claim HolidayWishClaim) (wish, proof string, err error)
}

// HolidayRegistrationGuardian is the contact data of the calling guardian at
// the school of the first offered child. Address is nil when the school
// knows none (#6682).
type HolidayRegistrationGuardian struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
	Address   *HolidayRegistrationAddress
}

// HolidayRegistrationFamily is the prefill of a holiday registration.
// Guardian is nil when no child is offered. AccountProof is the sealed proof
// of the signed-in account for the link (ADR 0216 point 2), for every
// account, also one without a child the link offers.
type HolidayRegistrationFamily struct {
	Guardian     *HolidayRegistrationGuardian
	Children     []HolidayRegistrationChild
	AccountProof string
}

// HolidayRegistrationFamily returns the children of the account at the
// schools of the link's Träger (scope, resolved from the link by the
// caller) that the account may register, with the guardian's contact data,
// and records each child's read in its school's data access log. When the
// period takes the holiday care wish into account, each child carries the
// statement of its wish (#4703): the same relationship right that offers
// the child admits its wish, and the same entry records the read.
func (s *Service) HolidayRegistrationFamily(ctx context.Context, accountID int64, scope HolidayRegistrationScope) (*HolidayRegistrationFamily, error) {
	if s.ChildDataAccess == nil {
		return nil, errors.New("parent: child data access log is not configured")
	}
	if scope.HolidayWishRule && s.HolidayWishes == nil {
		return nil, errors.New("parent: holiday wish statements are not configured")
	}
	if scope.OrganizationID <= 0 {
		return nil, errors.New("parent: organization_id must be positive")
	}
	organizationID := scope.OrganizationID
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	tenantIDs, byTenant := s.holidayRegistrationChildren(children, scope.SchoolIDs)
	family := &HolidayRegistrationFamily{Children: []HolidayRegistrationChild{}}
	for _, tenantID := range tenantIDs {
		err := s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
			return s.loadHolidayRegistrationFamily(txCtx, accountID, scope, tenantID, byTenant[tenantID], family)
		})
		if err != nil {
			return nil, fmt.Errorf("parent: holiday registration family: %w", err)
		}
	}
	if s.HolidayAccountProofs != nil && scope.PeriodID > 0 {
		if family.AccountProof, err = s.HolidayAccountProofs.HolidayParentAccountProof(organizationID, scope.PeriodID, accountID); err != nil {
			return nil, fmt.Errorf("parent: holiday registration account proof: %w", err)
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
		if !registrableChild(child, today) || !traegerSchools[child.TenantID] {
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
func (s *Service) loadHolidayRegistrationFamily(ctx context.Context, accountID int64, scope HolidayRegistrationScope, tenantID int64, children []*ChildSummary, family *HolidayRegistrationFamily) error {
	offered := make([]*HolidayRegistrationChild, 0, len(children))
	for _, child := range children {
		person, err := s.PersonRepo.FindByID(ctx, child.PersonID)
		if err != nil {
			return err
		}
		offeredChild := HolidayRegistrationChild{
			StudentID: child.StudentID, SchoolID: tenantID,
			FirstName: person.FirstName, LastName: person.LastName, BirthDate: person.Birthday,
		}
		if err := s.stateHolidayWish(ctx, scope, &offeredChild); err != nil {
			return err
		}
		offered = append(offered, &offeredChild)
		if err := s.ChildDataAccess.RecordHolidayRegistrationPrefill(ctx, ChildDataAccess{
			StudentID: child.StudentID, ActorAccountID: accountID, OrganizationID: scope.OrganizationID, AccessedAt: s.now(),
			HolidayWish: scope.HolidayWishRule && person.Birthday != nil,
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
	if err := s.attachHolidayRegistrationContacts(ctx, accountID, offered); err != nil {
		return err
	}
	for _, child := range offered {
		family.Children = append(family.Children, *child)
	}
	return nil
}

// holidayRegistrationGuardian reads the calling guardian's own profile,
// postal address and primary phone at the school in context.
func (s *Service) holidayRegistrationGuardian(ctx context.Context, guardianProfileID int64) (*HolidayRegistrationGuardian, error) {
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	guardian := &HolidayRegistrationGuardian{FirstName: profile.FirstName, LastName: profile.LastName, Email: deref(profile.Email), Address: holidayRegistrationAddress(profile)}
	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if primary := pickPrimaryPhone(phones, func(phone *GuardianPhoneNumber) bool { return phone.IsPrimary }); primary != nil {
		guardian.Phone = primary.PhoneNumber
	}
	return guardian, nil
}

// stateHolidayWish adds the statement of the child's holiday care wish when
// the period takes it into account (#4703). It reads in the tenant
// transaction of the child's school; a child without a known date of birth
// gets none, because the statement names the child by it.
func (s *Service) stateHolidayWish(ctx context.Context, scope HolidayRegistrationScope, child *HolidayRegistrationChild) error {
	if !scope.HolidayWishRule || child.BirthDate == nil {
		return nil
	}
	var err error
	child.HolidayWish, child.HolidayWishProof, err = s.HolidayWishes.HolidayWishStatement(ctx, HolidayWishClaim{
		OrganizationID: scope.OrganizationID, PeriodID: scope.PeriodID, SchoolID: child.SchoolID, StudentID: child.StudentID,
		FirstName: child.FirstName, LastName: child.LastName, BirthDate: *child.BirthDate, On: scope.FirstDay,
	})
	return err
}
