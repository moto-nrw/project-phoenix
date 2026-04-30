# Iteration 3 — Arrival Schedules, Drei-Schichten-Architektur & Industrievergleich

Ergebnis aus der Architektur-Session vom 14.04.2026. Themen: Ankunftszeiten, Point-in-Time vs. Zeitspannen, Industrie-Research (6 Open-Source-SIS, 15+ kommerzielle Produkte, iCal/Google Calendar Patterns).

---

## Entscheidungsprotokoll

### E11: Arrival Schedules statt `class_timetable`

**Vorher (RFC + Iteration 2):** `schedule.class_timetable` mit `school_end_time`, `periods`, `has_randstunde`, `expected_ogs_arrival` pro `education_group_id` pro Wochentag.

**Problem:**
1. Wir tracken keine Schulzeiten — nur wann die Betreuung beginnt
2. `education.groups` sind OGS-Betreuungsgruppen, KEINE Schulklassen
3. `students.school_class` ist ein String-Feld ("3a", "4b"), keine Entität mit FK
4. Eine neue `school_classes`-Entität wäre unnötiger Aufwand

**Nachher:** Arrival Schedules als **exaktes Mirror des Pickup-Schedule-Systems**:

| Pickup (existiert) | Arrival (neu) |
|---|---|
| `schedule.student_pickup_schedules` | `schedule.student_arrival_schedules` |
| `schedule.student_pickup_exceptions` | `schedule.student_arrival_exceptions` |
| `pickup_time TIME` | `expected_arrival TIME` |
| Pro Student, pro Wochentag | Pro Student, pro Wochentag |

**Begründung:**
- Bewährtes Pattern, bewiesene Code-Architektur (Model → Repo → Service → API → Frontend)
- Kein neues Entity nötig, keine Klassenverwaltung
- Per-Student-Granularität ermöglicht individuelle Ausnahmen (Förderunterricht, Therapie)
- Bulk-Endpoint löst das Dateneingabe-Problem (alle Kinder einer Klasse auf einmal setzen)

**Impact:**
- `schedule.class_timetable` entfällt komplett
- `schedule.class_timetable_exceptions` entfällt komplett
- Alle Referenzen auf `education_group_id` im Schulplan-Kontext entfallen
- RFC Sektion 5.3 wird durch dieses Dokument ersetzt

---

### E12: Point-in-Time vs. Zeitspanne — Saubere Trennung

**Erkenntnis:** Es gibt zwei fundamental verschiedene Konzepte im Tagesablauf eines Kindes:

| Konzept | Natur | Modell | `end_time`? | Lifecycle? |
|---------|-------|--------|-------------|------------|
| Ankunft 12:50 | Meilenstein (Punkt) | `student_arrival_schedules` | Nein | Nein |
| Mensa 12:00-12:45 | Aktivität (Spanne) | `activity_instances` | Ja, NOT NULL | planned→active→completed |
| Lernzeit 13:45-14:30 | Aktivität (Spanne) | `activity_instances` | Ja, NOT NULL | planned→active→completed |
| Abholung 15:30 | Meilenstein (Punkt) | `student_pickup_schedules` | Nein | Nein |

**Entscheidung:** `activity_instances.end_time` bleibt `NOT NULL`. Meilensteine werden NICHT als Activity Instances modelliert. Sie haben keinen Status-Lifecycle, keinen Supervisor, kein Check-in. Sie sind eigenständige Domain-Konzepte in eigenen Tabellen.

**Impact:** Keine Änderung am `activity_instances`-Schema nötig. Die Trennung ist korrekt wie designed.

---

### E13: Drei unabhängige Datenschichten

**Architektur-Entscheidung:** Arrival, Timetable und Pickup sind drei unabhängige Systeme, die nur auf der Lese-Seite zusammenkommen.

```
ARRIVAL                    TIMETABLE                   PICKUP
(Wann kommt das Kind?)     (Was macht das Kind?)       (Wann geht das Kind?)

student_arrival_schedules  activity_instances           student_pickup_schedules
student_arrival_exceptions instance_staff               student_pickup_exceptions
                           instance_students

Per Student, per Wochentag  Per Aktivität, per Tag      Per Student, per Wochentag
Point-in-Time              Zeitspanne                   Point-in-Time
Kein Lifecycle             planned→active→completed     Kein Lifecycle
Kein FK zueinander         Kein FK zu Arrival/Pickup    Kein FK zueinander
```

**Begründung:**
- Verschiedene Lifecycles (Meilensteine vs. Aktivitäten)
- Verschiedene Granularität (pro Kind vs. pro Aktivität)
- Verschiedene Owner (Büro pflegt Arrival/Pickup, Betreuer/Admin pflegen Timetable)
- Unabhängig deploybar (Arrival jetzt, Timetable später)

**Integration Points (alle read-side):**

