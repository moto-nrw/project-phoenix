package care

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
func (s *Service) GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	today := s.todayDate()
	facts := todayStatusFacts{NowHHMM: berlinHHMM(s.now())}

	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		absent, absErr := s.hasActiveAbsenceToday(txCtx, studentID, today)
		if absErr != nil {
			return absErr
		}
		facts.HasAbsence = absent

		if s.Attendance == nil {
			return nil
		}

		rows, attErr := s.Attendance.ListAttendance(txCtx, studentpresence.AttendanceFilter{StudentIDs: []int64{studentID}, FromDate: today.String(), UntilDate: today.String()})
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

		tracks, trackErr := s.Attendance.HasAttendance(txCtx, studentpresence.AttendanceFilter{
			FromDate: today.AddDays(-attendanceCultureLookbackDays).String(), UntilDate: today.String(),
		})
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
func (s *Service) resolveExpectedArrival(ctx context.Context, studentID int64, today timezone.Date) (expectedArrival, error) {
	if s.ArrivalSchedules == nil {
		return expectedArrival{}, nil
	}

	// Wochenenden sind nie Betreuungstage, und der Wochenplan kennt nur Montag
	// bis Freitag: eine Anfrage mit Weekday 6 oder 7 quittiert er mit
	// "invalid weekday". Also gar nicht erst fragen. Eine Ferienbetreuung am
	// Wochenende faellt trotzdem nicht durchs Raster, weil eine vorhandene
	// Anwesenheit in deriveTodayStatus vor dem Betreuungstag geprueft wird.
	// Ausnahme: Folgt das Wochenende der Schule dem Freitagsplan (#3921),
	// gilt der Wochenplan vom Freitag.
	planWeekday, careWeekday, err := planWeekdayOf(ctx, today)
	if err != nil || !careWeekday {
		return expectedArrival{resolved: err == nil}, err
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

	plan, err := s.ArrivalSchedules.GetStudentArrivalScheduleForWeekday(ctx, studentID, planWeekday)
	if err != nil {
		return expectedArrival{}, err
	}
	if plan == nil {
		return s.arrivalWithoutPlan(ctx, studentID, today)
	}
	return expectedArrival{resolved: true, isCareDay: true, hhmm: hhmm(plan.ExpectedArrival)}, nil
}

// arrivalWithoutPlan beantwortet einen Tag ohne Ankunftsplan: Eine
// Abholzeit fuer heute macht ihn trotzdem zum Betreuungstag.
func (s *Service) arrivalWithoutPlan(ctx context.Context, studentID int64, today timezone.Date) (expectedArrival, error) {
	if s.PickupSchedules == nil {
		return expectedArrival{resolved: true, isCareDay: false}, nil
	}
	pickup, err := s.PickupSchedules.GetEffectivePickupTimeForDate(ctx, studentID, today)
	if err != nil {
		return expectedArrival{}, err
	}
	return expectedArrival{resolved: true, isCareDay: pickup != nil && pickup.PickupTime != nil}, nil
}

// planWeekdayOf liefert den Wochentag, dessen Wochenplan heute gilt, und ob
// das ueberhaupt ein Betreuungstag sein kann: Montag bis Freitag, oder ein
// Wochenende, das dem Freitagsplan folgt (#3921).
func planWeekdayOf(ctx context.Context, today timezone.Date) (int, bool, error) {
	weekday, err := calendar.PlanWeekday(ctx, today)
	if err != nil {
		return 0, false, err
	}
	return weekday, weekday <= scheduleModels.WeekdayFriday, nil
}
