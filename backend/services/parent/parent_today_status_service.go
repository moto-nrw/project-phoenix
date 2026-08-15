package parent

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// attendanceCultureLookbackDays bestimmt, ueber wie viele Kalendertage
// zurueckgeschaut wird, um zu erkennen, ob die Schule ueberhaupt Anwesenheit
// pflegt. Ohne diese Pruefung wuerde ein Kind an einer Schule ohne
// Anwesenheitserfassung den ganzen Tag als "nicht angekommen" erscheinen und
// Eltern grundlos beunruhigen. Wer in 14 Kalendertagen keine einzige
// Anwesenheitszeile hat, bekommt "unknown" statt einer erfundenen Aussage.
const attendanceCultureLookbackDays = 14

// GetChildTodayStatus liefert den auf Elternsicht reduzierten Betreuungsstatus
// des laufenden Berliner Kalendertages.
//
// Die Antwort ist bewusst arm: kein Raum, keine Besuchshistorie, keine
// Rohereignisse, keine Mitarbeitendennamen. active.visits wird nie gelesen,
// damit kein Raumbezug nach aussen gelangen kann; die einzige Praesenzquelle
// ist active.attendance, die sowohl der Kiosk-Scan als auch die manuelle
// Erfassung im Personal-Portal fuellt.
//
// Ein Fehler beim Aufloesen der Fakten fuehrt zu DayStateUnknown statt zu
// einem 500er: fuer Eltern ist "Status derzeit nicht verfuegbar" die richtige
// Aussage, ein Serverfehler waere nur verwirrend.
func (s *service) GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	today := timezone.TodayDate()
	facts := todayStatusFacts{NowHHMM: hhmm(timezone.Now())}

	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		absent, absErr := s.hasActiveAbsenceToday(txCtx, studentID, today)
		if absErr != nil {
			return absErr
		}
		facts.HasAbsence = absent

		if s.AttendanceRepo == nil {
			return nil
		}

		rows, attErr := s.AttendanceRepo.FindByStudentAndDate(txCtx, studentID, today)
		if attErr != nil {
			return attErr
		}
		facts.AttendanceLoaded = true
		applyAttendanceRows(&facts, rows)

		history, histErr := s.AttendanceRepo.FindByStudentAndDateRange(
			txCtx, studentID, today.AddDays(-attendanceCultureLookbackDays), today,
		)
		if histErr != nil {
			return histErr
		}
		facts.SchoolTracksAttendance = len(history) > 0

		expected, expErr := s.resolveExpectedArrival(txCtx, studentID, today)
		if expErr != nil {
			return expErr
		}
		facts.IsCareDay = expected.isCareDay
		facts.ExpectedArrival = expected.hhmm
		return nil
	})
	if txErr != nil {
		s.Logger.Warn("parent_today_status_resolve_failed",
			"student_id", studentID,
			"error", txErr.Error(),
		)
		return &TodayStatus{State: DayStateUnknown}, nil
	}

	status := deriveTodayStatus(facts)
	return &status, nil
}

// expectedArrival buendelt die beiden Fakten aus dem Betreuungsplan, die die
// Ableitung braucht: ist heute ueberhaupt ein Betreuungstag, und ab wann wird
// das Kind erwartet.
type expectedArrival struct {
	isCareDay bool
	hhmm      string
}

// resolveExpectedArrival liest den Betreuungsplan des Kindes fuer heute. Eine
// Ausnahme fuer den Tag schlaegt den Wochenplan; fehlt beides, ist heute kein
// Betreuungstag.
func (s *service) resolveExpectedArrival(ctx context.Context, studentID int64, today timezone.Date) (expectedArrival, error) {
	if s.ArrivalSchedules == nil {
		return expectedArrival{}, nil
	}

	exception, err := s.ArrivalSchedules.GetStudentArrivalExceptionForDate(ctx, studentID, today)
	if err != nil {
		return expectedArrival{}, err
	}
	if exception != nil && exception.ExpectedArrival != nil {
		return expectedArrival{isCareDay: true, hhmm: hhmm(*exception.ExpectedArrival)}, nil
	}

	plan, err := s.ArrivalSchedules.GetStudentArrivalScheduleForWeekday(ctx, studentID, isoWeekdayOf(today))
	if err != nil {
		return expectedArrival{}, err
	}
	if plan == nil {
		return expectedArrival{isCareDay: false}, nil
	}
	return expectedArrival{isCareDay: true, hhmm: hhmm(plan.ExpectedArrival)}, nil
}

// isoWeekdayOf bildet einen Kalendertag auf die Wochentagszahl ab, die
// schedule.StudentArrivalSchedule verwendet (1 = Montag bis 5 = Freitag).
// Sonntag ist in Go 0 und wird auf 7 gehoben, damit ein Wochenendtag nie
// versehentlich als Montag gilt.
func isoWeekdayOf(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return int(scheduleModels.WeekdaySunday)
	}
	return weekday
}
