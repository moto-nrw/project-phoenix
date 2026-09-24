package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PickupAdjustmentDependencies are the collaborators of a permanent
// pickup-time change: Care Plan's own schedule services and records, the
// offering-change review for a switch to a matching offering, and the People
// Directory, Audit Platform and tenant runtime through ports.
type PickupAdjustmentDependencies struct {
	PickupSchedules  careplan.PickupScheduleService
	ArrivalSchedules careplan.ArrivalScheduleService
	Records          ports.PickupAdjustmentSchedules
	Baselines        careplan.PickupBaselineReader
	Offerings        careplan.DirectOfferingAdjustments
	Settings         ports.PickupAdjustmentSettings
	Audit            ports.PickupPlanAudit
	Students         ports.PickupAdjustmentStudents
	UnitOfWork       ports.TenantUnitOfWork
	// Fingerprint returns the lowercase hexadecimal SHA-256 of the content;
	// it binds an apply to the preview it was made from.
	Fingerprint func([]byte) string
	Today       func() calendar.Date
}

// PickupAdjustments previews and applies permanent pickup-time changes.
type PickupAdjustments struct {
	deps PickupAdjustmentDependencies
}

var _ careplan.PickupAdjustments = (*PickupAdjustments)(nil)

// NewPickupAdjustments builds the pickup adjustment over its collaborators.
func NewPickupAdjustments(deps PickupAdjustmentDependencies) (*PickupAdjustments, error) {
	if deps.PickupSchedules == nil || deps.ArrivalSchedules == nil || deps.Records == nil || deps.Baselines == nil ||
		deps.Offerings == nil || deps.Settings == nil || deps.Audit == nil || deps.Students == nil ||
		deps.UnitOfWork == nil || deps.Fingerprint == nil {
		return nil, errors.New("pickup adjustments: pickup and arrival schedules, records, baselines, offerings, settings, audit, students, unit of work and fingerprint are required")
	}
	if deps.Today == nil {
		deps.Today = calendar.TodayDate
	}
	return &PickupAdjustments{deps: deps}, nil
}

// today is the school's calendar day the adjustment plans from.
func (s *PickupAdjustments) today() calendar.Date {
	return s.deps.Today()
}

// Preview projects a proposed permanent pickup plan without writing it.
func (s *PickupAdjustments) Preview(ctx context.Context, input careplan.PickupAdjustmentPreviewInput) (*careplan.PickupAdjustmentPreview, error) {
	return s.preview(ctx, input, s.today())
}

func (s *PickupAdjustments) preview(ctx context.Context, input careplan.PickupAdjustmentPreviewInput, today calendar.Date) (*careplan.PickupAdjustmentPreview, error) {
	input, explicitByDay, err := normalizePickupAdjustmentInput(input, today)
	if err != nil {
		return nil, err
	}
	if input.Selections != nil {
		if err := s.preflightPickupOffering(ctx, input.StudentID, input.EffectiveFrom, today); err != nil {
			return nil, err
		}
	}
	plans, err := s.projectPickupAdjustment(ctx, input, explicitByDay)
	if err != nil {
		return nil, err
	}
	preview, err := s.basePickupAdjustmentPreview(ctx, input, plans)
	if err != nil {
		return nil, err
	}
	return s.completePickupAdjustmentPreview(ctx, input, plans, preview)
}

// pickupAdjustmentPlans are the current plan, its booking-derived part and
// the proposed plan of one pickup adjustment.
type pickupAdjustmentPlans struct {
	current, offering careplan.PickupWeek
	proposed          map[int]careplan.PickupAdjustmentSchedule
}

func (s *PickupAdjustments) completePickupAdjustmentPreview(
	ctx context.Context,
	input careplan.PickupAdjustmentPreviewInput,
	plans *pickupAdjustmentPlans,
	preview *careplan.PickupAdjustmentPreview,
) (*careplan.PickupAdjustmentPreview, error) {
	if err := s.attachOfferingAdjustment(ctx, input, plans.proposed, preview); err != nil {
		return nil, err
	}
	if err := s.attachRemovedManualNotes(ctx, input, preview); err != nil {
		return nil, err
	}
	currentArrival, err := s.currentArrivalPlan(ctx, input)
	if err != nil {
		return nil, err
	}
	preview.PreviewToken, err = s.pickupAdjustmentToken(pickupTokenContent{
		TenantID: s.deps.UnitOfWork.TenantID(ctx), Input: input, Preview: preview,
		Current: plans.current, Offering: plans.offering, CurrentArrival: currentArrival,
	})
	return preview, err
}

