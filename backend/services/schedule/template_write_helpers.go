package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// validateAssignableCategory rejects missing, cross-tenant, and archived
// categories before a timetable write creates a new reference. The repository
// read is tenant-scoped, so all three cases share the same client-facing error.
func validateAssignableCategory(
	ctx context.Context,
	repo activitiesModel.CategoryRepository,
	categoryID int64,
	op string,
) error {
	category, err := repo.FindByIDForShare(ctx, categoryID)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return &ScheduleError{Op: op, Err: ErrCategoryNotAssignable}
		}
		return &ScheduleError{Op: op, Err: err}
	}
	if category == nil || category.IsArchived() {
		return &ScheduleError{Op: op, Err: ErrCategoryNotAssignable}
	}
	return nil
}

// TemplateEducationGroupError marks an education_group_id precheck failure so
// handlers can surface it as a 400 while preserving the precise message. It is
// shared by the create, update, and split template flows.
type TemplateEducationGroupError struct{ Err error }

func (e *TemplateEducationGroupError) Error() string { return e.Err.Error() }

func (e *TemplateEducationGroupError) Unwrap() error { return e.Err }

// ValidateTemplateEducationGroup confirms an optional education_group_id refers
// to a positive id that belongs to the caller's tenant. A nil pointer means no
// group is linked and validates trivially. All failures are wrapped in
// TemplateEducationGroupError so callers render them uniformly as 400.
func (s *TimetableDataService) ValidateTemplateEducationGroup(ctx context.Context, groupID *int64) error {
	if groupID == nil {
		return nil
	}
	if *groupID <= 0 {
		return &TemplateEducationGroupError{Err: errors.New("education_group_id must be positive when set")}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return &TemplateEducationGroupError{Err: errors.New("no tenant in context")}
	}
	exists, err := s.deps.EducationGroupRepo.Exists(ctx, *groupID)
	if err != nil {
		return &TemplateEducationGroupError{Err: fmt.Errorf("validate education_group_id: %w", err)}
	}
	if !exists {
		return &TemplateEducationGroupError{Err: errors.New("education_group_id does not reference a group in this tenant")}
	}
	return nil
}

// FindOrCreateTimeframe returns the id of an existing schedule.timeframes row
// matching [start, end] or inserts a fresh one. Description is set to the
// template name on first creation as a debug hint, but is informational only —
// lookups go by time window. Shared by the template create and update paths so
// the find-or-create rule lives in exactly one place.
func (s *TimetableDataService) FindOrCreateTimeframe(ctx context.Context, start, end time.Time, descHint string) (int64, error) {
	existing, err := s.deps.TimeframeRepo.FindByTimeRange(ctx, start, end)
	if err == nil {
		for _, tf := range existing {
			if tf == nil {
				continue
			}
			// Match exact clock times; FindByTimeRange may return overlapping
			// windows depending on impl, so be precise. Do not use
			// time.Time.Equal here: schedule.timeframes stores SQL TIME, and
			// drivers may decode TIME with a different date anchor than the
			// handler's parseClockTime uses.
			if timezone.SameClockTime(tf.StartTime, start) && tf.EndTime != nil && timezone.SameClockTime(*tf.EndTime, end) {
				return tf.ID, nil
			}
		}
	}

	endCopy := end
	tf := &scheduleModel.Timeframe{
		StartTime:   start,
		EndTime:     &endCopy,
		IsActive:    true,
		Description: fmt.Sprintf("auto: %s", descHint),
	}
	tf.SetTenantID(tenant.FromContext(ctx))
	if err := s.deps.TimeframeRepo.Create(ctx, tf); err != nil {
		return 0, fmt.Errorf("create timeframe: %w", err)
	}
	return tf.ID, nil
}
