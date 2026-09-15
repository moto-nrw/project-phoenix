package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// EnrollmentBookingProjection joins application identity with owner-provided effective
// bookings. No compatibility-view reads or writes are used by this workflow.
type EnrollmentBookingProjection struct {
	*enrollment.Module
	bookings *careplan.OfferingBookings
}

// NewEnrollmentBookingProjection composes application identity and effective bookings
// for cleanup and consistency audits on the caller's tenant transaction.
func NewEnrollmentBookingProjection(applications *enrollment.Module) EnrollmentBookingProjection {
	return EnrollmentBookingProjection{Module: applications, bookings: carePlanCompose.NewOfferingBookings()}
}

func (a EnrollmentBookingProjection) AllCareOfferingLinks(ctx context.Context) ([]enrollment.CareOfferingLink, error) {
	return a.CareExitOfferingLinks(ctx, nil)
}

func (a EnrollmentBookingProjection) CareExitOfferingLinks(ctx context.Context, studentIDs []int64) ([]enrollment.CareOfferingLink, error) {
	children, err := a.CareExitApplicationLinks(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.ID)
	}
	return a.offeringLinks(ctx, ids)
}

func (a EnrollmentBookingProjection) ApprovedBookingOfferingLinks(ctx context.Context) ([]enrollment.CareOfferingLink, error) {
	children, err := a.CareExitApplicationLinks(ctx, nil)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child.Status == enrollment.ChildStatusApproved {
			ids = append(ids, child.ID)
		}
	}
	return a.offeringLinks(ctx, ids)
}

func (a EnrollmentBookingProjection) offeringLinks(ctx context.Context, ids []int64) ([]enrollment.CareOfferingLink, error) {
	bookings, err := a.bookings.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	links := make([]enrollment.CareOfferingLink, 0, len(bookings))
	for _, booking := range bookings {
		links = append(links, enrollment.CareOfferingLink{ID: booking.ID, TenantID: booking.TenantID,
			RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID, SelectedDays: booking.EffectiveSelectedDays(),
			ValidFrom: enrollmentBookingDate(booking.ValidFrom), ValidUntil: enrollmentBookingDate(booking.ValidUntil)})
	}
	sort.Slice(links, func(i, j int) bool { return links[i].ID < links[j].ID })
	return links, nil
}

func enrollmentBookingDate(date *careplan.Date) *enrollment.Date {
	if date == nil {
		return nil
	}
	return new(enrollment.Date(*date))
}

func carePlanBookingDate(date *enrollment.Date) *careplan.Date {
	if date == nil {
		return nil
	}
	return new(careplan.Date(*date))
}

func (a EnrollmentBookingProjection) LockCareExitOfferingLinks(ctx context.Context, childIDs []int64, _ enrollment.Date) error {
	return a.bookings.LockCareOfferingBookings(ctx, childIDs)
}

func (a EnrollmentBookingProjection) EndCareExitOfferingLinks(ctx context.Context, childIDs []int64, sourceChildID *int64, until enrollment.Date) (int64, error) {
	if sourceChildID != nil {
		if !slices.Contains(childIDs, *sourceChildID) {
			return 0, nil
		}
		childIDs = []int64{*sourceChildID}
	}
	return a.bookings.EndCareOfferingBookings(ctx, childIDs, careplan.Date(until))
}

func (a EnrollmentBookingProjection) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, until enrollment.Date, sourceChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error) {
	if len(studentIDs) == 0 {
		return []enrollment.CareExitOfferingSnapshot{}, nil
	}
	children, err := a.CareExitApplicationLinks(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(children))
	students := make(map[int64]int64, len(children))
	for _, child := range children {
		if child.CreatedStudentID == nil || !slices.Contains(studentIDs, *child.CreatedStudentID) || (sourceChildID != nil && child.ID != *sourceChildID) {
			continue
		}
		ids = append(ids, child.ID)
		students[child.ID] = *child.CreatedStudentID
	}
	bookings, err := a.bookings.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	choices, err := a.SubmittedOfferingChoices(ctx, ids)
	if err != nil {
		return nil, err
	}
	notes := make(map[[2]int64]*string, len(choices))
	for _, choice := range choices {
		notes[[2]int64{choice.RequestChildID, choice.CareOfferingID}] = choice.Notes
	}
	result := make([]enrollment.CareExitOfferingSnapshot, 0, len(bookings))
	for _, booking := range bookings {
		if booking.ValidUntil != nil && !careplan.Date(until).Before(*booking.ValidUntil) {
			continue
		}
		// Keep the ledger's previous-image JSON contract. This is a snapshot,
		// not a second write to the legacy storage provider.
		row := enrollment.RequestChildOffering{ID: booking.ID, TenantID: booking.TenantID,
			RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID,
			SelectedDays: booking.EffectiveSelectedDays(), ManualSelectedDays: booking.ManualSelectedDays, AutomaticSelectedDays: booking.AutomaticSelectedDays,
			Notes:     notes[[2]int64{booking.RequestChildID, booking.CareOfferingID}],
			CreatedAt: booking.CreatedAt, UpdatedAt: booking.UpdatedAt, ValidFrom: enrollmentBookingDate(booking.ValidFrom), ValidUntil: enrollmentBookingDate(booking.ValidUntil)}
		snapshot, err := json.Marshal(row)
		if err != nil {
			return nil, err
		}
		result = append(result, enrollment.CareExitOfferingSnapshot{TenantID: booking.TenantID, StudentID: students[booking.RequestChildID],
			RequestChildID: booking.RequestChildID, SourceRowID: booking.ID,
			WasDeleted: booking.ValidFrom != nil && !booking.ValidFrom.Before(careplan.Date(until)), Snapshot: snapshot})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceRowID < result[j].SourceRowID })
	return result, nil
}

func (a EnrollmentBookingProjection) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore) (int64, error) {
	restores := make([]careplan.CareOfferingBookingRestore, 0, len(snapshots))
	for _, snapshot := range snapshots {
		var row enrollment.RequestChildOffering
		if err := json.Unmarshal(snapshot.Snapshot, &row); err != nil {
			return 0, fmt.Errorf("decode care-exit offering snapshot: %w", err)
		}
		if row.ID != snapshot.SourceRowID {
			return 0, fmt.Errorf("care-exit snapshot booking identity mismatch")
		}
		manual := row.ManualSelectedDays
		if len(manual) == 0 && len(row.AutomaticSelectedDays) == 0 {
			manual = row.SelectedDays
		}
		restores = append(restores, careplan.CareOfferingBookingRestore{WasDeleted: snapshot.WasDeleted, Booking: careplan.CareOfferingBooking{
			ID: row.ID, TenantID: row.TenantID, RequestChildID: row.RequestChildID, CareOfferingID: row.CareOfferingID,
			ManualSelectedDays: manual, AutomaticSelectedDays: row.AutomaticSelectedDays,
			ValidFrom: carePlanBookingDate(row.ValidFrom), ValidUntil: carePlanBookingDate(row.ValidUntil), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}})
	}
	return a.bookings.RestoreCareOfferingBookings(ctx, restores)
}
