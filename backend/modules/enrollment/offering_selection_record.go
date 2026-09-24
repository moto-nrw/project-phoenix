package enrollment

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RequestChildOfferingRecord is one effective care booking of a request
// child, with its validity as calendar days - the selection linking a
// request child to a care offering.
type RequestChildOfferingRecord struct {
	ID                    int64     `json:"id"`
	TenantID              int64     `json:"tenant_id"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
	RequestChildID        int64     `json:"request_child_id"`
	CareOfferingID        int64     `json:"care_offering_id"`
	SelectedDays          []string  `json:"selected_days,omitempty"`
	ManualSelectedDays    []string  `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string  `json:"automatic_selected_days,omitempty"`
	Notes                 *string   `json:"notes,omitempty"`
	// ValidFrom / ValidUntil make an approved offering switch effective on its
	// requested date. ValidUntil is exclusive, matching student enrollments.
	ValidFrom  *calendar.Date `json:"valid_from,omitempty"`
	ValidUntil *calendar.Date `json:"valid_until,omitempty"`
}

// ApprovedOfferingChild is one approved, still-relevant offering selection
// with the enrollment-to-student resolution the offering-source flows need
// (#2137): roster resync of sourced Regeltermine and the editor's per-grade
// count preview.
type ApprovedOfferingChild struct {
	Link        *RequestChildOfferingRecord
	StudentID   int64
	SchoolClass string
}

// StudentCarePeriodRecord is one approved enrollment that materialized into
// a student, together with the care window of its phase as calendar days.
// The parents portal needs the window to decide which care period is the
// current one for a child that is enrolled across several school years
// (#1665).
type StudentCarePeriodRecord struct {
	RequestChildID    int64
	RequestID         int64
	PhaseID           int64
	PhaseName         string
	ServiceStartDate  calendar.Date
	ServiceEndDate    calendar.Date
	TargetGradeLevel  *int16
	TargetSchoolClass *string
}

// RequestChildOfferingRecordsOf converts offering selections, keeping the
// position of nil entries.
func RequestChildOfferingRecordsOf(values []*RequestChildOffering) []*RequestChildOfferingRecord {
	if values == nil {
		return nil
	}
	selections := make([]*RequestChildOfferingRecord, len(values))
	for index, value := range values {
		if value == nil {
			continue
		}
		selection := &RequestChildOfferingRecord{
			RequestChildID: value.RequestChildID, CareOfferingID: value.CareOfferingID,
			SelectedDays: value.SelectedDays, ManualSelectedDays: value.ManualSelectedDays,
			AutomaticSelectedDays: value.AutomaticSelectedDays, Notes: value.Notes,
		}
		selection.ID, selection.TenantID = value.ID, value.TenantID
		selection.CreatedAt, selection.UpdatedAt = value.CreatedAt, value.UpdatedAt
		if value.ValidFrom != nil {
			date := calendar.Date(*value.ValidFrom)
			selection.ValidFrom = &date
		}
		if value.ValidUntil != nil {
			date := calendar.Date(*value.ValidUntil)
			selection.ValidUntil = &date
		}
		selections[index] = selection
	}
	return selections
}

// OfferingSelectionRecordsAt reads the offering selections of one child that
// are effective on the given day.
func OfferingSelectionRecordsAt(ctx context.Context, owner interface {
	RequestChildOfferingsAtDate(context.Context, int64, Date) ([]*RequestChildOffering, error)
}, childID int64, onDate calendar.Date) ([]*RequestChildOfferingRecord, error) {
	values, err := owner.RequestChildOfferingsAtDate(ctx, childID, Date(onDate))
	if err != nil {
		return nil, err
	}
	return RequestChildOfferingRecordsOf(values), nil
}

// OfferingHistoryRecords reads every offering selection one child ever had.
func OfferingHistoryRecords(ctx context.Context, owner interface {
	RequestChildOfferingHistory(context.Context, int64) ([]*RequestChildOffering, error)
}, childID int64) ([]*RequestChildOfferingRecord, error) {
	values, err := owner.RequestChildOfferingHistory(ctx, childID)
	if err != nil {
		return nil, err
	}
	return RequestChildOfferingRecordsOf(values), nil
}

// OfferingHistoryRecordsForChildren reads the offering history of several
// children.
func OfferingHistoryRecordsForChildren(ctx context.Context, owner interface {
	RequestChildOfferingHistoryForChildren(context.Context, []int64) ([]*RequestChildOffering, error)
}, childIDs []int64) ([]*RequestChildOfferingRecord, error) {
	values, err := owner.RequestChildOfferingHistoryForChildren(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	return RequestChildOfferingRecordsOf(values), nil
}

// OfferingSelectionRecordsForChildrenAt reads the offering selections of
// several children that are effective on the given day.
func OfferingSelectionRecordsForChildrenAt(ctx context.Context, owner interface {
	RequestChildOfferingsForChildrenAtDate(context.Context, []int64, Date) ([]*RequestChildOffering, error)
}, childIDs []int64, onDate calendar.Date) ([]*RequestChildOfferingRecord, error) {
	values, err := owner.RequestChildOfferingsForChildrenAtDate(ctx, childIDs, Date(onDate))
	if err != nil {
		return nil, err
	}
	return RequestChildOfferingRecordsOf(values), nil
}

// StudentCarePeriodRecords reads the care periods of one student with their
// windows as calendar days.
func StudentCarePeriodRecords(ctx context.Context, owner interface {
	StudentCarePeriods(context.Context, int64) ([]*StudentCarePeriod, error)
}, studentID int64) ([]*StudentCarePeriodRecord, error) {
	periods, err := owner.StudentCarePeriods(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]*StudentCarePeriodRecord, 0, len(periods))
	for _, period := range periods {
		start, err := calendar.ParseDate(string(period.ServiceStartDate))
		if err != nil {
			return nil, err
		}
		end, err := calendar.ParseDate(string(period.ServiceEndDate))
		if err != nil {
			return nil, err
		}
		result = append(result, &StudentCarePeriodRecord{
			RequestChildID: period.RequestChildID, RequestID: period.RequestID, PhaseID: period.PhaseID,
			PhaseName: period.PhaseName, ServiceStartDate: start, ServiceEndDate: end,
			TargetGradeLevel: period.TargetGradeLevel, TargetSchoolClass: period.TargetSchoolClass,
		})
	}
	return result, nil
}
