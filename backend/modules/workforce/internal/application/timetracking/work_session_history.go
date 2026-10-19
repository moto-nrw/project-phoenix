package timetracking

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *workSessionService) GetCurrentSession(ctx context.Context, staffID int64) (*WorkSession, error) {
	session, err := s.repo.GetCurrentByStaffID(ctx, staffID)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf(errGetCurrentSession, err)
	}

	return session, nil
}

func (s *workSessionService) GetLatestOpenSession(ctx context.Context, staffID int64) (*WorkSession, error) {
	session, err := s.repo.GetLatestOpenByStaffID(ctx, staffID)
	if err != nil {
		if isRecordNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest open session: %w", err)
	}

	return session, nil
}

func (s *workSessionService) GetHistory(ctx context.Context, staffID int64, from, to calendar.Date) (*HistoryResponse, error) {
	sessions, err := s.repo.GetHistoryByStaffID(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get session history: %w", err)
	}
	return s.historyResponse(ctx, staffID, sessions, from, to)
}

func (s *workSessionService) GetHistoryIntersecting(ctx context.Context, staffID int64, from, to calendar.Date) (*HistoryResponse, error) {
	end := to.AddDays(1).BerlinMidnight()
	sessions, err := s.repo.ListOverlappingByStaffID(ctx, staffID, from.BerlinMidnight(), &end)
	if err != nil {
		return nil, fmt.Errorf("failed to get intersecting session history: %w", err)
	}
	return s.historyResponse(ctx, staffID, sessions, from, to)
}

// historyResponse builds the wire response for the blocks of [from, to]. The
// range is carried through because the weekly summaries are aggregated per
// calendar day: a block that reaches beyond the requested range (a night block
// at either border) must not contribute the minutes it spends outside it.
func (s *workSessionService) historyResponse(ctx context.Context, staffID int64, sessions []*WorkSession, from, to calendar.Date) (*HistoryResponse, error) {

	// Collect session IDs for batch edit count query
	sessionIDs := make([]int64, len(sessions))
	for i, session := range sessions {
		sessionIDs[i] = session.ID
	}

	// Batch fetch manual edit counts. System-authored edits (auto-checkout) and
	// stamp-time deviation reasons (#1844) are excluded so they don't surface
	// as "Manuell korrigiert".
	editCounts, err := s.auditRepo.CountManualBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get edit counts: %w", err)
	}
	auditCounts, err := s.auditRepo.CountBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit counts: %w", err)
	}
	breaksBySession, err := s.breakRepo.GetBySessionIDs(ctx, sessionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get breaks: %w", err)
	}

	responses := sessionHistoryResponses(sessions, breaksBySession, editCounts, auditCounts, s.now())

	targetsByWeek := s.withTargetOverrides(ctx, staffID, responses, s.getWeeklyTargetsForSummaries(ctx, staffID, responses))

	// Build weekly summaries
	weeklySummaries := s.buildWeeklySummaries(responses, targetsByWeek, from, to)

	return &HistoryResponse{
		Sessions:        responses,
		WeeklySummaries: weeklySummaries,
	}, nil
}

// sessionHistoryResponses uses each block's effective end for both work and break figures.
func sessionHistoryResponses(sessions []*WorkSession, breaksBySession map[int64][]*WorkSessionBreak, editCounts, auditCounts map[int64]int, now time.Time) []*SessionResponse {
	responses := make([]*SessionResponse, len(sessions))
	for i, session := range sessions {
		breaks := breaksBySession[session.ID]

		// The as-of clock of a block is its own end, not the wall clock: an
		// open block that has passed its live limit stopped counting there
		// (BalanceSessionEnd), and work AND break time have to be measured
		// against that same instant. Pinning a fake check-out on a copy
		// instead would silence totalBreakMinutes — it treats a checked-out
		// block's cache as complete — and a break still running across the
		// limit would be booked as work (#2402).
		asOf := BalanceSessionEnd(session, now)
		responses[i] = &SessionResponse{
			WorkSession: session,
			// A running break is NOT in the BreakMinutes cache, so netMinutes
			// alone would keep counting it as worked time and the day rows
			// would climb while the Monatskarte and the week KPI (which both
			// deduct it) stand still (#1842). The breaks are already loaded
			// above — no extra query.
			BreakMinutes:     totalBreakMinutes(session, breaks, asOf),
			NetMinutes:       netMinutesWithBreaks(session, breaks, asOf),
			IsOvertime:       isOvertime(session, breaks, asOf),
			IsBreakCompliant: isBreakCompliant(session, breaks, asOf),
			Breaks:           breaks,
			EditCount:        editCounts[session.ID],
			AuditCount:       auditCounts[session.ID],
		}
	}

	return responses
}

