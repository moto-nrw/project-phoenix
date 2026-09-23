package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The care-exit cleanup (#2487, #3427): what ending a child's care changes in
// the owners around Care Plan. Each owner performs its own write through its
// port inside the lifecycle's transaction; Care Plan keeps the reversible
// removal ledger that lets a planned exit be changed or cancelled. The
// counting half (the preview) and the writing half (the confirmation) stay
// side by side here, where a divergence is visible.

// careExitSourcePage bounds how many children one Enrollment lookup names.
// The booking evaluation covers a whole school; paging keeps each foreign ID
// set as small as one care-exit batch.
const careExitSourcePage = careplan.MaxCareExitBatchSize

func (s *CareLifecycle) pendingOfferingChanges(ctx context.Context, studentIDs []int64, lock bool) ([]careplan.OfferingChangeRequest, error) {
	changes, err := s.records.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{
		StudentIDs: studentIDs, Statuses: []string{careplan.OfferingChangePending},
		LockForUpdate: lock, Order: careplan.ChangeOrderCreated,
	})
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list pending offering changes: %w", err)
	}
	return changes, nil
}

// countOpenRequests counts every still-open family request per child across
// the request queues and the pending offering changes.
func (s *CareLifecycle) countOpenRequests(ctx context.Context, studentIDs []int64) (map[int64]int, error) {
	if len(studentIDs) == 0 {
		return map[int64]int{}, nil
	}
	counts, err := s.records.CountOpenCareRequests(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: count open parent requests: %w", err)
	}
	changes, err := s.pendingOfferingChanges(ctx, studentIDs, false)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		counts[change.StudentID]++
	}
	return counts, nil
}

func (s *CareLifecycle) lockOpenRequests(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if err := s.records.LockOpenCareRequests(ctx, studentIDs); err != nil {
		return fmt.Errorf("care lifecycle: lock open parent requests for care exit: %w", err)
	}
	_, err := s.pendingOfferingChanges(ctx, studentIDs, true)
	return err
}

// closeOpenRequests moves every still-open request of the given children to
// the care_ended terminal state. The decision text is German because the
// family sees it verbatim; reviewedBy is nil for the scheduler, whose
// "reviewer" is nobody.
func (s *CareLifecycle) closeOpenRequests(ctx context.Context, studentIDs []int64, reviewedBy *int64, at time.Time) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	closedRequests, err := s.records.CloseOpenCareRequests(ctx, studentIDs, careplan.CareEndedDecisionReason, reviewedBy, at)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: close open parent requests: %w", err)
	}
	closedChanges, err := s.records.ClosePendingOfferingChanges(ctx, studentIDs, careplan.CareEndedDecisionReason, reviewedBy, at)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: close open offering change requests: %w", err)
	}
	return int(closedRequests + closedChanges), nil
}

// findOpenPresence marks the children that still hold an open attendance
// row, an open room visit, or an open roster check-in.
func (s *CareLifecycle) findOpenPresence(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	present := make(map[int64]bool, len(studentIDs))
	if len(studentIDs) == 0 {
		return present, nil
	}
	ids, err := s.owners.Presence.ListOpenPresence(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: find open presence: %w", err)
	}
	rosterIDs, err := s.owners.Roster.ListOpenStudentAssignments(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: find open roster presence: %w", err)
	}
	for _, id := range append(ids, rosterIDs...) {
		present[id] = true
	}
	return present, nil
}

