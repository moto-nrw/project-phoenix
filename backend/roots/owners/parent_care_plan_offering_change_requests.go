package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentOfferingChangeRequests(source interface {
	careplan.OfferingChangeRequests
	careplan.CourseRequests
}) parentportal.OfferingChangeRequests {
	if source == nil {
		return nil
	}
	return parentportal.NewOfferingChangeRequests(parentportal.OfferingChangeRequestsCallbacks{
		CatalogAt: func(ctx context.Context, id int64, date calendar.Date) (*parentportal.OfferingChangeCatalogSnapshot, error) {
			value, err := source.CatalogAt(ctx, id, date)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerOfferingChangeCatalog), err
		},
		EarliestEffectiveFrom: func(ctx context.Context) (calendar.Date, error) {
			value, err := source.EarliestEffectiveFrom(ctx)
			return value, err
		},
		Edit: func(ctx context.Context, id int64, input parentportal.CreateOfferingChangeInputSnapshot, version string) (*parentportal.OfferingChangeRequestSnapshot, error) {
			value, err := source.Edit(ctx, id, parentCarePlanToOwnerCreateOfferingChangeInput(input), version)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerOfferingChangeRequest), err
		},
		GetForStudent: func(ctx context.Context, id int64) (*parentportal.OfferingChangeViewSnapshot, error) {
			value, err := source.GetForStudent(ctx, id)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerOfferingChangeView), err
		},
		SubmitOfferingChange: func(ctx context.Context, input parentportal.CreateOfferingChangeInputSnapshot) (*parentportal.OfferingChangeRequestSnapshot, error) {
			value, err := source.SubmitOfferingChange(ctx, parentCarePlanToOwnerCreateOfferingChangeInput(input))
			return parentCarePlanPointer(value, parentCarePlanFromOwnerOfferingChangeRequest), err
		},
		CourseCatalog: func(ctx context.Context, studentID int64, accountID int64) (*parentportal.CourseCatalogSnapshot, error) {
			value, err := source.CourseCatalog(ctx, studentID, accountID)
			return parentCarePlanPointer(value, parentCarePlanFromOwnerCourseCatalog), err
		},
		CreateCourseRequest: func(ctx context.Context, input parentportal.CreateCourseRequestInputSnapshot) (*parentportal.OfferingChangeRequestSnapshot, error) {
			value, err := source.CreateCourseRequest(ctx, parentCarePlanToOwnerCreateCourseRequestInput(input))
			return parentCarePlanPointer(value, parentCarePlanFromOwnerOfferingChangeRequest), err
		},
		WithdrawCourseRequest: func(ctx context.Context, studentID int64, accountID int64, offeringID int64) error {
			return source.WithdrawCourseRequest(ctx, studentID, accountID, offeringID)
		},
	})
}