// buildWeeklySummaries aggregates session data by ISO week, restricted to the
// requested [from, to] range.
//
// A block is laid out on the Berlin days its minutes actually fall on, and it
// may reach past either border of the range: the intersecting query returns a
// night block that began before `from`, and a block started on `to` runs into
// the next morning. Aggregating it whole would credit weeks the caller never
// asked about — including ISO weeks with no visible row to explain them — so
// only the days inside the range are counted (#2402).
func (s *workSessionService) buildWeeklySummaries(sessions []*SessionResponse, targetsByWeek map[summaryWeekKey]int, from, to calendar.Date) []WeeklySummary {
	weeks := weeklySummaryAccumulator{weekMap: make(map[summaryWeekKey]*WeeklySummary), targetsByWeek: targetsByWeek}
	now := time.Now()

	for _, session := range sessions {
		end := BalanceSessionEnd(session.WorkSession, now)
		countedWeeks := make(map[summaryWeekKey]struct{})
		firstDay, lastDay := summarySessionDays(session.CheckInTime, end, from, to)
		for day, minutes := range netMinutesByDate(session.WorkSession, session.Breaks, end, firstDay, lastDay) {
			year, week := day.UTCMidnight().ISOWeek()
			key := summaryWeekKey{Year: year, Week: week}

			summary := weeks.week(key)
			summary.TotalNetMinutes += minutes
			if _, counted := countedWeeks[key]; !counted {
				summary.SessionCount++
				countedWeeks[key] = struct{}{}
			}
		}
	}

	return weeklySummaryValues(weeks.weekMap, weeks.weekOrder)
}

// summarySessionDays returns the days a block contributes to, clamped to the
// summary range. The first day is derived from the check-in instant, never
// from the stored date: an admin Nachtrag files a block under a date it may
// choose freely (CreateSessionAsAdmin), so a block stamped 23:00 and filed on
// the following day would lose its pre-midnight minutes here while the
// balance — which starts at the check-in — still counts them.
func summarySessionDays(checkIn, end time.Time, from, to calendar.Date) (calendar.Date, calendar.Date) {
	firstDay, lastDay := calendar.DateFromTime(checkIn), calendar.DateFromTime(end)
	if firstDay.Before(from) {
		firstDay = from
	}
	if lastDay.After(to) {
		lastDay = to
	}
	return firstDay, lastDay
}

// weeklySummaryAccumulator collects the weekly summaries in the order their
// weeks are first encountered.
type weeklySummaryAccumulator struct {
	weekMap       map[summaryWeekKey]*WeeklySummary
	weekOrder     []summaryWeekKey
	targetsByWeek map[summaryWeekKey]int
}

// week returns the summary of the week, starting it with the week's target
// on first use.
func (w *weeklySummaryAccumulator) week(key summaryWeekKey) *WeeklySummary {
	if summary, exists := w.weekMap[key]; exists {
		return summary
	}
	var targetMinutes *int
	if target, ok := w.targetsByWeek[key]; ok {
		targetCopy := target
		targetMinutes = &targetCopy
	}
	summary := &WeeklySummary{
		WeekNumber:    key.Week,
		Year:          key.Year,
		TargetMinutes: targetMinutes,
	}
	w.weekMap[key] = summary
	w.weekOrder = append(w.weekOrder, key)
	return summary
}

// weeklySummaryValues keeps the encountered week order and derives the limit and target deltas.
func weeklySummaryValues(weekMap map[summaryWeekKey]*WeeklySummary, weekOrder []summaryWeekKey) []WeeklySummary {
	// Convert to sorted slice and compute derived fields
	const maxWeeklyMinutes = 48 * 60 // §3 ArbZG
	summaries := make([]WeeklySummary, 0, len(weekOrder))
	for _, key := range weekOrder {
		ws := weekMap[key]
		ws.IsOverWeeklyMax = ws.TotalNetMinutes > maxWeeklyMinutes

		if ws.TargetMinutes != nil {
			delta := ws.TotalNetMinutes - *ws.TargetMinutes
			ws.DeltaMinutes = &delta
		}

		summaries = append(summaries, *ws)
	}

	return summaries
}