// lockImpactRows freezes the rows whose values appear in the binding preview
// but are not part of the planning locks: the source offerings' names, the
// names and bracelets, and the open presence.
func (s *CareLifecycle) lockImpactRows(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	childIDs, err := s.owners.Enrollment.CreatedStudentRequestChildIDs(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("care lifecycle: find source applications for care exit lock: %w", err)
	}
	links, err := s.owners.Enrollment.CareExitOfferingLinks(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("care lifecycle: find source offerings for care exit lock: %w", err)
	}
	if offeringIDs := domain.SourceOfferingIDs(links, childIDs); len(offeringIDs) > 0 {
		if _, err := s.records.ListCareOfferings(ctx, careplan.CareOfferingFilter{
			IDs: offeringIDs, LockForUpdate: true, Order: careplan.OfferingOrderID,
		}); err != nil {
			return fmt.Errorf("care lifecycle: lock source offerings for care exit: %w", err)
		}
	}
	if err := s.directory.LockCareExitPeople(ctx, studentIDs); err != nil {
		return fmt.Errorf("care lifecycle: lock people for care exit: %w", err)
	}
	if err := s.owners.Presence.LockOpenPresence(ctx, studentIDs); err != nil {
		return fmt.Errorf("care lifecycle: lock presence for care exit: %w", err)
	}
	if err := s.owners.Roster.LockOpenStudentAssignments(ctx, studentIDs); err != nil {
		return fmt.Errorf("care lifecycle: lock roster presence for care exit: %w", err)
	}
	return nil
}

// lockPlanning locks the live roster rows, bookings, source bookings and
// weekly plans a care exit can remove. The confirmation takes these locks
// before rebuilding its token, so a plan row cannot change between the shown
// preview and the mutation.
func (s *CareLifecycle) lockPlanning(ctx context.Context, studentIDs []int64, after calendar.Date) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if err := s.owners.Roster.LockPlannedRosterForCareExit(ctx, studentIDs, after); err != nil {
		return fmt.Errorf("care lifecycle: lock planned roster rows for care exit: %w", err)
	}
	validUntil := after.AddDays(1)
	if err := s.owners.Bookings.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil); err != nil {
		return fmt.Errorf("care lifecycle: lock bookings for care exit: %w", err)
	}
	childIDs, err := s.owners.Enrollment.CreatedStudentRequestChildIDs(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("care lifecycle: find source applications for care exit lock: %w", err)
	}
	if err := s.owners.Enrollment.LockCareExitOfferingLinks(ctx, childIDs, validUntil); err != nil {
		return fmt.Errorf("care lifecycle: lock source bookings for care exit: %w", err)
	}
	return s.lockWeeklyPlans(ctx, studentIDs, validUntil)
}

func (s *CareLifecycle) lockWeeklyPlans(ctx context.Context, studentIDs []int64, from calendar.Date) error {
	filter := careplan.StudentScheduleFilter{StudentIDs: studentIDs, LockForUpdate: true}
	if _, err := s.records.ListPickupSchedules(ctx, filter); err != nil {
		return fmt.Errorf("care lifecycle: lock weekly pickup plan for care exit: %w", err)
	}
	if _, err := s.records.ListArrivalSchedules(ctx, filter); err != nil {
		return fmt.Errorf("care lifecycle: lock weekly arrival plan for care exit: %w", err)
	}
	filter.From = careplan.Date(from)
	if _, err := s.records.ListPickupExceptions(ctx, filter); err != nil {
		return fmt.Errorf("care lifecycle: lock pickup exceptions for care exit: %w", err)
	}
	if _, err := s.records.ListArrivalExceptions(ctx, filter); err != nil {
		return fmt.Errorf("care lifecycle: lock arrival exceptions for care exit: %w", err)
	}
	return nil
}

func (s *CareLifecycle) weeklyPlanPatterns(ctx context.Context, studentIDs []int64) (map[int64][]string, error) {
	if len(studentIDs) == 0 {
		return map[int64][]string{}, nil
	}
	arrivals, err := s.records.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs})
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list recurring arrival plans for care exit preview: %w", err)
	}
	pickups, err := s.records.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: studentIDs})
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: list recurring pickup plans for care exit preview: %w", err)
	}
	return domain.WeeklyPlanPatterns(arrivals, pickups), nil
}

