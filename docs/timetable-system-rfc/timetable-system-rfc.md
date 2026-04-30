# RFC: Timetable System

**Status:** Draft
**Author:** Yannick
**Date:** 2026-04-13
**Epic:** Timetables (Stundenpläne & Betreuungsplanung)

## 1. Problem Statement

OGS-Einrichtungen arbeiten mit papierbasierten Listen und manuellen Prozessen für die Tagesplanung. Betreuer haben ausgedruckte Listen für Mensa, Lernzeit, Freispiel etc. und haken Kinder manuell ab. Das Büro plant wöchentliche Einsatzpläne auf Papier oder in Excel.

Project Phoenix bildet bereits flexible, NFC-basierte Echtzeit-Sessions ab (`active.groups` + `active.visits`). Was fehlt, ist die **Planungsschicht**: geplante Tagesabläufe, die Kindern und Betreuern proaktiv angezeigt werden und als Grundlage für die tägliche Betreuung dienen.

## 2. Goals

- **Geplante Tagesabläufe** für Kinder und Betreuer abbilden (Mensa, Lernzeit, AGs, etc.)
- **Spontane Aktivitäten** ad-hoc erstellen (Yoga, Basteln nach 14:30)
- **Hybrid-Modus**: Geplante Blöcke mit spontanen Aktivitäten innerhalb bestimmter Zeitfenster
- **Nahtlose Integration** mit dem bestehenden `active.*` Echtzeit-System
- **Drei Ansichten**: Admin (Wochenplanung), Betreuer (Mein Tag), Kind (Tagesablauf)
- **Dienstplan-Linking**: Datengrundlage für zukünftige Auto-Generierung von Dienstplänen
- **Per-Tenant konfigurierbar** via Settings-System (flexible / planned / hybrid)

## 3. Non-Goals (this RFC)

- iServ/WebUntis-Schnittstellen (spätere Phase)
- Automatische Dienstplan-Generierung (spätere Phase, Datengrundlage wird aber gelegt)
- Eltern-App-Integration (separates Epic)
- NFC-basierte An-/Abmeldung an Timetable-Entries (existiert bereits für `active.groups`)

## 4. Architecture Overview

### 4.1 Three-Layer Model

```
┌─────────────────────────────────────────────────────┐
│  TEMPLATE LAYER (Planung)                           │
│  "Was passiert jede Woche?"                         │
│                                                     │
│  activities.groups      → Aktivität (Lernzeit, AG)  │
│  activities.schedules   → Wochentag + Zeitfenster   │
│  activities.student_enrollments → Geplante Kinder   │
│  activities.supervisors_planned → Geplante Betreuer │
│  schedule.class_timetable → Schulstundenplan        │
└──────────────────────┬──────────────────────────────┘
                       │ Materialisierung
                       │ (Scheduler, wöchentlich)
                       ▼
┌─────────────────────────────────────────────────────┐
│  INSTANCE LAYER (Konkreter Tag)                     │
│  "Was passiert heute?"                              │
│                                                     │
│  schedule.activity_instances    → Eintrag für Tag X │
│  schedule.instance_staff        → Zugewiesene Staff │
│  schedule.instance_students     → Erwartete Kinder  │
└──────────────────────┬──────────────────────────────┘
                       │ Start (Betreuer klickt)
                       │
                       ▼
┌─────────────────────────────────────────────────────┐
│  LIVE LAYER (Echtzeit) — EXISTIERT BEREITS          │
│  "Wer ist jetzt wo?"                                │
│                                                     │
│  active.groups            → Laufende Session        │
│  active.visits            → Kind ein-/ausgecheckt   │
│  active.group_supervisors → Betreuer in Session     │
└─────────────────────────────────────────────────────┘
```

### 4.2 Data Flow