func (s *workSessionService) getWeeklyTargetsForSummaries(ctx context.Context, staffID int64, sessions []*SessionResponse) map[summaryWeekKey]int {
	if len(sessions) == 0 {
		return nil
	}
	holidaySet, ok := s.holidayDatesForWeeks(ctx, sessionWeekStarts(sessions))
	if !ok {
		return nil
	}
	staff := s.resolveStaffForTargets(ctx, staffID)
	if s.scheduleRepo != nil {
		targets := s.weeklyTargetsFromDateValidSchedule(ctx, staffID, staff, sessions, holidaySet)
		if len(targets) > 0 {
			return targets
		}
	}
	if staff != nil && staff.WorkTimeModelID != nil {
		if s.workModelRepo == nil {
			return nil
		}
		model, err := s.workModelRepo.FindByID(ctx, *staff.WorkTimeModelID)
		if err != nil {
			return nil
		}
		anchor := model.RotationAnchorDate
		if staff.RotationAnchorDate != nil {
			anchor = *staff.RotationAnchorDate
		}
		targets := WeeklyTargetsFromTemplate(model, anchor, sessionWeekStarts(sessions))
		for weekStart, target := range targets {
			targets[weekStart] = target - holidayTemplateMinutes(model, anchor, weekStart, holidaySet)
		}
		return summaryKeysOf(targets)
	}
	return nil
}

// holidayDatesForWeeks loads the public holidays covering the given weeks.
// ok=false signals a resolver failure — then NO weekly Soll is shown rather
// than one that ignores holidays and fakes minus hours (#1418 3a).
func (s *workSessionService) holidayDatesForWeeks(ctx context.Context, weekStarts []calendar.Date) (map[calendar.Date]bool, bool) {
	if s.holidayReader == nil || len(weekStarts) == 0 {
		return nil, true
	}
	from, to := weekStarts[0], weekStarts[0]
	for _, weekStart := range weekStarts[1:] {
		if weekStart.Before(from) {
			from = weekStart
		}
		if weekStart.After(to) {
			to = weekStart
		}
	}
	set, err := s.holidayReader.HolidayDates(ctx, from, to.AddDays(6))
	if err != nil {
		s.getLogger().Warn("failed to load public holidays for weekly summaries",
			"error", err.Error(),
		)
		return nil, false
	}
	return set, true
}

func (s *workSessionService) resolveStaffForTargets(ctx context.Context, staffID int64) *StaffScheduleAssignment {
	if s.staffRepo == nil {
		return nil
	}
	staff, err := s.staffRepo.ScheduleAssignment(ctx, staffID)
	if err != nil {
		return nil
	}
	return staff
}

// weeklyTargetsFromDateValidSchedule resolves the weekly Soll from date-valid
// staff_work_schedules rows via the presence-owned helpers: one batched
// read over the span of all session weeks, then per-week in-memory resolution
// (validity windows, rotation weeks) through WeeklyTargetFromSchedule —
// the same helper the Dienstplan weekly summaries use.
func (s *workSessionService) weeklyTargetsFromDateValidSchedule(
	ctx context.Context,
	staffID int64,
	staff *StaffScheduleAssignment,
	sessions []*SessionResponse,
	holidaySet map[calendar.Date]bool,
) map[summaryWeekKey]int {
	weekStarts := sessionWeekStarts(sessions)
	if len(weekStarts) == 0 {
		return nil
	}
	from, to := weekStarts[0], weekStarts[0]
	for _, weekStart := range weekStarts[1:] {
		if weekStart.Before(from) {
			from = weekStart
		}
		if weekStart.After(to) {
			to = weekStart
		}
	}
	entries, err := s.scheduleRepo.FindByStaffIDsValidInRange(ctx, []int64{staffID}, from, to.AddDays(6))
	if err != nil || len(entries) == 0 {
		return nil
	}
	targetsByWeek := make(map[summaryWeekKey]int)
	for _, weekStart := range weekStarts {
		if target, ok := WeeklyTargetFromSchedule(entries, staffAnchorOf(staff), weekStart); ok {
			target -= holidayScheduleMinutes(entries, staffAnchorOf(staff), weekStart, holidaySet)
			targetsByWeek[summaryKeyOf(weekStart)] = target
		}
	}
	if len(targetsByWeek) == 0 {
		return nil
	}
	return targetsByWeek
}

