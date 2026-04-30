# Iteration 4 — Team-Feedback: Kalenderperioden, A/B-Wochen, Enrollment-Gültigkeit, Attendance-Modell

Einarbeitung des Team-Feedbacks von Christian und Flo (Slack-Thread 14./15.04.2026). Fokus: maximale Daten-/Backend-Flexibilität bei minimaler initialer UI-Komplexität.

---

## Feedback-Zusammenfassung

| # | Feedback | Von | Entscheidung |
|---|----------|-----|-------------|
| F1 | A/B-Wochen (alternierende Wochenpläne) | Christian + Flo | `week_pattern` auf Templates + Kalenderperioden |
| F2 | Ferienbetreuung (Roadmap-Item) | Christian + Flo | Schema vorbereitet, Implementierung deferred |
| F3 | Mensa-Rotation (statisch vs. variabel) | Christian | Bereits durch Templates + A/B-Wochen gelöst |
| F4 | Attendance-Status zu grob | Flo | Drei-Feld-Modell: status + substatus + note |
| F5 | Auto-Start / proaktive Vorschläge | Flo | Drei Stufen: Passiv (UI), Aktiv (SSE), Automatisch (Scheduler) |
| F6 | Vertretungs-Erkennung (fehlende Betreuer) | Flo | Gap-Detection Query (MVP) + Staff-Absences (V2) |
| F7 | Enrollments zeitlos (kein Gültigkeitszeitraum) | Flo | `valid_from`/`valid_until` auf Enrollments + Supervisors |

---

## Entscheidungsprotokoll

### E15: Kalenderperioden — Schema jetzt, UI später

**Problem:** Mehrere Feedback-Punkte konvergieren auf eine temporale Strukturierung:
- A/B-Wochen brauchen einen Ankerpunkt ("Wann beginnt Woche A?")
- Ferienbetreuung hat eigene Zeiträume mit eigenen Regeln
- Halbjahreswechsel erfordern Enrollment-Rollover

Ohne Perioden-Konzept leben A/B-Wochen-Settings global am Tenant — was bei Ferienbetreuung (keine Alternierung) bricht.

**Entscheidung:** `schedule.calendar_periods` wird jetzt als Tabelle angelegt, aber **nirgendwo als NOT NULL FK erzwungen**. Für den MVP auto-generiert das System eine Default-Periode pro Tenant. Kein Admin-UI nötig bis Ferienbetreuung implementiert wird.

**Warum nicht deferred?**
- Die Tabelle selbst ist trivial (1 Migration, 1 Model, 0 Impact auf bestehenden Code)
- A/B-Wochen-Config gehört auf die Periode, nicht auf den Tenant — sonst bricht es bei Ferienbetreuung
- Nachträgliches Einführen erfordert Backfill-Migration + FK-Nachrüstung auf allen abhängigen Tabellen
- Kosten jetzt: 1 Tabelle + 1 nullable FK auf 2-3 Tabellen. Kosten später: Schema-Migration unter Last

**Perioden-Typen:**

| `period_type` | Beispiel | A/B-Wochen? | Typische Dauer |
|---------------|----------|-------------|----------------|
| `school_year` | "Schuljahr 2026/27" | Ja, wenn Schule das nutzt | ~10 Monate |
| `semester` | "1. Halbjahr 2026/27" | Ja | ~5 Monate |
| `holiday` | "Herbstferien 2026" | Nein (`week_cycle_length=1`) | 1-6 Wochen |
| `custom` | "Projektwoche Mai" | Nein | Variabel |

**Default-Periode (auto-created):**
Beim ersten Zugriff auf Timetable-Features prüft der Service, ob eine aktive Periode existiert. Falls nicht, wird automatisch eine erstellt:

```
Name:               "Schuljahr {current_year}/{next_year}"
period_type:        "school_year"
start_date:         1. August des aktuellen Jahres
end_date:           31. Juli des nächsten Jahres
week_cycle_length:  1 (kein A/B)
is_active:          true
```

Kein Admin muss etwas konfigurieren. Das System funktioniert out-of-the-box mit einer Einzel-Periode.

---

### E16: A/B-Wochen über `week_pattern` + Perioden-Cycle

**Problem:** Manche Schulen haben alternierende Wochenpläne. AG Fußball ist montags in A-Wochen, AG Musik ist montags in B-Wochen.

**Evaluierte Optionen:**

| Option | Ansatz | Bewertung |
|--------|--------|-----------|
| A | `week_pattern SMALLINT` auf Templates + Modulo-Berechnung | Pragmatisch, deckt A/B ab |
| B | FK auf `schedule.recurrence_rules` (iCal-style RRULE) | Mächtig, aber Overkill für biweekly |
| C | Separate Templates für A/B mit verschiedenen `valid_from`-Ranges | Kein Schema-Änderung, aber Template-Bloat |

**Entscheidung: Option A** — `week_pattern` auf `activities.schedules`.

