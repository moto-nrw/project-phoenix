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
	value, err := a.callbacks.CatalogAt(ctx, id, date)
	return carePlanValuePointer(value, carePlanToAppOfferingChangeCatalog), err
}
func (a offeringChangeRequests) EarliestEffectiveFrom(ctx context.Context) (calendar.Date, error) {
	value, err := a.callbacks.EarliestEffectiveFrom(ctx)
	return value, err
}
func (a offeringChangeRequests) Edit(ctx context.Context, id int64, input care.CreateOfferingChangeInput, version string) (*care.OfferingChangeRequest, error) {
	value, err := a.callbacks.Edit(ctx, id, carePlanFromAppCreateOfferingChangeInput(input), version)
	return carePlanValuePointer(value, carePlanToAppOfferingChangeRequest), err
}
func (a offeringChangeRequests) GetForStudent(ctx context.Context, id int64) (*care.OfferingChangeView, error) {
	value, err := a.callbacks.GetForStudent(ctx, id)
	return carePlanValuePointer(value, carePlanToAppOfferingChangeView), err
}
func (a offeringChangeRequests) SubmitOfferingChange(ctx context.Context, input care.CreateOfferingChangeInput) (*care.OfferingChangeRequest, error) {
	value, err := a.callbacks.SubmitOfferingChange(ctx, carePlanFromAppCreateOfferingChangeInput(input))
	return carePlanValuePointer(value, carePlanToAppOfferingChangeRequest), err
}
func (a offeringChangeRequests) CourseCatalog(ctx context.Context, studentID int64, accountID int64) (*care.CourseCatalog, error) {
	value, err := a.callbacks.CourseCatalog(ctx, studentID, accountID)
	return carePlanValuePointer(value, carePlanToAppCourseCatalog), err
}
func (a offeringChangeRequests) CreateCourseRequest(ctx context.Context, input care.CreateCourseRequestInput) (*care.OfferingChangeRequest, error) {
	value, err := a.callbacks.CreateCourseRequest(ctx, carePlanFromAppCreateCourseRequestInput(input))
	return carePlanValuePointer(value, carePlanToAppOfferingChangeRequest), err
}
func (a offeringChangeRequests) WithdrawCourseRequest(ctx context.Context, studentID int64, accountID int64, offeringID int64) error {
	return a.callbacks.WithdrawCourseRequest(ctx, studentID, accountID, offeringID)
}
