# Eltern-Tagesstatus (Backend) — Umsetzungsplan

> **Für agentische Bearbeiter:** ERFORDERLICHE SUB-SKILL: `superpowers:subagent-driven-development` (empfohlen) oder `superpowers:executing-plans`, um diesen Plan Aufgabe für Aufgabe umzusetzen. Die Schritte nutzen Checkbox-Syntax (`- [ ]`) zur Nachverfolgung.

**Ziel:** Die Eltern-API liefert für ein verknüpftes Kind einen tagesaktuellen, auf sieben Zustände reduzierten Betreuungsstatus, und das Elternportal zeigt ihn im Kinderprofil an.

**Architektur:** Eine reine Ableitungsfunktion in `services/parent` bildet bereits aufgelöste Fakten (Anwesenheit, geplante Abwesenheit, Betreuungstag, erwartete Ankunft) auf genau einen Zustand ab. Eine Service-Methode löst diese Fakten in der Tenant-Transaktion des Kindes auf und prüft vorher die Guardian-Berechtigung. Ein dünner Handler reicht das Ergebnis als JSON heraus. Räume, Besuchshistorie und Mitarbeitendennamen werden nie gelesen.

**Tech-Stack:** Go 1.25, Chi, BUN, PostgreSQL 17, Testify. Frontend: Next.js 16, React 19, Vitest.

**Umsetzt:** Issue #2252. Spezifikation: `docs/superpowers/specs/2026-08-15-elternapp-redesign-design.md`, Abschnitt 6.

## Globale Randbedingungen

- Schichtdisziplin: Handler → Service → Repository. Handler greifen nie auf Repositories zu (CI-Ratsche `TestHandlerLayerRatchet`).
- Services konstruieren keine Queries (CI-Ratsche `TestServiceRepositoryRatchet`).
- Kalendertage sind `timezone.Date`, niemals `time.Time` (CI-Ratsche `TestDateColumnTypes`).
- Tests sind hermetisch: Fixtures aus `test/fixtures.go`, keine festen IDs, IDs erst ab `int64(42)` (CI-Ratsche `TestHermeticTestPatterns`).
- Logging über `*slog.Logger` mit snake_case-Schlüsseln. Keine Kindernamen auf Info-Level oder höher (CI-Ratsche `TestGDPRLogPIIRatchet`).
- Die Antwort enthält **niemals** Räume, Besuchshistorie, Rohereignisse, interne Notizen oder Namen von Mitarbeitenden.
- `active.visits` wird für Eltern nie gelesen.
- Zeitzone für alle Tagesgrenzen: `Europe/Berlin` über das Paket `internal/timezone`.
- Keine neuen Tenant-Einstellungen (Entscheidung E1 der Spezifikation).
- Die Statusanzeige ist **zweistufig**: `at_ogs` beantwortet groß und farbig die
  eine Frage "Ist mein Kind in der OGS?", `state` erklärt darunter Wann und
  Warum. Das Frontend leitet Ebene 1 nie selbst aus `state` ab.
- Sprache ist OGS-Sprache, wie Eltern und Team miteinander reden. Kein
  Verwaltungsvokabular, keine Systembegriffe.

## Mandat für die Oberfläche: Neubau, keine Renovierung

Übernommen aus Abschnitt 4a der Spezifikation. Verbindlich für diesen und jeden
weiteren Etappenplan dieses Vorhabens.

Für alles oberhalb der Datenschicht gilt ausdrücklich freie Hand. Die
Oberfläche der Eltern-App wird **von Grund auf neu gebaut**, nicht am Bestand
entlang verbessert. Maßstab ist eine gute Kita-Eltern-App, nicht das heutige
Elternportal.

**Was "von Grund auf" ausdrücklich erlaubt:**

- Bestehende Eltern-Komponenten **löschen und ersetzen**, statt sie zu
  erweitern. `child-detail.tsx`, `parent-dashboard.tsx`, `parent-page.tsx`,
  `child-care.tsx` und die Eltern-Zweige in Sidebar und Bottom-Nav sind
  Ausgangsmaterial, kein Bestandsschutz.
- **Neue Seitenstrukturen** erfinden, die es heute nicht gibt.
- **Bestehende Texte komplett neu schreiben** statt sie zu glätten.
- Karten, Abstände, Typografie und Dichte neu festlegen, solange die Bausteine
  im geteilten UI-Kit landen.

**Die Leitlinien:**

- **Layout nach Kita-App-Mustern:** eine Sache pro Bildschirm, große Flächen,
  klare Reihenfolge, nichts Dekoratives. Kein Dashboard-Gefühl, keine
  Kachelwände, keine Marketing-Hero-Karten. Der wichtigste Inhalt steht ohne
  Scrollen da.
- **Farbe trägt Bedeutung, nicht Schmuck.** Aus der moto-Palette, aber deutlich
  wärmer und zugänglicher eingesetzt als im Personal-Portal. Farbe nie als
  einziger Träger einer Information.
- **Sprache ist OGS- und Kita-Sprache**, so wie Eltern und Team tatsächlich
  miteinander reden. "Ist Ihr Kind heute krank?" statt "Abwesenheitsmeldung
  erfassen". Keine Systembegriffe, keine Verwaltungswörter, keine Anglizismen.
  Das gilt für alle vier Sprachkataloge, nicht nur für Deutsch.
- **Mobile, Tablet und Desktop werden jeweils eigenständig entworfen**, nicht
  ein Entwurf dreimal gestreckt. Mobile ist der Leitfall, Desktop bekommt eine
  eigene Aufteilung statt einer breiten leeren Spalte.
- **Bedienbar mit wenig digitaler Erfahrung:** Icons immer mit Textlabel, große
  Touchflächen, Folgen vor dem Absenden benennen, nur eine Hauptaktion je
  Bildschirm.

**Der Qualitätsmaßstab: wie eine gute App aus dem App Store.** Ziel ist die
Bedienqualität einer professionellen Store-App auf allen drei Gerätearten. Weil
"premium" als Anspruch nichts bewirkt, ist der Maßstab als Prüfliste
formuliert; eine Etappe gilt erst als fertig, wenn ihre Punkte erfüllt sind.

