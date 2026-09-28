package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// BookingMaterializationDependencies are the ports booking materialization
// (#3560) reads and writes through, and the hooks it shares with the other
// recurrence and pickup writers. Every call runs in the caller's tenant
// transaction.
type BookingMaterializationDependencies struct {
	// Catalog is Care Plan's care-offering catalog: its records and the
	// timetable-link rules the drafts apply.
	Catalog     *CareOfferingCatalog
	Rosters     ports.BookingRosters
	Templates   ports.BookingTemplates
	Periods     ports.BookingPeriods
	Enrollment  ports.BookingEnrollment
	Students    ports.BookingStudents
	Settings    ports.BookingSettings
	Bookings    ports.BookingCommands
	Withdrawals ports.BookingWithdrawals
	Audit       ports.AdjustmentAudit
	Pickup      ports.BookingPickup

	// RunInTx joins the ambient tenant transaction or opens one.
	RunInTx func(context.Context, func(context.Context) error) error
	// LockTemplateRecurrence takes the tenant recurrence gate that orders
	// every recurrence writer. Focused tests may leave it nil.
	LockTemplateRecurrence func(context.Context) error
	// ResyncPickupAutoExcusals re-derives the auto partial absences coupled
	// to the students' future pickup exceptions (#2360). Tests may leave it
	// nil.
	ResyncPickupAutoExcusals func(ctx context.Context, studentIDs []int64) error
	// ClearPickupWeekdayExtension closes the Timetable task created for a
	// manual weekly pickup once that override is reset. Tests may leave it
	// nil.
	ClearPickupWeekdayExtension func(ctx context.Context, studentID int64, weekday int) error
	// AnnouncePickupChange announces a changed pickup projection to staff
	// and guardians after the tenant transaction commits. Tests may leave it
	// nil.
	AnnouncePickupChange func(ctx context.Context, studentIDs []int64)

	Today  func() calendar.Date
	Logger *slog.Logger
}

// BookingMaterialization keeps the rosters, dated adjustments and pickup
// times derived from care-offering bookings in step (#3560).
type BookingMaterialization struct {
	deps    BookingMaterializationDependencies
	catalog *CareOfferingCatalog
}

var _ careplan.BookingMaterializationCapability = (*BookingMaterialization)(nil)

// NewBookingMaterialization builds the materialization over its ports.
func NewBookingMaterialization(deps BookingMaterializationDependencies) (*BookingMaterialization, error) {
	if deps.Catalog == nil || deps.Rosters == nil || deps.Templates == nil || deps.Periods == nil ||
		deps.Enrollment == nil || deps.Students == nil || deps.Settings == nil || deps.Bookings == nil ||
		deps.Audit == nil || deps.Pickup == nil || deps.RunInTx == nil {
		return nil, errors.New("booking materialization: catalog, rosters, templates, periods, enrollment, students, settings, bookings, audit, pickup and transaction runner are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	return &BookingMaterialization{deps: deps, catalog: deps.Catalog}, nil
}

func (m *BookingMaterialization) todayDate() calendar.Date {
	return m.deps.Today()
}

// lockTemplateRecurrence takes the tenant recurrence gate alone. Sourced
// roster changes, booking links, care-offering configuration and manual
// pickup resets are serialized by it.
func (m *BookingMaterialization) lockTemplateRecurrence(ctx context.Context) error {
	if m.deps.LockTemplateRecurrence == nil {
		return nil
	}
	if err := m.deps.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("decision: lock template recurrence: %w", err)
	}
	return nil
}

// LockOfferingDerivedWrites establishes the project-wide gate order before a
// caller locks student rows and then changes booking- or offering-derived
// state: the shared class-writes gate first, the recurrence gate second.
// Transaction-scoped locks are safe to acquire again downstream.
func (m *BookingMaterialization) LockOfferingDerivedWrites(ctx context.Context) error {
	if err := m.deps.Students.LockClassWrites(ctx); err != nil {
		return fmt.Errorf("lock class writes for offering-derived change: %w", err)
	}
	if m.deps.LockTemplateRecurrence == nil {
		return nil
	}
	if err := m.deps.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("decision: lock template recurrence: %w", err)
	}
	return nil
}

// careOfferingsEnabled reads enrollment.care_offerings_enabled.
func (m *BookingMaterialization) careOfferingsEnabled(ctx context.Context) (bool, error) {
	return m.deps.Settings.CareOfferingsEnabled(ctx)
}

// logSkippedSourcedTemplate reports a sourced template the decision fan-out
// had to skip (misconfigured period/schedules). Approvals must not fail on a
// template drifted since its save; the editor surfaces the mismatch.
func (m *BookingMaterialization) logSkippedSourcedTemplate(templateID int64, offeringIDs []int64, reason string, err error) {
	attrs := []any{
		slog.Int64("template_id", templateID),
		slog.Any("care_offering_ids", offeringIDs),
		slog.String("reason", reason),
	}
	if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	m.deps.Logger.Warn("decision: skipping offering-sourced template", attrs...)
}

// offeringsByID indexes offerings by id for the draft and adjustment rules.
func offeringsByID(offerings []careplan.CareOffering) map[int64]*careplan.CareOffering {
	byID := make(map[int64]*careplan.CareOffering, len(offerings))
	for i := range offerings {
		byID[offerings[i].ID] = &offerings[i]
	}
	return byID
}

// bookedOfferingIDs lists the distinct positive offering ids of the links in
// order.
func bookedOfferingIDs(links []*careplan.BookedOffering) []int64 {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	return ids
}

func bookedOfferingPointers(values []careplan.BookedOffering) []*careplan.BookedOffering {
	if values == nil {
		return nil
	}
	links := make([]*careplan.BookedOffering, len(values))
	for i := range values {
		links[i] = &values[i]
	}
	return links
}

func sortedIDSet(set map[int64]bool) []int64 {
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func cloneOptionalDate(date *calendar.Date) *calendar.Date {
	if date == nil {
		return nil
	}
	cloned := *date
	return &cloned
}

func copyDays(days []string) []string {
	if len(days) == 0 {
		return nil
	}
	out := make([]string, len(days))
	copy(out, days)
	return out
}