1. **Tagesablauf-View (Kind)** — aggregiert aus drei Quellen:
   ```
   12:50  ○ Erwartete Ankunft         ← arrival_schedules
   13:00  ━━━━ Mensa ━━━━━━ 13:45    ← activity_instance
   13:45  ━━━━ Lernzeit ━━━━ 14:30   ← activity_instance
   14:30  ━━━━ Freispiel ━━━ 15:15   ← activity_instance
   15:30  ○ Abholung                  ← pickup_schedules
   ```

2. **Conflict-Warnings bei Exception-Write**: Wenn eine Arrival Exception geschrieben wird, prüft der Service ob Aktivitäten betroffen sind → Soft-Warning im Response.

3. **Betreuer-Ansicht**: Zeigt pro Aktivität an, welche Kinder verspätet erwartet werden (aus Arrival Exception/Schedule abgeleitet).

---

### E14: Betreuungsvertrag = Arrival + Pickup (implizit)

**Branchenkontext:** Deutsche OGS-Software (OGS-Connect, Ganztagsplaner, PEDAV) nutzt den "Betreuungsvertrag" als zentrale Entität — "An welchen Tagen und Zeiten ist dieses Kind in der OGS?"

**Entscheidung:** Kein separates Betreuungsvertrag-Entity. Arrival + Pickup Schedules definieren das Betreuungsfenster implizit:

```
Arrival: Max kommt montags um 12:50
Pickup:  Max wird montags um 15:30 abgeholt
→ Betreuungsfenster Montag: 12:50 - 15:30
```

**"Ist Max heute erwartet?"** = Hat Max einen Arrival-Eintrag für diesen Wochentag UND keine cancellation Exception?

**Begründung:** Ein separates Entity wäre eine dritte Datenquelle, die mit Arrival + Pickup konsistent gehalten werden müsste. Die implizite Ableitung aus zwei bereits existierenden Quellen ist einfacher und weniger fehleranfällig.

---

## Arrival Schedules — Datenmodell

### `schedule.student_arrival_schedules`

Wöchentlich wiederkehrende erwartete Ankunftszeit pro Schüler.

```sql
CREATE TABLE schedule.student_arrival_schedules (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
    student_id        BIGINT NOT NULL REFERENCES users.students(id),
    weekday           SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 5),
    -- ISO 8601: 1=Mo, 5=Fr

    expected_arrival  TIME NOT NULL,
    -- "Wann wird dieses Kind in der OGS erwartet?"

    notes             VARCHAR(500),
    created_by        BIGINT NOT NULL REFERENCES users.staff(id),

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, student_id, weekday)
);

CREATE INDEX idx_arrival_schedules_student_id
    ON schedule.student_arrival_schedules(student_id);

ALTER TABLE schedule.student_arrival_schedules ENABLE ROW LEVEL SECURITY;
```

### `schedule.student_arrival_exceptions`

Datums-spezifische Abweichungen (Wandertag, Hitzefrei, Ausflug, etc.).

```sql
CREATE TABLE schedule.student_arrival_exceptions (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
    student_id        BIGINT NOT NULL REFERENCES users.students(id),
    exception_date    DATE NOT NULL,

    expected_arrival  TIME,
    -- Geänderte Ankunftszeit. NULL = Kind kommt heute nicht (cancelled/kein Unterricht)

    reason            VARCHAR(255),
    created_by        BIGINT NOT NULL REFERENCES users.staff(id),

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, student_id, exception_date)
);

CREATE INDEX idx_arrival_exceptions_student_id
    ON schedule.student_arrival_exceptions(student_id);
CREATE INDEX idx_arrival_exceptions_date
    ON schedule.student_arrival_exceptions(exception_date);

ALTER TABLE schedule.student_arrival_exceptions ENABLE ROW LEVEL SECURITY;
```

### Bulk-Endpoint für klassenbasierte Eingabe

Da `students.school_class` ein String-Feld ist, nutzen wir es als Filter für Bulk-Operationen:

```
POST /api/students/arrival-schedules/bulk
{
  "school_class": "3a",
  "schedules": [
    { "weekday": 1, "expected_arrival": "12:50" },
    { "weekday": 2, "expected_arrival": "12:50" },
    { "weekday": 3, "expected_arrival": "11:45" },
    { "weekday": 4, "expected_arrival": "12:50" },
    { "weekday": 5, "expected_arrival": "11:45" }
  ]
}
```

→ Upsert für alle Schüler mit `school_class = '3a'` im aktuellen Tenant.

Klassen-Dropdown im UI wird populated aus:
```sql
SELECT DISTINCT school_class FROM users.students
WHERE tenant_id = ? ORDER BY school_class;
```

### Resolution-Logik

Identisch zum Pickup-Pattern:

