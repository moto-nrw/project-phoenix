// Package application coordinates the caterer's identity and supplies with
// the school's meal orders (ADR 0077).
package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal/ports"
)

// Dependencies are the owner capabilities and the runtime the portal uses.
type Dependencies struct {
	Caterers      ports.Caterers
	Orders        ports.MealOrders
	SchoolNames   ports.SchoolNames
	CurrentSchool ports.CurrentSchool
	Lock          ports.Lock
	WithinSchool  ports.WithinSchool
}

// identityFlows are the Identity & Access flows the portal offers as they
// are: the revocation of an invitation, the sign-in and the caterer's
// password change.
type identityFlows interface {
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	LoginCatererWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error)
	ChangeCatererPassword(ctx context.Context, accountID int64, currentPassword, newPassword, ipAddress, userAgent string) error
}

// Portal implements the caterer portal workflow.
type Portal struct {
	identityFlows
	deps Dependencies
}

var _ catererportal.Portal = (*Portal)(nil)

// New builds the portal; every dependency is required.
func New(deps Dependencies) (*Portal, error) {
	if deps.Caterers == nil || deps.Orders == nil || deps.SchoolNames == nil || deps.CurrentSchool == nil || deps.Lock == nil || deps.WithinSchool == nil {
		return nil, errors.New("caterer portal: all dependencies are required")
	}
	return &Portal{identityFlows: deps.Caterers, deps: deps}, nil
}

// supplyLock names the school's supply lock, which the portal takes in the
// school's transaction (ADR 0067 addendum, condition 3). The lifecycle of
// the supply in Identity & Access (invitation, acceptance, end) and the
// caterer's confirmation in the Meal Plan take it before either owner reads
// or writes, so a confirmation never crosses the end of the supply it was
// checked against, and no invitation crosses an acceptance.
func supplyLock(schoolID int64) string {
	return fmt.Sprintf("caterer-school:%d", schoolID)
}

// SchoolCaterer reads the school's running supply and the school's
// invitations.
func (p *Portal) SchoolCaterer(ctx context.Context) (catererportal.SchoolCaterer, error) {
	supply, active, err := p.deps.Caterers.SchoolCaterer(ctx)
	if err != nil {
		return catererportal.SchoolCaterer{}, err
	}
	invitations, err := p.deps.Caterers.ListCatererInvitations(ctx)
	if err != nil {
		return catererportal.SchoolCaterer{}, err
	}
	result := catererportal.SchoolCaterer{Invitations: invitations}
	if active {
		result.Supply = &catererportal.SchoolSupply{CatererName: supply.Caterer.Name, DeliveryAddress: supply.DeliveryAddress, StartedAt: supply.StartedAt}
	}
	return result, nil
}

// CreateCatererInvitation mails the school's invitation under the supply
// lock: Identity & Access refuses it while a caterer supplies the school.
func (p *Portal) CreateCatererInvitation(ctx context.Context, request identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error) {
	if err := p.deps.Lock(ctx, supplyLock(p.deps.CurrentSchool(ctx))); err != nil {
		return identityaccess.CatererInvitation{}, err
	}
	return p.deps.Caterers.CreateCatererInvitation(ctx, request)
}

// EndSupply ends the school's supply under the supply lock, in the school's
// transaction; the caterer loses the school at once.
func (p *Portal) EndSupply(ctx context.Context, accountID int64) (bool, error) {
	if err := p.deps.Lock(ctx, supplyLock(p.deps.CurrentSchool(ctx))); err != nil {
		return false, err
	}
	return p.deps.Caterers.EndCatererSchool(ctx, accountID)
}

// PreviewInvitation reads the invitation the link names in its school.
func (p *Portal) PreviewInvitation(ctx context.Context, schoolID int64, token string) (preview identityaccess.CatererInvitationPreview, err error) {
	err = p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		preview, err = p.deps.Caterers.PreviewCatererInvitation(txCtx, token)
		return err
	})
	return preview, err
}

