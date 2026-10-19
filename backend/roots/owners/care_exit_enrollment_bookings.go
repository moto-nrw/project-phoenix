package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/tenant"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
)

// EnrollmentBookingProjection is Enrollment's booking projection with the
// care exit's writes to Care Plan's bookings: the lock, the end, the snapshot
// and the restore of the bookings a care exit ends, which the care exit
// workflow decides. No compatibility-view reads or writes are used.
type EnrollmentBookingProjection struct {
	*enrollment.BookingProjection
	Bookings careExitBookingWrites
}

// careExitBookingWrites are the care exit workflow's writes to Care Plan's
// bookings that the projection forwards.
type careExitBookingWrites interface {
	LockCareExitOfferingLinks(ctx context.Context, childIDs []int64, from enrollment.Date) error
	EndCareExitOfferingLinks(ctx context.Context, childIDs []int64, sourceChildID *int64, until enrollment.Date) (int64, error)
	CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, until enrollment.Date, sourceChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error)
	RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore) (int64, error)
}

// NewEnrollmentBookingProjection adds the care exit's booking writes to the
// projection the root binds, on the caller's tenant transaction.
func NewEnrollmentBookingProjection(projection *enrollment.BookingProjection) EnrollmentBookingProjection {
	return EnrollmentBookingProjection{
		BookingProjection: projection,
		Bookings:          careexit.NewBookings(projection, carePlanCompose.NewOfferingBookings(), tenant.FromContext),
	}
}

func (a EnrollmentBookingProjection) LockCareExitOfferingLinks(ctx context.Context, childIDs []int64, from enrollment.Date) error {
	return a.Bookings.LockCareExitOfferingLinks(ctx, childIDs, from)
}

func (a EnrollmentBookingProjection) EndCareExitOfferingLinks(ctx context.Context, childIDs []int64, sourceChildID *int64, until enrollment.Date) (int64, error) {
	return a.Bookings.EndCareExitOfferingLinks(ctx, childIDs, sourceChildID, until)
}

func (a EnrollmentBookingProjection) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, until enrollment.Date, sourceChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error) {
	return a.Bookings.CareExitOfferingSnapshots(ctx, studentIDs, until, sourceChildID)
}

func (a EnrollmentBookingProjection) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore) (int64, error) {
	return a.Bookings.RestoreCareExitOfferingLinks(ctx, snapshots)
}
