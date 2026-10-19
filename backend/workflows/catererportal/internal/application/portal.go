// Package application coordinates the caterer's identity and its supplies
// (ADR 0074).
package application

import (
	"context"
	"errors"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal"
	"github.com/moto-nrw/project-phoenix/workflows/catererportal/ports"
)

// Dependencies are the owner capabilities and the runtime the portal uses.
type Dependencies struct {
	Caterers      ports.Caterers
	Supplies      ports.Supplies
	SchoolNames   ports.SchoolNames
	CurrentSchool ports.CurrentSchool
	WithinSchool  ports.WithinSchool
}

// identityFlows are the Identity & Access flows the portal offers as they
// are: the invitation, its revocation and the sign-in.
type identityFlows interface {
	CreateCatererInvitation(ctx context.Context, request identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error)
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	LoginCatererWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (string, string, error)
}

// Portal implements the caterer portal workflow.
type Portal struct {
	identityFlows
	deps Dependencies
}

var _ catererportal.Portal = (*Portal)(nil)

// New builds the portal; every dependency is required.
func New(deps Dependencies) (*Portal, error) {
	if deps.Caterers == nil || deps.Supplies == nil || deps.SchoolNames == nil || deps.CurrentSchool == nil || deps.WithinSchool == nil {
		return nil, errors.New("caterer portal: all dependencies are required")
	}
	return &Portal{identityFlows: deps.Caterers, deps: deps}, nil
}

// SchoolCaterer reads the school's running supply, the caterer that runs it
// and the school's invitations.
func (p *Portal) SchoolCaterer(ctx context.Context) (catererportal.SchoolCaterer, error) {
	supply, active, err := p.deps.Supplies.CurrentSupply(ctx)
	if err != nil {
		return catererportal.SchoolCaterer{}, err
	}
	invitations, err := p.deps.Caterers.ListCatererInvitations(ctx)
	if err != nil {
		return catererportal.SchoolCaterer{}, err
	}
	result := catererportal.SchoolCaterer{Invitations: invitations}
	if !active {
		return result, nil
	}
	caterer, _, err := p.deps.Caterers.SchoolCaterer(ctx)
	if err != nil {
		return catererportal.SchoolCaterer{}, err
	}
	result.Supply = &catererportal.SchoolSupply{CatererName: caterer.Name, DeliveryAddress: supply.DeliveryAddress, StartedAt: supply.StartedAt}
	return result, nil
}

// EndSupply ends the supply and the caterer's link to the school in the
// school's transaction.
func (p *Portal) EndSupply(ctx context.Context, accountID int64) (bool, error) {
	supplied, err := p.deps.Supplies.EndSupply(ctx, accountID)
	if err != nil {
		return false, err
	}
	linked, err := p.deps.Caterers.EndCatererSchool(ctx)
	if err != nil {
		return false, err
	}
	return supplied || linked, nil
}

// PreviewInvitation reads the invitation the link names in its school.
func (p *Portal) PreviewInvitation(ctx context.Context, schoolID int64, token string) (preview identityaccess.CatererInvitationPreview, err error) {
	err = p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		preview, err = p.deps.Caterers.PreviewCatererInvitation(txCtx, token)
		return err
	})
	return preview, err
}

// AcceptInvitation spends the invitation and starts the supply in the
// school's transaction: the caterer never exists without its supply.
func (p *Portal) AcceptInvitation(ctx context.Context, schoolID int64, token string, registration identityaccess.CatererRegistration) error {
	return p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		_, err := p.deps.Caterers.AcceptCatererInvitation(txCtx, token, registration, func(hookCtx context.Context, acceptance identityaccess.CatererAcceptance) error {
			return p.deps.Supplies.StartSupply(hookCtx, acceptance.CatererID, acceptance.DeliveryAddress, acceptance.AccountID)
		})
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
		result.Schools = append(result.Schools, catererportal.School{ID: id, Name: names[id]})
	}
	return result, nil
}

// SchoolOrders reads one school's orders in that school's transaction, only
// while the caterer supplies it.
func (p *Portal) SchoolOrders(ctx context.Context, accountID, schoolID int64, from, to mealplan.Date) (catererportal.SchoolOrders, error) {
	caterer, err := p.supplier(ctx, accountID, schoolID)
	if err != nil {
		return catererportal.SchoolOrders{}, err
	}
	names, err := p.deps.SchoolNames(ctx, []int64{schoolID})
	if err != nil {
		return catererportal.SchoolOrders{}, err
	}
	result := catererportal.SchoolOrders{School: catererportal.School{ID: schoolID, Name: names[schoolID]}}
	err = p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		supply, _, supplyErr := p.deps.Supplies.CurrentSupply(txCtx)
		if supplyErr != nil {
			return supplyErr
		}
		result.DeliveryAddress = supply.DeliveryAddress
		result.Orders, supplyErr = p.deps.Supplies.CatererOrders(txCtx, caterer.ID, from, to)
		return supplyErr
	})
	return result, err
}

// ConfirmOrder records that the caterer received one of the school's fixed
// orders.
func (p *Portal) ConfirmOrder(ctx context.Context, accountID, schoolID, orderID int64) error {
	caterer, err := p.supplier(ctx, accountID, schoolID)
	if err != nil {
		return err
	}
	return p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
		return p.deps.Supplies.ConfirmOrder(txCtx, caterer.ID, orderID, accountID)
	})
}

// supplier is the account's caterer when it supplies the school; the Meal
// Plan checks the supply again inside the school's transaction.
func (p *Portal) supplier(ctx context.Context, accountID, schoolID int64) (identityaccess.Caterer, error) {
	caterer, schools, err := p.deps.Caterers.CatererSchools(ctx, accountID)
	if err != nil {
		return identityaccess.Caterer{}, err
	}
	if !slices.Contains(schools, schoolID) {
		return identityaccess.Caterer{}, catererportal.ErrSchoolNotSupplied
	}
	return caterer, nil
}

// SchoolDeliveryExport lays out the school's own orders of [from, to] as the
// delivery list, in the school's transaction.
func (p *Portal) SchoolDeliveryExport(ctx context.Context, from, to mealplan.Date) (mealplan.ListExport, error) {
	schoolID := p.deps.CurrentSchool(ctx)
	names, err := p.deps.SchoolNames(ctx, []int64{schoolID})
	if err != nil {
		return mealplan.ListExport{}, err
	}
	supply, _, err := p.deps.Supplies.CurrentSupply(ctx)
	if err != nil {
		return mealplan.ListExport{}, err
	}
	week, err := p.deps.Supplies.Orders(ctx, from, to)
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
		err := p.deps.WithinSchool(ctx, schoolID, func(txCtx context.Context) error {
			supply, _, supplyErr := p.deps.Supplies.CurrentSupply(txCtx)
			if supplyErr != nil {
				return supplyErr
			}
			week, ordersErr := p.deps.Supplies.CatererOrders(txCtx, caterer.ID, from, to)
			if ordersErr != nil {
				return ordersErr
			}
			schoolLines, linesErr := week.DeliveryLines(names[schoolID], supply.DeliveryAddress)
			lines = append(lines, schoolLines...)
			return linesErr
		})
		if err != nil {
			return mealplan.ListExport{}, err
		}
	}
	return mealplan.DeliveryExport(from, to, lines)
}
