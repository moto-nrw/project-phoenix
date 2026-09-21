package timetracking

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
)

// ErrAbsenceRebookingBlocked marks a rebooking the Leitung has to resolve
// first (#3258). The error text is German and shown as it is.
var ErrAbsenceRebookingBlocked = errors.New("absence rebooking blocked")

// maxRebookedAbsences bounds one rebooking. The call from the Wissingen case
// needs about ten entries; a school year of weekly entries stays below it.
const maxRebookedAbsences = 100

type absenceRebookingBlockedError struct{ reason string }

func (e absenceRebookingBlockedError) Error() string { return e.reason }
func (e absenceRebookingBlockedError) Is(target error) bool {
	return target == ErrAbsenceRebookingBlocked
}

func rebookingBlocked(format string, args ...any) error {
	return absenceRebookingBlockedError{reason: fmt.Sprintf(format, args...)}
}

// RebookAbsencesRequest moves stored absences of one staff member to another
// type without deleting them (#3258). The target is a standard type or a
// school-defined one, exactly like on create.
type RebookAbsencesRequest struct {
	StaffID        int64
	ActorAccountID int64
	AbsenceIDs     []int64
	AbsenceType    string
	AbsenceTypeID  *int64
	Reason         string
	// DryRun computes the effects without writing. An exhausted allowance or
	// vacation account is then reported in the result instead of as error.
	DryRun bool
}

// AbsenceRebookingResult is what a rebooking does, before (DryRun) or after
// it was written.
type AbsenceRebookingResult struct {
	Absences []*StaffAbsenceResponse `json:"absences"`
	// Days counts the weekdays of the entries, half days as 0.5.
	Days float64 `json:"days"`
	// BalanceDeltaMinutes is how much the Stundenkonto rises (positive) or
	// falls (negative): only Freizeitausgleich leaves a day uncredited.
	BalanceDeltaMinutes int `json:"balance_delta_minutes"`
	// Allowances is the target type's account per touched year, with the
	// days this rebooking takes in BookingDays.
	Allowances        []*AbsenceTypeAllowanceSummary `json:"allowances,omitempty"`
	AllowanceExceeded bool                           `json:"allowance_exceeded"`
	// Vacation is the Resturlaub per touched year when vacation is the old
	// or the new type.
	Vacation         []VacationRebookingYear `json:"vacation,omitempty"`
	VacationExceeded bool                    `json:"vacation_exceeded"`
	Applied          bool                    `json:"applied"`
}

// VacationRebookingYear is the Resturlaub of one year before and after.
type VacationRebookingYear struct {
	Year            int     `json:"year"`
	RemainingBefore float64 `json:"remaining_before"`
	RemainingAfter  float64 `json:"remaining_after"`
}

type rebookedAbsence struct {
	before StaffAbsence
	after  *StaffAbsence
}