**Feld-Semantik:**
- `0` = jede Woche (Default)
- `1` = Woche A
- `2` = Woche B
- `3`+ = theoretisch erweiterbar, praktisch irrelevant

**Cycle-Config auf der Periode:**
- `week_cycle_length SMALLINT DEFAULT 1` — 1 = keine Alternierung, 2 = A/B
- `week_cycle_anchor DATE` — "Dieses Datum ist Woche A (pattern=1)"

**Materialisierungs-Logik:**

```go
func shouldMaterialize(schedule Schedule, instanceDate time.Time, period CalendarPeriod) bool {
    if schedule.WeekPattern == 0 {
        return true // Jede Woche
    }
    if period.WeekCycleLength <= 1 {
        return true // Keine Alternierung in dieser Periode
    }

    // Tagesbasierte Differenz statt ISO-Wochennummern.
    // ISO-Wochen wrappen am Jahreswechsel (Woche 52 → Woche 1),
    // was die Modulo-Berechnung brechen würde.
    daysDiff := int(instanceDate.Sub(period.WeekCycleAnchor).Hours() / 24)
    weeksDiff := daysDiff / 7

    // Doppel-Modulo für korrekte Behandlung negativer Differenzen
    // (Anchor-Datum liegt nach Instance-Datum)
    currentPattern := ((weeksDiff%period.WeekCycleLength)+period.WeekCycleLength)%period.WeekCycleLength + 1
    return currentPattern == schedule.WeekPattern
}
```

**Impact auf bestehendes Schema:**
- `activities.schedules` bekommt `week_pattern SMALLINT NOT NULL DEFAULT 0`
- `activities.schedules` bekommt `calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id)` (nullable)
- Bestehende Schedules bekommen automatisch `week_pattern=0` (jede Woche) — kein Migrationsproblem

---

### E17: Enrollment-Gültigkeit via `valid_from`/`valid_until`

**Problem (Flo):** Template-Enrollments haben keine zeitliche Gültigkeit. Beim Halbjahreswechsel ändern sich AG-Zuordnungen. Ohne Zeitraum: alte Enrollments löschen (Historie weg) oder Stale-Daten.

**Evaluierte Optionen:**

| Option | Ansatz | Bewertung |
|--------|--------|-----------|
| A | `valid_from DATE, valid_until DATE` direkt auf Enrollments | Einfach, flexibel, Historie erhalten |
| B | Enrollments per `calendar_period_id` FK scoped | Sauber konzeptionell, aber tight coupling |
| C | Soft-Delete mit `replaced_at` | Historie da, aber Query-Komplexität |
| D | Semester-Entity mit Rollover | Explizit, aber aufwändig |

**Entscheidung: Option A** — Validity Dates direkt auf Enrollments und Supervisors.

**Schema-Änderungen:**

`activities.student_enrollments`:
- `enrollment_date` wird zu `valid_from DATE NOT NULL DEFAULT CURRENT_DATE` (Rename, gleiche Semantik)
- Neues Feld: `valid_until DATE` (nullable — NULL = unbefristet)
- Neues Feld: `calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id)` (nullable)

`activities.supervisors`:
- Neues Feld: `valid_from DATE NOT NULL DEFAULT CURRENT_DATE`
- Neues Feld: `valid_until DATE` (nullable)
- Neues Feld: `calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id)` (nullable)

**UNIQUE Constraint Update:**
Die bestehende `UNIQUE(student_id, activity_group_id)` muss angepasst werden, da ein Student in verschiedenen Halbjahren in der gleichen AG sein kann:

```sql
-- Vorher:
UNIQUE (student_id, activity_group_id)

-- Nachher: Partial Index — nur aktive Enrollments müssen unique sein
CREATE UNIQUE INDEX idx_enrollment_active_unique
    ON activities.student_enrollments (student_id, activity_group_id)
    WHERE valid_until IS NULL OR valid_until >= CURRENT_DATE;
```

**Materialisierungs-Filter:**

```sql
SELECT se.* FROM activities.student_enrollments se
WHERE se.activity_group_id = $1
  AND se.valid_from <= $2  -- instance_date
  AND (se.valid_until IS NULL OR se.valid_until >= $2)
```

**Halbjahreswechsel-Workflow:**

1. Admin öffnet "Halbjahreswechsel" Dialog
2. System zeigt alle aktiven Enrollments mit `valid_until IS NULL`
3. Admin wählt:
   - **Übernehmen**: Enrollments werden in neue Periode kopiert (neues `valid_from`, optional neue `calendar_period_id`). Alte Enrollments bekommen `valid_until = letzer Tag des alten Halbjahres`.
   - **Neu starten**: Alte Enrollments bekommen `valid_until`. Admin pflegt neue Zuordnungen manuell.
4. Historie bleibt vollständig erhalten — alte Enrollments existieren als abgelaufene Einträge weiter.

**Bulk-Endpoint:**

