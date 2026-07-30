// Parent-portal read view for the child's booked care offerings and activity
// groups (#1665). Before this view a guardian could see the weekly times and
// the meal plan, but never what the child is actually booked into — that lived
// only in the enrollment form they filled in months earlier.
//
// Two lists, because neither alone is complete:
//
//   - Offerings are what the family chose and what a change request talks
//     about. Some of them (schedule-only offerings without a linked group)
//     never produce an enrollment row at all.
//   - Groups are where the child actually is, including groups staff enrolled
//     it into by hand, which no offering covers.
//
// Children that never came through an enrollment (manually created, imported)
// have no offerings; they still get the group list, and CanRequest stays false
// because there is no request child for an approved change to be applied to.
package parent

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
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
}

// CareGroupMembership is one activity-group enrollment of the child: where the
// child is, from when, and on which weekdays.
type CareGroupMembership struct {
	GroupID   int64
	Name      string
	Weekdays  []int
	ValidFrom timezone.Date
	// ValidUntil is exclusive, mirroring the column. Nil means open-ended.
	ValidUntil *timezone.Date
	// FromOffering marks rows materialized from an enrollment selection, as
	// opposed to a group staff enrolled the child into directly.
	FromOffering bool
	// StartsLater is true when the row has not begun yet — an approved change
	// that takes effect on a future date shows up here before it applies.
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
	Groups      []CareGroupMembership
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
		Groups:    []CareGroupMembership{},
	}
	today := timezone.TodayDate()
	var period *enrollmentModels.StudentCarePeriod
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, resolveErr := s.currentCarePeriod(txCtx, studentID, today)
		if resolveErr != nil {
			return resolveErr
		}
		period = resolved
		if period != nil {
			view.PeriodName = period.PhaseName
			view.PeriodStart = period.ServiceStartDate
			view.PeriodEnd = period.ServiceEndDate
			offerings, offeringsErr := s.carePeriodOfferings(txCtx, period.RequestChildID)
			if offeringsErr != nil {
				return offeringsErr
			}
			view.Offerings = offerings
		}
		groups, groupsErr := s.careGroupMemberships(txCtx, studentID, today)
		if groupsErr != nil {
			return groupsErr
		}
		view.Groups = groups
		if s.OfferingChanges != nil {
			pending, pendingErr := s.OfferingChanges.GetForStudent(txCtx, studentID)
			if pendingErr != nil {
				return pendingErr
			}
			if pending != nil {
				view.LastDecision = pending.LastDecision
			}
			if pending != nil && pending.Request != nil {
				view.PendingRequest = &PendingOfferingChange{
					ID:              pending.Request.ID,
					CreatedAt:       pending.Request.CreatedAt,
					EffectiveFrom:   pending.Request.EffectiveFrom,
					Diff:            pending.Diff,
					SubmittedBySelf: pending.Request.SubmittedBy == accountID,
				}
				if pending.Request.ParentNote != nil {
					view.PendingRequest.Note = *pending.Request.ParentNote
				}
			}
			if earliest, earliestErr := s.OfferingChanges.EarliestEffectiveFrom(txCtx); earliestErr == nil {
				view.EarliestEffectiveFrom = earliest
			}
		}
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: get child care offerings: %w", txErr)
	}
	view.CanRequest, view.ChangesDisabledReason = s.resolveOfferingChangeAvailability(ctx, child, period, today)
	return view, nil
}