*Ehrliche Randbedingung:* Dies ist eine Web-App. Auf iOS entsteht der
App-Charakter erst nach Installation auf dem Home-Bildschirm, und nur
installiert funktionieren dort Push-Benachrichtigungen. Die
Home-Bildschirm-Anleitung (#2306) ist Voraussetzung, nicht Beiwerk.

*Überall:* Touchflächen mindestens 48 px. Fließtext ab 17 px, keine
Versalien-Mikrolabels. Ein typografischer Maßstab für die ganze App. Icons
immer mit Textlabel. Kein horizontaler Seiten-Scroll bei keiner Breite. Kein
Layout-Sprung beim Eintreffen von Daten, Skelette in der Form des Endzustands.
Rückmeldung auf jede Berührung binnen 100 ms über einen Aktiv-Zustand. Leere
Zustände nennen die Folge und bieten genau eine Aktion. Bewegung nur, wenn sie
etwas erklärt, nur über Transform und Opacity, `prefers-reduced-motion` wird
respektiert. Bedienbar bei 200 % Zoom und 320 px Breite, vollständig per
Tastatur, mit sichtbarem Fokusring.

*Mobile, der Leitfall:* Safe Areas über `env(safe-area-inset-*)` respektiert.
Bottom-Navigation fest, in Daumenreichweite, höchstens fünf Ziele, Icon und
Label, klarer Aktiv-Zustand. Hauptaktionen unten statt oben. Dialoge als Sheet
von unten mit angehefteter Aktionsleiste. Kein Verhalten, das Hover
voraussetzt. Installierbar als PWA.

*Tablet:* kein gestrecktes Handy. Zwei Spalten, wo Liste und Detail es
hergeben. Im Querformat Seitennavigation statt Bottom-Navigation. Dialoge als
mittige Fenster statt Vollbild-Sheets.

*Desktop:* dauerhafte Seitennavigation. Begrenzte Zeilenlänge für Fließtext.
Keine leeren Bildschirmhälften. Hover- und Fokuszustände für jedes bedienbare
Element.

**Die einzigen Grenzen:** neue Bausteine ins geteilte UI-Kit
(`frontend/src/components/ui/`), Farben aus der moto-Palette über
`moto-*`-Utilities, Kalenderdaten als `timezone.Date` bzw. `"YYYY-MM-DD"`,
`pnpm run check` ohne Warnung, jede sichtbare Änderung mit Vorher/Nachher-
Aufnahmen in Mobile, Tablet und Desktop belegt.

Im Zweifel gilt: die für Eltern verständlichere Lösung schlägt die zum Bestand
ähnlichere.

**Geltung in diesem Plan:** Dieser Plan ist der Backend-Teil und nimmt vom
Neubau bewusst **nichts** vorweg. Aufgabe 5 setzt den Status nur in das heute
bestehende Kinderprofil, weil #2252 genau das verlangt ("Die aktuelle
vollständige Elternansicht zeigt den neuen Status bereits unabhängig vom
späteren kompakten Layout"). Wer Aufgabe 5 ausführt, baut dort also **nicht**
das Kinderprofil um. Der Neubau von Navigation, Startseite und Kinderprofil ist
Gegenstand der Etappen 2 bis 5 mit eigenen Plänen, die dieses Mandat erneut
wortgleich tragen.

## Vorbereitung der Testdatenbank

Einmalig vor der ersten Aufgabe:

```bash
docker compose --profile test up -d postgres-test
cd backend && APP_ENV=test go run . migrate reset
```

## Dateiübersicht

| Datei | Verantwortung |
|---|---|
| `backend/services/parent/parent_today_status.go` | Zustandstypen und reine Ableitungsfunktion |
| `backend/services/parent/parent_today_status_internal_test.go` | Tabellentest der Ableitung |
| `backend/services/parent/parent_today_status_service.go` | Auflösung der Fakten, Berechtigungsprüfung |
| `backend/services/parent/parent_today_status_service_test.go` | Hermetischer Datenbanktest |
| `backend/services/parent/parent_service.go` | `Service`-Interface und `ServiceConfig` erweitern |
| `backend/api/parent/today_status_handlers.go` | HTTP-Handler und Antwortform |
| `backend/api/parent/api.go` | Route registrieren |
| `backend/api/parent/today_status_handlers_test.go` | Router-getriebener Test |
| `backend/services/factory.go` | `AttendanceRepo` in die Parent-Service-Konfiguration verdrahten |
| `frontend/src/lib/parent-api.ts` | Client-Funktion und Typ |
| `frontend/src/components/parent/child-today-status.tsx` | Anzeige des Status |
| `frontend/src/components/parent/child-detail.tsx` | Status einbinden |
| `frontend/src/i18n/messages/{de,en,ru,sq}.json` | Texte für sieben Zustände |

---

### Aufgabe 1: Reine Zustandsableitung

**Dateien:**
- Anlegen: `backend/services/parent/parent_today_status.go`
- Test: `backend/services/parent/parent_today_status_internal_test.go`

**Schnittstellen:**
- Nutzt: nichts aus früheren Aufgaben.
- Stellt bereit: `DayState` (string-Typ), die sieben Konstanten `DayState*`, `TodayStatus` (Antwortstruktur mit `State`, `Since`, `Until`, `ExpectedFrom`), `todayStatusFacts` (Eingabestruktur) und `deriveTodayStatus(todayStatusFacts) TodayStatus`.

- [ ] **Schritt 1: Den fehlschlagenden Test schreiben**

Datei `backend/services/parent/parent_today_status_internal_test.go`:

```go
package parent

import "testing"

func TestDeriveTodayStatus(t *testing.T) {
	cases := []struct {
		name      string
		facts     todayStatusFacts
		wantState DayState
		wantAtOgs *bool // nil bedeutet: keine Ja/Nein-Aussage
		wantSince string
		wantUntil string
		wantFrom  string
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
				AttendanceLoaded: true, SchoolTracksAttendance: false,
				IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "13:00",
			},
			wantState: DayStateUnknown,
		},
		{
			name: "geplante Abwesenheit schlaegt alles andere",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				HasAbsence: true, IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "13:00",
			},
			wantState: DayStateAbsent,
			wantAtOgs: boolPtr(false),
		},
		{
			name: "offene Anwesenheit ergibt anwesend",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				HasAttendanceToday: true, CheckIn: "12:38", CheckOut: "",
				IsCareDay: true, NowHHMM: "13:00",
			},
			wantState: DayStatePresent,
			wantAtOgs: boolPtr(true),
			wantSince: "12:38",
		},
		{
			name: "geschlossene Anwesenheit ergibt abgeholt",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				HasAttendanceToday: true, CheckIn: "12:38", CheckOut: "15:12",
				IsCareDay: true, NowHHMM: "16:00",
			},
			wantState: DayStateLeft,
			wantAtOgs: boolPtr(false),
			wantUntil: "15:12",
		},
		{
			name: "kein Betreuungstag ergibt keine Betreuung",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				IsCareDay: false, NowHHMM: "13:00",
			},
			wantState: DayStateNoCare,
			wantAtOgs: boolPtr(false),
		},
		{
			name: "vor der erwarteten Zeit ergibt erwartet",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "11:00",
			},
			wantState: DayStateExpected,
			wantAtOgs: boolPtr(false),
			wantFrom:  "12:30",
		},
		{
			name: "nach der erwarteten Zeit ohne Anwesenheit ergibt nicht angekommen",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				IsCareDay: true, ExpectedArrival: "12:30", NowHHMM: "13:00",
			},
			wantState: DayStateNotArrived,
			wantAtOgs: boolPtr(false),
			wantFrom:  "12:30",
		},
		{
			name: "Betreuungstag ohne bekannte Ankunftszeit ergibt unbekannt",
			facts: todayStatusFacts{
				AttendanceLoaded: true, SchoolTracksAttendance: true,
				IsCareDay: true, ExpectedArrival: "", NowHHMM: "13:00",
			},
			wantState: DayStateUnknown,
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
			case tc.wantAtOgs != nil && *got.AtOgs != *tc.wantAtOgs:
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
		})
	}
}
```

- [ ] **Schritt 2: Test laufen lassen, Fehlschlag bestätigen**

```bash
cd backend && go test ./services/parent/ -run TestDeriveTodayStatus -v
```

Erwartet: Übersetzungsfehler, `undefined: todayStatusFacts`, `undefined: deriveTodayStatus`.

- [ ] **Schritt 3: Minimale Implementierung schreiben**

Datei `backend/services/parent/parent_today_status.go`:

```go
package parent

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

func boolPtr(v bool) *bool { return &v }

// todayStatusFacts buendelt die bereits aufgeloesten Fakten eines Tages.
// Die Ableitung ist rein: sie liest nichts nach und kennt keine Uhr.
type todayStatusFacts struct {
	// AttendanceLoaded ist false, wenn die Anwesenheit nicht belastbar
	// geladen werden konnte. Dann gilt immer unknown.
	AttendanceLoaded bool
	// SchoolTracksAttendance ist false, wenn die Schule ueberhaupt keine
	// Anwesenheit pflegt. Ohne dieses Signal wuerde ein Kind den ganzen Tag
	// faelschlich als "nicht angekommen" erscheinen.
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
		return TodayStatus{AtOgs: boolPtr(false), State: DayStateAbsent}
	}
	if f.HasAttendanceToday {
		if f.CheckOut == "" {
			return TodayStatus{AtOgs: boolPtr(true), State: DayStatePresent, Since: f.CheckIn}
		}
		return TodayStatus{AtOgs: boolPtr(false), State: DayStateLeft, Until: f.CheckOut}
	}
	if !f.IsCareDay {
		return TodayStatus{AtOgs: boolPtr(false), State: DayStateNoCare}
	}
	if !f.SchoolTracksAttendance {
		return TodayStatus{State: DayStateUnknown}
	}
	if f.ExpectedArrival == "" {
		return TodayStatus{State: DayStateUnknown}
	}
	if f.NowHHMM < f.ExpectedArrival {
		return TodayStatus{AtOgs: boolPtr(false), State: DayStateExpected, ExpectedFrom: f.ExpectedArrival}
	}
	return TodayStatus{AtOgs: boolPtr(false), State: DayStateNotArrived, ExpectedFrom: f.ExpectedArrival}
}
```

- [ ] **Schritt 4: Test laufen lassen, Erfolg bestätigen**

```bash
cd backend && go test ./services/parent/ -run TestDeriveTodayStatus -v
```

Erwartet: PASS, neun Unterfälle.

- [ ] **Schritt 5: Committen**

```bash
git add backend/services/parent/parent_today_status.go backend/services/parent/parent_today_status_internal_test.go
git commit -m "feat: leite den reduzierten Eltern-Tagesstatus ab"
```

---

### Aufgabe 2: Service-Methode mit Berechtigungsprüfung

**Dateien:**
- Anlegen: `backend/services/parent/parent_today_status_service.go`
- Ändern: `backend/services/parent/parent_service.go` (`Service`-Interface, `ServiceConfig`)
- Ändern: `backend/services/factory.go` (`AttendanceRepo` verdrahten)
- Test: `backend/services/parent/parent_today_status_service_test.go`

**Schnittstellen:**
- Nutzt: `deriveTodayStatus(todayStatusFacts) TodayStatus` und `TodayStatus` aus Aufgabe 1. Bestehend: `s.resolvePermittedChild(ctx, accountID, studentID, permission) (*parentChild, error)`, `authorize.GuardianPermissionPortalAccess`, `tenant.WithTenantTx`, `timezone.TodayDate()`, `timezone.Now()`, `timezone.WallClock(t)`.
- Stellt bereit: `Service.GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error)`.

Genutzte Repository- und Service-Signaturen, alle bereits vorhanden:

```go
activeModels.AttendanceRepository.FindByStudentAndDate(ctx, studentID int64, date timezone.Date) ([]*active.Attendance, error)
activeModels.AttendanceRepository.FindByStudentAndDateRange(ctx, studentID int64, from, to timezone.Date) ([]*active.Attendance, error)
scheduleSvc.ArrivalScheduleService.GetStudentArrivalScheduleForWeekday(ctx, studentID int64, weekday int) (*schedule.StudentArrivalSchedule, error)
scheduleSvc.ArrivalScheduleService.GetStudentArrivalExceptionForDate(ctx, studentID int64, date timezone.Date) (*schedule.StudentArrivalException, error)
```

Feldnamen: `active.Attendance.CheckInTime time.Time`, `active.Attendance.CheckOutTime *time.Time`, `schedule.StudentArrivalSchedule.ExpectedArrival time.Time`, `schedule.StudentArrivalException.ExpectedArrival *time.Time`.

- [ ] **Schritt 1: Den fehlschlagenden Test schreiben**

Datei `backend/services/parent/parent_today_status_service_test.go`:

```go
package parent_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestGetChildTodayStatusPresent legt eine offene Anwesenheit an und erwartet
// den Zustand present mit der Check-in-Zeit.
func TestGetChildTodayStatusPresent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	fixture := newTodayStatusFixture(t, db)
	defer fixture.cleanup()

	checkIn := timezone.WallClock(time.Date(2026, 8, 15, 12, 38, 0, 0, time.UTC))
	fixture.openAttendance(checkIn)

	status, err := fixture.service.GetChildTodayStatus(
		context.Background(), fixture.accountID, fixture.studentID,
	)
	require.NoError(t, err)
	require.Equal(t, parentService.DayStatePresent, status.State)
	require.Equal(t, "12:38", status.Since)
	require.Empty(t, status.Until)
}

// TestGetChildTodayStatusRejectsForeignChild stellt sicher, dass ein Konto ohne
// Guardian-Verknuepfung keinen Status erhaelt.
func TestGetChildTodayStatusRejectsForeignChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	fixture := newTodayStatusFixture(t, db)
	defer fixture.cleanup()

	foreign := testpkg.CreateTestStudent(t, db, "Fremdes", "Kind", "2b")
	defer testpkg.CleanupStudent(t, db, foreign.ID)

	_, err := fixture.service.GetChildTodayStatus(
		context.Background(), fixture.accountID, foreign.ID,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// TestGetChildTodayStatusWithoutAttendanceCultureIsUnknown deckt den Rueckfall
// fuer Schulen ab, die gar keine Anwesenheit pflegen: der Status ist unknown,
// niemals not_arrived.
func TestGetChildTodayStatusWithoutAttendanceCultureIsUnknown(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	fixture := newTodayStatusFixture(t, db)
	defer fixture.cleanup()

	fixture.arrivalScheduleAt(timezone.TodayDate().Weekday(), "08:00")

	status, err := fixture.service.GetChildTodayStatus(
		context.Background(), fixture.accountID, fixture.studentID,
	)
	require.NoError(t, err)
	require.Equal(t, parentService.DayStateUnknown, status.State)
}

// TestGetChildTodayStatusRequiresPortalAccess prueft, dass eine
// Guardian-Verknuepfung ohne parent_portal.access abgelehnt wird.
func TestGetChildTodayStatusRequiresPortalAccess(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	fixture := newTodayStatusFixture(t, db)
	defer fixture.cleanup()

	fixture.setGuardianPermissions([]string{})

	_, err := fixture.service.GetChildTodayStatus(
		context.Background(), fixture.accountID, fixture.studentID,
	)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)

	fixture.setGuardianPermissions([]string{authorize.GuardianPermissionPortalAccess})
	_, err = fixture.service.GetChildTodayStatus(
		context.Background(), fixture.accountID, fixture.studentID,
	)
	require.NoError(t, err)
}
```

Der Bearbeiter legt `newTodayStatusFixture` im selben Test-Paket an. Vorbild ist die bestehende Fixture-Verwendung in `backend/services/parent/parent_care_schedule_service_test.go`: Tenant, Schule, Kind, Guardian-Profil und `users.students_guardians`-Verknüpfung über die `CreateTest*`-Helfer aus `backend/test/fixtures.go`, plus ein `cleanup()`, das alles wieder entfernt. Die Fixture kapselt zusätzlich:

- `openAttendance(checkIn time.Time)` — schreibt eine Zeile in `active.attendance` mit `check_out_time = NULL`,
- `arrivalScheduleAt(weekday time.Weekday, hhmm string)` — legt einen `schedule.student_arrival_schedules`-Eintrag an,
- `setGuardianPermissions(perms []string)` — setzt `users.students_guardians.permissions`,
- `service` — der über `parentService.NewService(parentService.ServiceConfig{...})` gebaute Dienst mit den in Schritt 3 ergänzten Feldern.

- [ ] **Schritt 2: Test laufen lassen, Fehlschlag bestätigen**

```bash
cd backend && go test ./services/parent/ -run TestGetChildTodayStatus -v
```

Erwartet: Übersetzungsfehler, `GetChildTodayStatus` ist im `Service`-Interface nicht definiert.

- [ ] **Schritt 3: Interface und Konfiguration erweitern**

In `backend/services/parent/parent_service.go`, im `Service`-Interface ergänzen:

```go
	// GetChildTodayStatus liefert den auf Elternsicht reduzierten
	// Betreuungsstatus des laufenden Berliner Kalendertages. Die Antwort
	// enthaelt niemals Raeume, Besuchshistorie oder Mitarbeitendennamen.
	GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error)
```

In `ServiceConfig` ergänzen:

```go
	// AttendanceRepo liefert die schulweite Anwesenheit des Kindes. Sie ist
	// die einzige Praesenzquelle fuer Eltern; active.visits wird nie gelesen,
	// damit kein Raumbezug nach aussen gelangt.
	AttendanceRepo activeModels.AttendanceRepository
```

In `backend/services/factory.go` an der Stelle, an der `parentService.ServiceConfig` befüllt wird, ergänzen:

```go
		AttendanceRepo: repos.Attendance(),
```

Der Bearbeiter prüft den tatsächlichen Namen des Zugriffs auf das Attendance-Repository in der Repository-Factory und verwendet ihn unverändert; er ist bereits vorhanden, weil `services/active` dasselbe Repository nutzt.

- [ ] **Schritt 4: Service-Methode implementieren**

Datei `backend/services/parent/parent_today_status_service.go`:

```go
package parent

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// attendanceCultureLookbackDays bestimmt, ueber wie viele Kalendertage
// zurueckgeschaut wird, um zu erkennen, ob die Schule ueberhaupt Anwesenheit
// pflegt. Ohne diese Pruefung wuerde ein Kind an einer Schule ohne
// Anwesenheitserfassung den ganzen Tag als "nicht angekommen" erscheinen und
// Eltern grundlos beunruhigen.
const attendanceCultureLookbackDays = 14

func (s *service) GetChildTodayStatus(ctx context.Context, accountID, studentID int64) (*TodayStatus, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	today := timezone.TodayDate()
	now := timezone.Now()
	facts := todayStatusFacts{
		NowHHMM: now.Format("15:04"),
	}

	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
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
	if err != nil {
		s.Logger.Warn("parent_today_status_resolve_failed",
			"student_id", studentID,
			"error", err.Error(),
		)
		return &TodayStatus{State: DayStateUnknown}, nil
	}

	status := deriveTodayStatus(facts)
	return &status, nil
}

// applyAttendanceRows verdichtet die Anwesenheitszeilen eines Tages auf die
// beiden Fakten, die Eltern sehen duerfen: seit wann das Kind da ist und wann
// es gegangen ist. Eine offene Zeile gewinnt immer.
func applyAttendanceRows(facts *todayStatusFacts, rows []*activeModels.Attendance) {
	var latestCheckOut *time.Time
	for _, row := range rows {
		if row == nil {
			continue
		}
		facts.HasAttendanceToday = true
		if row.CheckOutTime == nil {
			facts.CheckIn = timezone.WallClock(row.CheckInTime).Format("15:04")
			facts.CheckOut = ""
			return
		}
		if latestCheckOut == nil || row.CheckOutTime.After(*latestCheckOut) {
			latestCheckOut = row.CheckOutTime
			facts.CheckIn = timezone.WallClock(row.CheckInTime).Format("15:04")
		}
	}
	if latestCheckOut != nil {
		facts.CheckOut = timezone.WallClock(*latestCheckOut).Format("15:04")
	}
}

type expectedArrival struct {
	isCareDay bool
	hhmm      string
}

// resolveExpectedArrival ermittelt, ob heute ein Betreuungstag ist und ab wann
// das Kind erwartet wird. Eine Ausnahme fuer den Tag schlaegt den Wochenplan.
func (s *service) resolveExpectedArrival(ctx context.Context, studentID int64, today timezone.Date) (expectedArrival, error) {
	if s.ArrivalSchedules == nil {
		return expectedArrival{}, nil
	}
	exception, err := s.ArrivalSchedules.GetStudentArrivalExceptionForDate(ctx, studentID, today)
	if err != nil {
		return expectedArrival{}, err
	}
	if exception != nil && exception.ExpectedArrival != nil {
		return expectedArrival{
			isCareDay: true,
			hhmm:      timezone.WallClock(*exception.ExpectedArrival).Format("15:04"),
		}, nil
	}

	weekday := int(today.Weekday())
	plan, err := s.ArrivalSchedules.GetStudentArrivalScheduleForWeekday(ctx, studentID, weekday)
	if err != nil {
		return expectedArrival{}, err
	}
	if plan == nil {
		return expectedArrival{isCareDay: false}, nil
	}
	return expectedArrival{
		isCareDay: true,
		hhmm:      timezone.WallClock(plan.ExpectedArrival).Format("15:04"),
	}, nil
}

var _ = fmt.Sprintf // entfernen, sobald fmt tatsaechlich genutzt wird
```

Der Bearbeiter entfernt die letzte Zeile und den `fmt`-Import, wenn `fmt` ungenutzt bleibt, und ergänzt den Import von `activeModels "github.com/moto-nrw/project-phoenix/models/active"`.

- [ ] **Schritt 5: Test laufen lassen, Erfolg bestätigen**

```bash
cd backend && go test ./services/parent/ -run TestGetChildTodayStatus -v
```

Erwartet: PASS, vier Tests.

- [ ] **Schritt 6: Ratschen und Linter prüfen**

```bash
cd backend && go test ./test/ -run 'TestServiceRepositoryRatchet|TestHermeticTestPatterns|TestGDPRLogPIIRatchet' -v
golangci-lint run --timeout 10m
```

Erwartet: PASS und keine neuen Befunde.

- [ ] **Schritt 7: Committen**

```bash
git add backend/services/parent/ backend/services/factory.go
git commit -m "feat: liefere den Eltern-Tagesstatus im Parent-Service"
```

---

### Aufgabe 3: HTTP-Endpunkt

**Dateien:**
- Anlegen: `backend/api/parent/today_status_handlers.go`
- Ändern: `backend/api/parent/api.go`
- Test: `backend/api/parent/today_status_handlers_test.go`

**Schnittstellen:**
- Nutzt: `Service.GetChildTodayStatus(ctx, accountID, studentID) (*TodayStatus, error)` aus Aufgabe 2, `rs.parentAccountID(w, r) (int64, bool)` (bereits vorhanden, siehe `child_write_handlers.go`).
- Stellt bereit: `GET /parent/me/children/{studentId}/today` mit der JSON-Form `{"state","since","until","expected_from"}`.

- [ ] **Schritt 1: Den fehlschlagenden Test schreiben**

Datei `backend/api/parent/today_status_handlers_test.go`, im Paket `parent_test`. Der Test baut den `Resource` genauso auf wie `child_write_handlers_test.go` (gleiche `init()`-JWT-Pinnung, gleiche Fixtures) und prüft:

```go
// TestTodayStatusEndpointReturnsPresent stellt sicher, dass der Endpunkt den
// Zustand present samt Uhrzeit liefert.
func TestTodayStatusEndpointReturnsPresent(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	env := newParentAPIEnv(t, db)
	defer env.cleanup()
	env.openAttendanceAt("12:38")

	rec := env.get(t, "/me/children/"+strconv.FormatInt(env.studentID, 10)+"/today")

	require.Equal(t, http.StatusOK, rec.Code)

	var body struct {
		Data struct {
			State        string `json:"state"`
			Since        string `json:"since"`
			Until        string `json:"until"`
			ExpectedFrom string `json:"expected_from"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "present", body.Data.State)
	require.Equal(t, "12:38", body.Data.Since)
	require.Empty(t, body.Data.Until)
}

