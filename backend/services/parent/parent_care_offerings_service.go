// Parent-portal read view for the child's booked care offerings (#1665).
package parent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CareOfferingSelection is one booked care offering of the current care period.
type CareOfferingSelection struct {
	OfferingID int64
	Name       string
	// Description is the admin-authored explanation shown on the enrollment
	// form; guardians recognise offerings by it.
	Description string
	// Weekdays are the booked ISO weekdays (1=Mon..7=Sun), sorted. Empty means
	// the offering has no per-day choice.
	Weekdays []int
	// PriceCents is the offering's price when the school configured one.
	PriceCents      *int
	IncludesLunch   bool
	IncludesHoliday bool
	// ValidFrom is set for a scheduled future booking. Nil means the offering
	// has been effective since the start of the care period.
	ValidFrom *timezone.Date
	// ValidUntil is exclusive, mirroring the stored interval.
	ValidUntil *timezone.Date
	// StartsLater identifies an approved change that has not taken effect yet.
	StartsLater bool
}

// PendingOfferingChange is the child's open change request as the guardian sees
// it: the live "current → requested" diff, when it would take effect, and
// whether the CALLING guardian submitted it (only the submitter may withdraw).
type PendingOfferingChange struct {
	ID              int64
	CreatedAt       time.Time
	EffectiveFrom   timezone.Date
	Note            string
	Diff            []enrollmentSvc.OfferingChangeDiffEntry
	SubmittedBySelf bool
}

// ChildCareOfferings is the parent-facing view of what a child is booked into.
type ChildCareOfferings struct {
	// PeriodName / PeriodStart / PeriodEnd describe the care period the
	// offerings belong to. Zero values when the child has no enrollment.
	PeriodName  string
	PeriodStart timezone.Date
	PeriodEnd   timezone.Date
	Offerings   []CareOfferingSelection
	// CanRequest gates the "Änderung anfragen" button. False without an
	// approved enrollment behind the child, without the guardian permission,
	// or when the school has post-enrollment changes switched off.
	CanRequest bool
	// ChangesDisabledReason explains a false CanRequest to the UI so it can
	// say why instead of silently hiding the button.
	ChangesDisabledReason string
	// PendingRequest is the child's open change request, nil when none.
	PendingRequest *PendingOfferingChange
	// LastDecision is the most recent decided request inside the recency window,
	// so the outcome of a request is visible where it was submitted.
	LastDecision *enrollmentSvc.OfferingChangeDecision
	// EarliestEffectiveFrom is the first date a new request may take effect
	// under the school's notice period — the date picker's lower bound. Zero
	// when requesting is not possible anyway.
	EarliestEffectiveFrom timezone.Date
}

// Reasons for a disabled change button. Wire-stable identifiers; the German
// copy lives in the frontend.
const (
	OfferingChangesReasonNoEnrollment = "no_enrollment"
	OfferingChangesReasonNoPermission = "no_permission"
	OfferingChangesReasonSchoolOff    = "school_disabled"
	OfferingChangesReasonPeriodOver   = "period_over"
	OfferingChangesReasonNoTime       = "no_time_remaining"
)

// GetChildCareOfferings returns what the child is booked into today (plus
// anything already scheduled to start later). Authorization is
// parent_portal.access only: seeing the booking is part of seeing the child,
// independent of whether change requests are switched on.
func (s *service) GetChildCareOfferings(ctx context.Context, accountID, studentID int64) (*ChildCareOfferings, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	view := &ChildCareOfferings{
		Offerings: []CareOfferingSelection{},
	}
	today := timezone.TodayDate()
	var period *enrollmentModels.StudentCarePeriod
	var canRequest bool
	var changesDisabledReason string
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, loadErr := s.loadChildCareOfferings(txCtx, studentID, today, view)
		if loadErr != nil {
			return loadErr
		}
		period = resolved
		if err := s.loadOfferingChangeState(txCtx, accountID, studentID, view); err != nil {
			return err
		}
		canRequest, changesDisabledReason = s.resolveOfferingChangeAvailabilityForStudent(txCtx, child, studentID, period, today)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get child care offerings: %w", txErr)
	}
	view.CanRequest, view.ChangesDisabledReason = canRequest, changesDisabledReason
	return view, nil
}