// RebookAbsences changes the type of stored absences (#3258). The rows keep
// their dates, their entry date and their status; the Stundenkonto, the
// allowances and every view follow from the new type because all of them are
// computed on read. A closed month blocks the change, since its frozen
// closing balance would no longer match.
func (s *staffAbsenceService) RebookAbsences(ctx context.Context, req RebookAbsencesRequest) (*AbsenceRebookingResult, error) {
	reason := strings.TrimSpace(req.Reason)
	ids, err := validateRebookingRequest(req, reason)
	if err != nil {
		return nil, err
	}
	selectedType, typeID, baseType, err := s.resolveAbsenceTypeSelection(ctx, req.AbsenceTypeID, req.AbsenceType)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(timerecords.ValidAbsenceTypes, baseType) {
		return nil, fmt.Errorf("invalid absence type")
	}
	if baseType == AbsenceTypeSick {
		return nil, rebookingBlocked("In eine Krankmeldung lässt sich nicht umbuchen. Löschen Sie den Eintrag und tragen Sie die Krankmeldung neu ein.")
	}
	if err := s.lockStaffAbsenceWrites(ctx, req.StaffID); err != nil {
		return nil, err
	}

	entries, err := s.loadRebookedAbsences(ctx, req.StaffID, ids, baseType, typeID)
	if err != nil {
		return nil, err
	}
	absences, err := s.loadRebookingAbsences(ctx, req.StaffID, entries)
	if err != nil {
		return nil, err
	}
	result := &AbsenceRebookingResult{}
	for _, entry := range entries {
		if err := s.validateRebookedAbsence(ctx, entry); err != nil {
			return nil, err
		}
		result.Days += countWorkingDays(entry.after.DateStart, entry.after.DateEnd, effectiveStartHalf(entry.after), effectiveEndHalf(entry.after))
	}
	if err := validateRebookedAbsenceOverlaps(absences, entries); err != nil {
		return nil, err
	}
	delta, err := s.rebookingBalanceDelta(ctx, absences, entries)
	if err != nil {
		return nil, err
	}
	result.BalanceDeltaMinutes = delta

	if selectedType != nil && selectedType.AllowanceEnabled {
		allowances, err := s.absenceTypes.PreviewAllowanceRebooking(ctx, req.StaffID, *typeID, ids)
		switch {
		case errors.Is(err, ErrAbsenceTypeAllowanceExceeded):
			result.AllowanceExceeded = true
		case err != nil:
			return nil, err
		}
		result.Allowances = allowances
	}
	if result.Vacation, result.VacationExceeded, err = s.previewVacationRebooking(ctx, req.StaffID, entries); err != nil {
		return nil, err
	}

	if req.DryRun {
		result.Absences = s.withLabels(ctx, rebookedResponses(entries)...)
		return result, nil
	}
	if result.AllowanceExceeded {
		return nil, fmt.Errorf("%w: rebooking needs more days than the allowance has left", ErrAbsenceTypeAllowanceExceeded)
	}
	if result.VacationExceeded {
		return nil, fmt.Errorf("%w: rebooking needs more days than the vacation account has left", ErrVacationQuotaExceeded)
	}
	if err := s.writeRebookedAbsences(ctx, req.ActorAccountID, reason, entries); err != nil {
		return nil, err
	}
	result.Applied = true
	result.Absences = s.withLabels(ctx, rebookedResponses(entries)...)
	s.broadcastTimeTrackingChanged(ctx)
	return result, nil
}

func validateRebookingRequest(req RebookAbsencesRequest, reason string) ([]int64, error) {
	if req.StaffID <= 0 || req.ActorAccountID <= 0 {
		return nil, fmt.Errorf("invalid rebooking: staff and actor are required")
	}
	ids := make([]int64, 0, len(req.AbsenceIDs))
	for _, id := range req.AbsenceIDs {
		if id <= 0 {
			return nil, fmt.Errorf("invalid rebooking: absence id must be positive")
		}
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil, rebookingBlocked("Wählen Sie mindestens einen Eintrag aus.")
	}
	if len(ids) > maxRebookedAbsences {
		return nil, rebookingBlocked("Buchen Sie höchstens %d Einträge auf einmal um.", maxRebookedAbsences)
	}
	if !req.DryRun && reason == "" {
		return nil, rebookingBlocked("Geben Sie einen Grund für die Umbuchung an.")
	}
	return ids, nil
}

// loadRebookedAbsences reads the entries and prepares their new shape. Only
// entries the Leitung or the staff member entered directly qualify: a
// vacation request has its own workflow, and a sick report its plan cascade.
func (s *staffAbsenceService) loadRebookedAbsences(ctx context.Context, staffID int64, ids []int64, baseType string, typeID *int64) ([]rebookedAbsence, error) {
	entries := make([]rebookedAbsence, 0, len(ids))
	for _, id := range ids {
		absence, err := s.absenceRepo.FindByID(ctx, id)
		if err != nil {
			if modelBase.IsNoRows(err) {
				return nil, fmt.Errorf("absence not found")
			}
			return nil, err
		}
		if absence == nil || absence.StaffID != staffID {
			return nil, fmt.Errorf("absence not found")
		}
		day := absence.DateStart.Format("02.01.2006")
		switch {
		case absence.AbsenceType == AbsenceTypeSick:
			return nil, rebookingBlocked("Die Krankmeldung vom %s lässt sich nicht umbuchen. Löschen Sie sie und tragen Sie die richtige Art neu ein.", day)
		case absence.Status != AbsenceStatusReported:
			return nil, rebookingBlocked("Der Eintrag vom %s ist ein Antrag. Anträge lassen sich nicht umbuchen.", day)
		case absence.AbsenceType == baseType && sameAbsenceTypeID(absence.AbsenceTypeID, typeID):
			return nil, rebookingBlocked("Der Eintrag vom %s hat diese Art schon.", day)
		case !absence.DateEnd.Before(s.today()):
			return nil, rebookingBlocked("Der Eintrag ist noch nicht vorbei. Sie können ihn danach ändern.")
		}
		after := *absence
		after.AbsenceType, after.AbsenceTypeID = baseType, typeID
		after.WorkingDays = nil
		if baseType == AbsenceTypeVacation {
			after.StartHalfDay, after.EndHalfDay = effectiveBoundaryHalfDays(&after)
			workingDays := countWorkingDays(after.DateStart, after.DateEnd, after.StartHalfDay, after.EndHalfDay)
			after.WorkingDays = &workingDays
		}
		entries = append(entries, rebookedAbsence{before: *absence, after: &after})
	}
	return entries, nil
}