```
1. Admin erstellt Template
   activities.groups + schedules + enrollments + supervisors

2. Scheduler materialisiert (z.B. Freitag für nächste Woche)
   Template → schedule.activity_instances + instance_staff + instance_students

3. Betreuer öffnet App am Morgen
   Sieht: "Mein Tag" mit allen Instances wo er eingeplant ist

4. Betreuer startet Instance
   Instance.status → "active"
   → active.group wird automatisch erstellt
   → active.group_supervisors werden aus instance_staff kopiert
   Instance.active_group_id → FK zur active.group

5. Kinder checken ein
   Betreuer tippt Kind an → active.visit wird erstellt
   instance_students.attendance_status wird aktualisiert

6. Betreuer beendet Instance
   Instance.status → "completed"
   active.group.end_time wird gesetzt
   Offene visits werden geschlossen

7. Spontane Aktivität
   Betreuer erstellt Instance direkt (ohne Template)
   is_spontaneous = true, activity_group_id = NULL
   Rest identisch ab Schritt 4
```

## 5. Data Model

### 5.1 Template Layer — Erweiterungen an bestehenden Tabellen

#### `activities.groups` — Neue Felder

```sql
ALTER TABLE activities.groups
  ADD COLUMN type TEXT NOT NULL DEFAULT 'activity';
  -- Mögliche Werte: 'activity' (AG), 'care' (Betreuung: Mensa, Lernzeit, Freispiel),
  --                 'external' (DAZ, Musikunterricht, Sport-Förder)

ALTER TABLE activities.groups
  ADD COLUMN education_group_id BIGINT REFERENCES education.groups(id);
  -- Optional: Wenn die Aktivität an eine Schulklasse/Jahrgang gebunden ist
  -- z.B. "Lernzeit Jg.3 Gruppe A" → education_group_id = Klasse 3a+3b

ALTER TABLE activities.groups
  ADD COLUMN is_template BOOLEAN NOT NULL DEFAULT true;
  -- true = wiederkehrende Planungsvorlage
  -- false = einmalige Aktivität (Projekttag, Ausflug)
```

**Keine Änderung** an: `activities.schedules`, `activities.student_enrollments`, `activities.supervisors_planned`. Diese funktionieren bereits als Template-Daten.

### 5.2 Instance Layer — Neue Tabellen

#### `schedule.activity_instances`

Materialisierte Einträge für konkrete Tage. Entweder aus Template generiert oder spontan erstellt.

```sql
CREATE TABLE schedule.activity_instances (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id),
    date            DATE NOT NULL,

    -- Template-Referenz (NULL bei spontanen Aktivitäten)
    activity_group_id BIGINT REFERENCES activities.groups(id),

    -- Konkrete Daten für diesen Tag (kopiert aus Template, überschreibbar)
    title           TEXT NOT NULL,
    description     TEXT,
    start_time      TIME NOT NULL,
    end_time        TIME NOT NULL,
    room_id         BIGINT REFERENCES facilities.rooms(id),

    -- Lifecycle
    status          TEXT NOT NULL DEFAULT 'planned',
    -- planned: materialisiert, noch nicht gestartet
    -- active: Betreuer hat gestartet, active.group läuft
    -- completed: beendet
    -- cancelled: abgesagt (Grund in notes)

    -- Brücke zum Live-System
    active_group_id BIGINT REFERENCES active.groups(id),
    -- Gesetzt wenn status = 'active' oder 'completed'
    -- Ermöglicht: Instance → active.group → visits (Echtzeit-Check-ins)

    -- Metadata
    is_spontaneous  BOOLEAN NOT NULL DEFAULT false,
    notes           TEXT,
    created_by      BIGINT REFERENCES auth.accounts(id),
    started_by      BIGINT REFERENCES auth.accounts(id),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, date, activity_group_id, start_time)
    -- Verhindert doppelte Materialisierung
);

-- RLS
ALTER TABLE schedule.activity_instances ENABLE ROW LEVEL SECURITY;
```

#### `schedule.instance_staff`

Betreuer-Zuordnung pro Instance. Kann vom Template abweichen (Vertretung).

```sql
CREATE TABLE schedule.instance_staff (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id),
    instance_id     BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
    staff_id        BIGINT NOT NULL REFERENCES users.staff(id),
    is_substitute   BOOLEAN NOT NULL DEFAULT false,
    -- true = Vertretung (Original-Betreuer aus Template wurde ersetzt)

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (instance_id, staff_id)
);

ALTER TABLE schedule.instance_staff ENABLE ROW LEVEL SECURITY;
```

#### `schedule.instance_students`

Erwartete Kinder pro Instance. Basis für den Plan-vs-Realität-Abgleich.