func (s *service) loadChildCareOfferings(
	ctx context.Context,
	studentID int64,
	today timezone.Date,
	view *ChildCareOfferings,
) (*enrollmentModels.StudentCarePeriod, error) {
	period, err := s.currentCarePeriod(ctx, studentID, today)
	if err != nil {
		return nil, err
	}
	if period != nil {
		view.PeriodName = period.PhaseName
		view.PeriodStart = period.ServiceStartDate
		view.PeriodEnd = period.ServiceEndDate
		// Shared with the staff views so both sides answer "what is booked,
		// what starts later" from the same day (#2185).
		offeringDate := enrollmentSvc.BookingViewDate(today, period.ServiceEndDate)
		view.Offerings, err = s.carePeriodOfferings(ctx, period.RequestChildID, offeringDate)
		if err != nil {
			return nil, err
		}
	}
	return period, nil
}

func (s *service) loadOfferingChangeState(
	ctx context.Context,
	accountID, studentID int64,
	view *ChildCareOfferings,
) error {
	if s.OfferingChanges == nil {
		return nil
	}
	pending, err := s.OfferingChanges.GetForStudent(ctx, studentID)
	if err != nil {
		return err
	}
	if pending != nil {
		view.LastDecision = pending.LastDecision
	}
	if pending != nil && pending.Request != nil {
		view.PendingRequest = pendingOfferingChange(pending, accountID)
	}
	view.EarliestEffectiveFrom, err = s.OfferingChanges.EarliestEffectiveFrom(ctx)
	return err
}

func pendingOfferingChange(view *enrollmentSvc.OfferingChangeView, accountID int64) *PendingOfferingChange {
	if view.Request == nil {
		return nil
	}
	pending := &PendingOfferingChange{
		ID:              view.Request.ID,
		CreatedAt:       view.Request.CreatedAt,
		EffectiveFrom:   view.Request.EffectiveFrom,
		Diff:            view.Diff,
		SubmittedBySelf: view.Request.SubmittedBy == accountID,
	}
	if view.Request.ParentNote != nil {
		pending.Note = *view.Request.ParentNote
	}
	return pending
}

// GetChildOfferingCatalog returns the offerings the guardian may choose from,
// prefilled with the current booking. Requires the same permission as
// submitting: the catalog exposes the school's capacity situation.
func (s *service) GetChildOfferingCatalog(
	ctx context.Context,
	accountID, studentID int64,
) (*enrollmentSvc.OfferingChangeCatalog, error) {
	return s.GetChildOfferingCatalogAt(ctx, accountID, studentID, timezone.Date{})
}

func (s *service) GetChildOfferingCatalogAt(
	ctx context.Context,
	accountID, studentID int64,
	effectiveFrom timezone.Date,
) (*enrollmentSvc.OfferingChangeCatalog, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrOfferingChangeDisabled
	}
	var catalog *enrollmentSvc.OfferingChangeCatalog
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, resolveErr := s.OfferingChanges.CatalogAt(txCtx, studentID, effectiveFrom)
		if resolveErr != nil {
			return resolveErr
		}
		catalog = resolved
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return catalog, nil
}

// CreateOfferingChangeRequest stores a pending post-enrollment change request
// for staff review. Requires parent_portal.request.submit.
func (s *service) CreateOfferingChangeRequest(
	ctx context.Context,
	accountID, studentID int64,
	selections []enrollmentSvc.OfferingChangeSelection,
	effectiveFrom timezone.Date,
	note string,
	completeWithdrawalConfirmed bool,
) (*ChildCareOfferings, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrOfferingChangeDisabled
	}
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(timezone.TodayDate()) {
			return ErrChildCareEnded
		}
		_, createErr := s.OfferingChanges.Create(txCtx, enrollmentSvc.CreateOfferingChangeInput{
			StudentID:                   studentID,
			AccountID:                   accountID,
			Selections:                  selections,
			EffectiveFrom:               effectiveFrom,
			Note:                        note,
			CompleteWithdrawalConfirmed: completeWithdrawalConfirmed,
		})
		return createErr
	})
	if txErr != nil {
		return nil, txErr
	}
	s.Logger.Info("parent created offering change request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return s.GetChildCareOfferings(ctx, accountID, studentID)
}

