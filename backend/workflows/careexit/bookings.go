package careexit

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// BookingApplications is the Enrollment half of the offering bookings: the
// applications that created the children and the notes of their submitted
// offering choices.
type BookingApplications interface {
	CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]enrollment.CareExitApplicationLink, error)
	SubmittedOfferingChoices(ctx context.Context, childIDs []int64) ([]enrollment.SubmittedOfferingChoice, error)
}

// BookingStore is the Care Plan half: the booked offering intervals of the
// request children a care exit locks, ends, snapshots and restores.
type BookingStore interface {
	CareOfferingBookingHistory(ctx context.Context, childIDs []int64) ([]careplan.CareOfferingBooking, error)
	LockCareOfferingBookings(ctx context.Context, childIDs []int64) error
	EndCareOfferingBookings(ctx context.Context, childIDs []int64, until careplan.Date) (int64, error)
	RestoreCareOfferingBookings(ctx context.Context, restores []careplan.CareOfferingBookingRestore) (int64, error)
}

// Bookings ends and restores the Care Plan offering bookings behind the
// Enrollment applications of the children whose care ends. The ledger keeps
// each ended booking as an Enrollment offering snapshot.
type Bookings struct {
	applications BookingApplications
	store        BookingStore
	tenantID     func(context.Context) int64
}

// NewBookings binds the bookings to their owners. tenantID reads the school
// of the request, the boundary of every snapshot. Every dependency is
// required; a missing one is a composition error.
func NewBookings(applications BookingApplications, store BookingStore, tenantID func(context.Context) int64) Bookings {
	if applications == nil || store == nil || tenantID == nil {
		panic("care exit bookings: enrollment applications, care plan bookings and the tenant are required")
	}
	return Bookings{applications: applications, store: store, tenantID: tenantID}
}

func (b Bookings) LockCareExitOfferingLinks(ctx context.Context, childIDs []int64, _ enrollment.Date) error {
	return b.store.LockCareOfferingBookings(ctx, childIDs)
}

// EndCareExitOfferingLinks ends the bookings of the children. With a source
// child only that child's bookings end, and only when it is one of them.
func (b Bookings) EndCareExitOfferingLinks(ctx context.Context, childIDs []int64, sourceChildID *int64, until enrollment.Date) (int64, error) {
	if sourceChildID != nil {
		if !slices.Contains(childIDs, *sourceChildID) {
			return 0, nil
		}
		childIDs = []int64{*sourceChildID}
	}
	return b.store.EndCareOfferingBookings(ctx, childIDs, careplan.Date(until))
}

// CareExitOfferingSnapshots snapshots the bookings the end would take: every
// booking of the selected children still running after until. A booking that
// starts on or after until is deleted by the end, not capped. Only snapshots
// of the request's school are kept: the ledger that stores them is the care
// exit's reversible record.
func (b Bookings) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, until enrollment.Date, sourceChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error) {
	if len(studentIDs) == 0 {
		return []enrollment.CareExitOfferingSnapshot{}, nil
	}
	children, err := b.applications.CareExitApplicationLinks(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	ids, students := bookingChildren(children, studentIDs, sourceChildID)
	bookings, err := b.store.CareOfferingBookingHistory(ctx, ids)
	if err != nil {
		return nil, err
	}
	notes, err := b.choiceNotes(ctx, ids)
	if err != nil {
		return nil, err
	}
	tenantID := b.tenantID(ctx)
	result := make([]enrollment.CareExitOfferingSnapshot, 0, len(bookings))
	for _, booking := range bookings {
		if booking.ValidUntil != nil && !careplan.Date(until).Before(*booking.ValidUntil) {
			continue
		}
		snapshot, err := bookingSnapshot(booking, notes, students, until)
		if err != nil {
			return nil, err
		}
		if snapshot.TenantID != tenantID {
			continue
		}
		result = append(result, snapshot)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceRowID < result[j].SourceRowID })
	return result, nil
}

// choiceNotes keys the notes of the submitted offering choices by child and
// offering.
func (b Bookings) choiceNotes(ctx context.Context, childIDs []int64) (map[[2]int64]*string, error) {
	choices, err := b.applications.SubmittedOfferingChoices(ctx, childIDs)
	if err != nil {
		return nil, err
	}
	notes := make(map[[2]int64]*string, len(choices))
	for _, choice := range choices {
		notes[[2]int64{choice.RequestChildID, choice.CareOfferingID}] = choice.Notes
	}
	return notes, nil
}

// bookingSnapshot keeps the ledger's previous-image JSON contract. This is a
// snapshot, not a second write to the legacy storage provider.
func bookingSnapshot(booking careplan.CareOfferingBooking, notes map[[2]int64]*string, students map[int64]int64, until enrollment.Date) (enrollment.CareExitOfferingSnapshot, error) {
	row := enrollment.RequestChildOffering{ID: booking.ID, TenantID: booking.TenantID,
		RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID,
		SelectedDays: booking.EffectiveSelectedDays(), ManualSelectedDays: booking.ManualSelectedDays, AutomaticSelectedDays: booking.AutomaticSelectedDays,
		Notes:     notes[[2]int64{booking.RequestChildID, booking.CareOfferingID}],
		CreatedAt: booking.CreatedAt, UpdatedAt: booking.UpdatedAt, ValidFrom: enrollmentDate(booking.ValidFrom), ValidUntil: enrollmentDate(booking.ValidUntil)}
	snapshot, err := json.Marshal(row)
	if err != nil {
		return enrollment.CareExitOfferingSnapshot{}, err
	}
	return enrollment.CareExitOfferingSnapshot{TenantID: booking.TenantID, StudentID: students[booking.RequestChildID],
		RequestChildID: booking.RequestChildID, SourceRowID: booking.ID,
		WasDeleted: booking.ValidFrom != nil && !booking.ValidFrom.Before(careplan.Date(until)), Snapshot: snapshot}, nil
}

// bookingChildren maps the selected application children to their students.
func bookingChildren(children []enrollment.CareExitApplicationLink, studentIDs []int64, sourceChildID *int64) ([]int64, map[int64]int64) {
	ids := make([]int64, 0, len(children))
	students := make(map[int64]int64, len(children))
	for _, child := range children {
		if child.CreatedStudentID == nil || !slices.Contains(studentIDs, *child.CreatedStudentID) || (sourceChildID != nil && child.ID != *sourceChildID) {
			continue
		}
		ids = append(ids, child.ID)
		students[child.ID] = *child.CreatedStudentID
	}
	return ids, students
}

// RestoreCareExitOfferingLinks replays the snapshots onto the bookings. A
// snapshot from before the split into manual and automatic days restores its
// selected days as manual ones.
func (b Bookings) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore) (int64, error) {
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
			ValidFrom: carePlanDate(row.ValidFrom), ValidUntil: carePlanDate(row.ValidUntil), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}})
	}
	return b.store.RestoreCareOfferingBookings(ctx, restores)
}

func enrollmentDate(date *careplan.Date) *enrollment.Date {
	if date == nil {
		return nil
	}
	return new(enrollment.Date(*date))
}

func carePlanDate(date *enrollment.Date) *careplan.Date {
	if date == nil {
		return nil
	}
	return new(careplan.Date(*date))
}