```
POST /api/activities/enrollments/semester-rollover
{
  "close_date": "2027-01-31",           // valid_until für bestehende
  "new_valid_from": "2027-02-01",       // valid_from für neue
  "calendar_period_id": 5,              // Optional: neue Periode
  "groups": [
    {
      "group_id": 42,
      "action": "carry_over",           // carry_over | close_only
      "exclude_student_ids": [7, 12]    // Schüler die nicht übernommen werden
    }
  ]
}
```

---

### E18: Drei-Feld Attendance-Modell (status + substatus + note)

**Problem (Flo):** `attendance_status ENUM('expected','present','absent','excused')` ist zu grob. Fehlt: `late_expected` (SDUI hat das), Freitext für Kontext.

**Evaluierte Optionen:**

| Option | Ansatz | Bewertung |
|--------|--------|-----------|
| A | Breiteres Enum (8+ Werte) | Jeder neue Fall = Migration |
| B | Core-Status + Reason-Feld (Text) | Flexibel aber unstrukturiert |
| C | Core-Status + Substatus-Enum + Note | Stabile State-Machine + strukturierte Gründe + Freitext |
| D | Voll flexibel (Status als String, tenant-konfigurierbar) | Maximale Flex, minimale Sicherheit |

**Entscheidung: Option C** — Drei-Feld-Modell.

**Architektur:**

```
┌──────────────────────────────────────────────────────────────────┐
│  status (Core)           │  substatus (Kontext)  │  note (Text) │
│  System-gesteuert        │  Mensch-gesteuert     │  Freitext    │
│  State-Machine           │  Nullable Enum        │  Nullable    │
│  3 Werte, deterministisch│  Erweiterbar          │  Max 500     │
├──────────────────────────┼───────────────────────┼──────────────┤
│  expected                │  NULL (normal)        │              │
│  expected                │  late                 │  "Förderunt. │
│  present                 │  NULL (pünktlich)     │   bis 14:00" │
│  present                 │  late                 │              │
│  absent                  │  NULL (unentschuldigt)│              │
│  absent                  │  excused              │  "Arzttermin"│
│  absent                  │  sick                 │              │
│  absent                  │  field_trip           │  "Wandertag" │
└──────────────────────────┴───────────────────────┴──────────────┘
```

**Warum drei Felder statt eins?**

1. **Core-Status ist eine State-Machine** — das System steuert Übergänge:
   - `expected` → `present` (bei Check-in, Application Code, E4)
   - `expected` → `absent` (bei Instance-Ende, wenn nie eingecheckt)
   - Rückwärts-Transitionen möglich (Admin-Override)