// WithdrawOfferingChangeRequest flips the caller's own pending request to
// withdrawn. It stays available after the school switches the feature off, but
// not after the child's care has ended.
func (s *service) WithdrawOfferingChangeRequest(
	ctx context.Context,
	accountID, studentID, requestID int64,
) (*ChildCareOfferings, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrOfferingChangeDisabled
	}
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(timezone.TodayDate()) {
			return ErrChildCareEnded
		}
		return s.OfferingChanges.Withdraw(txCtx, requestID, accountID, studentID)
	})
	if txErr != nil {
		return nil, txErr
	}
	return s.GetChildCareOfferings(ctx, accountID, studentID)
}

// currentCarePeriod picks the care period the offering view should describe:
// the one covering today, else the next one starting in the future, else the
// most recent past one so a family between school years still sees what was
// booked last. Nil when the child has no approved enrollment at all.
func (s *service) currentCarePeriod(
	ctx context.Context,
	studentID int64,
	today timezone.Date,
) (*enrollmentModels.StudentCarePeriod, error) {
	if s.RequestChildRepo == nil {
		return nil, nil
	}
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("list care periods: %w", err)
	}
	if len(periods) == 0 {
		return nil, nil
	}
	// Repository order is latest window first.
	var upcoming, past *enrollmentModels.StudentCarePeriod
	for _, candidate := range periods {
		switch {
		case !candidate.ServiceStartDate.After(today) && !candidate.ServiceEndDate.Before(today):
			return candidate, nil
		case candidate.ServiceStartDate.After(today):
			upcoming = candidate
		case past == nil:
			past = candidate
		}
	}
	if upcoming != nil {
		return upcoming, nil
	}
	return past, nil
}

func (s *service) carePeriodOfferings(
	ctx context.Context,
	requestChildID int64,
	today timezone.Date,
) ([]CareOfferingSelection, error) {
	if s.RequestChildOfferingRepo == nil || s.CareOfferingRepo == nil {
		return []CareOfferingSelection{}, nil
	}
	links, err := s.RequestChildOfferingRepo.ListHistoryByRequestChildID(ctx, requestChildID)
	if err != nil {
		return nil, fmt.Errorf("list child offerings: %w", err)
	}
	if len(links) == 0 {
		return []CareOfferingSelection{}, nil
	}
	offeringIDs := uniqueOfferingIDs(links)
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("list care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			offeringByID[offering.ID] = offering
		}
	}
	items := make([]CareOfferingSelection, 0, len(links))
	for _, link := range links {
		if link == nil || (link.ValidUntil != nil && !link.ValidUntil.After(today)) {
			continue
		}
		item, ok := careOfferingSelection(offeringByID[link.CareOfferingID], link, today)
		if !ok {
			continue
		}
		items = append(items, item)
	}
	sortCareOfferingSelections(items, offerings)
	return items, nil
}

func uniqueOfferingIDs(links []*enrollmentModels.RequestChildOffering) []int64 {
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

func careOfferingSelection(
	offering *enrollmentModels.CareOffering,
	link *enrollmentModels.RequestChildOffering,
	today timezone.Date,
) (CareOfferingSelection, bool) {
	if offering == nil {
		// The offering was deleted from the catalog. Skipped rather than shown
		// as an unnamed row: a guardian cannot act on it either way.
		return CareOfferingSelection{}, false
	}
	item := CareOfferingSelection{
		OfferingID:      offering.ID,
		Name:            offering.Name,
		Weekdays:        weekdaysFromOfferingDays(careOfferingDays(offering, link)),
		PriceCents:      offering.PriceCents,
		IncludesLunch:   offering.IncludesLunch,
		IncludesHoliday: offering.IncludesHolidayCare,
		ValidFrom:       link.ValidFrom,
		ValidUntil:      link.ValidUntil,
	}
	item.StartsLater = link.ValidFrom != nil && link.ValidFrom.After(today)
	if offering.Description != nil {
		item.Description = *offering.Description
	}
	return item, true
}

func careOfferingDays(offering *enrollmentModels.CareOffering, link *enrollmentModels.RequestChildOffering) []string {
	if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
		return offering.AvailableDays
	}
	return link.SelectedDays
}