// TestTodayStatusEndpointHidesForeignChild prueft die Mandantentrennung.
func TestTodayStatusEndpointHidesForeignChild(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	env := newParentAPIEnv(t, db)
	defer env.cleanup()

	foreign := testpkg.CreateTestStudent(t, db, "Fremdes", "Kind", "2b")
	defer testpkg.CleanupStudent(t, db, foreign.ID)

	rec := env.get(t, "/me/children/"+strconv.FormatInt(foreign.ID, 10)+"/today")

	require.Equal(t, http.StatusForbidden, rec.Code)
}

// TestTodayStatusEndpointLeaksNoInternalFields pinnt die Antwortform: kein
// Raum, keine Besuchshistorie, keine Mitarbeitendennamen.
func TestTodayStatusEndpointLeaksNoInternalFields(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()

	env := newParentAPIEnv(t, db)
	defer env.cleanup()
	env.openAttendanceAt("12:38")

	rec := env.get(t, "/me/children/"+strconv.FormatInt(env.studentID, 10)+"/today")

	var raw map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &raw))
	data, ok := raw["data"].(map[string]any)
	require.True(t, ok)

	allowed := map[string]bool{
		"at_ogs": true, "state": true, "since": true,
		"until": true, "expected_from": true,
	}
	for key := range data {
		require.Truef(t, allowed[key], "unerwartetes Feld in der Elternantwort: %q", key)
	}
}
```

`newParentAPIEnv` kapselt Fixtures, den gebauten `Resource`, den gemünzten Parent-JWT und ein `get(path)`, das gegen `resource.Router()` läuft. Vorbild ist die bestehende Hilfsstruktur in `child_write_handlers_test.go`.

- [ ] **Schritt 2: Test laufen lassen, Fehlschlag bestätigen**

```bash
cd backend && go test ./api/parent/ -run TestTodayStatusEndpoint -v
```

Erwartet: 404 statt 200, weil die Route noch fehlt.

- [ ] **Schritt 3: Handler implementieren**

Datei `backend/api/parent/today_status_handlers.go`:

```go
package parent

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// TodayStatusResponse ist die vollstaendige Elternsicht auf den Betreuungstag.
// Die Felder sind bewusst die einzigen: jedes weitere Feld waere ein Leck aus
// der internen Anwesenheitserfassung.
type TodayStatusResponse struct {
	// AtOgs ist die erste Anzeigeebene: die Ja/Nein-Aussage "in der OGS".
	// null bedeutet, dass wir keine belastbare Aussage treffen koennen.
	AtOgs        *bool  `json:"at_ogs"`
	State        string `json:"state"`
	Since        string `json:"since,omitempty"`
	Until        string `json:"until,omitempty"`
	ExpectedFrom string `json:"expected_from,omitempty"`
}