2. **Substatus ist menschlicher Kontext** — Betreuer/Admin setzt ihn:
   - Beim Erstellen: `expected` + `late` (Flo's `late_expected`)
   - Beim Nachbearbeiten: `absent` + `excused` (Entschuldigung nachgereicht)
   - Auto-Detection: `present` + `late` (Check-in nach start_time)

3. **Note ist Freitext** — "Mutti holt um 14:30 ab", "Hat Bauchweh", "Nimmt nur an erster Hälfte teil"

**Substatus-Enum (initial, erweiterbar):**

```sql
-- Kein DB-Level ENUM — stattdessen CHECK Constraint für Erweiterbarkeit
CHECK (substatus IS NULL OR substatus IN (
    'late',        -- Verspätet (erwartet oder angekommen)
    'excused',     -- Entschuldigt abwesend
    'sick',        -- Krank gemeldet
    'field_trip',  -- Wandertag/Ausflug
    'other'        -- Sonstiges (Note nutzen für Details)
))
```

Kein DB-Level `CREATE TYPE ... AS ENUM` — stattdessen `TEXT` mit `CHECK` Constraint. Grund: CHECK Constraints sind einfacher zu migrieren (ein `ALTER TABLE ... DROP/ADD CONSTRAINT` statt `ALTER TYPE ... ADD VALUE` + Typ-Management). Außerdem erscheinen CHECK-Werte direkt im Schema — kein separater Typ der gesucht/gepflegt werden muss.

**Auto-Detection von `late`:**

```go
func determineSubstatus(checkinTime time.Time, instanceStartTime time.Time, existingSubstatus *string) *string {
    // Nicht überschreiben wenn schon manuell gesetzt
    if existingSubstatus != nil {
        return existingSubstatus
    }
    if checkinTime.After(instanceStartTime) {
        late := "late"
        return &late
    }
    return nil
}
```

**Impact auf E4 (Attendance Sync bei Check-in):**
Der Check-in Handler (Iteration 2) wird erweitert:

```go
// Schritt 4 aus E4, erweitert:
// UPDATE instance_students SET
//   status = 'present',
//   substatus = CASE WHEN NOW() > instance.start_time THEN 'late' ELSE substatus END,
//   checked_in_at = NOW()
// WHERE instance_id = X AND student_id = Y
```

---

### E19: Auto-Start — Drei Stufen der Proaktivität

**Problem (Flo):** Geplante Instances werden aktuell rein manuell gestartet. Das System sollte proaktiv darauf hinweisen.

**Entscheidung:** Drei unabhängig konfigurierbare Stufen. Jede Stufe baut auf der vorherigen auf, kann aber einzeln aktiviert werden.

**Stufe 1: Passiv (UI-only) — MVP**

Die "Mein Tag"-Ansicht des Betreuers zeigt visuelle Indikatoren:

| Instance-Status | Zeitliche Lage | Darstellung |
|----------------|----------------|-------------|
| `planned` | Startzeit in Zukunft | Normal, mit Countdown |
| `planned` | Startzeit erreicht | **Hervorgehoben**: pulsierender "Jetzt starten"-Button, gelber Rand |
| `planned` | Startzeit > 5min überschritten | **Dringend**: roter Rand, "Überfällig seit X Min" Badge |
| `active` | Laufend | Grüner Rand, Timer zeigt verbleibende Zeit |
| `completed` | Beendet | Ausgegraut |

Kein Backend nötig — rein Frontend-Logik basierend auf `instance.start_time` vs. `Date.now()`.

**Stufe 2: Aktiv (SSE-Events) — Phase 2**

Der Scheduler sendet SSE-Events an zugewiesene Betreuer:

```go
// Neuer Event-Typ im realtime Package
const EventInstanceDue   = "instance_due"
const EventInstanceOverdue = "instance_overdue"

// Scheduler prüft alle 60s:
for _, instance := range getPlannedInstances(now) {
    if instance.StartTime.Before(now) && instance.Status == "planned" {
        // SSE an alle zugewiesenen Betreuer
        hub.BroadcastToStaff(instance.StaffIDs, realtime.NewEvent(
            EventInstanceDue,
            instance.ID,
            realtime.EventData{
                InstanceTitle: &instance.Title,
                StartTime:     &instance.StartTime,
                MinutesOverdue: minutesSince(instance.StartTime),
            },
        ))
    }
}
```

Bestehendes SSE-System (`backend/realtime/`) wird um die neuen Event-Typen erweitert. Keine neue Infrastruktur nötig.

**Stufe 3: Automatisch (Scheduler) — Per Setting**

Das bestehende `timetable.auto_start_planned` Setting (Iteration 2) steuert, ob der Scheduler Instances automatisch startet:

```go
if autoStartEnabled {
    // Auto-create active.group + setze instance.status = 'active'
    activeGroup, err := activeService.StartActivitySession(ctx, ...)
    instance.Status = "active"
    instance.ActiveGroupID = &activeGroup.ID
    instance.StartedAt = &now
}
```

**Settings-Ergänzung:**

| Key | Type | Default | Tab | Beschreibung |
|-----|------|---------|-----|-------------|
| `timetable.auto_start_planned` | boolean | `false` | operations | Geplante Instances automatisch bei Startzeit starten |
| `timetable.overdue_threshold_minutes` | number | `5` (min:1, max:30) | operations | Minuten nach Startzeit bis "Überfällig"-Status (UI + SSE) |

---

### E20: Vertretungs-Erkennung als Gap-Detection

**Problem (Flo):** Wenn ein Betreuer krank ist, soll das System aufzeigen welche Instances keinen Betreuer haben.

**MVP: Query-basierte Gap-Detection** — Keine neue Entität, nur ein neuer API-Endpoint.

```
GET /api/timetable/gaps?date=2026-04-16&date_to=2026-04-20
```

**Response:**

```json
{
  "gaps": [
    {
      "instance_id": 142,
      "date": "2026-04-16",
      "title": "Lernzeit Jg.3",
      "start_time": "13:45",
      "end_time": "14:30",
      "room": "Raum 4",
      "staff_count": 0,
      "minimum_staff": 1
    },
    {
      "instance_id": 145,
      "date": "2026-04-16",
      "title": "AG Fußball",
      "start_time": "14:30",
      "end_time": "15:30",
      "room": "Schulhof",
      "staff_count": 0,
      "minimum_staff": 1
    }
  ],
  "understaffed": [
    {
      "instance_id": 143,
      "date": "2026-04-16",
      "title": "Mensa",
      "start_time": "12:00",
      "end_time": "12:45",
      "room": "Mensa",
      "staff_count": 1,
      "minimum_staff": 2,
      "assigned_staff": [
        { "id": 55, "name": "Franziska" }
      ]
    }
  ]
}
```

**Query (Core):**

```sql
SELECT
    ai.id,
    ai.date,
    ai.title,
    ai.start_time,
    ai.end_time,
    r.name AS room,
    COUNT(ist.id) AS staff_count
FROM schedule.activity_instances ai
LEFT JOIN schedule.instance_staff ist ON ist.instance_id = ai.id
LEFT JOIN facilities.rooms r ON r.id = ai.room_id
WHERE ai.tenant_id = $1
  AND ai.date BETWEEN $2 AND $3
  AND ai.status = 'planned'
GROUP BY ai.id, ai.date, ai.title, ai.start_time, ai.end_time, r.name
HAVING COUNT(ist.id) < 1  -- oder < minimum_staff wenn das Feld existiert
ORDER BY ai.date, ai.start_time;
```

**V2 (nicht im MVP): Staff-Absence Entity**

```sql
-- Später, wenn Vertretungsplan-UI kommt:
CREATE TABLE schedule.staff_absences (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id),
    staff_id      BIGINT NOT NULL REFERENCES users.staff(id),
    date_from     DATE NOT NULL,
    date_to       DATE NOT NULL,
    reason        TEXT,              -- 'sick', 'vacation', 'training', 'other'
    notes         TEXT,
    created_by    BIGINT NOT NULL REFERENCES auth.accounts(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Workflow V2:**
1. Admin erfasst Abwesenheit → System berechnet automatisch betroffene Instances
2. "Vertretungsplan"-View zeigt: Lücken + verfügbare Betreuer + One-Click-Zuordnung (E6)
3. Bei Erfassung: Soft-Warning "Diese 3 Instances morgen haben keinen Betreuer mehr"

**Warum nicht sofort?** Ohne Admin-UI für Abwesenheitserfassung bringt die Tabelle keinen Mehrwert. Die Gap-Detection Query liefert das gleiche Ergebnis, nur manuell angestoßen statt automatisch.

---

### E21: Mensa-Rotation — Bereits gelöst

**Problem (Christian):** Variieren die Klassen-/Gruppenzuteilungen der Mensen?

**Analyse:** In den meisten OGS-Einrichtungen ist die Mensa-Zuteilung statisch (Klasse 1a isst immer um 12:00). Falls sie rotiert:

- **Tägliche Rotation** (Klasse 1a: Mo 12:00, Di 12:30): Verschiedene Templates für verschiedene Wochentage → bereits abgedeckt durch `activities.schedules` mit verschiedenen Weekdays + Timeframes.
- **Wöchentliche Rotation** (A-Woche: 1a zuerst, B-Woche: 3b zuerst): A/B-Wochen (E16) deckt das ab — zwei Templates mit verschiedenen `week_pattern` Werten.

**Entscheidung:** Kein zusätzliches Modell nötig. Mensa-Rotation ist ein Use-Case, der durch die bestehende Template-Architektur + A/B-Wochen vollständig abgedeckt wird.

**Dokumentation:** Mensa-Rotation als explizites Beispiel in die API-Dokumentation aufnehmen, damit Schulen wissen wie sie es konfigurieren.

---

### E22: Ferienbetreuung — Kompatibilitäts-Constraints

**Status:** Auf der Roadmap, aber nicht im MVP für Schule am Berg.

**Was "kompatibel halten" konkret bedeutet:**

| Constraint | Warum | Wie sichergestellt |
|-----------|-------|-------------------|
| Kein NOT NULL FK auf Semester/Schuljahr | Ferienbetreuung hat eigene Perioden | `calendar_period_id` ist nullable überall |
| Weekday nicht auf 1-5 beschränkt | Ferienbetreuung kann Samstag haben | `activities.schedules.weekday` bleibt 1-7 (ISO) |
| Templates nicht an "Schulwochen" gebunden | Ferien-Templates haben andere Zeiten | `week_pattern=0` + Perioden-Scoping |
| Enrollments nicht an Schuljahr gebunden | Ferien-Enrollments gelten nur für Ferien-Zeitraum | `valid_from`/`valid_until` + optional `calendar_period_id` |
| Materialisierung nicht "immer laufen" | Nur aktive Perioden materialisieren | Scheduler filtert auf `is_active=true` Perioden |

**Implementierungs-Skizze (für später):**

```
1. Admin erstellt Periode: "Herbstferien 2026", type="holiday", 14.10.-25.10.
2. Admin erstellt Ferien-spezifische Activity Groups (oder nutzt bestehende)
3. Admin erstellt Schedules mit calendar_period_id → Herbstferien
4. Admin erstellt Enrollments mit valid_from=14.10., valid_until=25.10.
5. Materialisierung: Scheduler prüft aktive Perioden, materialisiert nur für diese
```

Kein einziges dieser Schritte erfordert Schema-Änderungen am hier vorgestellten Datenmodell. Alles, was fehlt, ist UI.

---

## Aktualisiertes Datenmodell

### Neue Tabelle: `schedule.calendar_periods`

```sql
CREATE TABLE schedule.calendar_periods (
    id                    BIGSERIAL PRIMARY KEY,
    tenant_id             BIGINT NOT NULL REFERENCES platform.schools(id),

    name                  TEXT NOT NULL,
    -- "Schuljahr 2026/27", "Herbstferien 2026", "1. Halbjahr 2026/27"

    period_type           TEXT NOT NULL CHECK (period_type IN (
                              'school_year', 'semester', 'holiday', 'custom'
                          )),

    start_date            DATE NOT NULL,
    end_date              DATE NOT NULL CHECK (end_date >= start_date),

    -- A/B-Wochen Konfiguration
    week_cycle_length     SMALLINT NOT NULL DEFAULT 1,
    -- 1 = keine Alternierung, 2 = A/B-Wochen
    week_cycle_anchor     DATE,
    -- Referenzdatum: "Dieses Datum ist Woche A (pattern=1)"
    -- NULL wenn week_cycle_length = 1

    is_active             BOOLEAN NOT NULL DEFAULT false,
    -- Nur aktive Perioden werden materialisiert
    -- Mehrere aktive Perioden erlaubt (z.B. Schuljahr + Projektwoche)

    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, name)
);

