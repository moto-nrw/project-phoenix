# Timetable-Epic E2E-Sweep — Abschlussbericht

Sechs End-to-End-Flows gegen die gemergten WPs B1 bis B14. Jeder Flow hermetisch, real DB + real HTTP-Stack, eigener Tenant + Cross-Tenant-Isolation in jedem Flow, Query-Budget-Messung an drei Endpoints.

## Ergebnisuebersicht

| Flow | Zweck | Status | Laufzeit |
|------|-------|--------|----------|
| A | Plan bis Report (Hauptader) | GRUEN | 0.13 s |
| B | Cancel + Exception-Conflicts | GRUEN | 0.09 s |
| C | Gaps + Substitute | GRUEN (nach E2E-C1-Fix) | 0.07 s |
| D | Re-plan Merge-Strategie | GRUEN | 0.10 s |
| E | Student Day + Unplanned Visit | GRUEN | 0.07 s |
| F | GDPR Cleanup | GRUEN | 0.04 s |

**6/6 gruen nach Fix von E2E-C1.**

## Kurzfassung der Befunde

| ID | Flow | Urteil | Kern |
|----|------|--------|------|
| **E2E-B1** | B | Plan-Ambiguity | `skipped_exception` schlaegt `skipped_existing` — Plan §6.1 ist missverstaendlich formuliert. |
| **E2E-C1** | C | **Code-Bug (GEFIXT)** | `/gaps` gab `assigned_staff_count=0` zurueck; sollte Gesamtzahl sein. Einzeiler-Fix in `gaps.go:168`: `AssignedStaffCount: len(rows)`. Plus ein Unit-Test-Assert in `gaps_test.go:167` von `0` auf `2` angepasst (Test encodierte Bug-Verhalten). Flow C jetzt gruen. |
| **E2E-C2** | C | Plan-Dokumentation | Query-Budget `≤ 2` galt nur fuer Handler-Queries; TenantTx-Middleware addiert 4-5 Queries. |
| **E2E-ENV-1** | — | Existing Test-Fragility | `backend/services/schedule/arrival_service_test.go` nutzt hardcoded `CreatedBy: 1` staff-ID ohne eigenen Fixture. Die Tests sind fragil gegen DB-state, in dem staff_id=1 nicht existiert. Nicht durch diese Suite verursacht, aber erkannt. Kandidat fuer Hermetic-Bereinigung. |

Dazu gab es einen Umfeld-Befund ohne Nummerierung: Der inzwischen entfernte Fixture-Cleanup war im Teardown wirkungslos und erzeugte laute BUN-Logs. #2847 hat den Helper entfernt.

---

## Kategorien

- **Code-Bug** — Produktionscode verhaelt sich anders als dokumentiert.
- **Plan-Bug** — Plan/PR-Text beschreibt etwas, das so nie eingebaut wurde.
- **Test-Setup-Bug** — Mein Test war kaputt, im Produktionscode nichts falsch (gefixt).

---

## E2E-B1 — skipped_exception vs. skipped_existing (Plan-Ambiguity)

- **Flow**: B, Schritt 3 (Re-materialize mit existing Instanz + cancelled Exception)
- **Urteil**: Plan-Ambiguity, kein Code-Bug
- **Erwartet (Plan §6.1 / WP-B8 PR)**: Die Merge-Strategie schuetzt `planned` Instanzen: re-materialize ueber eine existing `planned` Instanz soll `skipped_existing=1` liefern.
- **Beobachtet**: Der Service prueft `activity_exceptions` **vor** der existing-row-dedupe. Bei existing Instanz + cancelled Exception → `skipped_exception=1, skipped_existing=0`.
- **Reproducer**: `TestFlowB_CancelAndExceptionConflicts`, Schritt 3.
- **Bewertung**: Das Verhalten ist sinnvoll (eine gecancelte Aktivitaet soll nicht "zufaellig" per Re-Materialize gerettet werden). Der Plan-Text `§6.1` suggeriert aber eine andere Reihenfolge. Der Plan sollte klarstellen: "Exception wins over existing when candidate is evaluated."

## E2E-C1 — `assigned_staff_count` ist immer 0 in Gap-Response (Code-Bug, GEFIXT)