// AcceptInvitation spends the invitation in the school's transaction under
// the supply lock; Identity & Access starts the supply with it, so the
// caterer never exists without its school.
func (p *Portal) AcceptInvitation(ctx context.Context, schoolID int64, token string, registration identityaccess.CatererRegistration) error {
	return p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		if err := p.deps.Lock(txCtx, supplyLock(schoolID)); err != nil {
			return err
		}
		_, err := p.deps.Caterers.AcceptCatererInvitation(txCtx, token, registration)
		return err
	})
}

// Schools names the caterer and the schools it supplies now.
func (p *Portal) Schools(ctx context.Context, accountID int64) (catererportal.CatererSchools, error) {
	caterer, ids, err := p.deps.Caterers.CatererSchools(ctx, accountID)
	if err != nil {
		return catererportal.CatererSchools{}, err
	}
	names, err := p.deps.SchoolNames(ctx, ids)
	if err != nil {
		return catererportal.CatererSchools{}, err
	}
	result := catererportal.CatererSchools{CatererName: caterer.Name, Schools: make([]catererportal.School, 0, len(ids))}
	for _, id := range ids {
		if _, live := names[id]; !live {
			continue
		}
		result.Schools = append(result.Schools, catererportal.School{ID: id, Name: names[id]})
	}
	return result, nil
}

// SchoolOrders reads one school's orders in that school's transaction, only
// while the caterer supplies it.
func (p *Portal) SchoolOrders(ctx context.Context, accountID, schoolID int64, from, to mealplan.Date) (catererportal.SchoolOrders, error) {
	caterer, schoolName, err := p.supplier(ctx, accountID, schoolID)
	if err != nil {
		return catererportal.SchoolOrders{}, err
	}
	result := catererportal.SchoolOrders{School: catererportal.School{ID: schoolID, Name: schoolName}}
	err = p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		supply, supplyErr := p.supplyOf(txCtx, caterer.ID)
		if supplyErr != nil {
			return supplyErr
		}
		result.DeliveryAddress = supply.DeliveryAddress
		result.Orders, supplyErr = p.deps.Orders.CatererOrders(txCtx, supply.StartedAt, from, to)
		return supplyErr
	})
	return result, err
}

// ConfirmOrder records that the caterer received one of the school's fixed
// orders. The supply lock keeps the supply it was checked against running
// until the confirmation commits.
func (p *Portal) ConfirmOrder(ctx context.Context, accountID, schoolID, orderID int64) error {
	caterer, _, err := p.supplier(ctx, accountID, schoolID)
	if err != nil {
		return err
	}
	return p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		if err := p.deps.Lock(txCtx, supplyLock(schoolID)); err != nil {
			return err
		}
		supply, err := p.supplyOf(txCtx, caterer.ID)
		if err != nil {
			return err
		}
		return p.deps.Orders.ConfirmOrder(txCtx, orderID, supply.StartedAt, accountID)
	})
}

// supplyOf is the school's running supply in its own transaction when the
// caterer runs it: the check under the school's RLS, after the caterer's
// list of schools.
func (p *Portal) supplyOf(ctx context.Context, catererID int64) (identityaccess.CatererSupply, error) {
	supply, active, err := p.deps.Caterers.SchoolCaterer(ctx)
	if err != nil {
		return identityaccess.CatererSupply{}, err
	}
	if !active || supply.Caterer.ID != catererID {
		return identityaccess.CatererSupply{}, catererportal.ErrSchoolNotSupplied
	}
	return supply, nil
}

// supplier is the account's caterer when it supplies a non-deleted school;
// supplyOf checks the supply again inside the school's transaction.
func (p *Portal) supplier(ctx context.Context, accountID, schoolID int64) (identityaccess.Caterer, string, error) {
	caterer, schools, err := p.deps.Caterers.CatererSchools(ctx, accountID)
	if err != nil {
		return identityaccess.Caterer{}, "", err
	}
	if !slices.Contains(schools, schoolID) {
		return identityaccess.Caterer{}, "", catererportal.ErrSchoolNotSupplied
	}
	names, err := p.deps.SchoolNames(ctx, []int64{schoolID})
	if err != nil {
		return identityaccess.Caterer{}, "", err
	}
	name, live := names[schoolID]
	if !live {
		return identityaccess.Caterer{}, "", catererportal.ErrSchoolNotSupplied
	}
	return caterer, name, nil
}

