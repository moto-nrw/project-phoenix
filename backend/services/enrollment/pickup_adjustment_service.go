package enrollment

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

const (
	PickupAdjustmentResolutionException = "exception"
	PickupAdjustmentResolutionOffering  = "offering"
)

var (
	ErrPickupAdjustmentInvalid            = errors.New("pickup adjustment: invalid input")
	ErrPickupAdjustmentResolutionRequired = errors.New("pickup adjustment: explicit resolution is required")
	ErrPickupAdjustmentStale              = errors.New("pickup adjustment: preview is stale")
	ErrPickupAdjustmentFutureManualReset  = errors.New("pickup adjustment: manual pickup times can only be reset today")
	ErrPickupAdjustmentBulkConfirmation   = errors.New("pickup adjustment: bulk exceptions require confirmation")
	ErrPickupAdjustmentUnauthorized       = errors.New("pickup adjustment: student is not authorized")
	ErrPickupAdjustmentStudentNotFound    = errors.New("pickup adjustment: student not found")
)

type PickupAdjustmentSchedule struct {
	Weekday    int
	PickupTime string
	Notes      *string
}

type PickupAdjustmentArrivalSchedule struct {
	Weekday         int
	ExpectedArrival string
	Notes           *string
}

type PickupOfferingMatch struct {
	OfferingID   int64
	Name         string
	SelectedDays []string
	Selections   []OfferingChangeSelection
}

type PickupAdjustmentPreviewInput struct {
	StudentID               int64
	Schedules               []PickupAdjustmentSchedule
	ArrivalSchedules        *[]PickupAdjustmentArrivalSchedule
	CareDays                []int
	EffectiveFrom           timezone.Date
	Selections              []OfferingChangeSelection
	ExcludedAutoOfferingIDs []int64
}

type PickupAdjustmentPreview struct {
	PreviewToken         string
	EffectiveFrom        timezone.Date
	CurrentPlan          string
	ProposedPlan         string
	DeviatesFromOffering bool
	ResolutionRequired   bool
	MatchingOfferings    []PickupOfferingMatch
	OfferingCatalog      *OfferingChangeCatalog
	OfferingConsequences *OfferingChangePreview
	RemovedManualNotes   []PickupAdjustmentRemovedNote
}

type PickupAdjustmentRemovedNote struct {
	Weekday int
	Note    string
}

type PickupAdjustmentApplyInput struct {
	PickupAdjustmentPreviewInput
	PreviewToken     string
	Resolution       string
	Reason           string
	ActorAccountID   int64
	ActorRole        string
	CreatedByStaffID int64
	Authorize        func(context.Context, *usersModels.Student) (bool, error)
}

type PickupAdjustmentResult struct {
	Resolution string `json:"resolution"`
}

type PickupAdjustmentBulkInput struct {
	StudentIDs         []int64
	Schedules          []scheduleService.PickupScheduleInput
	ConfirmedException bool
	CreatedByStaffID   int64
	ActorAccountID     int64
	Authorize          func(context.Context, *usersModels.Student) (bool, error)
}

type PickupAdjustmentService interface {
	Preview(ctx context.Context, input PickupAdjustmentPreviewInput) (*PickupAdjustmentPreview, error)
	Apply(ctx context.Context, input PickupAdjustmentApplyInput) (*PickupAdjustmentResult, error)
	ApplyBulkExceptions(ctx context.Context, input PickupAdjustmentBulkInput) (*scheduleService.BulkUpsertResult, error)
}

type PickupAdjustmentServiceConfig struct {
	PickupSchedules     scheduleService.PickupScheduleService
	ArrivalSchedules    scheduleService.ArrivalScheduleService
	PickupScheduleRepo  scheduleModels.StudentPickupScheduleRepository
	ArrivalScheduleRepo scheduleModels.StudentArrivalScheduleRepository
	PickupBaselines     scheduleService.PickupBaselineReader
	Offerings           DirectOfferingAdjustmentCoordinator
	Settings            DecisionSettingsResolver
	Audit               usersService.StudentPickupPlanRecorder
	Students            usersModels.StudentRepository
	DB                  *bun.DB
}

type pickupAdjustmentService struct {
	PickupAdjustmentServiceConfig
}

func NewPickupAdjustmentService(cfg PickupAdjustmentServiceConfig) PickupAdjustmentService {
	return &pickupAdjustmentService{PickupAdjustmentServiceConfig: cfg}
}