CREATE INDEX idx_calendar_periods_tenant_active
    ON schedule.calendar_periods (tenant_id)
    WHERE is_active = true;

ALTER TABLE schedule.calendar_periods ENABLE ROW LEVEL SECURITY;
```

### Geänderte Tabelle: `activities.schedules`

Neue Felder:

```sql
ALTER TABLE activities.schedules
    ADD COLUMN week_pattern SMALLINT NOT NULL DEFAULT 0,
    -- 0 = jede Woche, 1 = Woche A, 2 = Woche B

    ADD COLUMN calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);
    -- Nullable: Wenn gesetzt, gilt dieser Schedule nur in dieser Periode
    -- Wenn NULL: Gilt in allen aktiven Perioden (Abwärtskompatibel)
```

### Geänderte Tabelle: `activities.student_enrollments`

```sql
-- Bestehende Spalte umbenennen:
ALTER TABLE activities.student_enrollments
    RENAME COLUMN enrollment_date TO valid_from;

-- Neue Spalten:
ALTER TABLE activities.student_enrollments
    ADD COLUMN valid_until DATE,
    -- NULL = unbefristet ("bis auf Weiteres")

    ADD COLUMN calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);
    -- Nullable: Optional für Perioden-basiertes Enrollment-Management

-- UNIQUE Constraint aktualisieren:
-- Vorher: UNIQUE(student_id, activity_group_id) — verhindert Re-Enrollment nach Halbjahreswechsel
-- Nachher: Nur unbefristete Enrollments müssen unique sein.
-- Abgeschlossene (mit valid_until) sind Historieneinträge — keine Unique-Prüfung.
-- Überlappende Gültigkeitszeiträume werden auf Application-Layer verhindert.
DROP INDEX IF EXISTS student_enrollments_student_id_activity_group_id_key;

