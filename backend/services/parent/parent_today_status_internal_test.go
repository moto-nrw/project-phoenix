package parent

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

// berlinClock baut einen Zeitpunkt an einem festen Tag in Europe/Berlin. Der
// Tag selbst ist fuer die Ableitung ohne Bedeutung, nur die Wandzeit zaehlt.
func berlinClock(t *testing.T, hour, minute int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("Europe/Berlin nicht ladbar: %v", err)
	}
	return time.Date(2026, 8, 15, hour, minute, 0, 0, loc)
}

// TestDeriveTodayStatus deckt alle sieben Zustaende der Elternprojektion ab.
// Die Ableitung ist rein: sie kennt keine Uhr und liest nichts nach, damit die
// fachliche Rangfolge (was passiert ist schlaegt was geplant war) ohne
// Datenbank pruefbar bleibt.
func TestDeriveTodayStatus(t *testing.T) {
	cases := []struct {
		name       string
		facts      todayStatusFacts
		wantState  DayState
		wantAtOgs  *bool // nil bedeutet: keine Ja/Nein-Aussage
		wantSince  string
		wantUntil  string
		wantFrom   string
		wantPickup string
	}{
		{
			name:      "Anwesenheit nicht ladbar ergibt unbekannt ohne Ja/Nein-Aussage",
			facts:     todayStatusFacts{AttendanceLoaded: false, IsCareDay: true},
			wantState: DayStateUnknown,
			wantAtOgs: nil,
		},
		{
			name: "Schule fuehrt keine Anwesenheit ergibt unbekannt",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: false, CareDayResolved: true,
				IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "13:00",
			},
			wantState: DayStateUnknown,
			wantAtOgs: nil,
		},
		{
			name: "geplante Abwesenheit schlaegt alles andere",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				HasAbsence: true, IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "13:00",
			},
			wantState: DayStateAbsent,
			wantAtOgs: atOgsFlag(false),
		},
		{
			name: "offene Anwesenheit ergibt anwesend",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				HasAttendanceToday: true, CheckIn: "12:38", CheckOut: "",
				IsCareDay: true, PickupTime: "15:30", NowHHMM: "13:00",
			},
			wantState:  DayStatePresent,
			wantAtOgs:  atOgsFlag(true),
			wantSince:  "12:38",
			wantPickup: "15:30",
		},
		{
			name: "geschlossene Anwesenheit ergibt abgeholt",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				HasAttendanceToday: true, CheckIn: "12:38", CheckOut: "15:12",
				IsCareDay: true, NowHHMM: "16:00",
			},
			wantState: DayStateLeft,
			wantAtOgs: atOgsFlag(false),
			wantUntil: "15:12",
		},
		{
			name: "kein Betreuungstag ergibt keine Betreuung",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				IsCareDay: false, NowHHMM: "13:00",
			},
			wantState: DayStateNoCare,
			wantAtOgs: atOgsFlag(false),
		},
		{
			name: "vor der erwarteten Zeit ergibt erwartet",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "11:00",
			},
			wantState: DayStateExpected,
			wantAtOgs: atOgsFlag(false),
			wantFrom:  "12:30",
		},
		{
			name: "nach der erwarteten Zeit ohne Anwesenheit ergibt nicht angekommen",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "13:00",
			},
			wantState: DayStateNotArrived,
			wantAtOgs: atOgsFlag(false),
			wantFrom:  "12:30",
		},
		{
			name: "Betreuungstag ohne bekannte Ankunftszeit ergibt unbekannt",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				IsCareDay: true, ExpectedArrival: "", NowHHMM: "13:00",
			},
			wantState: DayStateUnknown,
			wantAtOgs: nil,
		},
		{
			name: "unlesbarer Betreuungsplan ergibt unbekannt statt keine Betreuung",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				CareDayResolved: false, NowHHMM: "13:00",
			},
			wantState: DayStateUnknown,
			wantAtOgs: nil,
		},
		{
			name: "unlesbarer Betreuungsplan entwertet eine vorhandene Anwesenheit nicht",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				CareDayResolved: false,
				// Am Wochenende oder bei kaputtem Plan zaehlt weiter, was
				// tatsaechlich passiert ist.
				HasAttendanceToday: true, CheckIn: "09:15", CheckOut: "",
				NowHHMM: "10:00",
			},
			wantState: DayStatePresent,
			wantAtOgs: atOgsFlag(true),
			wantSince: "09:15",
		},
		{
			name: "geschlossene Anwesenheit an einem Nicht-Betreuungstag bleibt abgeholt",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true, CareDayResolved: true,
				HasAttendanceToday: true, CheckIn: "08:05", CheckOut: "11:40",
				IsCareDay: false, NowHHMM: "12:00",
			},
			wantState: DayStateLeft,
			wantAtOgs: atOgsFlag(false),
			wantUntil: "11:40",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveTodayStatus(tc.facts)
			if got.State != tc.wantState {
				t.Fatalf("State = %q, erwartet %q", got.State, tc.wantState)
			}
			switch {
			case tc.wantAtOgs == nil && got.AtOgs != nil:
				t.Errorf("AtOgs = %v, erwartet keine Aussage", *got.AtOgs)
			case tc.wantAtOgs != nil && got.AtOgs == nil:
				t.Errorf("AtOgs fehlt, erwartet %v", *tc.wantAtOgs)
			case tc.wantAtOgs != nil && got.AtOgs != nil && *got.AtOgs != *tc.wantAtOgs:
				t.Errorf("AtOgs = %v, erwartet %v", *got.AtOgs, *tc.wantAtOgs)
			}
			if got.Since != tc.wantSince {
				t.Errorf("Since = %q, erwartet %q", got.Since, tc.wantSince)
			}
			if got.Until != tc.wantUntil {
				t.Errorf("Until = %q, erwartet %q", got.Until, tc.wantUntil)
			}
			if got.ExpectedFrom != tc.wantFrom {
				t.Errorf("ExpectedFrom = %q, erwartet %q", got.ExpectedFrom, tc.wantFrom)
			}
			if got.PickupTime != tc.wantPickup {
				t.Errorf("PickupTime = %q, erwartet %q", got.PickupTime, tc.wantPickup)
			}
		})
	}
}