func (s *pickupAdjustmentService) Preview(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
) (*PickupAdjustmentPreview, error) {
	input, explicitByDay, err := normalizePickupAdjustmentInput(input)
	if err != nil {
		return nil, err
	}
	if s.PickupBaselines == nil || s.Settings == nil {
		return nil, fmt.Errorf("pickup adjustment: preview dependencies are not configured")
	}
	if input.Selections != nil {
		if err := s.preflightPickupOffering(ctx, input.StudentID, input.EffectiveFrom); err != nil {
			return nil, err
		}
	}
	current, offering, proposed, err := s.projectPickupAdjustment(ctx, input, explicitByDay)
	if err != nil {
		return nil, err
	}
	preview, err := s.basePickupAdjustmentPreview(ctx, input, current, offering, proposed)
	if err != nil {
		return nil, err
	}
	if err := s.attachOfferingAdjustment(ctx, input, proposed, preview); err != nil {
		return nil, err
	}
	if err := s.attachRemovedManualNotes(ctx, input, preview); err != nil {
		return nil, err
	}
	currentArrival, err := s.currentArrivalPlan(ctx, input)
	if err != nil {
		return nil, err
	}
	preview.PreviewToken, err = pickupAdjustmentToken(
		input, preview, current, offering, currentArrival, tenant.FromContext(ctx),
	)
	return preview, err
}

func (s *pickupAdjustmentService) currentArrivalPlan(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
) ([]PickupAdjustmentArrivalSchedule, error) {
	if input.ArrivalSchedules == nil {
		return nil, nil
	}
	if s.ArrivalScheduleRepo == nil {
		return nil, errors.New("pickup adjustment: stored arrival schedule repository is not configured")
	}
	rows, err := s.ArrivalScheduleRepo.FindByStudentID(ctx, input.StudentID)
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: read current arrival schedules: %w", err)
	}
	current := make([]PickupAdjustmentArrivalSchedule, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		item := PickupAdjustmentArrivalSchedule{Weekday: row.Weekday}
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

func (s *pickupAdjustmentService) attachRemovedManualNotes(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
	preview *PickupAdjustmentPreview,
) error {
	if len(input.Selections) == 0 || preview.OfferingConsequences == nil {
		return nil
	}
	if s.PickupScheduleRepo == nil {
		return errors.New("pickup adjustment: stored pickup schedule repository is not configured")
	}
	notesByWeekday := make(map[int]string)
	representedWeekdays := make(map[int]bool, len(input.Schedules))
	for _, row := range input.Schedules {
		representedWeekdays[row.Weekday] = true
		if row.Notes != nil {
			if note := strings.TrimSpace(*row.Notes); note != "" {
				notesByWeekday[row.Weekday] = note
			}
		}
	}
	existing, err := s.PickupScheduleRepo.FindByStudentID(ctx, input.StudentID)
	if err != nil {
		return err
	}
	for _, row := range existing {
		if row == nil || representedWeekdays[row.Weekday] ||
			row.Source == scheduleModels.PickupScheduleSourceCareOffering || row.Notes == nil {
			continue
		}
		note := strings.TrimSpace(*row.Notes)
		if note != "" {
			notesByWeekday[row.Weekday] = note
		}
	}
	for weekday, note := range notesByWeekday {
		preview.RemovedManualNotes = append(preview.RemovedManualNotes, PickupAdjustmentRemovedNote{
			Weekday: weekday,
			Note:    note,
		})
	}
	sort.Slice(preview.RemovedManualNotes, func(i, j int) bool {
		return preview.RemovedManualNotes[i].Weekday < preview.RemovedManualNotes[j].Weekday
	})
	return nil
}

func (s *pickupAdjustmentService) projectPickupAdjustment(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
	explicit map[int]PickupAdjustmentSchedule,
) (scheduleService.PickupWeek, scheduleService.PickupWeek, map[int]PickupAdjustmentSchedule, error) {
	weekStart := input.EffectiveFrom.AddDays(scheduleModels.WeekdayMonday - isoWeekday(input.EffectiveFrom))
	projection, err := s.PickupBaselines.Project(
		ctx, []int64{input.StudentID}, weekStart, weekStart.AddDays(4),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("pickup adjustment: project current plan: %w", err)
	}
	current := projection.WeeklyForDate(input.StudentID, weekStart)
	offering := projection.OfferingWeeklyForDate(input.StudentID, weekStart)
	return current, offering, effectiveProposedPickupPlan(input.CareDays, explicit, offering), nil
}

func (s *pickupAdjustmentService) basePickupAdjustmentPreview(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
	current, offering scheduleService.PickupWeek,
	proposed map[int]PickupAdjustmentSchedule,
) (*PickupAdjustmentPreview, error) {
	reviewEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyRequirePickupOfferingReview)
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: resolve offering review setting: %w", err)
	}
	deviates := pickupPlanDeviates(input.CareDays, proposed, offering)
	return &PickupAdjustmentPreview{
		EffectiveFrom:        input.EffectiveFrom,
		CurrentPlan:          pickupPlanLabel(current),
		ProposedPlan:         proposedPickupPlanMapLabel(proposed),
		DeviatesFromOffering: deviates,
		ResolutionRequired:   reviewEnabled && deviates,
		MatchingOfferings:    []PickupOfferingMatch{},
	}, nil
}

