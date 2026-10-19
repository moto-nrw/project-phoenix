package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// Public results are snapshots of the application values, including nil and empty.
func (p *Portal) ListExcusedRequests(ctx context.Context, accountID, studentID int64) ([]*ExcusedAbsenceRequestSnapshot, error) {
	v, err := p.ChildFlows.ListExcusedRequests(ctx, accountID, studentID)
	return carePlanValueSlice(v, func(value *care.ExcusedAbsenceRequest) *ExcusedAbsenceRequestSnapshot {
		return carePlanValuePointer(value, carePlanFromAppExcusedAbsenceRequest)
	}), err
}
func (p *Portal) EditExcusedRequest(ctx context.Context, accountID, studentID, requestID int64, dates []calendar.Date, note, expectedVersion string) (*ExcusedAbsenceRequestSnapshot, error) {
	v, err := p.ChildFlows.EditExcusedRequest(ctx, accountID, studentID, requestID, dates, note, expectedVersion)
	return carePlanValuePointer(v, carePlanFromAppExcusedAbsenceRequest), err
}
func (p *Portal) GetChildOfferingCatalog(ctx context.Context, accountID, studentID int64) (*OfferingChangeCatalogSnapshot, error) {
	v, err := p.ChildFlows.GetChildOfferingCatalog(ctx, accountID, studentID)
	return carePlanValuePointer(v, carePlanFromAppOfferingChangeCatalog), err
}
func (p *Portal) GetChildOfferingCatalogAt(ctx context.Context, accountID, studentID int64, effectiveFrom calendar.Date) (*OfferingChangeCatalogSnapshot, error) {
	v, err := p.ChildFlows.GetChildOfferingCatalogAt(ctx, accountID, studentID, effectiveFrom)
	return carePlanValuePointer(v, carePlanFromAppOfferingChangeCatalog), err
}
func (p *Portal) GetChildCourses(ctx context.Context, accountID, studentID int64) (*CourseCatalogSnapshot, error) {
	v, err := p.ChildFlows.GetChildCourses(ctx, accountID, studentID)
	return carePlanValuePointer(v, carePlanFromAppCourseCatalog), err
}
func (p *Portal) RequestChildCourse(ctx context.Context, accountID, studentID, offeringID int64, note string) (*CourseCatalogSnapshot, error) {
	v, err := p.ChildFlows.RequestChildCourse(ctx, accountID, studentID, offeringID, note)
	return carePlanValuePointer(v, carePlanFromAppCourseCatalog), err
}
func (p *Portal) WithdrawChildCourseRequest(ctx context.Context, accountID, studentID, requestID int64) (*CourseCatalogSnapshot, error) {
	v, err := p.ChildFlows.WithdrawChildCourseRequest(ctx, accountID, studentID, requestID)
	return carePlanValuePointer(v, carePlanFromAppCourseCatalog), err
}
func (p *Portal) GetChildCareOfferings(ctx context.Context, accountID, studentID int64) (*ChildCareOfferings, error) {
	v, err := p.ChildFlows.GetChildCareOfferings(ctx, accountID, studentID)
	return carePlanValuePointer(v, carePlanFromAppChildCareOfferings), err
}
func (p *Portal) CreateOfferingChangeRequest(ctx context.Context, accountID, studentID int64, selections []OfferingChangeSelectionSnapshot, effectiveFrom calendar.Date, note string, completeWithdrawalConfirmed bool, recipientGuardianProfileIDs []int64) (*ChildCareOfferings, error) {
	v, err := p.ChildFlows.CreateOfferingChangeRequest(ctx, accountID, studentID, carePlanValueSlice(selections, carePlanToAppOfferingChangeSelection), effectiveFrom, note, completeWithdrawalConfirmed, recipientGuardianProfileIDs)
	return carePlanValuePointer(v, carePlanFromAppChildCareOfferings), err
}
func (p *Portal) EditOfferingChangeRequest(ctx context.Context, accountID, studentID, requestID int64, selections []OfferingChangeSelectionSnapshot, effectiveFrom calendar.Date, note string, completeWithdrawalConfirmed bool, expectedVersion string) (*ChildCareOfferings, error) {
	v, err := p.ChildFlows.EditOfferingChangeRequest(ctx, accountID, studentID, requestID, carePlanValueSlice(selections, carePlanToAppOfferingChangeSelection), effectiveFrom, note, completeWithdrawalConfirmed, expectedVersion)
	return carePlanValuePointer(v, carePlanFromAppChildCareOfferings), err
}
func (p *Portal) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []calendar.Date, reason, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	v, err := p.ChildFlows.SubmitSickNote(ctx, accountID, studentID, dates, reason, status, recipientGuardianProfileIDs)
	return carePlanValuePointer(v, carePlanFromAppSickNoteResult), err
}

func (p *Portal) ListSickDays(ctx context.Context, accountID, studentID int64, from, to calendar.Date) ([]*StudentStatusDaySnapshot, error) {
	values, err := p.ChildFlows.ListSickDays(ctx, accountID, studentID, from, to)
	if values == nil {
		return nil, err
	}
	out := make([]*StudentStatusDaySnapshot, len(values))
	for i, row := range values {
		if row != nil {
			out[i] = &StudentStatusDaySnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: row.Date, Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
		}
	}
	return out, err
}