CREATE UNIQUE INDEX idx_enrollment_active_unique
    ON activities.student_enrollments (tenant_id, student_id, activity_group_id)
    WHERE valid_until IS NULL;
```

### Geänderte Tabelle: `activities.supervisors`

```sql
ALTER TABLE activities.supervisors
    ADD COLUMN valid_from DATE NOT NULL DEFAULT CURRENT_DATE,
    ADD COLUMN valid_until DATE,
    ADD COLUMN calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);

-- Analog: UNIQUE Constraint anpassen
-- Vorher: UNIQUE(staff_id, group_id)
-- Nachher: Nur unbefristete Zuordnungen müssen unique sein
DROP INDEX IF EXISTS supervisors_staff_id_group_id_key;

CREATE UNIQUE INDEX idx_supervisor_active_unique
    ON activities.supervisors (tenant_id, staff_id, group_id)
    WHERE valid_until IS NULL;
```

### Geänderte Tabelle: `schedule.instance_students` (aus Iteration 2)

```sql
CREATE TABLE schedule.instance_students (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
    instance_id       BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
    student_id        BIGINT NOT NULL REFERENCES users.students(id),
    room_id           BIGINT REFERENCES facilities.rooms(id),
    -- In welchem Raum ist dieses Kind?
    -- NULL = Primär-Raum der Instance (Standard)

    -- Drei-Feld Attendance-Modell (E18)
    status            TEXT NOT NULL DEFAULT 'expected'
                      CHECK (status IN ('expected', 'present', 'absent')),
    -- Core State Machine: System-gesteuert
    -- expected → present (Check-in)
    -- expected → absent (Instance-Ende, kein Check-in)

    substatus         TEXT CHECK (substatus IS NULL OR substatus IN (
                          'late',        -- Verspätet erwartet/angekommen
                          'excused',     -- Entschuldigt abwesend
                          'sick',        -- Krank gemeldet
                          'field_trip',  -- Wandertag/Ausflug
                          'other'        -- Sonstiges (Note nutzen)
                      )),
    -- Kontext/Grund: Mensch-gesteuert oder Auto-Detection
    -- NULL = kein spezieller Grund

    note              TEXT CHECK (note IS NULL OR length(note) <= 500),
    -- Freitext für alles was nicht in substatus passt

    checked_in_at     TIMESTAMPTZ,
    -- Aktualisiert durch Application Code bei Check-in (E4)

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (instance_id, student_id)
);

