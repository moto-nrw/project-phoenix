// Package schedule — split-series segment resolution (#2187).
//
// A template split chains segments via activities.groups.series_root_id: every
// capped predecessor keeps its schedules with an exclusive valid_until while
// exactly one living segment stays open. The Betreuungsplan grid renders
// occurrences with the template id that materialized them, so a click on a
// first-school-week occurrence can carry a fully capped predecessor's id. The
// helpers here resolve such an id to the segment that is still editable
// without introducing a second recurrence mechanism: the lineage comes from
// GroupRepository.FindTemplateSeries, the per-segment envelope from the same
// commonScheduleValidityEnvelope invariant every template write enforces.
package schedule

import (
	"cmp"
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// ErrTemplateSeriesFullyEnded is returned when a template id resolves to a
// split lineage that has no open segment left (the whole series was ended or
// archived). → 404.
var ErrTemplateSeriesFullyEnded = errors.New("template series has no editable segment")

// templateSeriesSegment is one segment of a template's split lineage plus its
// schedule validity envelope. ValidUntil == nil marks the living segment —
// the only one PUT /timetable/templates/{id} may edit.
type templateSeriesSegment struct {
	Group      *activitiesModel.Group
	Weekdays   []int // distinct schedule weekdays, ascending
	ValidFrom  *timezone.Date
	ValidUntil *timezone.Date
}

// loadTemplateSeriesSegments resolves templateID to its stable split-series
// root and returns every live segment of that lineage with its schedule
// envelope, ordered by group id ascending. A sibling segment whose schedules
// are missing or inconsistent is skipped with a warning — one corrupt segment
// must not block resolving or reconciling the rest of the series — but the
// requested template's own corruption stays a hard error so writes against it
// keep failing loudly.
func loadTemplateSeriesSegments(
	ctx context.Context,
	groupRepo activitiesModel.GroupRepository,
	scheduleRepo activitiesModel.ScheduleRepository,
	logger *slog.Logger,
	templateID int64,
) ([]templateSeriesSegment, error) {
	groups, err := groupRepo.FindTemplateSeries(ctx, templateID)
	if err != nil {
		return nil, &ScheduleError{Op: "load template series: find lineage", Err: err}
	}
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		if group != nil {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	if len(groupIDs) == 0 {
		return []templateSeriesSegment{}, nil
	}
	options := modelBase.NewQueryOptions()
	options.Filter = modelBase.NewFilter().In("activity_group_id", int64FilterArgs(groupIDs)...)
	scheduleRows, err := scheduleRepo.List(ctx, options)
	if err != nil {
		return nil, &ScheduleError{Op: "load template series: load schedules", Err: err}
	}
	schedulesByGroup := make(map[int64][]*activitiesModel.Schedule, len(groups))
	for _, row := range scheduleRows {
		schedulesByGroup[row.ActivityGroupID] = append(schedulesByGroup[row.ActivityGroupID], row)
	}
	segments := make([]templateSeriesSegment, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		schedules := schedulesByGroup[group.ID]
		if len(schedules) == 0 {
			// A template without schedule rows has no recurrence to resolve
			// against; it cannot be the living segment either.
			continue
		}
		validFrom, validUntil, err := commonScheduleValidityEnvelope(schedules)
		if err != nil {
			if group.ID == templateID {
				return nil, &ScheduleError{Op: "load template series: inspect schedule envelope", Err: err}
			}
			cmp.Or(logger, slog.Default()).Warn("skipping template series segment with inconsistent schedule envelope",
				slog.Int64("template_id", templateID),
				slog.Int64("segment_id", group.ID),
				slog.String("error", err.Error()),
			)
			continue
		}
		weekdays := make([]int, 0, len(schedules))
		for _, schedule := range schedules {
			if schedule != nil {
				weekdays = append(weekdays, schedule.Weekday)
			}
		}
		segments = append(segments, templateSeriesSegment{
			Group:      group,
			Weekdays:   uniqueSortedWeekdays(weekdays),
			ValidFrom:  validFrom,
			ValidUntil: validUntil,
		})
	}
	return segments, nil
}

// livingTemplateSegment returns the segment whose schedules are still open
// (exclusive valid_until unset). Several open segments would be a data
// anomaly; the highest id — the newest successor — wins.
func livingTemplateSegment(segments []templateSeriesSegment) *templateSeriesSegment {
	var living *templateSeriesSegment
	for i := range segments {
		if segments[i].ValidUntil == nil {
			living = &segments[i]
		}
	}
	return living
}

// ResolveLivingTemplateSegment resolves a template id the Betreuungsplan grid
// may have taken from an occurrence of an already-capped predecessor segment
// to the still-editable segment of the same split series (#2187). Returns the
// resolved id and whether it differs from the requested one. An unknown id or
// a lineage with no open segment left returns ErrTemplateSeriesFullyEnded.
func (s *TimetableDataService) ResolveLivingTemplateSegment(
	ctx context.Context,
	templateID int64,
) (int64, bool, error) {
	segments, err := loadTemplateSeriesSegments(
		ctx,
		s.deps.ActivityGroupRepo,
		s.deps.ActivityScheduleRepo,
		s.getLogger(),
		templateID,
	)
	if err != nil {
		return 0, false, err
	}
	living := livingTemplateSegment(segments)
	if living == nil {
		return 0, false, ErrTemplateSeriesFullyEnded
	}
	return living.Group.ID, living.Group.ID != templateID, nil
}
