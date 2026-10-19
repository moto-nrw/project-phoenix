// Package ports declares the consumer-owned seams of the caterer portal
// workflow. Owners satisfy them with their public capabilities.
package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// Caterers is the Identity & Access caterer capability.
type Caterers interface {
	CreateCatererInvitation(ctx context.Context, request identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error)
	ListCatererInvitations(ctx context.Context) ([]identityaccess.CatererInvitation, error)
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	PreviewCatererInvitation(ctx context.Context, token string) (identityaccess.CatererInvitationPreview, error)
	AcceptCatererInvitation(ctx context.Context, token string, registration identityaccess.CatererRegistration, start identityaccess.CatererAcceptanceHook) (identityaccess.CatererAcceptance, error)
	EndCatererSchool(ctx context.Context) (bool, error)
	CatererSchools(ctx context.Context, accountID int64) (identityaccess.Caterer, []int64, error)
	LoginCatererWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
}

// Supplies is the Meal Plan's supply and the caterer's view of the orders.
type Supplies interface {
	StartSupply(ctx context.Context, catererID int64, deliveryAddress string, accountID int64) error
	EndSupply(ctx context.Context, accountID int64) (bool, error)
	CurrentSupply(ctx context.Context) (mealplan.Supply, bool, error)
	CatererOrders(ctx context.Context, catererID int64, from, to mealplan.Date) (mealplan.OrderWeek, error)
	ConfirmOrder(ctx context.Context, catererID, orderID, accountID int64) error
}

// SchoolNames names schools by ID in one read; an unknown school is absent.
type SchoolNames func(ctx context.Context, ids []int64) (map[int64]string, error)

// WithinSchool opens the named school's tenant transaction, or joins the
// ambient one of the same school.
type WithinSchool func(ctx context.Context, schoolID int64, fn func(context.Context) error) error
