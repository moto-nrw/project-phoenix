# Iteration 2 — Timetable System RFC: Devil's Advocate Review

Ergebnis aus drei Runden Devil's-Advocate-Analyse gegen das ursprüngliche RFC (`timetable-system-rfc.md`). Alle Entscheidungen sind von Yannick bestätigt.

---

## Entscheidungsprotokoll

### E1: Ein System statt drei Modes

**Vorher:** `timetable.mode` Setting mit `flexible` / `planned` / `hybrid`.

**Nachher:** Ein einziges System. Jede Schule kann jederzeit sowohl geplante als auch spontane Aktivitäten erstellen. Keine Business-Logik-Einschränkungen. Keine Mode-Umschaltung.

**Begründung:** Drei Modi erzwingen künstliche Grenzen. Jede OGS hat feste Blöcke (Mensa, Lernzeit) UND spontane Aktivitäten. Ein "Flexible"-Modus, der keine Materialisierung erlaubt, ist für keine Schule mit festen Blöcken brauchbar. Stattdessen: Materialisierung ist optional aktivierbar per Setting, spontane Aktivitäten sind immer möglich.

**Impact:** 
- `timetable.mode` Setting entfällt
- `timetable.allow_spontaneous_activities` Setting entfällt
- DependsOn-OR-Erweiterung im Settings-System wird NICHT benötigt
- Vereinfacht Frontend (kein Mode-Switch UI)

---

### E2: `active.groups.GroupID` wird nullable

**Problem:** `active.groups.group_id` ist aktuell `NOT NULL` (FK zu `activities.groups`). Spontane Instances haben kein Template (`activity_group_id = NULL`). Beim Starten einer spontanen Instance muss eine `active.group` erstellt werden — aber ohne `GroupID`.

**Entscheidung:** Migration: `group_id` wird nullable (`*int64` im Go Model).

```sql
ALTER TABLE active.groups ALTER COLUMN group_id DROP NOT NULL;
```

**Impact:**
- Go Model `active.Group.GroupID`: `int64` → `*int64`
- Alle Queries/Services die `GroupID` nutzen müssen NULL-safe werden
- Bestehender NFC-Flow bleibt unverändert (setzt immer GroupID)
- `activities.groups` bleibt als Backend-Konzept erhalten (kein Refactoring nötig)

---

### E3: Multi-Room Instances via Override-Pattern

**Problem:** Lernzeit wird pro Jahrgang in zwei Räume aufgeteilt (Betreuerin + Lehrkraft). Ein einzelnes `room_id` Feld auf der Instance reicht nicht.

**Entscheidung:** `room_id` bleibt auf der Instance als Primär-Raum (NOT NULL). Für den Split-Fall haben `instance_staff` und `instance_students` ein eigenes `room_id` Feld als Override. `NULL` = Primär-Raum der Instance.

**Keine zusätzliche Tabelle nötig.** Eine Junction-Tabelle (`instance_rooms`) wurde evaluiert und verworfen — zu viel Komplexität für einen Fall der bei ~90% der Instances nicht eintritt. Das Override-Pattern ist einfacher: 1 Insert statt 2, keine Konsistenz-Checks zwischen drei Tabellen.

**Änderungen an RFC-Tabellen:**

- `schedule.activity_instances`: `room_id BIGINT NOT NULL` bleibt (Primär-Raum, immer gesetzt)
- `schedule.instance_staff`: bekommt `room_id BIGINT REFERENCES facilities.rooms(id)` — Override: NULL = Primär-Raum der Instance, gesetzt = anderer Raum
- `schedule.instance_students`: bekommt `room_id BIGINT REFERENCES facilities.rooms(id)` — Override: NULL = Primär-Raum der Instance, gesetzt = anderer Raum

**Beispiel Lernzeit Jg.3 (Split):**

```
Instance: "Lernzeit Jg.3", 13:45-14:30, room_id=4 (Primär)
├── instance_staff: [Franziska → room_id=NULL (=Raum 4)], [Frau Meyer → room_id=7]
└── instance_students: [Kind 1-15 → room_id=NULL (=Raum 4)], [Kind 16-30 → room_id=7]
```

**Beispiel Mensa (einfacher Fall):**

```
Instance: "Mensa", 12:00-12:45, room_id=12 (Mensa-Raum)
├── instance_staff: [Betreuerin → room_id=NULL (=Raum 12)]
└── instance_students: [alle Mensa-Kinder → room_id=NULL (=Raum 12)]
```

