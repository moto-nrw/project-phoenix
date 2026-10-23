package owners

import (
	"context"

	owner0 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// careExitEnrollment serves the care lifecycle's Enrollment port: the
// applications that created the children and the snapshots and restores of
// their source bookings, over the composed booking projection.
type careExitEnrollment struct {
	projection EnrollmentBookingProjection
}

func (e careExitEnrollment) CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	return e.projection.CreatedStudentRequestChildIDs(ctx, studentIDs)
}

func (e careExitEnrollment) CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]carePlanCompose.CareExitApplication, error) {
	links, err := e.projection.CareExitApplicationLinks(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.CareExitApplication, 0, len(links))
	for _, link := range links {
		result = append(result, carePlanCompose.CareExitApplication{
			ID: link.ID, TenantID: link.TenantID, CreatedStudentID: link.CreatedStudentID,
			MatchedStudentID: link.MatchedStudentID, Approved: link.IsApproved(),
		})
	}
	return result, nil
}

func (e careExitEnrollment) CareExitOfferingLinks(ctx context.Context, studentIDs []int64) ([]carePlanCompose.CareExitOfferingLink, error) {
	links, err := e.projection.CareExitOfferingLinks(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.CareExitOfferingLink, 0, len(links))
	for _, link := range links {
		result = append(result, carePlanCompose.CareExitOfferingLink{
			ID: link.ID, TenantID: link.TenantID, RequestChildID: link.RequestChildID,
			CareOfferingID: link.CareOfferingID, SelectedDays: link.SelectedDays,
			ValidFrom: enrollmentCareExitDay(link.ValidFrom), ValidUntil: enrollmentCareExitDay(link.ValidUntil),
		})
	}
	return result, nil
}

func (e careExitEnrollment) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, validUntil owner0.Date, sourceRequestChildID *int64) ([]carePlanCompose.CareExitOfferingSnapshot, error) {
	snapshots, err := e.projection.CareExitOfferingSnapshots(ctx, studentIDs, enrollment.Date(validUntil), sourceRequestChildID)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.CareExitOfferingSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		result = append(result, carePlanCompose.CareExitOfferingSnapshot{
			TenantID: snapshot.TenantID, StudentID: snapshot.StudentID, RequestChildID: snapshot.RequestChildID,
			SourceRowID: snapshot.SourceRowID, WasDeleted: snapshot.WasDeleted, Snapshot: snapshot.Snapshot,
		})
	}
	return result, nil
}

func (e careExitEnrollment) RestoreCareExitOfferingLinks(ctx context.Context, restores []carePlanCompose.CareExitOfferingRestore) (int64, error) {
	values := make([]enrollment.CareExitOfferingSnapshotRestore, 0, len(restores))
	for _, restore := range restores {
		values = append(values, enrollment.CareExitOfferingSnapshotRestore{
			SourceRowID: restore.SourceRowID, WasDeleted: restore.WasDeleted, Snapshot: restore.Snapshot,
		})
	}
	return e.projection.RestoreCareExitOfferingLinks(ctx, values)
}

func enrollmentCareExitDay(value *enrollment.Date) *owner0.Date {
	if value == nil {
		return nil
	}
	day := owner0.Date(*value)
	return &day
}
