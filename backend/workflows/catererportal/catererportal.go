// Package catererportal is the application workflow of the caterer (ADR
// 0077): the school invites its caterer and ends its supply, the caterer
// accepts the mailed invitation, signs in and reads and confirms the orders
// of every school it supplies. It coordinates Identity & Access (the caterer,
// its account and the supply) and the Meal Plan (the orders), and owns the
// order of their commands and the school's supply lock.
//
// The caterer's one read across schools is its list of schools, from the
// global identity read filtered on the caterer; each school's supply and
// orders are read in that school's own tenant transaction.
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

// MaxDeliveryDays bounds one delivery list: a week, the longest period the
// portal offers. The caterer's list reads every school it supplies, each in
// its own transaction, so the bound keeps one request's cost fixed per
// school.
const MaxDeliveryDays = 7

// ErrDeliveryRangeTooLong refuses a delivery list over more than
// MaxDeliveryDays days, before any school is read.
var ErrDeliveryRangeTooLong = errors.New("a delivery list covers at most seven days")

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
	// EndSupply ends the school's supply; the caterer loses the school at
	// once.
	EndSupply(ctx context.Context, accountID int64) (bool, error)
	// SchoolDeliveryExport is the school's delivery list of [from, to]: the
	// same document its caterer gets, over at most MaxDeliveryDays days.
	SchoolDeliveryExport(ctx context.Context, from, to mealplan.Date) (mealplan.ListExport, error)
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
	// DeliveryExport is the delivery list of [from, to] of every school the
	// caterer supplies, each read in its own transaction, over at most
	// MaxDeliveryDays days.
	DeliveryExport(ctx context.Context, accountID int64, from, to mealplan.Date) (mealplan.ListExport, error)
	// ChangeCatererPassword replaces the caterer's own password after
	// checking the current one like a login, and ends every session of the
	// account (#4496).
	ChangeCatererPassword(ctx context.Context, accountID int64, currentPassword, newPassword, ipAddress, userAgent string) error
}

// Portal is the whole workflow.
type Portal interface {
	SchoolSide
	Public
	CatererSide
}
