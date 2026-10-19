// Package catererportal is the application workflow of the caterer (ADR
// 0074): the school invites its caterer and ends its supply, the caterer
// accepts the mailed invitation, signs in and reads and confirms the orders
// of every school it supplies. It coordinates Identity & Access (the caterer,
// its account and its schools) and the Meal Plan (the supply and the orders).
//
// The caterer never searches across schools (#4088): its schools come from
// the global identity read, and each school's orders are read in that
// school's own tenant transaction.
package catererportal

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/mealplan"
)

// ErrSchoolNotSupplied refuses a caterer a school it does not supply.
var ErrSchoolNotSupplied = errors.New("the caterer does not supply this school")

// SchoolCaterer is the school's caterer as the school sees it: the running
// supply, if any, and the school's invitations.
type SchoolCaterer struct {
	Supply      *SchoolSupply
	Invitations []identityaccess.CatererInvitation
}

// SchoolSupply is the running supply: who supplies the school, where to and
// since when.
type SchoolSupply struct {
	CatererName     string
	Email           string
	DeliveryAddress string
	StartedAt       time.Time
}

// CatererSchools is the caterer and the schools it supplies now.
type CatererSchools struct {
	CatererName string
	Schools     []School
}

// School is a school the caterer supplies.
type School struct {
	ID   int64
	Name string
}

// SchoolOrders is a school's orders as its caterer reads them, with the
// delivery address of the supply.
type SchoolOrders struct {
	School          School
	DeliveryAddress string
	Orders          mealplan.OrderWeek
}

// SchoolSide is what a school does with its caterer. Every call runs in the
// school's transaction the caller opened.
type SchoolSide interface {
	SchoolCaterer(ctx context.Context) (SchoolCaterer, error)
	// CreateCatererInvitation mails the invitation to the caterer.
	CreateCatererInvitation(ctx context.Context, request identityaccess.CatererInvitationRequest) (identityaccess.CatererInvitation, error)
	RevokeCatererInvitation(ctx context.Context, invitationID int64) error
	// EndSupply ends the supply and the caterer's link to the school
	// together; the caterer loses the school at once.
	EndSupply(ctx context.Context, accountID int64) (bool, error)
}

// Public is the caterer's way in: the invitation the link names and the
// sign-in. Each opens the transaction it needs.
type Public interface {
	PreviewInvitation(ctx context.Context, schoolID int64, token string) (identityaccess.CatererInvitationPreview, error)
	AcceptInvitation(ctx context.Context, schoolID int64, token string, registration identityaccess.CatererRegistration) error
	// LoginCatererWithAudit signs a caterer in to the caterer portal.
	LoginCatererWithAudit(ctx context.Context, email, password, ipAddress, userAgent string) (accessToken, refreshToken string, err error)
}

// CatererSide is what a signed-in caterer reads and does.
type CatererSide interface {
	Schools(ctx context.Context, accountID int64) (CatererSchools, error)
	SchoolOrders(ctx context.Context, accountID, schoolID int64, from, to mealplan.Date) (SchoolOrders, error)
	ConfirmOrder(ctx context.Context, accountID, schoolID, orderID int64) error
}

// Portal is the whole workflow.
type Portal interface {
	SchoolSide
	Public
	CatererSide
}