// sourceOfferingsAfter names the source bookings of the given children that
// still run on or after validUntil.
func (s *CareLifecycle) sourceOfferingsAfter(ctx context.Context, studentIDs []int64, validUntil calendar.Date) (map[int64][]careplan.CareExitSourceOffering, error) {
	if len(studentIDs) == 0 {
		return map[int64][]careplan.CareExitSourceOffering{}, nil
	}
	sources, err := s.loadCareExitSources(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	return domain.SourceOfferingsAfter(studentIDs, validUntil, sources.applications, sources.offerings, sources.links), nil
}

type careExitSources struct {
	applications []domain.CareExitApplication
	offerings    []careplan.CareOffering
	links        []domain.CareExitOfferingLink
}

// loadCareExitSources reads the Enrollment applications and source bookings
// of the given children, one page of children at a time, and the school's
// care offerings once.
func (s *CareLifecycle) loadCareExitSources(ctx context.Context, studentIDs []int64) (careExitSources, error) {
	var sources careExitSources
	for start := 0; start < len(studentIDs); start += careExitSourcePage {
		page := studentIDs[start:min(start+careExitSourcePage, len(studentIDs))]
		applications, err := s.owners.Enrollment.CareExitApplicationLinks(ctx, page)
		if err != nil {
			return sources, fmt.Errorf("care lifecycle: load care-exit application links: %w", err)
		}
		sources.applications = append(sources.applications, applications...)
		if start == 0 {
			if sources.offerings, err = s.records.ListCareOfferings(ctx, careplan.CareOfferingFilter{Order: careplan.OfferingOrderID}); err != nil {
				return sources, fmt.Errorf("care lifecycle: list care offerings for care exit: %w", err)
			}
		}
		links, err := s.owners.Enrollment.CareExitOfferingLinks(ctx, page)
		if err != nil {
			return sources, fmt.Errorf("care lifecycle: load care-exit offering links: %w", err)
		}
		sources.links = append(sources.links, links...)
	}
	return sources, nil
}

// latestAttendanceDate is the latest day a child was actually there: a
// roster check-in or a recorded presence, whichever is later.
func (s *CareLifecycle) latestAttendanceDate(ctx context.Context, studentID int64) (*calendar.Date, error) {
	rosterDay, err := s.owners.Roster.LatestStudentAssignmentAttendanceDate(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: find latest roster attendance before care exit: %w", err)
	}
	day, err := s.owners.Presence.LatestPresenceDate(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: find latest attendance before care exit: %w", err)
	}
	if rosterDay != nil && (day == nil || rosterDay.After(*day)) {
		day = rosterDay
	}
	return day, nil
}

// closeOpenPresence closes whatever the children still have open when their
// care ends: the attendance row, the room visit and the roster check-in.
// Nothing is deleted; the day that happened stays in the history, it just
// stops being an unfinished one (#2487).
func (s *CareLifecycle) closeOpenPresence(ctx context.Context, studentIDs []int64, at time.Time) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	closed, err := s.owners.Presence.CloseOpenPresence(ctx, studentIDs, at)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: close open presence: %w", err)
	}
	rosterRows, err := s.owners.Roster.CloseOpenStudentAssignments(ctx, studentIDs, at)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: close open roster presence: %w", err)
	}
	return int(closed + rosterRows), nil
}

// removePlannedRosterAfter drops the children from every roster dated after
// their last care day and records each row in the ledger in the same
// transaction. The ledger is a verbatim copy rather than a note to rebuild
// from enrollments: an occurrence a supervisor customised by hand would
// otherwise come back plain (#405). It is dropped unreplayed once the exit
// takes effect and on a resume.
func (s *CareLifecycle) removePlannedRosterAfter(ctx context.Context, studentIDs []int64, after calendar.Date) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	removed, err := s.owners.Roster.RemovePlannedRosterForCareExit(ctx, studentIDs, after)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: delete planned roster rows after care end: %w", err)
	}
	if err := s.records.RecordCareExitRemovals(ctx, removed); err != nil {
		return 0, fmt.Errorf("care lifecycle: record planned roster rows after care end: %w", err)
	}
	return len(removed), nil
}

