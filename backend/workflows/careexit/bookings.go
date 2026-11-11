package careexit

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookingApplications is the Enrollment half of the offering bookings: the
// applications that created the children and the notes of their submitted
// offering choices.
type BookingApplications interface {
	CareExitApplicationLinks(ctx context.Context, studentIDs []int64) ([]enrollment.CareExitApplicationLink, error)
	SubmittedOfferingChoices(ctx context.Context, childIDs []int64) ([]enrollment.SubmittedOfferingChoice, error)
}

// BookingStore is the Care Plan half: the booked offering intervals of the
// request children a care exit snapshots and restores. Care Plan locks and
// ends them itself.
type BookingStore interface {
	CareOfferingBookingHistory(ctx context.Context, childIDs []int64) ([]careplan.CareOfferingBooking, error)
	RestoreCareOfferingBookings(ctx context.Context, restores []careplan.CareOfferingBookingRestore) (int64, error)
}

// Bookings snapshots and restores the Care Plan offering bookings behind the
// Enrollment applications of the children whose care ends. The ledger keeps
// each ended booking as an Enrollment offering snapshot, with the bounds its
// phase had then.
type Bookings struct {
	applications BookingApplications
	store        BookingStore
	phases       enrollment.PhaseBoundsReader
	tenantID     func(context.Context) int64
}

// NewBookings binds the bookings to their owners. phases is Enrollment's
// phase bounds reader (#6657). tenantID reads the school of the request,
// the boundary of every snapshot. Every dependency is required; a missing
// one is a composition error, a zero phases reader fails on its first read.
func NewBookings(applications BookingApplications, store BookingStore, phases enrollment.PhaseBoundsReader, tenantID func(context.Context) int64) Bookings {
	if applications == nil || store == nil || tenantID == nil {
		panic("care exit bookings: enrollment applications, care plan bookings and the tenant are required")
	}
	return Bookings{applications: applications, store: store, phases: phases, tenantID: tenantID}
}

// snapshotPhase is the phase of the booking's application as the care end
// read it. The restore compares it with the phase as it is then.
type snapshotPhase struct {
	ID           int64           `json:"id"`
	ServiceStart enrollment.Date `json:"service_start"`
	ServiceEnd   enrollment.Date `json:"service_end"`
}

// bookingSnapshotRow is the ledger's previous image of one booking. Older
// snapshots have no phase; they restore their bounds as recorded.
type bookingSnapshotRow struct {
	enrollment.RequestChildOffering
	Phase *snapshotPhase `json:"phase,omitempty"`
}