// getChildTodayStatus liefert den reduzierten Betreuungsstatus des laufenden
// Berliner Kalendertages fuer ein verknuepftes Kind.
func (rs *Resource) getChildTodayStatus(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := rs.childIDParam(w, r)
	if !ok {
		return
	}

	status, err := rs.ParentService.GetChildTodayStatus(r.Context(), accountID, studentID)
	if err != nil {
		switch {
		case errors.Is(err, parentService.ErrChildNotLinked),
			errors.Is(err, parentService.ErrGuardianPermissionDenied):
			common.RenderError(w, r, common.ErrorForbidden(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, TodayStatusResponse{
		AtOgs:        status.AtOgs,
		State:        string(status.State),
		Since:        status.Since,
		Until:        status.Until,
		ExpectedFrom: status.ExpectedFrom,
	}, "Today status retrieved")
}
```

Der Bearbeiter verwendet für `childIDParam` den bereits vorhandenen Helfer, mit dem die anderen `/me/children/{studentId}/…`-Handler ihre Pfad-ID lesen; der Name steht in `backend/api/parent/child_write_handlers.go`.

- [ ] **Schritt 4: Route registrieren**

In `backend/api/parent/api.go`, direkt neben `r.Get("/me/children/{studentId}/features", rs.getChildFeatures)`:

```go
		r.Get("/me/children/{studentId}/today", rs.getChildTodayStatus)
```

- [ ] **Schritt 5: Tests laufen lassen, Erfolg bestätigen**

```bash
cd backend && go test ./api/parent/ -run TestTodayStatusEndpoint -v
cd backend && go test ./test/ -run TestHandlerLayerRatchet -v
```

Erwartet: PASS.

- [ ] **Schritt 6: Committen**

```bash
git add backend/api/parent/
git commit -m "feat: stelle den Eltern-Tagesstatus als Endpunkt bereit"
```

---

### Aufgabe 4: Echtzeit-Aktualisierung bei Check-in und Checkout

**Dateien:**
- Ändern: `backend/services/active/attendance_service.go`
- Test: `backend/services/active/attendance_service_test.go` (bestehende Datei ergänzen)

**Schnittstellen:**
- Nutzt: `realtime.NewParentChildUpdatedEvent(guardianAccountID, studentID int64) Event` (vorhanden, `realtime/events.go:369`), `s.Broadcaster`.
- Stellt bereit: nichts für spätere Aufgaben. Das Frontend hört bereits auf `parent_child_updated` (siehe `frontend/src/components/parent/parent-realtime-bridge.tsx`).

Hintergrund: Der Ereignistyp und die Zustellung an Sorgeberechtigte existieren bereits, ausgelöst wird er heute nur nach personalseitigen Betreuungsänderungen (`wakeChildGuardians` in `backend/api/students/student_handlers.go:1067`). Diese Aufgabe löst ihn zusätzlich nach Anwesenheitswechseln aus.

- [ ] **Schritt 1: Den fehlschlagenden Test schreiben**

Der Test nutzt `testpkg.RecordingBroadcaster` (`backend/test/broadcaster.go`), hängt ihn in den Active-Service, führt einen Check-in und danach einen Checkout aus und erwartet je genau ein `parent_child_updated`-Ereignis mit der `student_id` des Kindes. Er prüft zusätzlich, dass ein absorbierter Doppel-Check-in **kein** zweites Ereignis erzeugt, spiegelbildlich zum vorhandenen Verhalten in `performCheckIn`.

- [ ] **Schritt 2: Test laufen lassen, Fehlschlag bestätigen**

```bash
cd backend && go test ./services/active/ -run TestAttendanceWakesGuardians -v
```

Erwartet: FAIL, keine Ereignisse aufgezeichnet.

- [ ] **Schritt 3: Auslösung implementieren**

In `performCheckIn` nach dem erfolgreichen Einfügen (nur im Zweig `inserted == true`) und in `performCheckOut` nach dem tatsächlichen Schließen einer offenen Zeile: die Sorgeberechtigten des Kindes ermitteln und je Konto ein `realtime.NewParentChildUpdatedEvent(guardianAccountID, studentID)` senden. Das Senden ist bewusst nach dem Commit, best-effort und ohne Fehlerweitergabe, wie bei den bestehenden Broadcast-Stellen in `services/active`.

- [ ] **Schritt 4: Test laufen lassen, Erfolg bestätigen**

```bash
cd backend && go test ./services/active/ -run TestAttendanceWakesGuardians -v
cd backend && go test ./services/active/... 
```

Erwartet: PASS, keine Regression.

- [ ] **Schritt 5: Committen**

```bash
git add backend/services/active/
git commit -m "feat: wecke Sorgeberechtigte bei Anwesenheitswechseln"
```

---

### Aufgabe 5: Anzeige im Elternportal

**Dateien:**
- Ändern: `frontend/src/lib/parent-api.ts`
- Anlegen: `frontend/src/components/parent/child-today-status.tsx`
- Anlegen: `frontend/src/components/parent/child-today-status.test.tsx`
- Ändern: `frontend/src/components/parent/child-detail.tsx`
- Ändern: `frontend/src/i18n/messages/de.json`, `en.json`, `ru.json`, `sq.json`

**Schnittstellen:**
- Nutzt: `GET /parent/me/children/{studentId}/today` aus Aufgabe 3.
- Stellt bereit: `getChildTodayStatus(studentId: string): Promise<ChildTodayStatus>` und die Komponente `<ChildTodayStatus studentId={...} />`.

Diese Aufgabe erfüllt das Kriterium aus #2252, dass die bestehende vollständige Elternansicht den Status bereits zeigt, unabhängig vom späteren kompakten Layout aus Etappe 3.

- [ ] **Schritt 1: Den fehlschlagenden Test schreiben**

Datei `frontend/src/components/parent/child-today-status.test.tsx`: rendert die Komponente mit gemocktem Client für alle sieben Zustände und prüft je:

1. **Ebene 1** steht da und stimmt: `at_ogs: true` → "In der OGS", `at_ogs: false` → "Nicht in der OGS", `at_ogs: null` → keine der beiden Zeilen ist im Dokument.
2. **Ebene 2** steht da und trägt die richtige Uhrzeit, z. B. `present` mit `since: "12:38"` → "Seit 12:38 Uhr da".
3. Jede Ausprägung hat **Icon und Text**, Farbe transportiert nie allein die Aussage.
4. Ein Kind mit `state: "left"` zeigt Ebene 1 als "Nicht in der OGS", nicht als "In der OGS" — die Regressionsprobe darauf, dass Ebene 1 aus `at_ogs` und nicht aus einer Frontend-Ableitung stammt.

- [ ] **Schritt 2: Test laufen lassen, Fehlschlag bestätigen**

```bash
cd frontend && pnpm vitest run src/components/parent/child-today-status.test.tsx
```

Erwartet: FAIL, Modul nicht gefunden.

- [ ] **Schritt 3: Client-Funktion ergänzen**

In `frontend/src/lib/parent-api.ts`, im Stil der bestehenden `getChildFeatures`:

```ts
export type ChildTodayState =
  | "expected"
  | "not_arrived"
  | "present"
  | "left"
  | "absent"
  | "no_care"
  | "unknown";

export interface ChildTodayStatus {
  /** Erste Anzeigeebene: in der OGS ja/nein. null = keine belastbare Aussage. */
  at_ogs: boolean | null;
  state: ChildTodayState;
  since?: string;
  until?: string;
  expected_from?: string;
}

export async function getChildTodayStatus(
  studentId: string,
): Promise<ChildTodayStatus> {
  // Pfad und Fehlerbehandlung analog zu getChildFeatures im selben Modul.
}
```

Der Bearbeiter legt zusätzlich die Next.js-Proxy-Route unter `frontend/src/app/api/parent/me/children/[studentId]/today/route.ts` an, gebaut mit `createGetHandler` aus `~/lib/parent/route-wrapper.server`, exakt wie die vorhandene Route für `features`.

- [ ] **Schritt 4: Komponente implementieren**

`child-today-status.tsx` rendert **zwei Ebenen**, wie in Spezifikation
Abschnitt 6 festgelegt:

```
┌────────────────────────────────┐
│  ●  In der OGS                 │   Ebene 1: aus at_ogs, 20 px, halbfett
│     Seit 12:38 Uhr da          │   Ebene 2: aus state, 15 px, grau
└────────────────────────────────┘
```

**Ebene 1** kommt ausschließlich aus `at_ogs` und kennt genau drei Ausprägungen:
`true` → "In der OGS", `false` → "Nicht in der OGS", `null` → Ebene 1 entfällt
ersatzlos, dann steht nur die Ebene-2-Zeile da. Das Frontend leitet die
Ja/Nein-Aussage **nicht** selbst aus `state` ab.

**Ebene 2** kommt aus `state` und trägt die Zeitangaben.

Farbe folgt Ebene 1, nicht Ebene 2, damit ein Blick auf die Farbe dieselbe
Aussage macht wie ein Blick auf den Text. Ausschließlich `moto-*`-Utilities:

| `at_ogs` | Farbe | Icon |
|---|---|---|
| `true` | grün | `CheckCircle2` |
| `false` | grau | `Home` |
| `null` | grau | `HelpCircle` |

Einzige Ausnahme: bei `state === "absent"` färbt sich das Icon-Feld rot, weil
eine Krankmeldung eine andere Aussage ist als "heute schon zu Hause". Ebene 1
bleibt dabei "Nicht in der OGS".

Farbe trägt nie allein die Information: jede Ausprägung hat Icon **und** Text.

- [ ] **Schritt 5: Übersetzungen ergänzen**

In allen vier Katalogen unter `parentChildDetail.todayStatus` je sieben Einträge. Deutsch:

```json
"todayStatus": {
  "atOgsYes": "In der OGS",
  "atOgsNo": "Nicht in der OGS",
  "present": "Seit {time} Uhr da",
  "left": "Um {time} Uhr nach Hause gegangen",
  "expected": "Kommt heute um {time} Uhr",
  "notArrived": "Wird seit {time} Uhr erwartet",
  "absent": "Heute abgemeldet",
  "noCare": "Heute keine Betreuung",
  "unknown": "Status derzeit nicht verfügbar"
}
```

Die ersten beiden Schlüssel sind Ebene 1, die übrigen Ebene 2. Sprachliche
Vorgabe: OGS-Sprache, so wie Eltern und Team miteinander reden. Nicht
"Anwesenheitsstatus", nicht "eingecheckt", nicht "Betreuungszeitraum aktiv".

Die Katalogschlüssel müssen in allen vier Sprachen identisch sein, `pnpm run check` führt `verify-locales` aus und schlägt bei Abweichungen fehl.

- [ ] **Schritt 6: In das Kinderprofil einbinden**

In `frontend/src/components/parent/child-detail.tsx` ersetzt `<ChildTodayStatus>` die erste Zeile des Abschnitts "Heute", oberhalb der bestehenden Zeilen für Abmeldung und Abholung. Die Komponente lauscht auf das bereits vorhandene Fensterereignis `parent-conversation-refresh` mit passender `studentId` und lädt dann neu, damit sie auf `parent_child_updated` reagiert.

- [ ] **Schritt 7: Tests und Qualitätslauf**

```bash
cd frontend && pnpm vitest run src/components/parent/
cd frontend && pnpm run check
```

Erwartet: PASS, null Warnungen.

- [ ] **Schritt 8: Vorher/Nachher-Aufnahmen erstellen**

Nach der `ui-before-after`-Skill je eine Aufnahme des Kinderprofils in 390×844, 834×1194 und 1440×900 gegen `origin/development` und gegen diesen Branch. Die lokalen Pfade am Ende nennen, damit sie manuell an die PR gehängt werden können.

- [ ] **Schritt 9: Committen**

```bash
git add frontend/src/
git commit -m "feat: zeige den Tagesstatus im Kinderprofil"
```

---

## Selbstprüfung

**Abdeckung gegen #2252:**

| Kriterium aus dem Issue | Aufgabe |
|---|---|
| Parent-API stellt reduzierten Tagesstatus bereit | 2, 3 |
| Ableitung aus Anwesenheit und geplanten Abwesenheiten, Europe/Berlin | 1, 2 |
| Aktive Anwesenheit gilt unabhängig vom Raum | 1, 2 |
| Nach letztem Checkout gilt abgemeldet | 1 |
| Krankmeldung wird von "noch nicht angekommen" unterschieden | 1 |
| Ohne belastbare Daten neutraler unbekannter Zustand | 1, 2 |
| Keine Räume, Historie, Rohereignisse, Mitarbeitendennamen | 3 (Formpinnung) |
| Guardian-Verknüpfung mit Portalzugriff erforderlich | 2, 3 |
| Aktualisierung über die Eltern-Echtzeitverbindung | 4, 5 |
| Vollständige Elternansicht zeigt den Status bereits | 5 |
| Tests über alle Zustände, Tageswechsel, Fehlerfall, Berechtigung, Aktualisierung | 1, 2, 3, 4, 5 |

**Abweichung von der Spezifikation:** Abschnitt 6 der Spezifikation nennt in der Endpunktform zusätzlich `pickup_today` und `pickup_changed`. Diese beiden Felder gehören nicht zu #2252 und werden heute bereits im Frontend aus den vorhandenen Betreuungsdaten abgeleitet (`resolveTodayPickup` in `child-care.tsx`). Sie bleiben dort, um keine zweite Ableitung derselben Information zu schaffen. Die Spezifikation wird entsprechend nachgezogen.

**Offene Präzisierung für den Bearbeiter:** In Aufgabe 2 Schritt 3 und Aufgabe 3 Schritt 3 sind zwei Namen aus dem Bestand zu übernehmen statt zu erfinden: der Zugriff auf das Attendance-Repository in der Repository-Factory und der vorhandene Helfer zum Lesen der Pfad-ID `studentId`. Beide existieren; der Plan nennt bewusst die Fundstelle statt eines geratenen Namens.