// capBookings ends every activity booking of the given children at
// validUntil (exclusive), deleting the ones left with no interval at all, and
// records both in the ledger so a changed or cancelled exit can put them
// back. It ignores provenance on purpose: the child leaves the school, so a
// booking materialized from an approved enrollment request ends too.
func (s *CareLifecycle) capBookings(ctx context.Context, studentIDs []int64, validUntil calendar.Date) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	changes, err := s.owners.Bookings.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: end activity bookings after care end: %w", err)
	}
	if err := s.records.RecordCareExitRemovals(ctx, deletedBookingRemovals(changes.Deleted)); err != nil {
		return 0, fmt.Errorf("care lifecycle: record deleted future bookings after care end: %w", err)
	}
	if err := s.records.RecordCareExitRemovals(ctx, cappedBookingRemovals(changes.Capped)); err != nil {
		return 0, fmt.Errorf("care lifecycle: record capped bookings after care end: %w", err)
	}
	return int64(len(changes.Deleted) + len(changes.Capped)), nil
}

// endSourceBookings snapshots and ends the source bookings the children's
// applications created, and with endSchedules also their recurring arrival
// and pickup plans.
func (s *CareLifecycle) endSourceBookings(ctx context.Context, studentIDs []int64, validUntil calendar.Date, endSchedules bool) (int64, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	snapshots, err := s.owners.Enrollment.CareExitOfferingSnapshots(ctx, studentIDs, validUntil, nil)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: snapshot source bookings before care exit: %w", err)
	}
	removals := make([]careplan.CareExitSourceRemoval, 0, len(snapshots))
	for _, snapshot := range snapshots {
		removals = append(removals, careplan.CareExitSourceRemoval{
			TenantID: snapshot.TenantID, StudentID: snapshot.StudentID, Kind: careplan.CareExitSourceBooking,
			SourceRowID: snapshot.SourceRowID, WasDeleted: snapshot.WasDeleted, Snapshot: snapshot.Snapshot,
		})
	}
	if err := s.records.RecordCareExitSourceRemovals(ctx, removals); err != nil {
		return 0, fmt.Errorf("care lifecycle: record source bookings before care exit: %w", err)
	}
	ended, err := s.endSourceBookingRows(ctx, studentIDs, validUntil)
	if err != nil || !endSchedules {
		return ended, err
	}
	plans, err := s.records.EndStudentSchedulesForCareExit(ctx, studentIDs, careplan.Date(validUntil))
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: end weekly plans after care exit: %w", err)
	}
	return ended + plans, nil
}

func (s *CareLifecycle) endSourceBookingRows(ctx context.Context, studentIDs []int64, validUntil calendar.Date) (int64, error) {
	childIDs, err := s.owners.Enrollment.CreatedStudentRequestChildIDs(ctx, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: find source applications for care exit: %w", err)
	}
	if len(childIDs) == 0 {
		return 0, nil
	}
	ended, err := s.owners.Enrollment.EndCareExitOfferingLinks(ctx, childIDs, nil, validUntil)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: end source bookings after care exit: %w", err)
	}
	return ended, nil
}

