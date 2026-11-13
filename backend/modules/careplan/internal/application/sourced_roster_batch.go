package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// rosterBatch runs the existing roster decisions against one immutable snapshot.
// Only its buffered diff reaches the owners, after every template accepted it.
type rosterBatch struct {
	ports.BookingRosters
	rows       map[int64][]ports.RosterEnrollment
	changes    ports.RosterChanges
	reconciles []ports.RosterReconcile
}

// These single-child commands are outside a template roster diff.
func (b *rosterBatch) StudentEnrollments(context.Context, int64) ([]ports.RosterEnrollment, error) {
	return nil, fmt.Errorf("roster batch: student-wide read is outside the snapshot")
}
func (b *rosterBatch) DeleteRequestChildEnrollments(context.Context, int64, int64) error {
	return fmt.Errorf("roster batch: child-wide delete is outside the diff")
}
func (b *rosterBatch) BackfillRequestChildSource(context.Context, int64, int64, []int64) error {
	return fmt.Errorf("roster batch: provenance backfill is outside the diff")
}

func (b *rosterBatch) GroupEnrollments(_ context.Context, id int64) ([]ports.RosterEnrollment, error) {
	return b.rows[id], nil
}
func (b *rosterBatch) CreateEnrollment(_ context.Context, row ports.RosterEnrollment) error {
	b.changes.Create = append(b.changes.Create, row)
	return nil
}
func (b *rosterBatch) DeleteEnrollment(_ context.Context, id int64) error {
	b.changes.Delete = append(b.changes.Delete, id)
	return nil
}
func (b *rosterBatch) SetEnrollmentValidUntil(_ context.Context, id int64, end calendar.Date) error {
	b.changes.Ends[id] = end
	return nil
}
func (b *rosterBatch) ReconcileTemplateRosters(_ context.Context, id int64, students []int64, from calendar.Date, prior []ports.RosterEnrollment) error {
	b.reconciles = append(b.reconciles, ports.RosterReconcile{TemplateID: id, StudentIDs: students, From: from, Prior: prior})
	return nil
}
func (b *rosterBatch) flush(ctx context.Context) error {
	if err := b.SaveRosterChanges(ctx, b.changes); err != nil {
		return err
	}
	return b.ReconcileTemplateRosterBatch(ctx, b.reconciles)
}

type rosterEnrollmentSnapshot struct {
	ports.BookingEnrollment
	phases   catalogBatchPhases
	children []ports.ApprovedOfferingChild
}

func (s rosterEnrollmentSnapshot) Phase(_ context.Context, id int64) (ports.BookingPhase, error) {
	for _, phase := range s.phases.rows {
		if phase.ID == id {
			return ports.BookingPhase{OfferingPhase: phase}, nil
		}
	}
	return ports.BookingPhase{}, ports.ErrBookingRowNotFound
}
func (s rosterEnrollmentSnapshot) ApprovedChildren(_ context.Context, ids []int64, _ calendar.Date) ([]ports.ApprovedOfferingChild, error) {
	result := []ports.ApprovedOfferingChild{}
	for _, child := range s.children {
		if slices.Contains(ids, child.Link.CareOfferingID) {
			result = append(result, child)
		}
	}
	return result, nil
}

func (m *BookingMaterialization) sourcedRosterBatch(ctx context.Context, templates []ports.SourcedTemplate, from calendar.Date) (*BookingMaterialization, *rosterBatch, error) {
	ids := make([]int64, 0, len(templates))
	for _, tmpl := range templates {
		ids = append(ids, tmpl.ID)
	}
	catalog, enrollment, err := m.rosterBatchSnapshot(ctx, from)
	if err != nil {
		return nil, nil, err
	}
	rows, err := m.deps.Rosters.GroupsEnrollments(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	batch := &rosterBatch{BookingRosters: m.deps.Rosters, rows: map[int64][]ports.RosterEnrollment{}, changes: ports.RosterChanges{Ends: map[int64]calendar.Date{}}}
	for _, row := range rows {
		batch.rows[row.ActivityGroupID] = append(batch.rows[row.ActivityGroupID], row)
	}
	materialization := *m
	materialization.catalog = catalog
	materialization.deps.Rosters = batch
	materialization.deps.Enrollment = enrollment
	return &materialization, batch, nil
}

func (m *BookingMaterialization) rosterBatchSnapshot(ctx context.Context, from calendar.Date) (*CareOfferingCatalog, rosterEnrollmentSnapshot, error) {
	offerings, err := m.catalog.listTenantRecords(ctx)
	if err != nil {
		return nil, rosterEnrollmentSnapshot{}, err
	}
	phases, err := m.deps.Enrollment.Phases(ctx)
	if err != nil {
		return nil, rosterEnrollmentSnapshot{}, err
	}
	periods, err := m.deps.Periods.Periods(ctx)
	if err != nil {
		return nil, rosterEnrollmentSnapshot{}, err
	}
	timetable, ok := m.catalog.deps.Timetable.(catalogTimetableBatch)
	if !ok {
		return nil, rosterEnrollmentSnapshot{}, fmt.Errorf("roster batch: timetable snapshot is required")
	}
	snapshot, err := timetable.CatalogSnapshot(ctx)
	if err != nil {
		return nil, rosterEnrollmentSnapshot{}, err
	}
	offeringIDs := make([]int64, 0, len(offerings))
	for _, offering := range offerings {
		offeringIDs = append(offeringIDs, offering.ID)
	}
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, offeringIDs, from)
	if err != nil {
		return nil, rosterEnrollmentSnapshot{}, err
	}
	phaseSnapshot := catalogBatchPhases{rows: phases}
	catalog := *m.catalog
	catalog.deps.Records = catalogBatchRecords{CatalogRecords: m.catalog.deps.Records, rows: offerings}
	catalog.deps.Phases = phaseSnapshot
	catalog.deps.Calendar = catalogBatchCalendar{CatalogCalendar: m.catalog.deps.Calendar, rows: periods}
	catalog.deps.Timetable = catalogBatchTimetable{snapshot: snapshot}

	return &catalog, rosterEnrollmentSnapshot{BookingEnrollment: m.deps.Enrollment, phases: phaseSnapshot, children: children}, nil
}