func (s *pickupAdjustmentService) attachOfferingAdjustment(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
	proposed map[int]PickupAdjustmentSchedule,
	preview *PickupAdjustmentPreview,
) error {
	if s.Offerings == nil {
		return nil
	}
	direct, err := s.Offerings.PreviewDirectOfferingAdjustment(ctx, directPreviewInput(input))
	if errors.Is(err, ErrCareOfferingsDisabled) || errors.Is(err, ErrOfferingChangeNoEnrollment) {
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

func directPreviewInput(input PickupAdjustmentPreviewInput) DirectOfferingAdjustmentInput {
	return DirectOfferingAdjustmentInput{
		StudentID: input.StudentID, EffectiveFrom: input.EffectiveFrom,
		Selections: input.Selections, ExcludedAutoOfferingIDs: input.ExcludedAutoOfferingIDs,
	}
}

func (s *pickupAdjustmentService) Apply(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
) (*PickupAdjustmentResult, error) {
	if s.DB == nil || s.PickupSchedules == nil || s.Audit == nil || s.Students == nil {
		return nil, fmt.Errorf("pickup adjustment: apply dependencies are not configured")
	}
	normalized, _, err := normalizePickupAdjustmentInput(input.PickupAdjustmentPreviewInput)
	if err != nil {
		return nil, err
	}
	input.PickupAdjustmentPreviewInput = normalized
	resolution, err := pickupAdjustmentResolution(input)
	if err != nil {
		return nil, err
	}
	var result *PickupAdjustmentResult
	err = tenant.WithTenantTx(ctx, s.DB, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		var applyErr error
		result, applyErr = s.applyPickupAdjustment(txCtx, input, resolution)
		return applyErr
	})
	if err != nil {
		tenant.MarkRollback(ctx)
	}
	return result, err
}

func pickupAdjustmentResolution(input PickupAdjustmentApplyInput) (string, error) {
	resolution := strings.TrimSpace(input.Resolution)
	if resolution == "" {
		resolution = PickupAdjustmentResolutionException
	}
	if resolution != PickupAdjustmentResolutionException && resolution != PickupAdjustmentResolutionOffering {
		return "", fmt.Errorf("%w: unknown resolution %q", ErrPickupAdjustmentInvalid, resolution)
	}
	if resolution == PickupAdjustmentResolutionException && input.EffectiveFrom.After(timezone.TodayDate()) {
		return "", fmt.Errorf("%w: lasting exceptions take effect immediately", ErrPickupAdjustmentInvalid)
	}
	return resolution, nil
}

func (s *pickupAdjustmentService) applyPickupAdjustment(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
	resolution string,
) (*PickupAdjustmentResult, error) {
	if err := s.preparePickupAdjustment(ctx, input, resolution); err != nil {
		return nil, err
	}
	if err := s.lockAndAuthorizePickupStudent(ctx, input); err != nil {
		return nil, err
	}
	if resolution == PickupAdjustmentResolutionOffering {
		if err := s.preflightPickupOffering(ctx, input.StudentID, input.EffectiveFrom); err != nil {
			return nil, err
		}
	}
	preview, err := s.Preview(ctx, input.PickupAdjustmentPreviewInput)
	if err != nil {
		return nil, err
	}
	if !samePickupAdjustmentToken(input.PreviewToken, preview.PreviewToken) {
		return nil, ErrPickupAdjustmentStale
	}
	if preview.ResolutionRequired && strings.TrimSpace(input.Resolution) == "" {
		return nil, ErrPickupAdjustmentResolutionRequired
	}
	if err := s.applyPickupResolution(ctx, input, preview, resolution); err != nil {
		return nil, err
	}
	// Arrival schedules have no effective date. A future offering change must
	// therefore leave the current arrival plan untouched.
	if input.ArrivalSchedules != nil && appliesArrivalSchedulesImmediately(input.EffectiveFrom) {
		if s.ArrivalSchedules == nil {
			return nil, errors.New("pickup adjustment: arrival schedule service is not configured")
		}
		rows, err := pickupArrivalScheduleRows(
			input.StudentID, input.CreatedByStaffID, *input.ArrivalSchedules,
		)
		if err != nil {
			return nil, err
		}
		if err := s.ArrivalSchedules.UpsertBulkStudentArrivalSchedules(
			ctx, input.StudentID, rows,
		); err != nil {
			return nil, fmt.Errorf("pickup adjustment: update arrival schedules: %w", err)
		}
	}
	return &PickupAdjustmentResult{Resolution: resolution}, nil
}

func appliesArrivalSchedulesImmediately(effectiveFrom timezone.Date) bool {
	return !effectiveFrom.After(timezone.TodayDate())
}

func (s *pickupAdjustmentService) preparePickupAdjustment(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
	resolution string,
) error {
	if resolution != PickupAdjustmentResolutionOffering {
		return nil
	}
	if s.Offerings == nil {
		return fmt.Errorf("pickup adjustment: offering coordinator is not configured")
	}
	return s.Offerings.PrepareDirectOfferingAdjustment(ctx, directOfferingInput(input))
}

func (s *pickupAdjustmentService) lockAndAuthorizePickupStudent(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
) error {
	student, err := s.Students.FindByIDForUpdate(ctx, input.StudentID)
	if errors.Is(err, sql.ErrNoRows) || student == nil {
		return ErrPickupAdjustmentStudentNotFound
	}
	if err != nil {
		return fmt.Errorf("pickup adjustment: lock student: %w", err)
	}
	if input.Authorize == nil {
		return nil
	}
	allowed, err := input.Authorize(ctx, student)
	if err != nil || !allowed {
		return ErrPickupAdjustmentUnauthorized
	}
	return nil
}

func (s *pickupAdjustmentService) applyPickupResolution(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
	preview *PickupAdjustmentPreview,
	resolution string,
) error {
	if resolution == PickupAdjustmentResolutionOffering {
		return s.applyPickupOffering(ctx, input, preview)
	}
	return s.applyPickupException(ctx, input, preview)
}

func directOfferingInput(input PickupAdjustmentApplyInput) DirectOfferingAdjustmentInput {
	return DirectOfferingAdjustmentInput{
		StudentID: input.StudentID, EffectiveFrom: input.EffectiveFrom,
		Selections: input.Selections, ExcludedAutoOfferingIDs: input.ExcludedAutoOfferingIDs,
		Reason: input.Reason, ActorAccountID: input.ActorAccountID, ActorRole: input.ActorRole,
	}
}

func (s *pickupAdjustmentService) preflightPickupOffering(
	ctx context.Context,
	studentID int64,
	effectiveFrom timezone.Date,
) error {
	if !effectiveFrom.After(timezone.TodayDate()) {
		return nil
	}
	if s.PickupScheduleRepo == nil {
		return fmt.Errorf("pickup adjustment: stored pickup schedule repository is not configured")
	}
	existing, err := s.PickupScheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	if hasManualPickupRows(existing) {
		return ErrPickupAdjustmentFutureManualReset
	}
	return nil
}

func (s *pickupAdjustmentService) ApplyBulkExceptions(
	ctx context.Context,
	input PickupAdjustmentBulkInput,
) (*scheduleService.BulkUpsertResult, error) {
	if s.PickupSchedules == nil || s.Settings == nil {
		return nil, fmt.Errorf("pickup adjustment: bulk dependencies are not configured")
	}
	reviewEnabled, err := s.Settings.ResolveBool(ctx, configModel.KeyRequirePickupOfferingReview)
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: resolve offering review setting: %w", err)
	}
	filter := scheduleService.ArrivalScheduleBulkFilter{StudentIDs: input.StudentIDs, Authorize: input.Authorize}
	if !reviewEnabled {
		result, applyErr := s.PickupSchedules.BulkUpsertPickupSchedules(
			ctx, filter, input.Schedules, input.CreatedByStaffID,
		)
		if applyErr != nil {
			tenant.MarkRollback(ctx)
		}
		return result, applyErr
	}
	if !input.ConfirmedException {
		return nil, ErrPickupAdjustmentBulkConfirmation
	}
	if s.DB == nil || s.Audit == nil || s.Students == nil {
		return nil, fmt.Errorf("pickup adjustment: bulk review dependencies are not configured")
	}
	return s.applyReviewedBulkExceptions(ctx, input, filter)
}