func (s *PickupAdjustments) currentArrivalPlan(ctx context.Context, input careplan.PickupAdjustmentPreviewInput) ([]careplan.PickupAdjustmentArrivalSchedule, error) {
	if input.ArrivalSchedules == nil {
		return nil, nil
	}
	rows, err := s.deps.Records.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{input.StudentID}})
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: read current arrival schedules: %w", err)
	}
	current := make([]careplan.PickupAdjustmentArrivalSchedule, 0, len(rows))
	for _, row := range rows {
		item := careplan.PickupAdjustmentArrivalSchedule{Weekday: row.Weekday}
		if !row.ExpectedArrival.IsZero() {
			item.ExpectedArrival = row.ExpectedArrival.Format("15:04")
		}
		if row.Notes != nil {
			note := *row.Notes
			item.Notes = &note
		}
		current = append(current, item)
	}
	sort.Slice(current, func(i, j int) bool { return current[i].Weekday < current[j].Weekday })
	return current, nil
}

func (s *PickupAdjustments) pickupRows(ctx context.Context, studentID int64) ([]careplan.PickupSchedule, error) {
	return s.deps.Records.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{studentID}})
}

// attachRemovedManualNotes names the notes of the manual pickup rows a
// switch to an offering removes.
func (s *PickupAdjustments) attachRemovedManualNotes(ctx context.Context, input careplan.PickupAdjustmentPreviewInput, preview *careplan.PickupAdjustmentPreview) error {
	if len(input.Selections) == 0 || preview.OfferingConsequences == nil {
		return nil
	}
	existing, err := s.pickupRows(ctx, input.StudentID)
	if err != nil {
		return err
	}
	notesByWeekday := make(map[int]string)
	for _, row := range existing {
		if row.Source == careplan.ScheduleSourceCareOffering || row.Notes == nil {
			continue
		}
		if note := strings.TrimSpace(*row.Notes); note != "" {
			notesByWeekday[row.Weekday] = note
		}
	}
	for weekday, note := range notesByWeekday {
		preview.RemovedManualNotes = append(preview.RemovedManualNotes, careplan.PickupAdjustmentRemovedNote{Weekday: weekday, Note: note})
	}
	sort.Slice(preview.RemovedManualNotes, func(i, j int) bool {
		return preview.RemovedManualNotes[i].Weekday < preview.RemovedManualNotes[j].Weekday
	})
	return nil
}

func (s *PickupAdjustments) projectPickupAdjustment(
	ctx context.Context,
	input careplan.PickupAdjustmentPreviewInput,
	explicit map[int]careplan.PickupAdjustmentSchedule,
) (*pickupAdjustmentPlans, error) {
	weekStart := input.EffectiveFrom.AddDays(pickupWeekdayMonday - isoWeekday(input.EffectiveFrom))
	projection, err := s.deps.Baselines.Project(ctx, []int64{input.StudentID}, weekStart, weekStart.AddDays(4))
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: project current plan: %w", err)
	}
	offering := projection.OfferingWeeklyForDate(input.StudentID, weekStart)
	return &pickupAdjustmentPlans{
		current: projection.WeeklyForDate(input.StudentID, weekStart), offering: offering,
		proposed: effectiveProposedPickupPlan(input.CareDays, explicit, offering),
	}, nil
}

func (s *PickupAdjustments) basePickupAdjustmentPreview(
	ctx context.Context,
	input careplan.PickupAdjustmentPreviewInput,
	plans *pickupAdjustmentPlans,
) (*careplan.PickupAdjustmentPreview, error) {
	reviewEnabled, err := s.deps.Settings.PickupOfferingReviewRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: resolve offering review setting: %w", err)
	}
	deviates := pickupPlanDeviates(input.CareDays, plans.proposed, plans.offering)
	return &careplan.PickupAdjustmentPreview{
		EffectiveFrom: input.EffectiveFrom, CurrentPlan: pickupPlanLabel(plans.current),
		ProposedPlan: proposedPickupPlanMapLabel(plans.proposed), DeviatesFromOffering: deviates,
		ResolutionRequired: reviewEnabled && deviates, MatchingOfferings: []careplan.PickupOfferingMatch{},
	}, nil
}