```
1. Ist heute Wochenende? → Keine Ankunftszeit
2. Gibt es eine Exception für heute?
   - Ja, expected_arrival IS NOT NULL → Exception-Zeit verwenden
   - Ja, expected_arrival IS NULL → Kind kommt heute nicht
3. Gibt es einen Wochenplan-Eintrag für diesen Wochentag?
   - Ja → Schedule-Zeit verwenden
4. Kein Eintrag → Kind hat keine geplante Ankunftszeit
```

---

## Arrival ↔ Timetable: Integration Points

### Beim Schreiben einer Arrival Exception

Der Service prüft, ob Activity Instances betroffen sind:

```go
// Im ArrivalExceptionService
func (s *service) CreateException(ctx context.Context, exception *ArrivalException) (*ArrivalException, []Warning, error) {
    // 1. Exception speichern
    if err := s.repo.Create(ctx, exception); err != nil {
        return nil, nil, err
    }

    // 2. Betroffene Instances finden (optional, read-side)
    if exception.ExpectedArrival != nil {
        instances, _ := s.instanceRepo.FindByStudentAndDate(ctx, exception.StudentID, exception.ExceptionDate)
        for _, inst := range instances {
            if inst.StartTime.Before(*exception.ExpectedArrival) {
                warnings = append(warnings, Warning{
                    Type:          "late_for_activity",
                    InstanceTitle: inst.Title,
                    InstanceStart: inst.StartTime,
                    MinutesLate:   int(exception.ExpectedArrival.Sub(inst.StartTime).Minutes()),
                })
            }
        }
    }

    return exception, warnings, nil
}
```

**Kein Write-Coupling:** Die Warning ist informativ. Die Exception wird immer gespeichert. Keine FKs zwischen den Systemen.

### In der Betreuer-Ansicht (Activity Instance Students)

```json
GET /api/timetable/instances/{id}/students

{
  "total_expected": 15,
  "present": 12,
  "late_expected": 1,
  "missing": 2,
  "students": [
    { "id": 1, "name": "Anna M.", "status": "present", "checked_in_at": "13:47" },
    { "id": 7, "name": "Max W.", "status": "expected_late", "expected_arrival": "12:00",
      "note": "Schulausflug" },
    { "id": 12, "name": "Lena T.", "status": "expected" }
  ]
}
```

`expected_late` wird zur Query-Zeit berechnet: Student ist in `instance_students` mit `status=expected`, ABER hat eine Arrival Exception/Schedule die nach `instance.start_time` liegt.

---

## Industrie-Research: Zusammenfassung

### Untersuchte Systeme

| System | Typ | Relevanz |
|--------|-----|----------|
| **Gibbon** (PHP, Open Source) | Schul-SIS, 9 Timetable-Tabellen | Höchste — Dual-Track (Stundenplan + Activities) |
| **OpenSIS** (PHP, Open Source) | Schul-SIS, US High School | FIXED/VARIABLE/BLOCKED Schedule-Typen |
| **Frappe Education** (Python, Open Source) | Schul-SIS, flat model | Jede Occurrence ist eine DB-Row |
| **OpenEduCat** (Python/Odoo, Open Source) | Schul-SIS, Session-basiert | Register → Sheet → Line Attendance-Hierarchie |
| **Fedena** (Ruby, Open Source) | Schul-SIS, minimal | Pattern-only, keine Instances |
| **SchoolTool** (Python/Zope, Open Source) | Schul-SIS, Object-DB | Elegantestes Exception-Modell (Ganztags-Ersetzung) |
| **Amilia** (REST API, kommerziell) | Kinder-/Freizeitprogramme | Activity → Occurrence → Attendance, dem RFC am nächsten |
| **Famly** (GraphQL, kommerziell) | Kita/Daycare | ContractedHours vs. AttendedMinutes Reconciliation |
| **OGS-Connect** (kommerziell, DE) | OGS-Software | Betreuungsvertrag als Fundament |
| **Ganztagsplaner** (kommerziell, DE) | OGS-Software | Individualisierte Tagespläne |
| **Aurora Ganztag** (kommerziell, DE) | OGS-Software | Auto-Kurszuweisung + Konflikterkennung |

### Branchenübergreifende Erkenntnisse

**1. Alle Systeme sind activity-centric, nicht student-centric.**
Studenten verbinden sich über Enrollment/Junction Tables mit Events. Keines speichert einen persönlichen Stundenplan pro Student — er wird immer abgeleitet.

**2. Template→Instance ist der Industriestandard für Attendance-Systeme.**
Reine Kalender (Google, iCal) berechnen Occurrences zur Laufzeit. Attendance-Systeme materialisieren, weil sie pro Occurrence Daten speichern müssen (Anwesenheit, Staff-Zuordnung, Status). Amilia macht genau das.

