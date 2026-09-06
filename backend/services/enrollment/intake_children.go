package enrollment

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type RequestChildrenReader interface {
	ChildrenForRequest(ctx context.Context, requestID int64, forUpdate bool) ([]*capability.RequestChild, error)
}

type ChildCreator interface {
	InsertChild(context.Context, *capability.RequestChild) error
}

type RolloverChildren interface {
	RequestChildOfferingsAtDate(context.Context, int64, capability.Date) ([]*capability.RequestChildOffering, error)
	RequestChildOfferingsForChildrenAtDate(context.Context, []int64, capability.Date) ([]*capability.RequestChildOffering, error)
	InsertRequestChildOffering(context.Context, *capability.RequestChildOffering) error
	ChildCreator
	ChildrenByID(context.Context, []int64) ([]*capability.RequestChild, error)
	ChildrenByPhaseStatuses(context.Context, int64, []string) ([]*capability.RequestChild, error)
	ReviewRolloverChild(context.Context, int64, string, *string, *int16, int64) error
	TransitionPhaseChildren(context.Context, int64, string, string) (int, error)
}

func rolloverChildrenByStatuses(ctx context.Context, owner RolloverChildren, phaseID int64, statuses []string) ([]*enrollmentModels.RequestChild, error) {
	values, err := owner.ChildrenByPhaseStatuses(ctx, phaseID, statuses)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}

func rolloverChildrenByID(ctx context.Context, owner RolloverChildren, ids []int64) ([]*enrollmentModels.RequestChild, error) {
	values, err := owner.ChildrenByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}

type ChildIDReader interface {
	ChildByID(context.Context, int64) (*capability.RequestChild, error)
}

type DecisionChildren interface {
	OfferingCapacityReader
	OfferingSelectionBatchReader
	OfferingSelectionReader
	OfferingSelectionWriter
	ChildIDReader
	RequestChildrenReader
	ReportChildren
	RestoreWithdrawnChildren(context.Context, int64, []int64) ([]int64, error)
	UpdateChildStatus(context.Context, int64, string, *string, int64) error
	UpdateChildActivationPlan(context.Context, int64, string, *capability.Date) error
	LinkCreatedStudent(context.Context, int64, int64) error
}

func updateDecisionActivationPlan(ctx context.Context, owner DecisionChildren, id int64, mode string, on *timezone.Date) error {
	var date *capability.Date
	if on != nil {
		value := capability.Date(on.String())
		date = &value
	}
	return owner.UpdateChildActivationPlan(ctx, id, mode, date)
}

type OfferingChildrenReader interface {
	OfferingCapacityPeak(context.Context, int64, []int64, capability.Date, capability.Date) (int, error)
	RequestChildOfferingsAtDates(context.Context, map[int64]capability.Date) ([]*capability.RequestChildOffering, error)
	OfferingSelectionReader
	ChildByID(context.Context, int64) (*capability.RequestChild, error)
	ChildrenByID(context.Context, []int64) ([]*capability.RequestChild, error)
	StudentCarePeriods(context.Context, int64) ([]*capability.StudentCarePeriod, error)
}

func offeringChildByID(ctx context.Context, owner ChildIDReader, id int64) (*enrollmentModels.RequestChild, error) {
	value, err := owner.ChildByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return intakeChildValue(value)
}

func offeringChildrenByID(ctx context.Context, owner OfferingChildrenReader, ids []int64) ([]*enrollmentModels.RequestChild, error) {
	values, err := owner.ChildrenByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}

// StudentCarePeriod is one approved enrollment that materialized into a
// student, together with the care window of its phase. The parents portal
// needs the window to decide which care period is the current one for a child
// that is enrolled across several school years (#1665).
type StudentCarePeriod struct {
	RequestChildID   int64
	RequestID        int64
	PhaseID          int64
	PhaseName        string
	ServiceStartDate timezone.Date
	ServiceEndDate   timezone.Date
}

type StudentCarePeriodReader interface {
	StudentCarePeriods(context.Context, int64) ([]*capability.StudentCarePeriod, error)
}

func ReadStudentCarePeriods(ctx context.Context, owner StudentCarePeriodReader, studentID int64) ([]*StudentCarePeriod, error) {
	periods, err := owner.StudentCarePeriods(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]*StudentCarePeriod, 0, len(periods))
	for _, period := range periods {
		start, err := timezone.ParseDate(string(period.ServiceStartDate))
		if err != nil {
			return nil, err
		}
		end, err := timezone.ParseDate(string(period.ServiceEndDate))
		if err != nil {
			return nil, err
		}
		result = append(result, &StudentCarePeriod{RequestChildID: period.RequestChildID, RequestID: period.RequestID, PhaseID: period.PhaseID, PhaseName: period.PhaseName, ServiceStartDate: start, ServiceEndDate: end})
	}
	return result, nil
}