// TestApplyAttendanceRowsPrefersOpenRow stellt sicher, dass eine offene
// Anwesenheit immer gewinnt, auch wenn davor am selben Tag bereits eine
// geschlossene Zeile steht (Kind war weg und ist zurueckgekommen).
func TestApplyAttendanceRowsPrefersOpenRow(t *testing.T) {
	closedOut := berlinClock(t, 11, 40)
	rows := []*activeModels.Attendance{
		{CheckInTime: berlinClock(t, 8, 5), CheckOutTime: &closedOut},
		{CheckInTime: berlinClock(t, 13, 5), CheckOutTime: nil},
	}

	facts := todayStatusFacts{}
	applyAttendanceRows(&facts, rows)

	if !facts.HasAttendanceToday {
		t.Fatal("HasAttendanceToday sollte true sein")
	}
	if facts.CheckOut != "" {
		t.Errorf("CheckOut = %q, erwartet leer weil eine Zeile offen ist", facts.CheckOut)
	}
	if facts.CheckIn != "13:05" {
		t.Errorf("CheckIn = %q, erwartet 13:05 aus der offenen Zeile", facts.CheckIn)
	}
}

// TestIsoWeekdayOfMapsAWholeWeek pins the ISO mapping the arrival schedule
// expects: Monday = 1 through Sunday = 7. Go's own Weekday numbers Sunday 0,
// so an unshifted value would make Sunday look like a Monday care day.
func TestIsoWeekdayOfMapsAWholeWeek(t *testing.T) {
	// 2026-08-17 is a Monday.
	monday := timezone.NewDate(2026, 8, 17)
	want := []int{
		scheduleModels.WeekdayMonday,
		scheduleModels.WeekdayTuesday,
		scheduleModels.WeekdayWednesday,
		scheduleModels.WeekdayThursday,
		scheduleModels.WeekdayFriday,
		scheduleModels.WeekdaySaturday,
		scheduleModels.WeekdaySunday,
	}
	for offset, expected := range want {
		date := monday.AddDays(offset)
		if got := isoWeekdayOf(date); got != expected {
			t.Errorf("isoWeekdayOf(%s) = %d, erwartet %d", date, got, expected)
		}
	}
}

// TestIsWeekendCoversTheWeek keeps Saturday and Sunday out of the weekly plan,
// which only knows Monday to Friday and rejects anything else.
func TestIsWeekendCoversTheWeek(t *testing.T) {
	monday := timezone.NewDate(2026, 8, 17)
	want := []bool{false, false, false, false, false, true, true}
	for offset, expected := range want {
		date := monday.AddDays(offset)
		if got := isWeekend(date); got != expected {
			t.Errorf("isWeekend(%s) = %v, erwartet %v", date, got, expected)
		}
	}
}