**3. "Expected Attendance" wird nie gespeichert.**
Gibbon, OpenSIS, Fedena berechnen Expected Attendance zur Query-Zeit aus Enrollment + Schedule. Nur Actual Attendance wird persistiert. Ausnahme: Dein RFC speichert `instance_students.attendance_status` — was eine bewusste Denormalisierung für Performance ist (E4). Amilia macht es genauso.

**4. iCal RFC 5545 bestätigt dein Exception-Pattern.**
EXDATE = Cancellation, RECURRENCE-ID = Modified Occurrence. Deine `activity_exceptions` und `arrival_exceptions` folgen exakt diesem Muster.

**5. Deutsche OGS-Software nutzt den Betreuungsvertrag als Fundament.**
→ Gelöst durch Arrival + Pickup Schedules (E14).

### RFC-Validierung gegen Industrie

| RFC-Aspekt | Industriestandard | Bewertung |
|---|---|---|
| Event-centric + Junction Tables | Alle 6 Open-Source-Systeme | Korrekt |
| Bounded Materialisierung | Amilia, Google Calendar | Korrekt |
| Template + Exception Pattern | iCal RFC 5545, OGS-Connect | Korrekt |
| Application Code Attendance Sync (E4) | OpenEduCat, Amilia | Korrekt |
| `is_spontaneous` Flag (E1, Iteration 1) | Amilia: Drop-In mit `DropInOccurrenceId` | Korrekt |
| Multi-Room Override (E3) | Kein System so sauber | Besser als Industrie |
| ConflictService Soft-Warnings (E7) | Aurora Ganztag | Korrekt |
| Live-Layer Bridge (`active.groups`) | Kein vergleichbares System | Einzigartig |

### Referenz-Repos

- **Gibbon**: `github.com/GibbonEdu/core` — PHP/MySQL, ausgereiftestes Timetable-Schema
- **Amilia**: REST API Docs — dem RFC-Design am nächsten (Activity → Occurrence → Attendance)
- **teambition/rrule-go**: Go RRULE-Library falls später RRULE-Support benötigt wird

---

## Aktualisierte Phasenplanung

```
Phase 0 (jetzt):     Arrival Schedules
                      → Pickup-Mirror Pattern
                      → Bulk-Endpoint für Klasseneingabe
                      → Eigenständig, keine Timetable-Abhängigkeit

Phase 1 (danach):     Activity Instances + Materialisierung
                      → Kern des Timetable-Systems
                      → active.groups Bridge

Phase 2 (parallel):   Tagesablauf-Aggregation
                      → API: Student Day = Arrival + Instances + Pickup
                      → Conflict-Warnings bei Exception-Writes

Phase 3 (später):     Betreuer-Ansichten + Admin-Wochenplaner
                      → "Mein Tag" View
                      → Spontane Aktivitäten
                      → Vertretungs-Management
```

**Phase 0 ist unabhängig von allen anderen Phasen** und kann sofort implementiert werden.

---

## Entfernte Konzepte (vs. Original-RFC)

| Konzept | Status | Ersetzt durch |
|---------|--------|---------------|
| `schedule.class_timetable` | Entfernt | `schedule.student_arrival_schedules` |
| `schedule.class_timetable_exceptions` | Entfernt | `schedule.student_arrival_exceptions` |
| `timetable.mode` Setting (flexible/planned/hybrid) | Entfernt (Iteration 2) | Ein System, keine Modi |
| `timetable.allow_spontaneous_activities` Setting | Entfernt (Iteration 2) | Immer erlaubt |
| `education_group_id` auf `class_timetable` | Entfernt | `student_id` direkt |
| `school_end_time`, `periods`, `has_randstunde` | Entfernt | Nicht benötigt — wir tracken keine Schulzeiten |

---

## Verbleibende offene Fragen

Aus Iteration 2 übernommen + neue:

- [ ] **Spontanbesucher:** Kind checkt in eine Instance ein, ist aber nicht in `instance_students`. Neuen Eintrag mit `status=present` anlegen? Oder nur in `active.visits` erfassen?
- [ ] **Vertretung Detail:** Original-Eintrag in `instance_staff` löschen oder als "absent" markieren?
- [ ] **"Woche neu planen" UI:** Confirmation-Dialog mit Diff? Oder einfacher "Sind Sie sicher?" Dialog?
- [ ] **Mensa als rollender Checkpoint:** Passt das Block-Modell (start_time/end_time) mit pro-Gruppe-Aufteilung? Oder braucht Mensa einen eigenen Instance-Typ?
- [ ] **Arrival Bulk: Überschreiben oder Mergen?** Wenn Bulk für "3a" ausgeführt wird und Max bereits individuelle Zeiten hat — überschreiben oder warnen?
- [ ] **Arrival Notes:** Brauchen wir `student_arrival_notes` analog zu `student_pickup_notes`? Oder reicht das `reason`-Feld auf der Exception?
