package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WeekProtocolEveningHour is the Berlin hour from which the worker records
// the day (#4769): the care day is over, late check-outs included.
const WeekProtocolEveningHour = 20

// WeekProtocolSchoolName names the protocol of a school without fixed
// groups.
const WeekProtocolSchoolName = "Ganze Schule"

// Why an evening run recorded nothing.
const (
	weekProtocolSkipBeforeEvening = "before_evening"
	weekProtocolSkipNoGroups      = "no_groups"
	weekProtocolSkipQuietWeekend  = "quiet_weekend"
)

// RunWeekProtocolEvening records the day from the evening hour on, turns the week into a
// draft from Friday evening, and closes the protocols whose following week
// is over at any hour. Everything runs in one transaction of the school in
// context.
func (s *Service) RunWeekProtocolEvening(ctx context.Context, now time.Time, sources timetable.WeekProtocolSources) (result timetable.WeekProtocolEveningResult, err error) {
	if sources.Notices == nil || sources.Calendar == nil || sources.Groups == nil || sources.Attendance == nil || sources.SchoolMode == nil || sources.Renderer == nil {
		return result, errors.New("week protocol evening: all sources are required")
	}
	e := weekProtocolEvening{service: s, sources: sources}
	local := now.In(calendar.Berlin)
	day := calendar.DateFromTime(now)
	weekStart := day.StartOfISOWeek()
	evening := local.Hour() >= WeekProtocolEveningHour
	err = s.runWrite(ctx, "run_week_protocol_evening", true, func(txCtx context.Context, _ *domain.OperationStats) error {
		result = timetable.WeekProtocolEveningResult{}
		if !evening {
			result.SkippedReason = weekProtocolSkipBeforeEvening
		} else {
			captured, reason, captureErr := e.captureDay(txCtx, day, now)
			if captureErr != nil {
				return captureErr
			}
			result.DaysCaptured, result.SkippedReason = captured, reason
		}
		// From Friday evening the week is a draft; the weekend keeps drafting
		// a week whose Friday evening was missed.
		var draftUpTo *calendar.Date
		if (evening && day.Weekday() == time.Friday) || calendar.IsWeekend(day) {
			draftUpTo = &weekStart
		}
		drafted, closed, advanceErr := s.store.AdvanceWeekProtocols(txCtx, weekStart.AddDays(-7), draftUpTo, now)
		if advanceErr != nil {
			return advanceErr
		}
		result.Drafted, result.AutoClosed = drafted, len(closed)
		return s.storeAutoClosedWeekProtocolPDFs(txCtx, closed, sources.Renderer)
	})
	return result, err
}

// storeAutoClosedWeekProtocolPDFs gives every protocol the run closed its
// PDF, like one closed by hand, in the run's transaction.
func (s *Service) storeAutoClosedWeekProtocolPDFs(ctx context.Context, ids []int64, renderer timetable.WeekProtocolRenderer) error {
	for _, id := range ids {
		protocol, found, err := s.store.FindWeekProtocol(ctx, id, false)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("week protocol %d closed but not found", id)
		}
		if err := s.storeWeekProtocolPDF(ctx, protocol, renderer); err != nil {
			return err
		}
	}
	return nil
}

// weekProtocolEvening is one evening run over the owner and its sources.
type weekProtocolEvening struct {
	service *Service
	sources timetable.WeekProtocolSources
}

// captureDay records the day of every protocol of the school once.
func (e weekProtocolEvening) captureDay(ctx context.Context, day calendar.Date, now time.Time) (int, string, error) {
	targets, err := e.targets(ctx)
	if err != nil {
		return 0, "", err
	}
	if len(targets) == 0 {
		return 0, weekProtocolSkipNoGroups, nil
	}
	facts, err := e.dayFacts(ctx, day, targets)
	if err != nil {
		return 0, "", err
	}
	if calendar.IsWeekend(day) && !anyWeekProtocolActivity(facts) {
		return 0, weekProtocolSkipQuietWeekend, nil
	}
	captured := 0
	for _, target := range targets {
		protocolID, status, ensureErr := e.service.store.EnsureWeekProtocol(ctx, target.target, day.StartOfISOWeek())
		if ensureErr != nil {
			return 0, "", ensureErr
		}
		if status == timetable.WeekProtocolStatusClosed {
			continue
		}
		recorded, captureErr := e.service.store.CaptureWeekProtocolDay(ctx, protocolID, day, facts[target.key], now)
		if captureErr != nil {
			return 0, "", captureErr
		}
		if recorded {
			captured++
		}
	}
	return captured, "", nil
}

// weekProtocolTarget is one protocol of the evening; key is its group, or 0
// for the whole school.
type weekProtocolTarget struct {
	key      int64
	groupIDs []int64
	target   timetable.WeekProtocolTarget
}

// targets are the school's OGS groups, or the whole school over all its
// groups when it works without fixed groups.
func (e weekProtocolEvening) targets(ctx context.Context) ([]weekProtocolTarget, error) {
	groups, err := e.sources.Groups(ctx)
	if err != nil {
		return nil, fmt.Errorf("list week protocol groups: %w", err)
	}
	openCare, err := e.sources.SchoolMode(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve group mode: %w", err)
	}
	sort.Slice(groups, func(i, j int) bool { return groups[i].ID < groups[j].ID })
	if openCare {
		ids := make([]int64, 0, len(groups))
		for _, group := range groups {
			ids = append(ids, group.ID)
		}
		return []weekProtocolTarget{{groupIDs: ids, target: timetable.WeekProtocolTarget{GroupName: WeekProtocolSchoolName}}}, nil
	}
	targets := make([]weekProtocolTarget, 0, len(groups))
	for _, group := range groups {
		id := group.ID
		targets = append(targets, weekProtocolTarget{
			key: id, groupIDs: []int64{id},
			target: timetable.WeekProtocolTarget{GroupID: &id, GroupName: group.Name},
		})
	}
	return targets, nil
}

func anyWeekProtocolActivity(facts map[int64]timetable.WeekProtocolDayFacts) bool {
	for _, day := range facts {
		if day.Present > 0 || len(day.Blocks) > 0 {
			return true
		}
	}
	return false
}