// attachOfferingAdjustment adds the booking consequences of the proposed
// selection and the offerings whose pickup times match the proposed plan.
// Without care offerings or an enrollment there is nothing to switch to.
func (s *PickupAdjustments) attachOfferingAdjustment(
	ctx context.Context,
	input careplan.PickupAdjustmentPreviewInput,
	proposed map[int]careplan.PickupAdjustmentSchedule,
	preview *careplan.PickupAdjustmentPreview,
) error {
	direct, err := s.deps.Offerings.PreviewDirectOfferingAdjustment(ctx, directPreviewInput(input))
	if errors.Is(err, careplan.ErrCareOfferingsDisabled) || errors.Is(err, careplan.ErrOfferingChangeNoEnrollment) {
		return nil
	}
	if err != nil || direct == nil {
		return err
	}
	preview.OfferingCatalog = direct.Catalog
	preview.OfferingConsequences = direct.Consequences
	preview.MatchingOfferings, err = s.matchingPickupOfferings(ctx, input, direct.Catalog, proposed)
	return err
}

func directPreviewInput(input careplan.PickupAdjustmentPreviewInput) careplan.DirectOfferingAdjustmentInput {
	return careplan.DirectOfferingAdjustmentInput{
		StudentID: input.StudentID, EffectiveFrom: input.EffectiveFrom,
		Selections: input.Selections, ExcludedAutoOfferingIDs: input.ExcludedAutoOfferingIDs,
	}
}

func (s *PickupAdjustments) matchingPickupOfferings(
	ctx context.Context,
	input careplan.PickupAdjustmentPreviewInput,
	catalog *careplan.OfferingChangeCatalog,
	proposed map[int]careplan.PickupAdjustmentSchedule,
) ([]careplan.PickupOfferingMatch, error) {
	if catalog == nil || !pickupPlanHasExactlyDays(proposed, input.CareDays) {
		return []careplan.PickupOfferingMatch{}, nil
	}
	careDays := canonicalDaysFromWeekdays(input.CareDays)
	matches := make([]careplan.PickupOfferingMatch, 0)
	for _, item := range catalog.Items {
		match, ok, err := s.matchingPickupOffering(ctx, input, catalog, item, careDays, proposed)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (s *PickupAdjustments) matchingPickupOffering(
	ctx context.Context,
	input careplan.PickupAdjustmentPreviewInput,
	catalog *careplan.OfferingChangeCatalog,
	item careplan.OfferingChangeCatalogItem,
	careDays []string,
	proposed map[int]careplan.PickupAdjustmentSchedule,
) (careplan.PickupOfferingMatch, bool, error) {
	selectedDays, eligible := pickupOfferingSelectedDays(item, careDays)
	if !eligible {
		return careplan.PickupOfferingMatch{}, false, nil
	}
	selections := pickupOfferingCandidateSelections(catalog, item, selectedDays)
	direct, err := s.deps.Offerings.PreviewDirectOfferingAdjustment(ctx, careplan.DirectOfferingAdjustmentInput{
		StudentID: input.StudentID, EffectiveFrom: input.EffectiveFrom,
		Selections: selections, ExcludedAutoOfferingIDs: input.ExcludedAutoOfferingIDs,
	})
	if err != nil {
		return careplan.PickupOfferingMatch{}, false, err
	}
	if direct == nil || !materializedPickupMatches(direct.MaterializedPickupTimes, proposed) {
		return careplan.PickupOfferingMatch{}, false, nil
	}
	return careplan.PickupOfferingMatch{
		OfferingID: item.OfferingID, Name: item.Name, SelectedDays: append([]string(nil), selectedDays...),
		Selections: cloneOfferingSelections(selections),
	}, true, nil
}

// preflightPickupOffering refuses a future switch while manual pickup rows
// exist: they have no effective date and can only be reset today.
func (s *PickupAdjustments) preflightPickupOffering(ctx context.Context, studentID int64, effectiveFrom, today calendar.Date) error {
	if !effectiveFrom.After(today) {
		return nil
	}
	existing, err := s.pickupRows(ctx, studentID)
	if err != nil {
		return err
	}
	if hasManualPickupRows(existing) {
		return careplan.ErrPickupAdjustmentFutureManualReset
	}
	return nil
}
