package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/models/users"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// careExitEnrollment serves the care lifecycle's Enrollment port: the
// applications that created the children and the source bookings a care exit
// ends and restores, over the composed booking projection.
type careExitEnrollment struct{ projection EnrollmentBookingProjection }

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
			MatchedStudentID: link.MatchedStudentID, Approved: link.Status == enrollment.ChildStatusApproved,
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

func (e careExitEnrollment) LockCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, from users.CalendarDate) error {
	return e.projection.LockCareExitOfferingLinks(ctx, requestChildIDs, enrollment.Date(from))
}

// CareExitOfferingSnapshots keeps only the snapshots of the request's school:
// the ledger that stores them is the care exit's reversible record.
func (e careExitEnrollment) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, validUntil users.CalendarDate, sourceRequestChildID *int64) ([]carePlanCompose.CareExitOfferingSnapshot, error) {
	snapshots, err := e.projection.CareExitOfferingSnapshots(ctx, studentIDs, enrollment.Date(validUntil), sourceRequestChildID)
	if err != nil {
		return nil, err
	}
	tenantID := usersRepo.TenantIDFromContext(ctx)
	result := make([]carePlanCompose.CareExitOfferingSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		if snapshot.TenantID != tenantID {
			continue
		}
		result = append(result, carePlanCompose.CareExitOfferingSnapshot{
			TenantID: snapshot.TenantID, StudentID: snapshot.StudentID, RequestChildID: snapshot.RequestChildID,
			SourceRowID: snapshot.SourceRowID, WasDeleted: snapshot.WasDeleted, Snapshot: snapshot.Snapshot,
		})
	}
	return result, nil
}

func (e careExitEnrollment) EndCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, sourceRequestChildID *int64, validUntil users.CalendarDate) (int64, error) {
	return e.projection.EndCareExitOfferingLinks(ctx, requestChildIDs, sourceRequestChildID, enrollment.Date(validUntil))
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

func enrollmentCareExitDay(value *enrollment.Date) *users.CalendarDate {
	if value == nil {
		return nil
	}
	day := users.CalendarDate(*value)
	return &day
}