**Alle Räume einer Instance ermitteln:**

```sql
SELECT DISTINCT room_id FROM (
    SELECT room_id FROM schedule.activity_instances WHERE id = :instance_id
    UNION
    SELECT room_id FROM schedule.instance_staff WHERE instance_id = :instance_id AND room_id IS NOT NULL
) rooms;
```

**Validierung:** Jede Instance MUSS einen Primär-Raum haben (`room_id NOT NULL`). Spontane Instances müssen bei Erstellung einen Raum angeben.

---

### E4: Attendance-Status wird bei Check-in/Check-out aktualisiert

**Problem:** `instance_students.attendance_status` und `active.visits` könnten auseinanderlaufen (Dual Source of Truth).

**Entscheidung:** Application Code aktualisiert `instance_students.attendance_status` zum Zeitpunkt des Check-in/Check-out. Kein DB-Trigger, kein Computed-at-Read-Time.

**Ablauf:**
1. Betreuer tippt Kind an → Check-in Handler wird aufgerufen
2. Handler erstellt `active.visit` (wie bisher)
3. Handler prüft: Gibt es eine aktive Instance für diese `active.group`? (über `activity_instances.active_group_id`)
4. Wenn ja: Update `instance_students SET attendance_status = 'present', checked_in_at = NOW() WHERE instance_id = X AND student_id = Y`
5. Bei Check-out: analog `attendance_status` bleibt 'present' (war da), `active.visit.exit_time` wird gesetzt

**Sonderfall:** Kind checkt ein, ist aber NICHT in `instance_students` (Spontanbesucher). Dann:
- Neuer `instance_students`-Eintrag mit `attendance_status = 'present'` (nicht 'expected')
- Oder: Kind wird nur in `active.visits` erfasst, nicht in `instance_students` — je nach Anforderung

**Impact:** Check-in Handler (`api/iot/checkin/` und neuer App-basierter Check-in) muss Instance-Awareness bekommen.

---

### E5: Fehlende-Kinder-Anzeige als Kern-Feature

**Nicht Nice-to-have, sondern Kern-Feature.** Die Check-in-Liste zeigt sofort, welche erwarteten Kinder fehlen.

**Pflicht für v1:**

```
Lernzeit Jg.3 — 13:45-14:30 — Raum 4
─────────────────────────────────
✓ Anna M.      13:47
✓ Ben K.       13:48
✓ Clara S.     13:49
... (10 weitere)
─────────────────────────────────
⚠ FEHLT: Max W., Lena T., Tim R.
─────────────────────────────────
12/15 anwesend
```

**API-Response:**

```json
GET /api/timetable/instances/{id}/students

{
  "total_expected": 15,
  "present": 12,
  "missing": [
    { "id": 7, "name": "Max W.", "status": "expected" },
    { "id": 12, "name": "Lena T.", "status": "expected" },
    { "id": 19, "name": "Tim R.", "status": "expected" }
  ],
  "students": [
    { "id": 1, "name": "Anna M.", "status": "present", "checked_in_at": "2026-09-15T13:47:00Z" },
    { "id": 7, "name": "Max W.", "status": "expected", "checked_in_at": null },
    ...
  ]
}
```

**Später (nicht v1):** Push-Benachrichtigung nach X Minuten ("3 Kinder fehlen seit 10 Minuten").

---

### E6: Vertretung per One-Click

**Problem:** Betreuer krank → alle Instances des Tages haben keinen Betreuer. Admin muss jede Instance einzeln anpassen.

**Entscheidung:** Ein "Vertretung zuweisen" Endpoint der alle Instances eines Tages überschreibt.

```
POST /api/timetable/substitute
{
  "absent_staff_id": 42,
  "substitute_staff_id": 55,
  "date": "2026-09-15"
}
```

**Logik:**
1. Finde alle `instance_staff` Einträge von `staff_id=42` an diesem Datum
2. Für jeden Eintrag: erstelle neuen Eintrag für `substitute_staff_id=55` mit `is_substitute=true`, übernimm `room_id`
3. Original-Eintrag bleibt (für Dokumentation/Auswertung wer eigentlich eingeplant war), bekommt neues Flag oder wird als "absent" markiert

**Offener Design-Punkt:** Soll der Original-Eintrag gelöscht oder als "absent" markiert werden? Für Auswertungen ("wie oft musste Vertretung einspringen?") ist Behalten sinnvoller.

---

### E7: Konflikt-Erkennung bei Planung