ALTER TABLE schedule.instance_students ENABLE ROW LEVEL SECURITY;
```

### Optional hinzugefügt: `calendar_period_id` auf `schedule.activity_instances`

```sql
ALTER TABLE schedule.activity_instances
    ADD COLUMN calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);
    -- Welche Periode hat diese Instance materialisiert?
    -- Nullable: Spontane Instances haben keine Periode
    -- Wird bei Materialisierung automatisch gesetzt
```

---

## Aktualisierte Settings

### Operations Tab — Ergänzungen

| Key | Type | Default | DependsOn | Beschreibung |
|-----|------|---------|-----------|-------------|
| `timetable.materialization_enabled` | boolean | `false` | — | Automatische Wochenplanung aktivieren |
| `timetable.materialization_weekday` | select | `5` (Fr) | materialization_enabled eq true | Wochentag für Materialisierung |
| `timetable.materialization_weeks_ahead` | number | `1` (min:1, max:4) | materialization_enabled eq true | Wochen im Voraus |
| `timetable.auto_start_planned` | boolean | `false` | — | Geplante Instances automatisch starten |
| `timetable.overdue_threshold_minutes` | number | `5` (min:1, max:30) | — | Minuten bis "Überfällig"-Badge |
| `timetable.show_expected_children_count` | boolean | `true` | — | Erwartete Kinderanzahl anzeigen |

### GDPR Tab — Unverändert

| Key | Type | Default | DependsOn | Beschreibung |
|-----|------|---------|-----------|-------------|
| `gdpr.timetable_retention_days` | number | `365` (min:30, max:1825) | data_cleanup_enabled eq true | Aufbewahrungsdauer Timetable-Daten |

**Gesamt: 7 Timetable-Settings** (vorher 6, neu: `overdue_threshold_minutes`).

**Hinweis:** A/B-Wochen sind KEIN Setting, sondern werden pro Kalenderperiode konfiguriert. Grund: Verschiedene Perioden können verschiedene Cycle-Lengths haben (Schuljahr: A/B, Ferien: keine Alternierung).

---

## Neue API-Endpoints

### Gap-Detection (E20)

```
GET /api/timetable/gaps?date=2026-04-16                    # Einzelner Tag
GET /api/timetable/gaps?date=2026-04-16&date_to=2026-04-20 # Zeitraum
```

### Semester-Rollover (E17)

```
POST /api/activities/enrollments/semester-rollover          # Bulk: Halbjahreswechsel
GET  /api/activities/enrollments?valid_at=2026-04-15        # Filtern nach Gültigkeit
```

### Kalenderperioden (E15)

```
GET    /api/timetable/periods                              # Alle Perioden des Tenants
POST   /api/timetable/periods                              # Neue Periode erstellen
PUT    /api/timetable/periods/{id}                         # Periode bearbeiten
DELETE /api/timetable/periods/{id}                         # Periode löschen (nur wenn keine Templates referenzieren)
```

**MVP: Nur GET wird initial gebraucht** (System erstellt Default-Periode automatisch). CRUD wird für Ferienbetreuung-UI benötigt.

---

## Aktualisierte Tabellenübersicht

| Tabelle | Status | Änderungen in dieser Iteration |
|---------|--------|-------------------------------|
| `schedule.calendar_periods` | **Neu** | Kalenderperioden mit A/B-Wochen Config |
| `schedule.activity_instances` | Neu (Iter. 2) | + `calendar_period_id` (nullable FK) |
| `schedule.instance_staff` | Neu (Iter. 2) | Unverändert |
| `schedule.instance_students` | Neu (Iter. 2) | **Überarbeitet**: `attendance_status` → `status` + `substatus` + `note` |
| `schedule.activity_exceptions` | Neu (RFC) | Unverändert |
| `activities.schedules` | **Geändert** | + `week_pattern`, + `calendar_period_id` |
| `activities.student_enrollments` | **Geändert** | `enrollment_date` → `valid_from`, + `valid_until`, + `calendar_period_id`, UNIQUE angepasst |
| `activities.supervisors` | **Geändert** | + `valid_from`, + `valid_until`, + `calendar_period_id`, UNIQUE angepasst |
| `active.groups` | Geändert (Iter. 2) | `group_id` nullable — unverändert |

---

## Aktualisierte Phasenplanung

```
Phase 0 (jetzt):     Arrival Schedules (unverändert, unabhängig)
                      → Pickup-Mirror Pattern
                      → Bulk-Endpoint für Klasseneingabe

Phase 1 (danach):     Kalenderperioden + Template-Erweiterungen
                      → calendar_periods Tabelle + Default-Periode
                      → week_pattern auf activities.schedules
                      → valid_from/valid_until auf enrollments + supervisors
                      → Migration: enrollment_date → valid_from

Phase 2 (danach):     Activity Instances + Materialisierung
                      → Kern des Timetable-Systems
                      → Drei-Feld Attendance-Modell
                      → active.groups Bridge
                      → Materialisierung mit A/B-Wochen + Validity-Filter