- **Flow**: C, Schritt 8 (`GET /gaps`)
- **Urteil**: Code-Bug — **gefixt in dieser Branch**
- **Erwartet (PR #1303 Doc-Block)**:
  ```json
  "gaps":[{
    "assigned_staff_count": 2, "absent_staff_count": 2
  }]
  ```
  Ein Gap mit zwei zugewiesenen Staff-Members (beide krank) soll `assigned_staff_count=2` zurueckgeben.
- **War**: Response zeigte `assigned_staff_count=0, absent_staff_count=2`.
- **Quelle**: `backend/api/timetable/gaps.go:168` — `AssignedStaffCount: 0,` mit missverstaendlichem Kommentar.
- **Wurzel**: Der Kommentar missinterpretierte die Semantik. `nonAbsentCounts[inst.ID] == 0` bedeutet "null NICHT-abwesende Staff", nicht "null zugewiesene Staff". Bei zwei Supervisors, die beide krank gemeldet sind, ist `nonAbsentCount=0`, aber `assignedCount=2`.
- **Fix**: `AssignedStaffCount: len(rows)` — die vollstaendige Liste liegt bereits im Code, einfach die Laenge nutzen.
- **Test-Anpassung (Option B, explizit vom User autorisiert)**: `backend/api/timetable/gaps_test.go:167` war `assert.Equal(t, 0, got.Gaps[0].AssignedStaffCount)` — dieser Assert encodierte das Bug-Verhalten, nicht die Business-Rule. Geaendert zu `assert.Equal(t, 2, ...)` passend zum PR-Vertrag.
- **Impact**: Admin-UI zeigt jetzt korrekt "2 Staff zugewiesen, 0 anwesend, 2 abwesend" statt widerspruechliches "0 zugewiesen, 2 abwesend".
- **Reproducer**: `TestFlowC_GapsAndSubstitute`, Schritt 8 — jetzt gruen.

## E2E-C2 — Query-Budget /gaps ≤ 2 ist ohne Middleware-Kontext

- **Flow**: C, Schritt 3 und 8
- **Urteil**: Plan-Dokumentations-Luecke
- **Erwartet (PR #1303)**: `/gaps` ≤ 2 Queries (bulk query mit GROUP BY).
- **Beobachtet**: 6 Queries (baseline, keine Gaps) bzw. 7 Queries (mit einem Gap).
- **Wurzel**: Die Message `≤ 2` bezog sich auf die Handler-Queries (`FindByTenantAndDateRange`, `CountNonAbsentByInstanceIDs`). Die `TenantTxMiddleware` addiert 4-5 Queries pro Request (`SET LOCAL ROLE`, `SELECT set_config(...)`, etc.) zusammen mit der Transaktions-BEGIN/COMMIT. Bei einem Gap kommt zusaetzlich `FindByInstanceID` pro Gap hinzu (siehe gaps.go:145).
- **Bewertung**: Kein Bug. Aber das PR-Target im Plan sollte prezisiert werden auf "Handler-Queries ≤ 2" oder das Budget auf die Realitaet angepasst werden (≤ 10 end-to-end).
- **Reproducer**: `TestFlowC_GapsAndSubstitute`, Schritte 3 + 8 (Log-Output).

## Umfeld-Notiz — Cleanup-Rauschen

Historischer Befund: Alle Flows produzierten im Teardown BUN-Fehler, weil der damalige Fixture-Cleanup ein ungültiges Model verwendete. #2847 hat den expliziten Cleanup vollständig entfernt.

---

## Was die Suite deckt und was nicht

**Abgedeckt in dieser Runde:**
- Ganzer Materialize → Start → Check-in → Complete → Report Pfad (Flow A)
- Exception-Propagation (cancelled und modified) inkl. Arrival-Exception-Interaktion (Flow B)
- Substitute Dry-Run-First-Atomicity bei 409-Konflikt (Flow C)
- Active-Group-Supervisors-Rotation beim Substitute auf `active` Instanz (Flow C)
- Gap-Detection bei komplett krankgemeldetem Staff (Flow C)
- Re-plan-Merge-Strategie fuer alle Status-Kombinationen + Spontan-Instanz (Flow D)
- Unplanned-Visit-Surfacing im Student-Day (Flow E)
- GDPR-Retention-Cleanup inkl. CASCADE, Audit-Rows, Idempotenz (Flow F)
- Tenant-Isolation an jedem Flow

**Nicht abgedeckt (bewusst out of scope):**
- A/B-Wochen-Boundary — ist durch B8-Unit-Tests + DST-Tests abgedeckt.
- SSE-Events — separate Infra.
- IoT-Check-in via `/api/iot/*` — PyrePortal-Track.
- Schema-Invarianten (FKs, CHECKs) — durch B5-Tests abgedeckt.
- Auth-Edge-Cases (abgelaufener JWT, fehlende Permissions) — Infra.
- Retrospektive Materialisierung — out of scope §9.
- Performance-Skalierung (z.B. 100 Templates × 1000 Kinder) — Benchmark-Territorium, nicht E2E.

## Empfehlungen

1. **E2E-C1 fixen** (Einzeiler in `backend/api/timetable/gaps.go:168`): `AssignedStaffCount: len(rows)` statt `0`. Sobald der Bug gefixt ist, wird Flow C automatisch gruen.
2. **E2E-B1 im Plan praezisieren**: `docs/timetable-system-plan.md` §6.1 um einen Hinweis erweitern, dass die Exception-Checks vor der existing-row-Dedupe laufen. Das Verhalten ist sinnvoll, der Plan-Text widerspricht ihm.
3. **E2E-C2 im Plan verorten**: die PR-Beschreibungen `≤ 2` / `≤ 22` / `≤ 12` auf Handler-Queries zu stellen und die Middleware-Overhead-Erwartung separat zu dokumentieren. Oder bei allen das End-to-End-Budget messen und realistisch angeben.
4. **Cleanup-Helper**: mit #2847 erledigt; Fixture-Zeilen gehören jetzt dem Paket-Clone.
5. **E2E-Suite ins CI integrieren**: sobald E2E-C1 gefixt ist, passt die ganze Suite in einen `go test ./test/e2e/...` Build-Step. Dauer: insgesamt unter einer Sekunde inklusive DB-Setup.

## Wie ausfuehren

```bash
# Test-DB sicherstellen
docker compose --profile test up -d postgres-test

# Alle Flows
APP_ENV=test go test ./test/e2e/timetable/ -v

# Einzelner Flow
APP_ENV=test go test ./test/e2e/timetable/ -run TestFlowC_GapsAndSubstitute -v
```

## Dateistruktur

```
backend/test/e2e/timetable/
├── shared_setup.go              # scenario, mountRouter, buildTemplate, helpers
├── flow_a_happy_path_test.go    # Plan → Report
├── flow_b_cancel_conflicts_test.go
├── flow_c_gaps_substitute_test.go  # enthaelt den E2E-C1-Repro
├── flow_d_replan_week_test.go
├── flow_e_student_day_test.go
├── flow_f_gdpr_cleanup_test.go
└── E2E_FINDINGS.md              # dieses Dokument
```