func sortCareOfferingSelections(
	items []CareOfferingSelection,
	offerings []*enrollmentModels.CareOffering,
) {
	sortOrderByID := make(map[int64]int, len(offerings))
	for _, offering := range offerings {
		if offering != nil {
			sortOrderByID[offering.ID] = offering.SortOrder
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := sortOrderByID[items[i].OfferingID], sortOrderByID[items[j].OfferingID]
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left < right
	})
}

// weekdaysFromOfferingDays maps stored day abbreviations ("mon") to ISO
// weekday numbers, dropping anything unrecognised.
func weekdaysFromOfferingDays(days []string) []int {
	weekdays := make([]int, 0, len(days))
	seen := make(map[int]bool, len(days))
	for _, day := range days {
		weekday, ok := enrollmentModels.CanonicalDayToISOWeekday(day)
		if !ok || seen[weekday] {
			continue
		}
		seen[weekday] = true
		weekdays = append(weekdays, weekday)
	}
	sort.Ints(weekdays)
	return weekdays
}

// offeringChangesEnabledForTenant resolves the school's post-enrollment change
// gate. Fails CLOSED, unlike the messaging gate: a school that never switched
// this on must not receive requests because the config lookup blipped, and the
// read view degrades to "school has it off" rather than offering a button whose
// submit would be refused.
func (s *service) offeringChangesEnabledForTenant(ctx context.Context, tenantID int64) bool {
	if s.Settings == nil {
		return false
	}
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyEnrollmentOfferingChangesEnabled)
	if err != nil {
		s.Logger.Warn("parent: resolve offering-changes setting failed, failing closed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}
	if !enabled {
		return false
	}
	careOfferingsEnabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModel.KeyEnrollmentCareOfferingsEnabled)
	if err != nil {
		s.Logger.Warn("parent: resolve care-offerings setting failed, failing closed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return careOfferingsEnabled
}

func (s *service) resolveOfferingChangeAvailabilityForStudent(
	ctx context.Context,
	child *parentChild,
	studentID int64,
	period *enrollmentModels.StudentCarePeriod,
	today timezone.Date,
) (bool, string) {
	if period == nil {
		return false, OfferingChangesReasonNoEnrollment
	}
	if !child.hasPermission(authorize.GuardianPermissionRequestSubmit) {
		return false, OfferingChangesReasonNoPermission
	}
	if s.OfferingChanges != nil {
		earliest, err := s.OfferingChanges.EarliestEffectiveFrom(ctx)
		if err != nil || (studentID > 0 && !s.hasCarePeriodOnOrAfter(ctx, studentID, earliest)) ||
			(studentID == 0 && earliest.After(period.ServiceEndDate)) {
			return false, OfferingChangesReasonNoTime
		}
	} else if period.ServiceEndDate.Before(today) {
		return false, OfferingChangesReasonPeriodOver
	}
	if !s.offeringChangesEnabledForTenant(ctx, child.tenantID) {
		return false, OfferingChangesReasonSchoolOff
	}
	return true, ""
}

// hasCarePeriodOnOrAfter mirrors the offering-change catalog's period
// selection: a lead time may land in a pre-approved upcoming period.
func (s *service) hasCarePeriodOnOrAfter(ctx context.Context, studentID int64, date timezone.Date) bool {
	if s.RequestChildRepo == nil {
		return false
	}
	periods, err := s.RequestChildRepo.ListCarePeriodsByStudentID(ctx, studentID)
	if err != nil {
		return false
	}
	for _, candidate := range periods {
		if !candidate.ServiceEndDate.Before(date) {
			return true
		}
	}
	return false
}