```sql
CREATE TABLE schedule.instance_students (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id),
    instance_id     BIGINT NOT NULL REFERENCES schedule.activity_instances(id) ON DELETE CASCADE,
    student_id      BIGINT NOT NULL REFERENCES users.students(id),

    attendance_status TEXT NOT NULL DEFAULT 'expected',
    -- expected: Kind sollte da sein (aus Planung)
    -- present: Kind ist da (Check-in erfolgt)
    -- absent: Kind ist nicht erschienen
    -- excused: Kind entschuldigt abwesend

    checked_in_at   TIMESTAMPTZ,
    -- Zeitstempel des Check-ins (Convenience, canonical ist active.visits)

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (instance_id, student_id)
);

ALTER TABLE schedule.instance_students ENABLE ROW LEVEL SECURITY;
```

### 5.3 Schulplan — Neue Tabelle

#### `schedule.class_timetable`

Wann endet der Unterricht für welche Klasse? Basis für "erwartete Ankunft in der OGS".

```sql
CREATE TABLE schedule.class_timetable (
    id                  BIGSERIAL PRIMARY KEY,
    tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
    education_group_id  BIGINT NOT NULL REFERENCES education.groups(id),
    weekday             SMALLINT NOT NULL CHECK (weekday BETWEEN 1 AND 5),
    -- ISO 8601: 1=Mo, 5=Fr

    periods             SMALLINT NOT NULL CHECK (periods BETWEEN 1 AND 8),
    -- Anzahl Schulstunden an diesem Tag

    school_end_time     TIME NOT NULL,
    -- Wann endet der Unterricht (z.B. 11:30, 12:45, 13:30)

    has_randstunde      BOOLEAN NOT NULL DEFAULT false,
    -- Nur relevant für Jg. 1-2: Kind bleibt bis nach 5. Stunde

    expected_ogs_arrival TIME NOT NULL,
    -- Wann wird das Kind in der OGS erwartet?
    -- Kann von school_end_time abweichen (Weg, Umziehen, etc.)

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, education_group_id, weekday)
);

ALTER TABLE schedule.class_timetable ENABLE ROW LEVEL SECURITY;
```

#### `schedule.class_timetable_exceptions`

Ausnahmen: Wandertag, Hitzefrei, Studientag, etc.

```sql
CREATE TABLE schedule.class_timetable_exceptions (
    id                  BIGSERIAL PRIMARY KEY,
    tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
    education_group_id  BIGINT NOT NULL REFERENCES education.groups(id),
    exception_date      DATE NOT NULL,

    exception_type      TEXT NOT NULL,
    -- 'cancelled' (kein Unterricht), 'modified' (geänderte Zeiten), 'holiday'

    school_end_time     TIME,
    -- Nur bei 'modified': abweichende Endzeit
    expected_ogs_arrival TIME,
    -- Nur bei 'modified': abweichende Ankunft

    reason              TEXT,
    created_by          BIGINT REFERENCES auth.accounts(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, education_group_id, exception_date)
);

ALTER TABLE schedule.class_timetable_exceptions ENABLE ROW LEVEL SECURITY;
```

### 5.4 Activity Exceptions — Neue Tabelle

Konsistent mit dem Pickup-Schedule-Pattern: Template + Exceptions.

#### `schedule.activity_exceptions`

Für geplante Abweichungen vom Template BEVOR materialisiert wird.

```sql
CREATE TABLE schedule.activity_exceptions (
    id                  BIGSERIAL PRIMARY KEY,
    tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
    activity_group_id   BIGINT NOT NULL REFERENCES activities.groups(id),
    exception_date      DATE NOT NULL,

    exception_type      TEXT NOT NULL,
    -- 'cancelled': Aktivität fällt aus an diesem Tag
    -- 'modified': Geänderte Zeit/Raum an diesem Tag

    -- Override-Felder (nur bei 'modified')
    start_time          TIME,
    end_time            TIME,
    room_id             BIGINT REFERENCES facilities.rooms(id),

    reason              TEXT,
    created_by          BIGINT REFERENCES auth.accounts(id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE (tenant_id, activity_group_id, exception_date)
);

ALTER TABLE schedule.activity_exceptions ENABLE ROW LEVEL SECURITY;
```

## 6. Materialisierung