// GetChildOfferingCatalog returns the offerings the guardian may choose from,
// prefilled with the current booking. Requires the same permission as
// submitting: the catalog exposes the school's capacity situation.
func (s *service) GetChildOfferingCatalog(
	ctx context.Context,
	accountID, studentID int64,
) (*enrollmentSvc.OfferingChangeCatalog, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrOfferingChangeDisabled
	}
	var catalog *enrollmentSvc.OfferingChangeCatalog
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, resolveErr := s.OfferingChanges.Catalog(txCtx, studentID)
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
) (*ChildCareOfferings, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrOfferingChangeDisabled
	}
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, createErr := s.OfferingChanges.Create(txCtx, enrollmentSvc.CreateOfferingChangeInput{
			StudentID:     studentID,
			AccountID:     accountID,
			Selections:    selections,
			EffectiveFrom: effectiveFrom,
			Note:          note,
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
// withdrawn. Stays available even after the school switches the feature off, so
// an outstanding request can always be wound down.
func (s *service) WithdrawOfferingChangeRequest(
	ctx context.Context,
	accountID, studentID, requestID int64,
) (*ChildCareOfferings, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionRequestSubmit)
	if err != nil {
		return nil, err
	}
	if s.OfferingChanges == nil {
		return nil, enrollmentSvc.ErrOfferingChangeDisabled
	}
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
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
) ([]CareOfferingSelection, error) {
	if s.RequestChildOfferingRepo == nil || s.CareOfferingRepo == nil {
		return []CareOfferingSelection{}, nil
	}
	links, err := s.RequestChildOfferingRepo.ListByRequestChildID(ctx, requestChildID)
	if err != nil {
		return nil, fmt.Errorf("list child offerings: %w", err)
	}
	if len(links) == 0 {
		return []CareOfferingSelection{}, nil
	}
	offeringIDs := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		offeringIDs = append(offeringIDs, link.CareOfferingID)
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("list care offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}
	items := make([]CareOfferingSelection, 0, len(links))
	for _, link := range links {
		offering := offeringByID[link.CareOfferingID]
		if offering == nil {
			// The offering was deleted from the catalog. Skipped rather than
			// shown as an unnamed row: a guardian cannot act on it either way.
			continue
		}
		item := CareOfferingSelection{
			OfferingID:      offering.ID,
			Name:            offering.Name,
			Weekdays:        weekdaysFromOfferingDays(link.SelectedDays),
			PriceCents:      offering.PriceCents,
			IncludesLunch:   offering.IncludesLunch,
			IncludesHoliday: offering.IncludesHolidayCare,
		}
		if offering.Description != nil {
			item.Description = *offering.Description
		}
		items = append(items, item)
	}
	sortOrderByID := make(map[int64]int, len(offerings))
	for _, offering := range offerings {
		sortOrderByID[offering.ID] = offering.SortOrder
	}
	sort.SliceStable(items, func(i, j int) bool {
		left, right := sortOrderByID[items[i].OfferingID], sortOrderByID[items[j].OfferingID]
		if left == right {
			return items[i].Name < items[j].Name
		}
		return left < right
	})
	return items, nil
}

// careGroupMemberships lists the child's group enrollments that are current or
// still ahead. Rows that already ended are left out: the view answers "where is
// my child", not "where has it ever been".
func (s *service) careGroupMemberships(
	ctx context.Context,
	studentID int64,
	today timezone.Date,
) ([]CareGroupMembership, error) {
	if s.StudentEnrollmentRepo == nil {
		return []CareGroupMembership{}, nil
	}
	rows, err := s.StudentEnrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("list student enrollments: %w", err)
	}
	relevant := make([]*activitiesModels.StudentEnrollment, 0, len(rows))
	groupIDs := make([]int64, 0, len(rows))
	seenGroup := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		// valid_until is exclusive: a row ending today is already over.
		if row.ValidUntil != nil && !row.ValidUntil.After(today) {
			continue
		}
		relevant = append(relevant, row)
		if !seenGroup[row.ActivityGroupID] {
			seenGroup[row.ActivityGroupID] = true
			groupIDs = append(groupIDs, row.ActivityGroupID)
		}
	}
	if len(relevant) == 0 {
		return []CareGroupMembership{}, nil
	}
	nameByID := map[int64]string{}
	if s.ActivityGroupRepo != nil {
		groups, groupsErr := s.ActivityGroupRepo.FindByIDs(ctx, groupIDs)
		if groupsErr != nil {
			return nil, fmt.Errorf("list activity groups: %w", groupsErr)
		}
		for _, group := range groups {
			if group != nil {
				nameByID[group.ID] = group.Name
			}
		}
	}
	items := make([]CareGroupMembership, 0, len(relevant))
	for _, row := range relevant {
		name, ok := nameByID[row.ActivityGroupID]
		if !ok {
			// A group the guardian cannot be told the name of is not worth a
			// nameless card.
			continue
		}
		weekdays := append([]int(nil), row.SelectedWeekdays...)
		sort.Ints(weekdays)
		items = append(items, CareGroupMembership{
			GroupID:      row.ActivityGroupID,
			Name:         name,
			Weekdays:     weekdays,
			ValidFrom:    row.ValidFrom,
			ValidUntil:   row.ValidUntil,
			FromOffering: row.EnrollmentRequestChildID != nil,
			StartsLater:  row.ValidFrom.After(today),
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].ValidFrom == items[j].ValidFrom {
			return items[i].Name < items[j].Name
		}
		return items[i].ValidFrom.Before(items[j].ValidFrom)
	})
	return items, nil
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
	return enabled
}

// offeringChangeLeadDays is the configured minimum notice for a dated change.
// Falls back to the registry default on error for the same reason the gate
// fails closed: a wrong-but-safe lead time beats a 500 on a read view.
func (s *service) offeringChangeLeadDays(ctx context.Context, tenantID int64) int {
	const fallbackLeadDays = 14
	if s.Settings == nil {
		return fallbackLeadDays
	}
	days, err := s.Settings.ResolveIntForTenant(ctx, tenantID, configModel.KeyEnrollmentOfferingChangesLeadDays)
	if err != nil {
		s.Logger.Warn("parent: resolve offering-changes lead days failed, using default",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return fallbackLeadDays
	}
	if days < 0 {
		return 0
	}
	return days
}

// resolveOfferingChangeAvailability decides whether the guardian may open a
// change request, and if not, why. Order matters: the most specific,
// least-fixable reason wins, so a guardian without permission is not told the
// school switched the feature off.
func (s *service) resolveOfferingChangeAvailability(
	ctx context.Context,
	child *parentChild,
	period *enrollmentModels.StudentCarePeriod,
	today timezone.Date,
) (bool, string) {
	if period == nil {
		return false, OfferingChangesReasonNoEnrollment
	}
	if !child.hasPermission(authorize.GuardianPermissionRequestSubmit) {
		return false, OfferingChangesReasonNoPermission
	}
	if period.ServiceEndDate.Before(today) {
		return false, OfferingChangesReasonPeriodOver
	}
	if !s.offeringChangesEnabledForTenant(ctx, child.tenantID) {
		return false, OfferingChangesReasonSchoolOff
	}
	return true, ""
}