// staffAnchorOf returns the staff-level rotation anchor when one is set.
func staffAnchorOf(staff *StaffScheduleAssignment) *calendar.Date {
	if staff == nil {
		return nil
	}
	return staff.RotationAnchorDate
}

// sessionWeekStarts returns the deduplicated Monday week starts of the given
// sessions in first-seen order.
func sessionWeekStarts(sessions []*SessionResponse) []calendar.Date {
	weekStarts := make([]calendar.Date, 0)
	seen := make(map[calendar.Date]struct{})
	now := time.Now()
	for _, session := range sessions {
		end := BalanceSessionEnd(session.WorkSession, now)
		for day := calendar.DateFromTime(session.CheckInTime); !day.After(calendar.DateFromTime(end)); day = day.AddDays(1) {
			weekStart := day.StartOfISOWeek()
			if _, ok := seen[weekStart]; ok {
				continue
			}
			seen[weekStart] = struct{}{}
			weekStarts = append(weekStarts, weekStart)
		}
	}
	return weekStarts
}

// summaryKeyOf maps a Monday week start onto the ISO year/week summary key
// (a Monday uniquely determines its ISO week, so the mapping is loss-free).
func summaryKeyOf(weekStart calendar.Date) summaryWeekKey {
	year, week := weekStart.UTCMidnight().ISOWeek()
	return summaryWeekKey{Year: year, Week: week}
}

// summaryKeysOf rekeys the Monday-date targets of the shared model resolver
// onto summary week keys.
func summaryKeysOf(targets map[calendar.Date]int) map[summaryWeekKey]int {
	if len(targets) == 0 {
		return nil
	}
	targetsByWeek := make(map[summaryWeekKey]int, len(targets))
	for weekStart, target := range targets {
		targetsByWeek[summaryKeyOf(weekStart)] = target
	}
	return targetsByWeek
}

func (s *workSessionService) GetSessionEdits(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error) {
	// Verify ownership: session must belong to requesting staff
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, workforce.ErrSessionNotOwned
	}

	return s.loadSessionEditsView(ctx, session, sessionID)
}

func (s *workSessionService) GetSessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error) {
	session, err := s.repo.FindByID(ctx, sessionID)
	if err != nil {
		return nil, s.handleSessionNotFoundError(err)
	}
	if session.StaffID != staffID {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAdminSessionNotOwned, Cause: fmt.Errorf("session does not belong to staff %d", staffID)}
	}

	return s.loadSessionEditsView(ctx, session, sessionID)
}

// loadSessionEditsView is the shared body that loads edits and decorates
// them with editor display names. Names are resolved through a single batch
// staff+person query so we never N+1 the audit log.
func (s *workSessionService) loadSessionEditsView(ctx context.Context, session *WorkSession, sessionID int64) ([]*WorkSessionEditView, error) {
	edits, err := s.auditRepo.GetBySessionID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session edits: %w", err)
	}
	if len(edits) == 0 {
		return []*WorkSessionEditView{}, nil
	}

	// Collect editor staff IDs and resolve them to names in one shot.
	editorIDSet := make(map[int64]struct{}, len(edits))
	for _, e := range edits {
		editorIDSet[e.EditedBy] = struct{}{}
	}
	editorIDs := slices.Collect(maps.Keys(editorIDSet))

	staffMap := map[int64]WorkSessionStaffName{}
	if s.staffRepo != nil {
		var err error
		staffMap, err = s.staffRepo.StaffNames(ctx, editorIDs)
		if err != nil {
			if s.logger != nil {
				s.logger.Warn("failed to resolve editor names for audit view",
					slog.String("error", err.Error()),
				)
			}
			staffMap = map[int64]WorkSessionStaffName{}
		}
	}
	if staffMap == nil {
		staffMap = map[int64]WorkSessionStaffName{}
	}

	views := make([]*WorkSessionEditView, len(edits))
	for i, e := range edits {
		name := ""
		if e.EditedBy == 0 {
			name = "System"
		} else if staff, ok := staffMap[e.EditedBy]; ok {
			name = strings.TrimSpace(staff.FirstName + " " + staff.LastName)
		}
		views[i] = &WorkSessionEditView{
			WorkSessionEdit: e,
			EditorName:      name,
			IsSelfEdit:      e.EditedBy == session.StaffID,
		}
	}
	return views, nil
}