**Entscheidung:** Konflikte werden bei Instance-Erstellung und -Bearbeitung geprüft. Soft-Warning, kein Hard-Block.

**Prüfzeitpunkte:**

| Zeitpunkt | Prüfung | Reaktion |
|-----------|---------|----------|
| Instance erstellen/bearbeiten | Raum-Overlap, Staff-Doppelbelegung, Student-Doppelbelegung | Soft-Warning im API-Response |
| Materialisierung | Template-Konflikte | Warning in Log + in Materialisierungs-Report |
| Spontane Aktivität | Gleiche Prüfungen | Override erlaubt (User entscheidet bewusst) |

**Service Interface:**

```go
type ConflictService interface {
    CheckConflicts(ctx context.Context, date time.Time, startTime, endTime string, 
        roomIDs []int64, staffIDs []int64, studentIDs []int64, 
        excludeInstanceID *int64) []Conflict
}

type Conflict struct {
    Type          string // "room", "staff", "student"
    ConflictsWith int64  // ID der kollidierenden Instance
    Description   string // "Raum 4 ist von 13:45-14:30 belegt durch Lernzeit Jg.2"
}
```

**API-Response bei Konflikten:**

```json
POST /api/timetable/instances
Response 201 Created:
{
  "instance": { ... },
  "warnings": [
    {
      "type": "room",
      "description": "Raum 4 ist von 13:45-14:30 belegt durch Lernzeit Jg.2"
    }
  ]
}
```

---

### E8: Materialisierung — Scheduler UND manueller Button

**Entscheidung:** Beides. Automatische Materialisierung per Scheduler (wenn aktiviert) UND manueller "Nächste Woche vorbereiten" Button im Admin-UI.

**Settings:**

| Key | Type | Default | DependsOn |
|-----|------|---------|-----------|
| `timetable.materialization_enabled` | boolean | `false` | — |
| `timetable.materialization_weekday` | select | `5` (Fr) | materialization_enabled eq true |
| `timetable.materialization_weeks_ahead` | number | `1` | materialization_enabled eq true |

**Manueller Endpoint:**

```
POST /api/timetable/materialize
{
  "week": "2026-W38"     // ISO 8601 Woche
}
```

**Merge-Strategie bei Neu-Materialisierung:**

| Instance-Status | Verhalten |
|----------------|-----------|
| `planned` (nicht manuell geändert) | Override — wird aus Template neu generiert |
| `planned` (manuell geändert) | Override — Planung hat Vorrang. Manuelle Änderungen gehen verloren. |
| `active` (gerade laufend) | Kein Eingriff — bleibt unverändert |
| `completed` | Kein Eingriff — historische Daten bleiben |
| `cancelled` | Kein Eingriff — bleibt cancelled |

**Konsequenz:** Wenn Admin "Woche neu planen" klickt, werden alle `planned` Instances gelöscht und aus Templates neu erstellt. Laufende und abgeschlossene Instances bleiben unangetastet.

**UI-Hinweis:** Vor Neu-Materialisierung zeigt das Frontend: "X geplante Einträge werden überschrieben. Y laufende/abgeschlossene Einträge bleiben erhalten."

---

### E9: GDPR Retention als eigenes Setting

**Entscheidung:** Historische Timetable-Daten bekommen ein eigenes GDPR-Cleanup-Setting, unabhängig von der bestehenden Attendance-Cleanup.

**Neues Setting:**

| Key | Type | Default | Tab | DependsOn |
|-----|------|---------|-----|-----------|
| `gdpr.timetable_retention_days` | number | `365` | gdpr | gdpr.data_cleanup_enabled eq true |

**Cleanup-Logik:** Completed/Cancelled Instances älter als X Tage werden mit allen `instance_staff`, `instance_students` gelöscht. Die verknüpften `active.groups` + `active.visits` werden separat durch das bestehende GDPR-Cleanup gehandhabt.

---

### E10: UNIQUE Constraint Fix für spontane Instances

**Problem:** PostgreSQL behandelt `NULL != NULL` in UNIQUE Constraints. Die geplante UNIQUE Constraint `(tenant_id, date, activity_group_id, start_time)` verhindert keine Duplikate bei spontanen Instances (wo `activity_group_id IS NULL`).

**Optionen:**

A) Partial UNIQUE Index nur für Template-basierte Instances:

```sql
CREATE UNIQUE INDEX idx_activity_instances_template_unique 
    ON schedule.activity_instances (tenant_id, date, activity_group_id, start_time)
    WHERE activity_group_id IS NOT NULL;
```

B) Zusätzlicher Application-Level Check bei spontaner Erstellung (z.B. gleicher Titel + gleiche Zeit → Warning).

**Empfehlung:** Option A als Schema-Level Schutz + Option B als UX-Verbesserung.

---

## Aktualisiertes Settings-Kapitel

Alle Timetable-Settings nach Konsolidierung:

### Operations Tab

| Key | Type | Default | DependsOn | Beschreibung |
|-----|------|---------|-----------|-------------|
| `timetable.materialization_enabled` | boolean | `false` | — | Automatische Wochenplanung aktivieren |
| `timetable.materialization_weekday` | select | `5` (Fr) | materialization_enabled eq true | Wochentag für Materialisierung |
| `timetable.materialization_weeks_ahead` | number | `1` (min:1, max:4) | materialization_enabled eq true | Wochen im Voraus |
| `timetable.auto_start_planned` | boolean | `false` | — | Geplante Instances automatisch starten bei Startzeit |
| `timetable.show_expected_children_count` | boolean | `true` | — | Erwartete Kinderanzahl in Betreuer-Ansicht |

### GDPR Tab

| Key | Type | Default | DependsOn | Beschreibung |
|-----|------|---------|-----------|-------------|
| `gdpr.timetable_retention_days` | number | `365` (min:30, max:1825) | data_cleanup_enabled eq true | Aufbewahrungsdauer für abgeschlossene Timetable-Daten |

**Von 6 auf 6 Settings** (aber komplett andere Zusammensetzung als im Original-RFC).

---

## Aktualisiertes Datenmodell

### `schedule.activity_instances` — Überarbeitet

```sql
CREATE TABLE schedule.activity_instances (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
    date              DATE NOT NULL,

    -- Template-Referenz (NULL bei spontanen Aktivitäten)
    activity_group_id BIGINT REFERENCES activities.groups(id),

    -- Konkrete Daten für diesen Tag
    title             TEXT NOT NULL,
    description       TEXT,
    start_time        TIME NOT NULL,
    end_time          TIME NOT NULL,
    room_id           BIGINT NOT NULL REFERENCES facilities.rooms(id),
    -- Primär-Raum. Immer gesetzt (auch bei spontanen Instances).
    -- Bei Multi-Room (Lernzeit-Split): Dies ist der Haupt-Raum.
    -- Zusätzliche Räume werden über instance_staff.room_id abgebildet.

    -- Lifecycle
    status            TEXT NOT NULL DEFAULT 'planned',
    -- planned | active | completed | cancelled

    -- Brücke zum Live-System
    active_group_id   BIGINT REFERENCES active.groups(id),

    -- Metadata
    is_spontaneous    BOOLEAN NOT NULL DEFAULT false,
    notes             TEXT,
    created_by        BIGINT REFERENCES auth.accounts(id),
    started_by        BIGINT REFERENCES auth.accounts(id),
    started_at        TIMESTAMPTZ,
    completed_at      TIMESTAMPTZ,

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partial UNIQUE: verhindert Template-Duplikate, erlaubt spontane Duplikate
CREATE UNIQUE INDEX idx_activity_instances_template_unique 
    ON schedule.activity_instances (tenant_id, date, activity_group_id, start_time)
    WHERE activity_group_id IS NOT NULL;

ALTER TABLE schedule.activity_instances ENABLE ROW LEVEL SECURITY;
```

### `schedule.instance_staff` — Überarbeitet

```sql
CREATE TABLE schedule.instance_staff (
    id            BIGSERIAL PRIMARY KEY,
    tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id),
    instance_id   BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
    staff_id      BIGINT NOT NULL REFERENCES users.staff(id),
    room_id       BIGINT REFERENCES facilities.rooms(id),
    -- In welchem Raum arbeitet dieser Betreuer?
    -- NULL = Primär-Raum der Instance

    is_primary    BOOLEAN NOT NULL DEFAULT false,
    -- Hauptbetreuer für diesen Raum

    is_substitute BOOLEAN NOT NULL DEFAULT false,
    -- Vertretung für originalen Betreuer

    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (instance_id, staff_id)
);

ALTER TABLE schedule.instance_staff ENABLE ROW LEVEL SECURITY;
```

### `schedule.instance_students` — Überarbeitet