func (s *pickupAdjustmentService) applyReviewedBulkExceptions(
	ctx context.Context,
	input PickupAdjustmentBulkInput,
	filter scheduleService.ArrivalScheduleBulkFilter,
) (*scheduleService.BulkUpsertResult, error) {
	var result *scheduleService.BulkUpsertResult
	err := tenant.WithTenantTx(ctx, s.DB, tenant.FromContext(ctx), func(txCtx context.Context, _ bun.Tx) error {
		before, err := s.lockAndSnapshotBulkStudents(txCtx, input)
		if err != nil {
			return err
		}
		result, err = s.PickupSchedules.BulkUpsertPickupSchedules(
			txCtx, filter, input.Schedules, input.CreatedByStaffID,
		)
		if err != nil {
			return err
		}
		return s.auditBulkPickupPlans(txCtx, input, result.AffectedStudentIDs, before)
	})
	if err != nil {
		tenant.MarkRollback(ctx)
	}
	return result, err
}

func (s *pickupAdjustmentService) lockAndSnapshotBulkStudents(
	ctx context.Context,
	input PickupAdjustmentBulkInput,
) (map[int64]string, error) {
	studentIDs := slices.Clone(input.StudentIDs)
	sort.Slice(studentIDs, func(i, j int) bool { return studentIDs[i] < studentIDs[j] })
	for _, studentID := range studentIDs {
		student, err := s.Students.FindByIDForUpdate(ctx, studentID)
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("%w: student %d", scheduleService.ErrBulkStudentNotFound, studentID)
		}
		if err != nil {
			return nil, fmt.Errorf("pickup adjustment: lock student %d: %w", studentID, err)
		}
		if input.Authorize != nil {
			allowed, authorizeErr := input.Authorize(ctx, student)
			if authorizeErr != nil || !allowed {
				return nil, fmt.Errorf("%w: student %d", scheduleService.ErrBulkStudentUnauthorized, studentID)
			}
		}
	}
	return s.projectPickupPlanLabels(ctx, studentIDs)
}