### 6.1 Ablauf

Der bestehende Scheduler (`services/scheduler/scheduler.go`) erhält einen neuen Job:

```
Freitag (konfigurierbar per Setting) für die Folgewoche:

1. Für jeden aktiven Tenant:
2.   Hole alle activities.groups WHERE is_template = true
3.   Für jede Gruppe:
4.     Hole activities.schedules → Wochentage + Zeitfenster
5.     Für jeden Wochentag der Folgewoche:
6.       Prüfe schedule.activity_exceptions → Ausfall/Änderung?
7.       Erstelle schedule.activity_instances mit:
8.         - Daten aus Template (title, room, times)
9.         - Überschreibungen aus Exception (falls vorhanden)
10.        - Status 'planned'
11.      Kopiere activities.supervisors_planned → schedule.instance_staff
12.      Kopiere activities.student_enrollments → schedule.instance_students
13.        (mit attendance_status = 'expected')
```

### 6.2 Idempotenz

Die UNIQUE Constraint `(tenant_id, date, activity_group_id, start_time)` verhindert doppelte Materialisierung. Bei erneutem Lauf werden nur fehlende Instances erstellt. Bereits materialisierte (und ggf. manuell angepasste) Instances bleiben unberührt.

### 6.3 Template-Änderungen nach Materialisierung

Wenn ein Admin das Template ändert nachdem Instances materialisiert wurden:
- **Noch nicht gestartete Instances**: Können optional neu generiert werden (Admin-Aktion: "Woche neu planen")
- **Bereits gestartete/abgeschlossene**: Bleiben unverändert (historische Daten)
- **Neue Kinder/Betreuer im Template**: Werden beim nächsten Materialisierungslauf berücksichtigt

## 7. Settings Integration

Neue Settings im bestehenden Registry-System:

| Key | Type | Default | Tab | Beschreibung |
|-----|------|---------|-----|-------------|
| `timetable.mode` | select | `flexible` | operations | `flexible` / `planned` / `hybrid` — steuert UI-Sichtbarkeit |
| `timetable.materialization_weekday` | select | `5` (Fr) | operations | Wochentag für automatische Materialisierung (1=Mo..5=Fr) |
| `timetable.materialization_weeks_ahead` | number | `1` | operations | Wie viele Wochen im Voraus materialisieren |
| `timetable.allow_spontaneous_activities` | boolean | `true` | operations | Betreuer dürfen spontane Aktivitäten erstellen |
| `timetable.auto_start_planned` | boolean | `false` | operations | Geplante Instances automatisch starten wenn Startzeit erreicht |
| `timetable.show_expected_children_count` | boolean | `true` | operations | Zeige erwartete Kinderanzahl in Betreuer-Ansicht |

DependsOn-Ketten:
- `materialization_weekday` → depends on `mode` eq `planned` OR `hybrid`
- `materialization_weeks_ahead` → depends on `mode` eq `planned` OR `hybrid`
- `allow_spontaneous_activities` → depends on `mode` eq `flexible` OR `hybrid`
- `auto_start_planned` → depends on `mode` eq `planned` OR `hybrid`

## 8. API Endpoints (Entwurf)

### Admin / Büro

```
GET    /api/timetable/instances?week=2026-W38        Liste aller Instances einer Woche
POST   /api/timetable/instances                       Spontane Instance erstellen
PUT    /api/timetable/instances/{id}                  Instance bearbeiten (Raum, Zeit, Staff)
DELETE /api/timetable/instances/{id}                  Instance absagen (status → cancelled)
POST   /api/timetable/instances/{id}/start            Instance starten → active.group
POST   /api/timetable/instances/{id}/complete         Instance beenden
POST   /api/timetable/materialize                     Manuell materialisieren (Admin-only)

GET    /api/timetable/class-timetable                 Schulplan für alle Klassen
PUT    /api/timetable/class-timetable/{id}            Schulplan-Eintrag bearbeiten
POST   /api/timetable/class-timetable-exceptions      Ausnahme hinzufügen

GET    /api/timetable/exceptions                      Activity-Exceptions
POST   /api/timetable/exceptions                      Exception erstellen (Ausfall/Änderung)
```

### Betreuer (Mobile)