// SchoolDeliveryExport lays out the school's own orders of [from, to] as the
// delivery list, in the school's transaction.
func (p *Portal) SchoolDeliveryExport(ctx context.Context, from, to mealplan.Date) (mealplan.ListExport, error) {
	if err := deliveryWindow(from, to); err != nil {
		return mealplan.ListExport{}, err
	}
	schoolID := p.deps.CurrentSchool(ctx)
	names, err := p.deps.SchoolNames(ctx, []int64{schoolID})
	if err != nil {
		return mealplan.ListExport{}, err
	}
	supply, active, err := p.deps.Caterers.SchoolCaterer(ctx)
	if err != nil {
		return mealplan.ListExport{}, err
	}
	if !active {
		return mealplan.ListExport{}, catererportal.ErrSchoolNotSupplied
	}
	week, err := p.deps.Orders.Orders(ctx, from, to)
	if err != nil {
		return mealplan.ListExport{}, err
	}
	lines, err := week.DeliveryLines(names[schoolID], supply.DeliveryAddress)
	if err != nil {
		return mealplan.ListExport{}, err
	}
	return mealplan.DeliveryExport(from, to, lines)
}

// DeliveryExport lays out the orders of [from, to] of every school the
// caterer supplies as one delivery list. Each school is read in its own
// transaction, with what its caterer may see.
func (p *Portal) DeliveryExport(ctx context.Context, accountID int64, from, to mealplan.Date) (mealplan.ListExport, error) {
	if err := deliveryWindow(from, to); err != nil {
		return mealplan.ListExport{}, err
	}
	caterer, schools, err := p.deps.Caterers.CatererSchools(ctx, accountID)
	if err != nil {
		return mealplan.ListExport{}, err
	}
	names, err := p.deps.SchoolNames(ctx, schools)
	if err != nil {
		return mealplan.ListExport{}, err
	}
	var lines [][]string
	for _, schoolID := range schools {
		if _, live := names[schoolID]; !live {
			continue
		}
		schoolLines, err := p.catererDeliveryLines(ctx, caterer.ID, schoolID, names[schoolID], from, to)
		lines = append(lines, schoolLines...)
		if skipsDelivery(err) {
			continue
		}
		if err != nil {
			return mealplan.ListExport{}, err
		}
	}
	return mealplan.DeliveryExport(from, to, lines)
}

// catererDeliveryLines reads one school's lines of the caterer's delivery
// list in that school's transaction, with what its caterer may see.
func (p *Portal) catererDeliveryLines(ctx context.Context, catererID, schoolID int64, school string, from, to mealplan.Date) (lines [][]string, err error) {
	err = p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		supply, supplyErr := p.supplyOf(txCtx, catererID)
		if supplyErr != nil {
			return supplyErr
		}
		week, ordersErr := p.deps.Orders.CatererOrders(txCtx, supply.StartedAt, from, to)
		if ordersErr != nil {
			return ordersErr
		}
		lines, err = week.DeliveryLines(school, supply.DeliveryAddress)
		return err
	})
	if err != nil {
		return nil, err
	}
	return lines, nil
}

// deliveryWindow refuses a delivery list over more than MaxDeliveryDays
// days. A window the Meal Plan cannot read at all is its own error.
func deliveryWindow(from, to mealplan.Date) error {
	start, startErr := calendar.ParseDate(string(from))
	end, endErr := calendar.ParseDate(string(to))
	if startErr == nil && endErr == nil && start.DaysUntil(end) >= catererportal.MaxDeliveryDays {
		return catererportal.ErrDeliveryRangeTooLong
	}
	return nil
}

// skipsDelivery reports a school that has nothing to deliver right now: it
// switched its meal plan or registration off, or ended the supply during
// the export. The other schools' lines still go out.
func skipsDelivery(err error) bool {
	return errors.Is(err, mealplan.ErrDisabled) || errors.Is(err, mealplan.ErrRegistrationDisabled) || errors.Is(err, catererportal.ErrSchoolNotSupplied)
}
