package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Public results are the application values themselves, including nil and empty.
func (p *Portal) ListExcusedRequests(ctx context.Context, accountID, studentID int64) ([]*ExcusedAbsenceRequestSnapshot, error) {
	return p.ChildFlows.ListExcusedRequests(ctx, accountID, studentID)
}
func (p *Portal) EditExcusedRequest(ctx context.Context, accountID, studentID, requestID int64, dates []calendar.Date, note, expectedVersion string) (*ExcusedAbsenceRequestSnapshot, error) {
	return p.ChildFlows.EditExcusedRequest(ctx, accountID, studentID, requestID, dates, note, expectedVersion)
}
func (p *Portal) GetChildOfferingCatalog(ctx context.Context, accountID, studentID int64) (*OfferingChangeCatalogSnapshot, error) {
	return p.ChildFlows.GetChildOfferingCatalog(ctx, accountID, studentID)
}
func (p *Portal) GetChildOfferingCatalogAt(ctx context.Context, accountID, studentID int64, effectiveFrom calendar.Date) (*OfferingChangeCatalogSnapshot, error) {
	return p.ChildFlows.GetChildOfferingCatalogAt(ctx, accountID, studentID, effectiveFrom)
}
func (p *Portal) GetChildCourses(ctx context.Context, accountID, studentID int64) (*CourseCatalogSnapshot, error) {
	return p.ChildFlows.GetChildCourses(ctx, accountID, studentID)
}
func (p *Portal) RequestChildCourse(ctx context.Context, accountID, studentID, offeringID int64, note string) (*CourseCatalogSnapshot, error) {
	return p.ChildFlows.RequestChildCourse(ctx, accountID, studentID, offeringID, note)
}
func (p *Portal) WithdrawChildCourseRequest(ctx context.Context, accountID, studentID, requestID int64) (*CourseCatalogSnapshot, error) {
	return p.ChildFlows.WithdrawChildCourseRequest(ctx, accountID, studentID, requestID)
}
func (p *Portal) GetChildCareOfferings(ctx context.Context, accountID, studentID int64) (*ChildCareOfferings, error) {
	return p.ChildFlows.GetChildCareOfferings(ctx, accountID, studentID)
}
func (p *Portal) CreateOfferingChangeRequest(ctx context.Context, accountID, studentID int64, selections []OfferingChangeSelectionSnapshot, effectiveFrom calendar.Date, note string, completeWithdrawalConfirmed bool, recipientGuardianProfileIDs []int64) (*ChildCareOfferings, error) {
	return p.ChildFlows.CreateOfferingChangeRequest(ctx, accountID, studentID, selections, effectiveFrom, note, completeWithdrawalConfirmed, recipientGuardianProfileIDs)
}
func (p *Portal) EditOfferingChangeRequest(ctx context.Context, accountID, studentID, requestID int64, selections []OfferingChangeSelectionSnapshot, effectiveFrom calendar.Date, note string, completeWithdrawalConfirmed bool, expectedVersion string) (*ChildCareOfferings, error) {
	return p.ChildFlows.EditOfferingChangeRequest(ctx, accountID, studentID, requestID, selections, effectiveFrom, note, completeWithdrawalConfirmed, expectedVersion)
}
func (p *Portal) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []calendar.Date, reason, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	v, err := p.ChildFlows.SubmitSickNote(ctx, accountID, studentID, dates, reason, status, recipientGuardianProfileIDs)
	if v == nil {
		return nil, err
	}
	result := carePlanFromAppSickNoteResult(*v)
	return &result, err
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