// restoreRemovals puts back everything the children's current care exit took
// away and empties their ledger (#2487). It is the inverse of the roster
// removal, the booking cap and the source-booking end, and it runs when a
// planned exit is CANCELLED and at the start of every re-run over the same
// child, so changing the last care day always applies to the untouched plan.
//
// It is deliberately forgiving. A roster row somebody re-created by hand, a
// room or status day deleted since, an instance that has meanwhile completed:
// none of those may fail the restore, because the alternative is a child
// stuck in a state nobody can leave. Skipped rows are simply not restored;
// the ledger is cleared either way, since it describes one exit and that exit
// is over.
func (s *CareLifecycle) restoreRemovals(ctx context.Context, studentIDs []int64) (int, error) {
	if len(studentIDs) == 0 {
		return 0, nil
	}
	ledger, err := s.records.ListCareExitRemovals(ctx, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: list care exit removals: %w", err)
	}
	sourceLedger, err := s.records.ListCareExitSourceRemovals(ctx, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: list care exit source removals: %w", err)
	}
	roster := rosterRemovals(ledger)
	// Timetable validates the surviving room and care-plan references and
	// restores its roster in this same transaction.
	restored, err := s.owners.Roster.RestoreRosterForCareExit(ctx, studentIDs, roster)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: restore roster rows after care exit change: %w", err)
	}
	bookingRows, err := s.restoreBookings(ctx, studentIDs, ledger)
	if err != nil {
		return 0, err
	}
	restored += bookingRows
	// Source bookings capped by the exit recover their original exclusive end
	// and deleted ones keep their original ids; Enrollment owns both writes.
	sourceRows, err := s.owners.Enrollment.RestoreCareExitOfferingLinks(ctx, sourceBookingRestores(sourceLedger, studentIDs))
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: restore source bookings after care exit: %w", err)
	}
	planRows, err := s.records.RestoreStudentSchedulesForCareExit(ctx, studentIDs)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: restore weekly plans after care exit: %w", err)
	}
	restored += int(sourceRows + planRows)
	if err := s.reconnectRosterPickupExceptions(ctx, studentIDs, roster); err != nil {
		return 0, err
	}
	if err := s.discardRemovals(ctx, studentIDs); err != nil {
		return 0, err
	}
	return restored, nil
}

// restoreBookings lets the Timetable owner restore the capped and deleted
// bookings. It keeps the original ids, re-validates the calendar period and
// enrollment provenance, and treats a duplicate re-creation as a no-op.
func (s *CareLifecycle) restoreBookings(ctx context.Context, studentIDs []int64, ledger []careplan.CareExitRemoval) (int, error) {
	periodIDs, err := s.owners.Calendar.ListCalendarPeriodIDs(ctx)
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: list calendar periods for booking restore: %w", err)
	}
	restored, err := s.owners.Bookings.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, periodIDs, bookingRestores(ledger))
	if err != nil {
		return 0, fmt.Errorf("care lifecycle: restore activity bookings after care exit change: %w", err)
	}
	return restored, nil
}

// reconnectRosterPickupExceptions runs after the weekly plans are back:
// rosters are restored before their pickup exceptions because the original
// roster ledger is shared with older exits. Without it a cancellation would
// silently turn an exception-bound roster row into an ordinary row.
func (s *CareLifecycle) reconnectRosterPickupExceptions(ctx context.Context, studentIDs []int64, roster []careplan.CareExitRemoval) error {
	var archived []int64
	for _, removal := range roster {
		if removal.PickupExceptionID != nil {
			archived = append(archived, *removal.PickupExceptionID)
		}
	}
	valid := []int64{}
	if len(archived) > 0 {
		exceptions, err := s.records.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{IDs: archived})
		if err != nil {
			return fmt.Errorf("care lifecycle: reconnect restored roster pickup exception: %w", err)
		}
		for _, exception := range exceptions {
			valid = append(valid, exception.ID)
		}
	}
	if err := s.owners.Roster.ReconnectCareExitAssignmentPickupExceptions(ctx, studentIDs, valid, roster); err != nil {
		return fmt.Errorf("care lifecycle: reconnect restored roster pickup exception: %w", err)
	}
	return nil
}

// discardRemovals drops the ledger without replaying it: when the exit
// becomes final and on a resume, where the school plans the returning child
// again by hand rather than have last term's plan switch itself back on.
func (s *CareLifecycle) discardRemovals(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if err := s.records.DiscardCareExitRemovals(ctx, studentIDs); err != nil {
		return fmt.Errorf("care lifecycle: discard care exit removals: %w", err)
	}
	return nil
}