// lockedPhases reads the phases with the phase bounds reader, in one query.
// A phase that is gone has no entry.
func (b Bookings) lockedPhases(ctx context.Context, ids []int64) (map[int64]snapshotPhase, error) {
	wanted := slices.DeleteFunc(slices.Compact(slices.Sorted(slices.Values(ids))), func(id int64) bool { return id <= 0 })
	locked, err := b.phases.LockedPhases(ctx, wanted)
	if err != nil {
		return nil, fmt.Errorf("care exit: read the phases of the bookings: %w", err)
	}
	phases := make(map[int64]snapshotPhase, len(locked))
	for id, phase := range locked {
		start, end, err := phase.ServiceWindow()
		if err != nil {
			return nil, err
		}
		phases[id] = snapshotPhase{ID: id, ServiceStart: enrollment.Date(start), ServiceEnd: enrollment.Date(end)}
	}
	return phases, nil
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
	ids, students, phaseOf := bookingChildren(children, studentIDs, sourceChildID)
	phases, err := b.lockedPhases(ctx, slices.Collect(maps.Values(phaseOf)))
	if err != nil {
		return nil, err
	}
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
		var phase *snapshotPhase
		if recorded, ok := phases[phaseOf[booking.RequestChildID]]; ok {
			phase = &recorded
		}
		snapshot, err := bookingSnapshot(booking, notes, students, phase, until)
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
func bookingSnapshot(booking careplan.CareOfferingBooking, notes map[[2]int64]*string, students map[int64]int64, phase *snapshotPhase, until enrollment.Date) (enrollment.CareExitOfferingSnapshot, error) {
	row := bookingSnapshotRow{Phase: phase}
	row.RequestChildOffering = enrollment.RequestChildOffering{ID: booking.ID, TenantID: booking.TenantID,
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

// bookingChildren maps the selected application children to their students
// and phases.
func bookingChildren(children []enrollment.CareExitApplicationLink, studentIDs []int64, sourceChildID *int64) ([]int64, map[int64]int64, map[int64]int64) {
	ids := make([]int64, 0, len(children))
	students := make(map[int64]int64, len(children))
	phases := make(map[int64]int64, len(children))
	for _, child := range children {
		if child.CreatedStudentID == nil || !slices.Contains(studentIDs, *child.CreatedStudentID) || (sourceChildID != nil && child.ID != *sourceChildID) {
			continue
		}
		ids = append(ids, child.ID)
		students[child.ID] = *child.CreatedStudentID
		phases[child.ID] = child.PhaseID
	}
	return ids, students, phases
}

// RestoreCareExitOfferingLinks replays the snapshots onto the bookings. A
// snapshot from before the split into manual and automatic days restores its
// selected days as manual ones. A snapshot that recorded its phase hands
// Care Plan the move from those bounds to the phase as the phase bounds
// reader reads it now, so the bounds that were the phase's follow it
// (#6657); today is the care lifecycle's day.
func (b Bookings) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore, today enrollment.Date) (int64, error) {
	rows := make([]bookingSnapshotRow, 0, len(snapshots))
	phaseIDs := []int64{}
	tenantID := b.tenantID(ctx)
	for _, snapshot := range snapshots {
		var row bookingSnapshotRow
		if err := json.Unmarshal(snapshot.Snapshot, &row); err != nil {
			return 0, fmt.Errorf("decode care-exit offering snapshot: %w", err)
		}
		if row.ID != snapshot.SourceRowID {
			return 0, fmt.Errorf("care-exit snapshot booking identity mismatch")
		}
		rows = append(rows, row)
		// Care Plan skips another school's rows; their phases are not read.
		if row.Phase != nil && row.TenantID == tenantID {
			phaseIDs = append(phaseIDs, row.Phase.ID)
		}
	}
	current, err := b.lockedPhases(ctx, phaseIDs)
	if err != nil {
		return 0, err
	}
	restores := make([]careplan.CareOfferingBookingRestore, 0, len(snapshots))
	for i, row := range rows {
		snapshot := snapshots[i]
		manual := row.ManualSelectedDays
		if len(manual) == 0 && len(row.AutomaticSelectedDays) == 0 {
			manual = row.SelectedDays
		}
		restores = append(restores, careplan.CareOfferingBookingRestore{WasDeleted: snapshot.WasDeleted, Booking: careplan.CareOfferingBooking{
			ID: row.ID, TenantID: row.TenantID, RequestChildID: row.RequestChildID, CareOfferingID: row.CareOfferingID,
			ManualSelectedDays: manual, AutomaticSelectedDays: row.AutomaticSelectedDays,
			ValidFrom: carePlanDate(row.ValidFrom), ValidUntil: carePlanDate(row.ValidUntil), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		}, PhaseBounds: phaseMove(row.Phase, current, today)})
	}
	return b.store.RestoreCareOfferingBookings(ctx, restores)
}

// phaseMove is the move of the snapshot's phase from the recorded bounds to
// the ones read now; nil without a recorded phase.
func phaseMove(recorded *snapshotPhase, current map[int64]snapshotPhase, today enrollment.Date) *careplan.PhaseBoundsMove {
	if recorded == nil {
		return nil
	}
	now, ok := current[recorded.ID]
	if !ok {
		return nil
	}
	return &careplan.PhaseBoundsMove{
		PhaseID:     recorded.ID,
		StartBefore: calendar.Date(recorded.ServiceStart), StartAfter: calendar.Date(now.ServiceStart),
		EndBefore: calendar.Date(recorded.ServiceEnd), EndAfter: calendar.Date(now.ServiceEnd),
		Today: calendar.Date(today),
	}
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
