// Package ports declares the consumer-owned seams of the caterer portal
// workflow. Owners satisfy them with their public capabilities.
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// Caterers is the Identity & Access caterer capability: the caterer, its
// account and the school's supply by it.
type Caterers interface {
	CreateCatererInvitation(ctx context.Context, request identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error)
	ListCatererInvitations(ctx context.Context) ([]identityaccess.CatererInvitation, error)
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	PreviewCatererInvitation(ctx context.Context, token string) (identityaccess.CatererInvitationPreview, error)
	AcceptCatererInvitation(ctx context.Context, token string, registration identityaccess.CatererRegistration) (identityaccess.CatererAcceptance, error)
	EndCatererSchool(ctx context.Context, accountID int64) (bool, error)
	SchoolCaterer(ctx context.Context) (identityaccess.CatererSupply, bool, error)
	CatererSchools(ctx context.Context, accountID int64) (identityaccess.Caterer, []int64, error)
	ChangeCatererPassword(ctx context.Context, accountID int64, currentPassword, newPassword, ipAddress, userAgent string) error
}

// MealOrders is the Meal Plan's orders as the school and its caterer read
// them, and the caterer's confirmation.
type MealOrders interface {
	CatererOrders(ctx context.Context, since time.Time, from, to mealplan.Date) (mealplan.OrderWeek, error)
	// Orders is the school's own view of its orders.
	Orders(ctx context.Context, from, to mealplan.Date) (mealplan.OrderWeek, error)
	ConfirmOrder(ctx context.Context, orderID int64, since time.Time, accountID int64) error
}

// SchoolNames names non-deleted schools by ID in one read; unknown or deleted
// schools are absent. Presence in the result is required for caterer access.
type SchoolNames func(ctx context.Context, ids []int64) (map[int64]string, error)

// CurrentSchool is the school of the caller's transaction.
type CurrentSchool func(ctx context.Context) int64

// Lock takes an exclusive lock held until the school's transaction ends.
type Lock func(ctx context.Context, key string) error

// WithinSchool opens the named school's tenant transaction, or joins the
// ambient one of the same school.
type WithinSchool func(ctx context.Context, schoolID int64, fn func(context.Context) error) error