```
GET    /api/timetable/my-day?date=2026-09-15          "Mein Tag" — alle Instances wo ich eingeplant bin
GET    /api/timetable/my-week                         "Meine Woche" — Wochenübersicht
GET    /api/timetable/instances/{id}/students          Erwartete + anwesende Kinder einer Instance
POST   /api/timetable/instances/{id}/checkin/{student} Kind einchecken
POST   /api/timetable/instances/{id}/checkout/{student} Kind auschecken
```

### Kind-Profil

```
GET    /api/timetable/student/{id}/day?date=2026-09-15  Tagesablauf eines Kindes
GET    /api/timetable/student/{id}/week                  Wochenablauf
```

## 9. Frontend Views (Konzept)

### 9.1 Admin: Wochenplaner

- Grid-View: X-Achse = Wochentage, Y-Achse = Zeitslots
- Farbcodiert nach Typ (care=blau, activity=grün, external=orange)
- Drag & Drop für Instances (Raum/Zeit ändern)
- Klick auf Instance → Detail-Panel mit Staff + Kinder
- "Woche neu materialisieren" Button

### 9.2 Betreuer: Mein Tag

- Vertikale Timeline des Tages
- Jede Instance als Card mit: Titel, Zeit, Raum, erwartete Kinder (Anzahl)
- "Starten" Button → öffnet Check-in-Liste
- Check-in-Liste: Alle erwarteten Kinder mit Tap-to-Check-in
- Spontan-Button: "Neue Aktivität" (wenn erlaubt per Setting)

### 9.3 Kind: Tagesablauf

- Timeline-View: Was hat dieses Kind heute?
- Pro Block: Aktivität, Zeit, Raum, zuständiger Betreuer
- Live-Indikator: "Gerade in: Lernzeit Raum 4"
- Vergangene Einträge: Check-in-Zeit, Dauer

## 10. Konsistenz-Pattern (Template + Exception)

Architektonisch nutzen drei Bereiche dasselbe Muster:

| Domain | Template-Tabelle | Exception-Tabelle | Instance |
|--------|-----------------|-------------------|----------|
| **Timetable** | `activities.groups` + `schedules` | `schedule.activity_exceptions` | `schedule.activity_instances` (materialisiert) |
| **Pickup** | `schedule.student_pickup_schedules` | `schedule.student_pickup_exceptions` | On-the-fly berechnet |
| **Schulplan** | `schedule.class_timetable` | `schedule.class_timetable_exceptions` | On-the-fly berechnet |

Gleiches Denkmuster, unterschiedliche Granularität. Kein God-Table.

## 11. Migration Path

### Phase 1: Schema + Backend (Wochen 1-3)
- Neue Tabellen anlegen (Migrations)
- `activities.groups` erweitern (type, education_group_id, is_template)
- Models + Repositories
- Materialisierungs-Job im Scheduler
- Settings registrieren

### Phase 2: API + Core Logic (Wochen 3-5)
- Instance CRUD
- Start/Complete Lifecycle (Brücke zu active.groups)
- Check-in/Check-out via instance_students + active.visits
- Class Timetable CRUD

### Phase 3: Frontend — Admin (Wochen 5-7)
- Wochenplaner-View
- Schulplan-Editor
- Template-Verwaltung (erweiterte activities.groups UI)

### Phase 4: Frontend — Betreuer Mobile (Wochen 7-9)
- "Mein Tag" View
- Check-in-Liste
- Spontane Aktivität erstellen

### Phase 5: Frontend — Kind-Profil (Woche 9-10)
- Tagesablauf-View
- Live-Indikator

## 12. Open Questions

- [ ] Soll die Materialisierung auch für vergangene Wochen nachholen? (z.B. Schule vergisst Template anzulegen, will rückwirkend Daten haben)
- [ ] Wie detailliert wird der Schulplan? Nur Endzeit pro Klasse/Tag, oder einzelne Schulstunden?
- [ ] Brauchen wir Kapazitäts-Checks bei Materialisierung? (Raum hat max 20, aber 25 Kinder eingeplant)
- [ ] Notification-System: Betreuer kriegt Push wenn seine Instance in 10min startet?
- [ ] Soll "Mein Tag" auch Instances anderer Betreuer anzeigen (read-only), damit man den Gesamtüberblick hat?