func (s *pickupAdjustmentService) auditBulkPickupPlans(
	ctx context.Context,
	input PickupAdjustmentBulkInput,
	studentIDs []int64,
	before map[int64]string,
) error {
	after, err := s.projectPickupPlanLabels(ctx, studentIDs)
	if err != nil {
		return err
	}
	for _, studentID := range studentIDs {
		prior, ok := before[studentID]
		if !ok {
			return errors.New("pickup adjustment: bulk preview is missing")
		}
		if prior == after[studentID] {
			continue
		}
		if err := s.Audit.RecordPickupPlanForActor(
			ctx, studentID, prior, after[studentID],
			"Dauerhafte Ausnahme", "", input.ActorAccountID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *pickupAdjustmentService) projectPickupPlanLabels(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]string, error) {
	if s.PickupBaselines == nil {
		return nil, fmt.Errorf("pickup adjustment: pickup baseline reader is not configured")
	}
	today := timezone.TodayDate()
	weekStart := today.AddDays(scheduleModels.WeekdayMonday - isoWeekday(today))
	projection, err := s.PickupBaselines.Project(ctx, studentIDs, weekStart, weekStart.AddDays(4))
	if err != nil {
		return nil, err
	}
	labels := make(map[int64]string, len(studentIDs))
	for _, studentID := range studentIDs {
		labels[studentID] = pickupPlanLabel(projection.WeeklyForDate(studentID, weekStart))
	}
	return labels, nil
}

func (s *pickupAdjustmentService) applyPickupException(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
	preview *PickupAdjustmentPreview,
) error {
	rows, err := pickupScheduleRows(input.StudentID, input.CreatedByStaffID, input.Schedules)
	if err != nil {
		return err
	}
	if err := s.PickupSchedules.UpsertBulkStudentPickupSchedulesForDate(
		ctx, input.StudentID, input.EffectiveFrom, rows,
	); err != nil {
		return err
	}
	if preview.CurrentPlan == preview.ProposedPlan {
		return nil
	}
	return s.Audit.RecordPickupPlanForActor(
		ctx, input.StudentID, preview.CurrentPlan, preview.ProposedPlan,
		"Dauerhafte Ausnahme", input.Reason, input.ActorAccountID,
	)
}

func (s *pickupAdjustmentService) applyPickupOffering(
	ctx context.Context,
	input PickupAdjustmentApplyInput,
	preview *PickupAdjustmentPreview,
) error {
	if s.Offerings == nil || len(input.Selections) == 0 || preview.OfferingConsequences == nil {
		return fmt.Errorf("%w: an offering selection preview is required", ErrPickupAdjustmentInvalid)
	}
	if s.PickupScheduleRepo == nil {
		return fmt.Errorf("pickup adjustment: stored pickup schedule repository is not configured")
	}
	if !selectsExactPickupOffering(input.Selections, preview.MatchingOfferings) {
		return fmt.Errorf("%w: selected care offering does not match the pickup plan", ErrPickupAdjustmentInvalid)
	}
	if err := s.Offerings.ApplyDirectOfferingAdjustment(ctx, directOfferingInput(input)); err != nil {
		return err
	}
	if err := s.deleteManualPickupSchedules(ctx, input.StudentID); err != nil {
		return err
	}
	return s.Audit.RecordPickupPlanForActor(
		ctx, input.StudentID, preview.CurrentPlan, preview.ProposedPlan,
		"Angebot geändert", input.Reason, input.ActorAccountID,
	)
}

func (s *pickupAdjustmentService) deleteManualPickupSchedules(
	ctx context.Context,
	studentID int64,
) error {
	// The student row is locked by Apply. Pickup-plan writers use the same lock,
	// so this is the complete manual plan represented by the before/after preview.
	existing, err := s.PickupScheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}
	for _, row := range existing {
		if row != nil && row.ID > 0 && row.Source != scheduleModels.PickupScheduleSourceCareOffering {
			if err := s.PickupSchedules.DeleteStudentPickupSchedule(ctx, row.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func selectsExactPickupOffering(
	selections []OfferingChangeSelection,
	matches []PickupOfferingMatch,
) bool {
	for _, candidate := range matches {
		if offeringSelectionsEqual(selections, candidate.Selections) {
			return true
		}
	}
	return false
}

func offeringSelectionsEqual(left, right []OfferingChangeSelection) bool {
	if len(left) != len(right) {
		return false
	}
	left = cloneOfferingSelections(left)
	right = cloneOfferingSelections(right)
	sort.Slice(left, func(i, j int) bool { return left[i].OfferingID < left[j].OfferingID })
	sort.Slice(right, func(i, j int) bool { return right[i].OfferingID < right[j].OfferingID })
	for i := range left {
		if left[i].OfferingID != right[i].OfferingID ||
			!slices.Equal(canonicalDays(left[i].SelectedDays), canonicalDays(right[i].SelectedDays)) {
			return false
		}
	}
	return true
}

func normalizePickupAdjustmentInput(
	input PickupAdjustmentPreviewInput,
) (PickupAdjustmentPreviewInput, map[int]PickupAdjustmentSchedule, error) {
	if input.StudentID <= 0 {
		return input, nil, fmt.Errorf("%w: student is required", ErrPickupAdjustmentInvalid)
	}
	if len(input.Schedules) > 0 && len(input.CareDays) == 0 {
		return input, nil, fmt.Errorf("%w: pickup times require care days", ErrPickupAdjustmentInvalid)
	}
	if input.EffectiveFrom.IsZero() {
		input.EffectiveFrom = timezone.TodayDate()
	}
	if input.EffectiveFrom.Before(timezone.TodayDate()) {
		return input, nil, fmt.Errorf("%w: effective date is in the past", ErrPickupAdjustmentInvalid)
	}
	input.CareDays = append([]int(nil), input.CareDays...)
	sort.Ints(input.CareDays)
	input.CareDays = slices.Compact(input.CareDays)
	for _, weekday := range input.CareDays {
		if weekday < scheduleModels.WeekdayMonday || weekday > scheduleModels.WeekdayFriday {
			return input, nil, fmt.Errorf("%w: invalid care weekday %d", ErrPickupAdjustmentInvalid, weekday)
		}
	}
	if err := normalizePickupArrivalSchedules(&input); err != nil {
		return input, nil, err
	}
	byDay := make(map[int]PickupAdjustmentSchedule, len(input.Schedules))
	for i := range input.Schedules {
		row := &input.Schedules[i]
		row.PickupTime = strings.TrimSpace(row.PickupTime)
		if row.Weekday < scheduleModels.WeekdayMonday || row.Weekday > scheduleModels.WeekdayFriday {
			return input, nil, fmt.Errorf("%w: invalid weekday %d", ErrPickupAdjustmentInvalid, row.Weekday)
		}
		if _, duplicate := byDay[row.Weekday]; duplicate {
			return input, nil, fmt.Errorf("%w: duplicate weekday %d", ErrPickupAdjustmentInvalid, row.Weekday)
		}
		if _, err := time.Parse("15:04", row.PickupTime); err != nil {
			return input, nil, fmt.Errorf("%w: invalid pickup time for weekday %d", ErrPickupAdjustmentInvalid, row.Weekday)
		}
		byDay[row.Weekday] = *row
	}
	sort.Slice(input.Schedules, func(i, j int) bool { return input.Schedules[i].Weekday < input.Schedules[j].Weekday })
	return input, byDay, nil
}

func normalizePickupArrivalSchedules(input *PickupAdjustmentPreviewInput) error {
	if input.ArrivalSchedules == nil {
		return nil
	}
	rows := append([]PickupAdjustmentArrivalSchedule(nil), (*input.ArrivalSchedules)...)
	seen := make(map[int]bool, len(rows))
	weekdays := make([]int, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		row.ExpectedArrival = strings.TrimSpace(row.ExpectedArrival)
		if row.Weekday < scheduleModels.WeekdayMonday || row.Weekday > scheduleModels.WeekdayFriday || seen[row.Weekday] {
			return fmt.Errorf("%w: invalid or duplicate arrival weekday %d", ErrPickupAdjustmentInvalid, row.Weekday)
		}
		if row.ExpectedArrival != "" {
			if _, err := time.Parse("15:04", row.ExpectedArrival); err != nil {
				return fmt.Errorf("%w: invalid arrival time for weekday %d", ErrPickupAdjustmentInvalid, row.Weekday)
			}
		}
		if row.Notes != nil && len(*row.Notes) > 500 {
			return fmt.Errorf("%w: arrival notes are too long", ErrPickupAdjustmentInvalid)
		}
		seen[row.Weekday] = true
		weekdays = append(weekdays, row.Weekday)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Weekday < rows[j].Weekday })
	sort.Ints(weekdays)
	if !slices.Equal(weekdays, input.CareDays) {
		return fmt.Errorf("%w: arrival weekdays must match care days", ErrPickupAdjustmentInvalid)
	}
	input.ArrivalSchedules = &rows
	return nil
}

func pickupScheduleRows(
	studentID, createdBy int64,
	input []PickupAdjustmentSchedule,
) ([]*scheduleModels.StudentPickupSchedule, error) {
	rows := make([]*scheduleModels.StudentPickupSchedule, 0, len(input))
	for _, item := range input {
		parsed, err := time.Parse("15:04", item.PickupTime)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid pickup time", ErrPickupAdjustmentInvalid)
		}
		rows = append(rows, &scheduleModels.StudentPickupSchedule{
			StudentID:  studentID,
			Weekday:    item.Weekday,
			PickupTime: parsed,
			Notes:      item.Notes,
			CreatedBy:  createdBy,
			Source:     scheduleModels.PickupScheduleSourceStaff,
		})
	}
	return rows, nil
}

func pickupArrivalScheduleRows(
	studentID, createdBy int64,
	input []PickupAdjustmentArrivalSchedule,
) ([]*scheduleModels.StudentArrivalSchedule, error) {
	rows := make([]*scheduleModels.StudentArrivalSchedule, 0, len(input))
	for _, item := range input {
		var expectedArrival time.Time
		if item.ExpectedArrival != "" {
			parsed, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+item.ExpectedArrival)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid arrival time", ErrPickupAdjustmentInvalid)
			}
			expectedArrival = parsed
		}
		rows = append(rows, &scheduleModels.StudentArrivalSchedule{
			StudentID:       studentID,
			Weekday:         item.Weekday,
			ExpectedArrival: expectedArrival,
			Notes:           item.Notes,
			CreatedBy:       createdBy,
		})
	}
	return rows, nil
}

