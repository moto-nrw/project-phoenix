package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type OfferingChangeRequests interface{ care.OfferingChangeRequests }
type OfferingChangeRequestsCallbacks struct {
	CatalogAt             func(ctx context.Context, id int64, date calendar.Date) (*OfferingChangeCatalogSnapshot, error)
	EarliestEffectiveFrom func(ctx context.Context) (calendar.Date, error)
	Edit                  func(ctx context.Context, id int64, input CreateOfferingChangeInputSnapshot, version string) (*OfferingChangeRequestSnapshot, error)
	GetForStudent         func(ctx context.Context, id int64) (*OfferingChangeViewSnapshot, error)
	SubmitOfferingChange  func(ctx context.Context, input CreateOfferingChangeInputSnapshot) (*OfferingChangeRequestSnapshot, error)
	CourseCatalog         func(ctx context.Context, studentID int64, accountID int64) (*CourseCatalogSnapshot, error)
	CreateCourseRequest   func(ctx context.Context, input CreateCourseRequestInputSnapshot) (*OfferingChangeRequestSnapshot, error)
	WithdrawCourseRequest func(ctx context.Context, studentID int64, accountID int64, offeringID int64) error
}

func NewOfferingChangeRequests(callbacks OfferingChangeRequestsCallbacks) OfferingChangeRequests {
	return offeringChangeRequests{callbacks}
}

type offeringChangeRequests struct {
	callbacks OfferingChangeRequestsCallbacks
}

func (a offeringChangeRequests) CatalogAt(ctx context.Context, id int64, date calendar.Date) (*care.OfferingChangeCatalog, error) {
	return a.callbacks.CatalogAt(ctx, id, date)
}
func (a offeringChangeRequests) EarliestEffectiveFrom(ctx context.Context) (calendar.Date, error) {
	return a.callbacks.EarliestEffectiveFrom(ctx)
}
func (a offeringChangeRequests) Edit(ctx context.Context, id int64, input care.CreateOfferingChangeInput, version string) (*care.OfferingChangeRequest, error) {
	return a.callbacks.Edit(ctx, id, input, version)
}
func (a offeringChangeRequests) GetForStudent(ctx context.Context, id int64) (*care.OfferingChangeView, error) {
	return a.callbacks.GetForStudent(ctx, id)
}
func (a offeringChangeRequests) SubmitOfferingChange(ctx context.Context, input care.CreateOfferingChangeInput) (*care.OfferingChangeRequest, error) {
	return a.callbacks.SubmitOfferingChange(ctx, input)
}
func (a offeringChangeRequests) CourseCatalog(ctx context.Context, studentID int64, accountID int64) (*care.CourseCatalog, error) {
	return a.callbacks.CourseCatalog(ctx, studentID, accountID)
}
func (a offeringChangeRequests) CreateCourseRequest(ctx context.Context, input care.CreateCourseRequestInput) (*care.OfferingChangeRequest, error) {
	return a.callbacks.CreateCourseRequest(ctx, input)
}
func (a offeringChangeRequests) WithdrawCourseRequest(ctx context.Context, studentID int64, accountID int64, offeringID int64) error {
	return a.callbacks.WithdrawCourseRequest(ctx, studentID, accountID, offeringID)
}