func rosterRemovals(ledger []careplan.CareExitRemoval) []careplan.CareExitRemoval {
	result := make([]careplan.CareExitRemoval, 0, len(ledger))
	for _, removal := range ledger {
		if removal.Kind == careplan.CareExitRemovalRoster && removal.InstanceID != nil {
			result = append(result, removal)
		}
	}
	return result
}

func bookingRestores(ledger []careplan.CareExitRemoval) []domain.CareExitBookingRestore {
	result := make([]domain.CareExitBookingRestore, 0)
	for _, removal := range ledger {
		if removal.Kind != careplan.CareExitRemovalBooking || removal.EnrollmentID == nil {
			continue
		}
		booking := domain.CareExitBooking{
			ID: *removal.EnrollmentID, TenantID: removal.TenantID, StudentID: removal.StudentID,
			ValidUntil: calendarDate(removal.PreviousValidUntil), CalendarPeriodID: removal.CalendarPeriodID,
			EnrollmentRequestChildID: removal.EnrollmentRequestChildID, SelectedWeekdays: removal.SelectedWeekdays,
			AttendanceStatus: removal.AttendanceStatus, Weekday: removal.Weekday,
		}
		if removal.ActivityGroupID != nil {
			booking.ActivityGroupID = *removal.ActivityGroupID
		}
		if removal.ValidFrom != nil {
			booking.ValidFrom = calendar.Date(*removal.ValidFrom)
		}
		result = append(result, domain.CareExitBookingRestore{
			CareExitBooking: booking, WasDeleted: removal.WasDeleted,
			PreviousValidUntil: calendarDate(removal.PreviousValidUntil),
		})
	}
	return result
}

func deletedBookingRemovals(rows []domain.CareExitBooking) []careplan.CareExitRemoval {
	result := make([]careplan.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		enrollmentID, groupID := row.ID, row.ActivityGroupID
		from := careplan.Date(row.ValidFrom)
		result = append(result, careplan.CareExitRemoval{
			StudentID: row.StudentID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			WasDeleted: true, PreviousValidUntil: carePlanDate(row.ValidUntil), ActivityGroupID: &groupID,
			ValidFrom: &from, CalendarPeriodID: row.CalendarPeriodID, EnrollmentRequestChildID: row.EnrollmentRequestChildID,
			SelectedWeekdays: row.SelectedWeekdays, AttendanceStatus: row.AttendanceStatus, Weekday: row.Weekday,
		})
	}
	return result
}

func cappedBookingRemovals(rows []domain.CareExitBookingCap) []careplan.CareExitRemoval {
	result := make([]careplan.CareExitRemoval, 0, len(rows))
	for _, row := range rows {
		enrollmentID := row.ID
		result = append(result, careplan.CareExitRemoval{
			StudentID: row.StudentID, Kind: careplan.CareExitRemovalBooking, EnrollmentID: &enrollmentID,
			PreviousValidUntil: carePlanDate(row.PreviousValidUntil),
		})
	}
	return result
}

// sourceBookingRestores selects the source-booking ledger entries of the
// given children.
func sourceBookingRestores(removals []careplan.CareExitSourceRemoval, studentIDs []int64) []domain.CareExitOfferingRestore {
	students := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		students[id] = true
	}
	restores := make([]domain.CareExitOfferingRestore, 0, len(removals))
	for _, removal := range removals {
		if removal.Kind != careplan.CareExitSourceBooking || !students[removal.StudentID] {
			continue
		}
		restores = append(restores, domain.CareExitOfferingRestore{
			SourceRowID: removal.SourceRowID, WasDeleted: removal.WasDeleted, Snapshot: removal.Snapshot,
		})
	}
	return restores
}
