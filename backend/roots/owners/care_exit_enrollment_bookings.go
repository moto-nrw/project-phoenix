package owners

import (
	"context"

	"github.com/moto-ogs/project-phoenix/tenant"

	carePlanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/workflows/careexit"
)

// EnrollmentBookingProjection is Enrollment's booking projection with the
// care exit's snapshot and restore of Care Plan's bookings, which the care
// exit workflow decides. Care Plan locks and ends the bookings itself.
// No compatibility-view reads or writes are used.
type EnrollmentBookingProjection struct {
	*enrollment.BookingProjection
	Bookings careExitBookingWrites
}

// careExitBookingWrites are the care exit workflow's snapshot and restore of
// Care Plan's bookings that the projection forwards.
type careExitBookingWrites interface {
	CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, until enrollment.Date, sourceChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error)
	RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore, today enrollment.Date) (int64, error)
}

// NewEnrollmentBookingProjection adds the care exit's booking snapshot and
// restore to the projection the root binds, on the caller's tenant
// transaction. phases is Enrollment's phase bounds reader behind the
// recurrence gate (#6657).
func NewEnrollmentBookingProjection(projection *enrollment.BookingProjection, phases enrollment.PhaseBoundsReader) EnrollmentBookingProjection {
	return EnrollmentBookingProjection{
		BookingProjection: projection,
		Bookings:          careexit.NewBookings(projection, carePlanCompose.NewOfferingBookings(), phases, tenant.FromContext),
	}
}

func (a EnrollmentBookingProjection) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, until enrollment.Date, sourceChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error) {
	return a.Bookings.CareExitOfferingSnapshots(ctx, studentIDs, until, sourceChildID)
}

func (a EnrollmentBookingProjection) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore, today enrollment.Date) (int64, error) {
	return a.Bookings.RestoreCareExitOfferingLinks(ctx, snapshots, today)
}