func (s *pickupAdjustmentService) matchingPickupOfferings(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
	catalog *OfferingChangeCatalog,
	proposed map[int]PickupAdjustmentSchedule,
) ([]PickupOfferingMatch, error) {
	if catalog == nil {
		return []PickupOfferingMatch{}, nil
	}
	canonicalCareDays := canonicalDaysFromWeekdays(input.CareDays)
	if !pickupPlanHasExactlyDays(proposed, input.CareDays) {
		return []PickupOfferingMatch{}, nil
	}
	matches := make([]PickupOfferingMatch, 0)
	for _, item := range catalog.Items {
		match, ok, err := s.matchingPickupOffering(ctx, input, catalog, item, canonicalCareDays, proposed)
		if err != nil {
			return nil, err
		}
		if ok {
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (s *pickupAdjustmentService) matchingPickupOffering(
	ctx context.Context,
	input PickupAdjustmentPreviewInput,
	catalog *OfferingChangeCatalog,
	item OfferingChangeCatalogItem,
	careDays []string,
	proposed map[int]PickupAdjustmentSchedule,
) (PickupOfferingMatch, bool, error) {
	selectedDays, eligible := pickupOfferingSelectedDays(item, careDays)
	if !eligible {
		return PickupOfferingMatch{}, false, nil
	}
	selections := pickupOfferingCandidateSelections(catalog, item, selectedDays)
	direct, err := s.Offerings.PreviewDirectOfferingAdjustment(ctx, DirectOfferingAdjustmentInput{
		StudentID: input.StudentID, EffectiveFrom: input.EffectiveFrom,
		Selections: selections, ExcludedAutoOfferingIDs: input.ExcludedAutoOfferingIDs,
	})
	if err != nil {
		return PickupOfferingMatch{}, false, err
	}
	if direct == nil || !materializedPickupMatches(direct.MaterializedPickupTimes, proposed) {
		return PickupOfferingMatch{}, false, nil
	}
	return PickupOfferingMatch{
		OfferingID: item.OfferingID, Name: item.Name, SelectedDays: slices.Clone(selectedDays),
		Selections: cloneOfferingSelections(selections),
	}, true, nil
}

func pickupOfferingSelectedDays(item OfferingChangeCatalogItem, careDays []string) ([]string, bool) {
	if !item.IsActive || item.Selected || !item.CountsAsCare {
		return nil, false
	}
	if item.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
		return nil, slices.Equal(canonicalDays(item.AvailableDays), careDays)
	}
	if !canonicalDaySubset(careDays, item.AvailableDays) {
		return nil, false
	}
	return slices.Clone(careDays), true
}

func pickupOfferingCandidateSelections(
	catalog *OfferingChangeCatalog,
	target OfferingChangeCatalogItem,
	selectedDays []string,
) []OfferingChangeSelection {
	selections := make([]OfferingChangeSelection, 0)
	for _, item := range catalog.Items {
		if !item.Selected || item.Automatic || item.CountsAsCare ||
			(target.SelectionGroup != "" && item.SelectionGroup == target.SelectionGroup) {
			continue
		}
		selections = append(selections, OfferingChangeSelection{
			OfferingID: item.OfferingID, SelectedDays: slices.Clone(item.SelectedDays),
		})
	}
	return append(selections, OfferingChangeSelection{
		OfferingID: target.OfferingID, SelectedDays: slices.Clone(selectedDays),
	})
}

func materializedPickupMatches(
	profile map[string]string,
	proposed map[int]PickupAdjustmentSchedule,
) bool {
	if len(profile) != len(proposed) {
		return false
	}
	for weekday, row := range proposed {
		if strings.TrimSpace(profile[canonicalDayForISOWeekday(weekday)]) != row.PickupTime {
			return false
		}
	}
	return true

}

func cloneOfferingSelections(input []OfferingChangeSelection) []OfferingChangeSelection {
	result := make([]OfferingChangeSelection, 0, len(input))
	for _, item := range input {
		result = append(result, OfferingChangeSelection{
			OfferingID: item.OfferingID, SelectedDays: slices.Clone(item.SelectedDays),
		})
	}
	return result
}

func pickupPlanDeviates(
	careDays []int,
	proposed map[int]PickupAdjustmentSchedule,
	offering scheduleService.PickupWeek,
) bool {
	if !pickupPlanHasExactlyDays(proposed, careDays) {
		return true
	}
	offeringDays := make([]int, 0, len(offering))
	for weekday, row := range offering {
		if row != nil {
			offeringDays = append(offeringDays, weekday)
		}
	}
	sort.Ints(offeringDays)
	if !slices.Equal(careDays, offeringDays) {
		return true
	}
	for _, weekday := range careDays {
		row := offering[weekday]
		if row == nil || row.PickupTime.Format("15:04") != proposed[weekday].PickupTime {
			return true
		}
	}
	return false
}

func pickupPlanHasExactlyDays(plan map[int]PickupAdjustmentSchedule, careDays []int) bool {
	days := make([]int, 0, len(plan))
	for weekday := range plan {
		days = append(days, weekday)
	}
	sort.Ints(days)
	expected := slices.Clone(careDays)
	sort.Ints(expected)
	return slices.Equal(days, expected)
}

func effectiveProposedPickupPlan(
	careDays []int,
	explicit map[int]PickupAdjustmentSchedule,
	offering scheduleService.PickupWeek,
) map[int]PickupAdjustmentSchedule {
	result := make(map[int]PickupAdjustmentSchedule, len(explicit)+len(careDays))
	for weekday, row := range explicit {
		result[weekday] = row
	}
	for _, weekday := range careDays {
		if _, exists := result[weekday]; exists {
			continue
		}
		if row := offering[weekday]; row != nil {
			result[weekday] = PickupAdjustmentSchedule{
				Weekday: weekday, PickupTime: row.PickupTime.Format("15:04"), Notes: row.Notes,
			}
		}
	}
	return result
}

func hasManualPickupRows(rows []*scheduleModels.StudentPickupSchedule) bool {
	for _, row := range rows {
		if row != nil && row.Source != scheduleModels.PickupScheduleSourceCareOffering {
			return true
		}
	}
	return false
}

func pickupPlanLabel(week scheduleService.PickupWeek) string {
	rows := make([]PickupAdjustmentSchedule, 0, len(week))
	for weekday, row := range week {
		if row != nil {
			rows = append(rows, PickupAdjustmentSchedule{
				Weekday: weekday, PickupTime: row.PickupTime.Format("15:04"), Notes: row.Notes,
			})
		}
	}
	return proposedPickupPlanLabel(rows)
}

func proposedPickupPlanLabel(rows []PickupAdjustmentSchedule) string {
	rows = slices.Clone(rows)
	sort.Slice(rows, func(i, j int) bool { return rows[i].Weekday < rows[j].Weekday })
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		label := fmt.Sprintf("%s %s Uhr", shortGermanWeekday(row.Weekday), row.PickupTime)
		if row.Notes != nil && strings.TrimSpace(*row.Notes) != "" {
			label += fmt.Sprintf(" (Notiz: %s)", *row.Notes)
		}
		parts = append(parts, label)
	}
	if len(parts) == 0 {
		return "Kein Wochenplan"
	}
	return strings.Join(parts, ", ")
}

func proposedPickupPlanMapLabel(rows map[int]PickupAdjustmentSchedule) string {
	list := make([]PickupAdjustmentSchedule, 0, len(rows))
	for _, row := range rows {
		list = append(list, row)
	}
	return proposedPickupPlanLabel(list)
}

func shortGermanWeekday(weekday int) string {
	return map[int]string{1: "Mo", 2: "Di", 3: "Mi", 4: "Do", 5: "Fr"}[weekday]
}

func canonicalDaysFromWeekdays(weekdays []int) []string {
	days := make([]string, 0, len(weekdays))
	for _, weekday := range weekdays {
		days = append(days, canonicalDayForISOWeekday(weekday))
	}
	return canonicalDays(days)
}

func canonicalDayForISOWeekday(weekday int) string {
	return map[int]string{1: "mon", 2: "tue", 3: "wed", 4: "thu", 5: "fri", 6: "sat", 7: "sun"}[weekday]
}

func canonicalDaySubset(days, available []string) bool {
	available = canonicalDays(available)
	for _, day := range days {
		if !slices.Contains(available, day) {
			return false
		}
	}
	return true
}

func isoWeekday(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func pickupAdjustmentToken(
	input PickupAdjustmentPreviewInput,
	preview *PickupAdjustmentPreview,
	current, offering scheduleService.PickupWeek,
	currentArrival []PickupAdjustmentArrivalSchedule,
	tenantID int64,
) (string, error) {
	payload := struct {
		TenantID       int64
		Input          PickupAdjustmentPreviewInput
		Preview        *PickupAdjustmentPreview
		Current        scheduleService.PickupWeek
		Offering       scheduleService.PickupWeek
		CurrentArrival []PickupAdjustmentArrivalSchedule
	}{tenantID, input, preview, current, offering, currentArrival}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("pickup adjustment: build preview token: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func samePickupAdjustmentToken(left, right string) bool {
	leftBytes, leftErr := hex.DecodeString(strings.TrimSpace(left))
	rightBytes, rightErr := hex.DecodeString(strings.TrimSpace(right))
	return leftErr == nil && rightErr == nil && len(leftBytes) == sha256.Size &&
		len(rightBytes) == sha256.Size && subtle.ConstantTimeCompare(leftBytes, rightBytes) == 1
}