```sql
CREATE TABLE schedule.instance_students (
    id                BIGSERIAL PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
    instance_id       BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
    student_id        BIGINT NOT NULL REFERENCES users.students(id),
    room_id           BIGINT REFERENCES facilities.rooms(id),
    -- In welchem Raum ist dieses Kind?
    -- NULL = Primär-Raum der Instance (Standard bei Mensa, Freispiel)
    -- Gesetzt bei Lernzeit-Split (Kind ist in Raum 4 oder Raum 7)

    attendance_status TEXT NOT NULL DEFAULT 'expected',
    -- expected | present | absent | excused

    checked_in_at     TIMESTAMPTZ,
    -- Aktualisiert durch Application Code bei Check-in

    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (instance_id, student_id)
);

ALTER TABLE schedule.instance_students ENABLE ROW LEVEL SECURITY;
```

### Tabellenübersicht (komplett)

| Tabelle | Neu/Geändert | Zweck |
|---------|-------------|-------|
| `schedule.activity_instances` | Neu | Tageseinträge (geplant + spontan), inkl. Primär-Raum |
| `schedule.instance_staff` | Neu + room_id | Betreuer-Zuordnung mit optionalem Raum-Override |
| `schedule.instance_students` | Neu + room_id | Kinder-Zuordnung mit Raum + Attendance |
| `schedule.class_timetable` | Neu | Schulstundenplan (unverändert vom Original-RFC) |
| `schedule.class_timetable_exceptions` | Neu | Ausnahmen zum Schulplan |
| `schedule.activity_exceptions` | Neu | Ausnahmen zu Templates |
| `active.groups` | **Geändert** | `group_id` wird nullable |
| `activities.groups` | Geändert | +type, +education_group_id, +is_template |

---

## Neue API-Endpoints (Ergänzungen zum Original-RFC)

### Vertretung

```
POST   /api/timetable/substitute                    Vertretung zuweisen (alle Instances eines Tages)
```

### Materialisierung

```
POST   /api/timetable/materialize                   Manuell materialisieren (überschreibt geplante)
```

### Konflikt-Prüfung

Konflikte werden automatisch bei Instance-CRUD geprüft und als `warnings[]` im Response zurückgegeben. Kein separater Endpoint nötig.

---

## Verbleibende offene Fragen

- [ ] **Spontanbesucher:** Kind checkt in eine Instance ein, ist aber nicht in `instance_students`. Neuen Eintrag mit `status=present` anlegen? Oder nur in `active.visits` erfassen?
- [ ] **Vertretung Detail:** Original-Eintrag in `instance_staff` löschen oder als "absent" markieren?
- [ ] **class_timetable Scope:** Minimal (nur Endzeit pro Klasse/Tag) oder ausführlich (Stundenzahl, Randstunde, etc.)? Aufschiebbar?
- [ ] **"Woche neu planen" UI:** Confirmation-Dialog mit Diff? Oder einfacher "Sind Sie sicher?" Dialog?
- [ ] **Mensa als rollender Checkpoint:** Passt das Block-Modell (start_time/end_time) mit pro-Gruppe-Aufteilung? Oder braucht Mensa einen eigenen Instance-Typ?

---

## Zusammenfassung der Änderungen vs. Original-RFC

| Bereich | Original-RFC | Nach Devil's Advocate |
|---------|-------------|----------------------|
| Modes | 3 Modi (flexible/planned/hybrid) | Ein System, keine Modi |
| Settings | 6 (inkl. Mode-Switch) | 6 (andere Zusammensetzung, kein Mode) |
| DependsOn | Braucht OR-Erweiterung | Standard eq-Conditions reichen |
| Räume | 1 room_id pro Instance | room_id bleibt (Primär), Multi-Room via Override auf Staff/Students |
| active.groups.GroupID | NOT NULL | Nullable |
| Fehlende Kinder | Nicht spezifiziert | Kern-Feature mit visueller Anzeige |
| Vertretung | Nicht spezifiziert | One-Click Substitute Endpoint |
| Konflikte | Nicht spezifiziert | ConflictService mit Soft-Warnings |
| Materialisierung | Nur Scheduler | Scheduler + manueller Button |
| Merge-Strategie | Nicht spezifiziert | Override planned, schütze active/completed |
| GDPR | Nicht spezifiziert | Eigenes Retention-Setting |
| UNIQUE Constraint | Volle Tabelle (NULL-Loch) | Partial Index nur für Templates |
| Attendance Sync | Nicht spezifiziert | Application Code bei Check-in/Check-out |