// IntakeChildren is the Enrollment child capability used by intake and edits.
type IntakeChildren interface {
	OfferingSelectionBatchReader
	OfferingCapacityReader
	ReplaceRequestChildOfferings(context.Context, int64, []*capability.RequestChildOffering) error
	InsertRequestChildOffering(context.Context, *capability.RequestChildOffering) error
	InsertChild(context.Context, *capability.RequestChild) error
	ChildrenForRequest(context.Context, int64, bool) ([]*capability.RequestChild, error)
	ChildrenForRequests(context.Context, []int64) ([]*capability.RequestChild, error)
	DeleteRequestChildren(context.Context, int64) error
	UpdateChildStatus(context.Context, int64, string, *string, int64) error
	UpdateChildData(context.Context, *capability.RequestChild) error
	UpdateMatchedStudent(context.Context, int64, *int64) error
}

func createIntakeChild(ctx context.Context, owner ChildCreator, child *enrollmentModels.RequestChild) error {
	value, err := intakeChildInput(child)
	if err != nil {
		return err
	}
	if err := owner.InsertChild(ctx, value); err != nil {
		return err
	}
	result, err := intakeChildValue(value)
	if err != nil {
		return err
	}
	*child = *result
	return nil
}
func updateIntakeChild(ctx context.Context, owner IntakeChildren, child *enrollmentModels.RequestChild) error {
	value, err := intakeChildInput(child)
	if err != nil {
		return err
	}
	return owner.UpdateChildData(ctx, value)
}
func listIntakeChildren(ctx context.Context, owner RequestChildrenReader, requestID int64, forUpdate bool) ([]*enrollmentModels.RequestChild, error) {
	values, err := owner.ChildrenForRequest(ctx, requestID, forUpdate)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}
func intakeChildInput(r *enrollmentModels.RequestChild) (*capability.RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &capability.RequestChild{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestID: r.RequestID, FirstName: r.FirstName, LastName: r.LastName, TargetGradeLevel: r.TargetGradeLevel, TargetSchoolClass: r.TargetSchoolClass, Status: r.Status, StatusReason: r.StatusReason, ActivationMode: r.ActivationMode, ReviewedAt: r.ReviewedAt, ReviewedBy: r.ReviewedBy, CreatedStudentID: r.CreatedStudentID, MatchedStudentID: r.MatchedStudentID, SortOrder: r.SortOrder, RolloverSourceChildID: r.RolloverSourceChildID, ReviewReason: r.ReviewReason}
	result.DateOfBirth = r.DateOfBirth
	if r.ActivateOn != nil {
		date := *r.ActivateOn
		result.ActivateOn = &date
	}
	data, err := json.Marshal(r.CustomData)
	if err != nil {
		return nil, err
	}
	result.CustomData = data
	return result, nil
}
func intakeChildValue(r *capability.RequestChild) (*enrollmentModels.RequestChild, error) {
	if r == nil {
		return nil, nil
	}
	result := &enrollmentModels.RequestChild{}
	result.ID = r.ID
	result.TenantID = r.TenantID
	result.CreatedAt = r.CreatedAt
	result.UpdatedAt = r.UpdatedAt
	result.RequestID = r.RequestID
	result.FirstName = r.FirstName
	result.LastName = r.LastName
	result.TargetGradeLevel = r.TargetGradeLevel
	result.TargetSchoolClass = r.TargetSchoolClass
	result.Status = r.Status
	result.StatusReason = r.StatusReason
	result.ActivationMode = r.ActivationMode
	result.ReviewedAt = r.ReviewedAt
	result.ReviewedBy = r.ReviewedBy
	result.CreatedStudentID = r.CreatedStudentID
	result.MatchedStudentID = r.MatchedStudentID
	result.SortOrder = r.SortOrder
	result.RolloverSourceChildID = r.RolloverSourceChildID
	result.ReviewReason = r.ReviewReason
	dob, err := timezone.ParseDate(string(r.DateOfBirth))
	if err != nil {
		return nil, err
	}
	result.DateOfBirth = capability.Date(dob.String())
	if r.ActivateOn != nil {
		date, err := timezone.ParseDate(string(*r.ActivateOn))
		if err != nil {
			return nil, err
		}
		ownerDate := capability.Date(date.String())
		result.ActivateOn = &ownerDate
	}
	if len(r.CustomData) > 0 {
		if err := json.Unmarshal(r.CustomData, &result.CustomData); err != nil {
			return nil, err
		}
	}
	return result, nil
}
func intakeChildValues(values []*capability.RequestChild) ([]*enrollmentModels.RequestChild, error) {
	var result []*enrollmentModels.RequestChild
	for _, value := range values {
		converted, err := intakeChildValue(value)
		if err != nil {
			return nil, err
		}
		result = append(result, converted)
	}
	return result, nil
}

type ReportChildren interface {
	RequestChildOfferingsForChildrenAtDate(context.Context, []int64, capability.Date) ([]*capability.RequestChildOffering, error)
	ChildrenForRequests(context.Context, []int64) ([]*capability.RequestChild, error)
}

func listIntakeChildrenForRequests(ctx context.Context, owner ReportChildren, requestIDs []int64) ([]*enrollmentModels.RequestChild, error) {
	values, err := owner.ChildrenForRequests(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	return intakeChildValues(values)
}
