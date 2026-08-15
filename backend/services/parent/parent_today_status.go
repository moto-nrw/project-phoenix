package parent

import (
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
)

// DayState ist die auf Elternsicht reduzierte Projektion eines Betreuungstages.
// Sie enthaelt bewusst keine Raum-, Bewegungs- oder Mitarbeitendendaten.
type DayState string

const (
	DayStateExpected   DayState = "expected"
	DayStateNotArrived DayState = "not_arrived"
	DayStatePresent    DayState = "present"
	DayStateLeft       DayState = "left"
	DayStateAbsent     DayState = "absent"
	DayStateNoCare     DayState = "no_care"
	DayStateUnknown    DayState = "unknown"
)

// TodayStatus ist das Ergebnis der Ableitung. Zeiten sind Wandzeiten im Format
// HH:MM und leer, wenn sie fuer den Zustand keine Bedeutung haben.
//
// AtOgs ist die erste Ebene der Anzeige: die eine Ja/Nein-Aussage, die Eltern
// in einer Sekunde erfassen sollen. Sie ist nil bei DayStateUnknown, weil eine
// Aussage, die wir nicht belegen koennen, schlimmer waere als zu schweigen.
// State ist die zweite Ebene und erklaert das Wann und Warum.
type TodayStatus struct {
	AtOgs        *bool
	State        DayState
	Since        string
	Until        string
	ExpectedFrom string
}

func atOgsFlag(v bool) *bool { return &v }

// todayStatusFacts buendelt die bereits aufgeloesten Fakten eines Tages.
// Die Ableitung ist rein: sie liest nichts nach und kennt keine Uhr.
type todayStatusFacts struct {
	// AttendanceLoaded ist false, wenn die Anwesenheit nicht belastbar
	// geladen werden konnte. Dann gilt immer unknown.
	AttendanceLoaded bool
	// SchoolTracksAttendance ist false, wenn die Schule ueberhaupt keine
	// Anwesenheit pflegt. Ohne dieses Signal wuerde ein Kind den ganzen Tag
	// faelschlich als "nicht angekommen" erscheinen und Eltern grundlos
	// beunruhigen.
	SchoolTracksAttendance bool
	HasAbsence             bool
	IsCareDay              bool
	HasAttendanceToday     bool
	CheckIn                string
	CheckOut               string
	ExpectedArrival        string
	NowHHMM                string
}

// deriveTodayStatus bildet die Fakten auf genau einen Zustand ab. Die
// Reihenfolge der Pruefungen ist die fachliche Rangfolge: was tatsaechlich
// passiert ist, schlaegt was geplant war.
func deriveTodayStatus(f todayStatusFacts) TodayStatus {
	if !f.AttendanceLoaded {
		return TodayStatus{State: DayStateUnknown}
	}
	if f.HasAbsence {
		return TodayStatus{AtOgs: atOgsFlag(false), State: DayStateAbsent}
	}
	if f.HasAttendanceToday {
		if f.CheckOut == "" {
			return TodayStatus{AtOgs: atOgsFlag(true), State: DayStatePresent, Since: f.CheckIn}
		}
		return TodayStatus{AtOgs: atOgsFlag(false), State: DayStateLeft, Until: f.CheckOut}
	}
	if !f.IsCareDay {
		return TodayStatus{AtOgs: atOgsFlag(false), State: DayStateNoCare}
	}
	if !f.SchoolTracksAttendance {
		return TodayStatus{State: DayStateUnknown}
	}
	if f.ExpectedArrival == "" {
		return TodayStatus{State: DayStateUnknown}
	}
	if f.NowHHMM < f.ExpectedArrival {
		return TodayStatus{AtOgs: atOgsFlag(false), State: DayStateExpected, ExpectedFrom: f.ExpectedArrival}
	}
	return TodayStatus{AtOgs: atOgsFlag(false), State: DayStateNotArrived, ExpectedFrom: f.ExpectedArrival}
}

// applyAttendanceRows verdichtet die Anwesenheitszeilen eines Tages auf die
// beiden Fakten, die Eltern sehen duerfen: seit wann das Kind da ist und wann
// es gegangen ist. Eine offene Zeile gewinnt immer, auch wenn davor am selben
// Tag schon eine geschlossene steht.
func applyAttendanceRows(facts *todayStatusFacts, rows []*activeModels.Attendance) {
	var latestCheckOut *time.Time
	for _, row := range rows {
		if row == nil {
			continue
		}
		facts.HasAttendanceToday = true
		if row.CheckOutTime == nil {
			facts.CheckIn = hhmm(row.CheckInTime)
			facts.CheckOut = ""
			return
		}
		if latestCheckOut == nil || row.CheckOutTime.After(*latestCheckOut) {
			latestCheckOut = row.CheckOutTime
			facts.CheckIn = hhmm(row.CheckInTime)
		}
	}
	if latestCheckOut != nil {
		facts.CheckOut = hhmm(*latestCheckOut)
	}
}

// hhmm gibt die Berliner Wandzeit eines Zeitpunkts als HH:MM zurueck.
func hhmm(t time.Time) string {
	return timezone.WallClock(t).Format("15:04")
}