// loadRebookingAbsences returns every row that can affect the changed days.
// The staff lock is already held, so this is the stable input for both the
// overlap check and the balance preview.
func (s *staffAbsenceService) loadRebookingAbsences(ctx context.Context, staffID int64, entries []rebookedAbsence) ([]*StaffAbsence, error) {
	start, end := rebookedDateRange(entries)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences for rebooking: %w", err)
	}
	return absences, nil
}

func (s *staffAbsenceService) validateRebookedAbsence(ctx context.Context, entry rebookedAbsence) error {
	after := entry.after
	day := after.DateStart.Format("02.01.2006")
	if err := s.rejectClosedMonths(ctx, after.StaffID, after.DateStart, after.DateEnd); err != nil {
		return err
	}
	// Only vacation keeps half days at the edges of a longer range; every
	// other type knows a half day on single-day entries only.
	if after.AbsenceType != AbsenceTypeVacation && !after.HalfDay && (after.StartHalfDay || after.EndHalfDay) {
		return rebookingBlocked("Der Urlaub ab %s hat einen halben Tag am Rand. Löschen Sie ihn und tragen Sie die Tage mit der richtigen Art neu ein.", day)
	}
	switch after.AbsenceType {
	case AbsenceTypeCompTime:
		if err := validateSingleDayHalfDayAbsence(after.AbsenceType, after.HalfDay, after.DateStart, after.DateEnd); err != nil {
			return err
		}
		err := s.rejectPreAccountCompTime(ctx, after.AbsenceType, after.DateStart, after.DateEnd)
		if err != nil && strings.HasPrefix(err.Error(), "invalid comp_time") {
			return rebookingBlocked("Der Eintrag vom %s liegt außerhalb des Stundenkontos. Dort ist kein Freizeitausgleich möglich.", day)
		}
		if err != nil {
			return err
		}
	case AbsenceTypeVacation:
		if after.WorkingDays == nil || *after.WorkingDays <= 0 {
			return rebookingBlocked("Der Eintrag vom %s hat keinen Arbeitstag. Urlaub braucht mindestens einen.", day)
		}
		err := s.rejectVacationBeforeOpening(ctx, after)
		if errors.Is(err, ErrVacationOpeningAbsencesBeforeCutoff) {
			return rebookingBlocked("Der Eintrag vom %s liegt vor der Urlaubsübernahme. Dort lässt sich kein Urlaub eintragen.", day)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// validateRebookedAbsenceOverlaps keeps rebooking aligned with creation:
// blocking absences never share a day. Rebooking keeps rows separate, so it
// cannot use creation's merge path for equal types either.
func validateRebookedAbsenceOverlaps(absences []*StaffAbsence, entries []rebookedAbsence) error {
	for _, entry := range entries {
		for _, absence := range absences {
			if absence.ID == entry.after.ID || !blocksAbsenceRange(absence.Status) {
				continue
			}
			if !absence.DateEnd.Before(entry.after.DateStart) && !entry.after.DateEnd.Before(absence.DateStart) {
				return rebookingBlocked("Der Eintrag überschneidet sich mit einer anderen Abwesenheit. Löschen Sie einen Eintrag und tragen Sie ihn neu ein.")
			}
		}
	}
	return nil
}

// rejectClosedMonths blocks a change inside a closed month: its closing
// balance is frozen, so the Stundenkonto would not follow the new type.
func (s *staffAbsenceService) rejectClosedMonths(ctx context.Context, staffID int64, start, end timezone.Date) error {
	if s.snapshotRepo == nil {
		return nil
	}
	for key := monthOf(start); !monthOf(end).before(key); key = key.next() {
		snapshot, err := s.snapshotRepo.LatestClosedMonth(ctx, staffID, key.Year, key.Month)
		if err != nil {
			return fmt.Errorf("failed to check month close state for rebooking: %w", err)
		}
		if snapshot != nil && snapshot.Year == key.Year && snapshot.Month == key.Month {
			return rebookingBlocked(
				"Der %s ist abgeschlossen. Öffnen Sie den Monat zuerst wieder. Das geht im Reiter Zeiterfassung.",
				germanMonth(key),
			)
		}
	}
	return nil
}

var germanMonthNames = [...]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

func germanMonth(key monthKey) string {
	return fmt.Sprintf("%s %d", germanMonthNames[key.Month-1], key.Year)
}

// rebookingBalanceDelta is the Stundenkonto change once all changed days have
// passed. It prices the complete affected absence set before and after the
// replacement, so the preview uses the Monatskarte's lowest-ID overlap rule.
func (s *staffAbsenceService) rebookingBalanceDelta(ctx context.Context, absences []*StaffAbsence, entries []rebookedAbsence) (int, error) {
	if !rebookingChangesCompTime(entries) {
		return 0, nil
	}
	if s.monthService == nil {
		return 0, fmt.Errorf("rebooking requires the month service")
	}
	before, after := rebookingBalanceAbsences(absences, entries)
	delta := 0
	for _, entry := range entries {
		for start := entry.after.DateStart; !start.After(entry.after.DateEnd); {
			end := start.AddDays(maxDailyTargetRangeDays)
			if entry.after.DateEnd.Before(end) {
				end = entry.after.DateEnd
			}
			targets, err := s.monthService.GetDailyTargets(ctx, entry.after.StaffID, start, end)
			if err != nil {
				return 0, fmt.Errorf("failed to resolve targets for rebooking: %w", err)
			}
			targetByDay := make(map[timezone.Date]int, len(targets))
			for _, target := range targets {
				targetByDay[target.Date] = target.TargetMinutes
			}
			delta += creditedAbsenceMinutes(after, start, end, targetByDay) - creditedAbsenceMinutes(before, start, end, targetByDay)
			start = end.AddDays(1)
		}
	}
	return delta, nil
}

func rebookingChangesCompTime(entries []rebookedAbsence) bool {
	for _, entry := range entries {
		if (entry.before.AbsenceType == AbsenceTypeCompTime) != (entry.after.AbsenceType == AbsenceTypeCompTime) {
			return true
		}
	}
	return false
}

func rebookingBalanceAbsences(absences []*StaffAbsence, entries []rebookedAbsence) ([]*StaffAbsence, []*StaffAbsence) {
	replacements := make(map[int64]*StaffAbsence, len(entries))
	for _, entry := range entries {
		replacements[entry.after.ID] = entry.after
	}
	before := slices.Clone(absences)
	after := make([]*StaffAbsence, 0, len(absences))
	for _, absence := range absences {
		if replacement, ok := replacements[absence.ID]; ok {
			after = append(after, replacement)
			continue
		}
		after = append(after, absence)
	}
	return before, after
}

func creditedAbsenceMinutes(absences []*StaffAbsence, start, end timezone.Date, targets map[timezone.Date]int) int {
	total := 0
	walkCreditedAbsenceDays(absences, start, end,
		func(d timezone.Date) int { return targets[d] },
		func(_ timezone.Date, _ *StaffAbsence, minutes int, _ float64) { total += minutes })
	return total
}

func rebookedDateRange(entries []rebookedAbsence) (timezone.Date, timezone.Date) {
	start, end := entries[0].after.DateStart, entries[0].after.DateEnd
	for _, entry := range entries[1:] {
		if entry.after.DateStart.Before(start) {
			start = entry.after.DateStart
		}
		if entry.after.DateEnd.After(end) {
			end = entry.after.DateEnd
		}
	}
	return start, end
}

// previewVacationRebooking recomputes the Resturlaub of every year an entry
// touches when vacation is the old or the new type. It reports an overdraft
// only for years the rebooking makes worse, like the other allowances.
func (s *staffAbsenceService) previewVacationRebooking(ctx context.Context, staffID int64, entries []rebookedAbsence) ([]VacationRebookingYear, bool, error) {
	var years []int
	replaced := make(map[int64]*StaffAbsence, len(entries))
	for _, entry := range entries {
		if entry.before.AbsenceType != AbsenceTypeVacation && entry.after.AbsenceType != AbsenceTypeVacation {
			continue
		}
		replaced[entry.after.ID] = entry.after
		for year := entry.after.DateStart.Year(); year <= entry.after.DateEnd.Year(); year++ {
			if !slices.Contains(years, year) {
				years = append(years, year)
			}
		}
	}
	slices.Sort(years)
	result := make([]VacationRebookingYear, 0, len(years))
	exceeded := false
	for _, year := range years {
		in, err := s.loadVacationQuotaInputs(ctx, staffID, year)
		if err != nil {
			return nil, false, err
		}
		changed := make([]*StaffAbsence, 0, len(in.absences))
		for _, absence := range in.absences {
			if next, ok := replaced[absence.ID]; ok {
				changed = append(changed, next)
				continue
			}
			changed = append(changed, absence)
		}
		before := computeVacationQuotaSummary(staffID, year, in.entitled, in.carryover, in.absences, in.opening)
		after := computeVacationQuotaSummary(staffID, year, in.entitled, in.carryover, changed, in.opening)
		if after.RemainingDays < -vacationDayEpsilon && after.RemainingDays < before.RemainingDays-vacationDayEpsilon {
			exceeded = true
		}
		result = append(result, VacationRebookingYear{Year: year, RemainingBefore: before.RemainingDays, RemainingAfter: after.RemainingDays})
	}
	return result, exceeded, nil
}

func (s *staffAbsenceService) writeRebookedAbsences(ctx context.Context, actorAccountID int64, reason string, entries []rebookedAbsence) error {
	if s.auditRepo == nil {
		return fmt.Errorf("staff absence audit repository is not configured")
	}
	now := time.Now()
	for _, entry := range entries {
		after := entry.after
		after.UpdatedAt = now
		if err := after.Validate(); err != nil {
			return fmt.Errorf("invalid absence data: %w", err)
		}
		if err := s.absenceRepo.Update(ctx, after); err != nil {
			return fmt.Errorf("failed to rebook absence: %w", err)
		}
		status := after.Status
		if err := s.auditRepo.Create(ctx, &StaffAbsenceAudit{
			AbsenceID:  after.ID,
			FromStatus: &status,
			ToStatus:   status,
			ActorID:    actorAccountID,
			Note:       reason,
			TypeChange: &timerecords.StaffAbsenceTypeChange{
				FromType: entry.before.AbsenceType, FromTypeID: entry.before.AbsenceTypeID,
				ToType: after.AbsenceType, ToTypeID: after.AbsenceTypeID,
			},
		}); err != nil {
			return fmt.Errorf("failed to audit absence rebooking: %w", err)
		}
	}
	s.getLogger().Info("staff absences rebooked",
		"staff_id", entries[0].after.StaffID,
		"absence_count", len(entries),
		"absence_type", entries[0].after.AbsenceType,
	)
	return nil
}

func rebookedResponses(entries []rebookedAbsence) []*StaffAbsenceResponse {
	responses := make([]*StaffAbsenceResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, toAbsenceResponse(entry.after))
	}
	return responses
}

func effectiveStartHalf(a *StaffAbsence) bool {
	start, _ := effectiveBoundaryHalfDays(a)
	return start
}

func effectiveEndHalf(a *StaffAbsence) bool {
	_, end := effectiveBoundaryHalfDays(a)
	return end
}
