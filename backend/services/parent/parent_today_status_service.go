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
// Eltern grundlos beunruhigen. Gemessen wird SCHULWEIT (irgendeine
// Anwesenheitszeile des Tenants im Fenster), nicht am einzelnen Kind: ein neu
// aufgenommenes oder laenger abwesendes Kind hat selbst keine Historie, die
// Schule erfasst aber trotzdem. Hat die ganze Schule in 14 Kalendertagen
// keine einzige Zeile, gilt "unknown" statt einer erfundenen Aussage.
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

	today := s.todayDate()
	facts := todayStatusFacts{NowHHMM: hhmm(s.now())}

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

		if !facts.HasAbsence && facts.HasAttendanceToday && facts.CheckOut == "" && s.PickupSchedules != nil {
			pickup, pickupErr := s.PickupSchedules.GetEffectivePickupTimeForDate(txCtx, studentID, today)
			if pickupErr != nil {
				s.Logger.Warn("parent_today_status_pickup_unreadable",
					"student_id", studentID,
					"error", pickupErr.Error(),
				)
			} else if pickup != nil && pickup.PickupTime != nil {
				facts.PickupTime = hhmm(*pickup.PickupTime)
			}
		}

		tracks, trackErr := s.AttendanceRepo.HasAnyInRange(
			txCtx, today.AddDays(-attendanceCultureLookbackDays), today,
		)
		if trackErr != nil {
			return trackErr
		}
		facts.SchoolTracksAttendance = tracks

		// Ein nicht lesbarer Betreuungsplan darf die bereits geladene
		// Anwesenheit nicht entwerten: wer nachweislich da ist, ist da, auch
		// wenn wir seinen Plan nicht kennen. Deshalb ist dieser Fehler nicht
		// fatal, er laesst nur CareDayResolved auf false.
		expected, expErr := s.resolveExpectedArrival(txCtx, studentID, today)
		if expErr != nil {
			s.Logger.Warn("parent_today_status_care_plan_unreadable",
				"student_id", studentID,
				"error", expErr.Error(),
			)
			return nil
		}
		facts.CareDayResolved = expected.resolved
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
	// resolved ist false, wenn der Betreuungsplan gar nicht befragt werden
	// konnte. Das ist etwas anderes als "heute kein Betreuungstag": ohne
	// Auskunft duerfen wir weder "keine Betreuung" noch "nicht angekommen"
	// behaupten.
	resolved  bool
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

	// Wochenenden sind nie Betreuungstage, und der Wochenplan kennt nur Montag
	// bis Freitag: eine Anfrage mit Weekday 6 oder 7 quittiert er mit
	// "invalid weekday". Also gar nicht erst fragen. Eine Ferienbetreuung am
	// Wochenende faellt trotzdem nicht durchs Raster, weil eine vorhandene
	// Anwesenheit in deriveTodayStatus vor dem Betreuungstag geprueft wird.
	if isWeekend(today) {
		return expectedArrival{resolved: true, isCareDay: false}, nil
	}

	exception, err := s.ArrivalSchedules.GetStudentArrivalExceptionForDate(ctx, studentID, today)
	if err != nil {
		return expectedArrival{}, err
	}
	if exception != nil {
		if exception.ExpectedArrival == nil {
			return expectedArrival{resolved: true, isCareDay: false}, nil
		}
		return expectedArrival{resolved: true, isCareDay: true, hhmm: hhmm(*exception.ExpectedArrival)}, nil
	}

	plan, err := s.ArrivalSchedules.GetStudentArrivalScheduleForWeekday(ctx, studentID, isoWeekdayOf(today))
	if err != nil {
		return expectedArrival{}, err
	}
	if plan == nil {
		if s.PickupSchedules != nil {
			pickup, pickupErr := s.PickupSchedules.GetEffectivePickupTimeForDate(ctx, studentID, today)
			if pickupErr != nil {
				return expectedArrival{}, pickupErr
			}
			if pickup != nil && pickup.PickupTime != nil {
				return expectedArrival{resolved: true, isCareDay: true}, nil
			}
		}
		return expectedArrival{resolved: true, isCareDay: false}, nil
	}
	return expectedArrival{resolved: true, isCareDay: true, hhmm: hhmm(plan.ExpectedArrival)}, nil
}

// isoWeekdayOf bildet einen Kalendertag auf die Wochentagszahl ab, die
// schedule.StudentArrivalSchedule verwendet (1 = Montag bis 5 = Freitag).
// Sonntag ist in Go 0 und wird auf 7 gehoben, damit ein Wochenendtag nie
// versehentlich als Montag gilt.
func isoWeekdayOf(date timezone.Date) int {
	weekday := int(date.Weekday())
	if weekday == 0 {
		return scheduleModels.WeekdaySunday
	}
	return weekday
}

// isWeekend meldet Samstag und Sonntag. Der Wochenplan kennt sie nicht.
func isWeekend(date timezone.Date) bool {
	weekday := isoWeekdayOf(date)
	return weekday == scheduleModels.WeekdaySaturday || weekday == scheduleModels.WeekdaySunday
}