Phase 3 (parallel):   Tagesablauf-Aggregation + Vertretung
                      → API: Student Day = Arrival + Instances + Pickup
                      → Gap-Detection Endpoint
                      → Conflict-Warnings bei Exception-Writes
                      → Passive Auto-Start (UI-Indikatoren)

Phase 4 (später):     Betreuer-Ansichten + Admin-Wochenplaner
                      → "Mein Tag" View mit SSE-Events (Aktiver Auto-Start)
                      → Spontane Aktivitäten
                      → Semester-Rollover UI
                      → Vertretungs-Management

Phase 5 (Roadmap):    Ferienbetreuung
                      → Perioden-Management UI
                      → Ferien-spezifische Templates + Enrollments
                      → Kein Schema-Änderung nötig (!)
```

**Phase 1 ist neu** und schiebt sich zwischen Arrival Schedules und Activity Instances. Da es nur Schema-Änderungen + Migrationen sind (kein UI), ist der Aufwand gering und blockiert nichts.

---

## Beantwortete offene Fragen

Aus vorherigen Iterationen:

| Frage | Antwort | Entscheidung |
|-------|---------|-------------|
| Vertretung: Original-Eintrag löschen oder "absent" markieren? | **Behalten + absent markieren.** Für Auswertungen ("wie oft musste Vertretung einspringen?") ist Behalten sinnvoller. Neues Feld `is_absent BOOLEAN DEFAULT false` auf `instance_staff`. | E20 |
| Mensa als rollender Checkpoint oder Block? | **Block-Modell reicht.** Mensa ist eine Zeitspanne mit Start/Ende. Rotation über Templates + A/B-Wochen. | E21 |

## Verbleibende offene Fragen

- [ ] **Spontanbesucher:** Kind checkt in eine Instance ein, ist aber nicht in `instance_students`. Neuen Eintrag mit `status=present` anlegen? Oder nur in `active.visits` erfassen?
- [ ] **"Woche neu planen" UI:** Confirmation-Dialog mit Diff? Oder einfacher "Sind Sie sicher?" Dialog?
- [ ] **Arrival Bulk: Überschreiben oder Mergen?** Wenn Bulk für "3a" ausgeführt wird und Max bereits individuelle Zeiten hat — überschreiben oder warnen?
- [ ] **Arrival Notes:** Brauchen wir `student_arrival_notes` analog zu `student_pickup_notes`? Oder reicht das `reason`-Feld auf der Exception?
- [ ] **Substatus-Erweiterung:** Brauchen wir `early_pickup` als Substatus? Oder wird das über die Pickup-Exception abgedeckt?
- [ ] **Perioden-Overlap:** Dürfen sich aktive Perioden zeitlich überlappen? (z.B. "Schuljahr 2026/27" + "Projektwoche Mai" gleichzeitig aktiv) → Vermutlich ja, aber Materialisierung muss dann aus der "spezifischeren" Periode die Templates nehmen.
- [ ] **minimum_staff Feld:** Braucht `schedule.activity_instances` ein `minimum_staff SMALLINT DEFAULT 1` Feld für die Gap-Detection? Oder ist `COUNT(staff) < 1` ausreichend?

---

## Zusammenfassung der Änderungen vs. Iteration 3

| Bereich | Iteration 3 | Iteration 4 |
|---------|-------------|-------------|
| Temporale Strukturierung | Keine (Templates sind zeitlos) | Kalenderperioden + Validity Dates |
| A/B-Wochen | Nicht adressiert | `week_pattern` + Perioden-Cycle-Config |
| Enrollment-Gültigkeit | `enrollment_date` (Start only) | `valid_from` + `valid_until` + optional Perioden-FK |
| Supervisor-Gültigkeit | Keine zeitliche Begrenzung | `valid_from` + `valid_until` + optional Perioden-FK |
| Attendance-Modell | `attendance_status` (4 Werte) | Drei-Feld: `status` (3) + `substatus` (5+) + `note` |
| Auto-Start | `auto_start_enabled` Setting | Drei Stufen: Passiv/Aktiv/Automatisch |
| Vertretungs-Erkennung | Nicht adressiert | Gap-Detection Query (MVP) + Absences (V2) |
| Ferienbetreuung | Nicht adressiert | Schema vorbereitet, Implementierung deferred |
| Mensa-Rotation | Nicht adressiert | Als Use-Case durch Templates + A/B gelöst dokumentiert |
| Phasen | 4 Phasen (0-3) | 6 Phasen (0-5), Phase 1 für Schema-Prep eingefügt |
| Neue Tabellen | 0 | 1 (`calendar_periods`) |
| Geänderte Tabellen | 0 | 4 (`schedules`, `enrollments`, `supervisors`, `instance_students`) |
| Neue Settings | 0 | 1 (`overdue_threshold_minutes`) |
| Neue Endpoints | 0 | 3 (Gaps, Rollover, Periods) |
