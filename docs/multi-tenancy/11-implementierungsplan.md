# Multi-Tenancy: Implementierungsplan

Dieses Dokument beschreibt **wie** die Multi-Tenancy-Migration umgesetzt wird: Reihenfolge, Arbeitsteilung, Git-Workflow und Risikomanagement. Es baut auf den Architektur-Entscheidungen (00-05, DEBATE.md) auf und setzt diese in einen konkreten Arbeitsplan um.

**Verwandte Dokumente:**
- [00-anforderungen.md](00-anforderungen.md) - Was gebaut wird
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema & RLS
- [03-backend.md](03-backend.md) - Backend-Implementierung
- [04-frontend.md](04-frontend.md) - Frontend-Implementierung
- [05-testing.md](05-testing.md) - Test-Strategie

---

## 1. Rahmenbedingungen

### 1.1 Ausgangssituation

- **1 Tenant in Production** auf einem Server (bestehende OGS)
- **2 Entwickler** arbeiten an der Migration
- **~350 Dateien** betroffen (58 Tabellen, 54 Repositories, 100+ Frontend-Routes)
- Die Anwendung muss waehrend der gesamten Migration fuer den bestehenden Tenant funktionieren
- Keine Feature-Entwicklung parallel geplant — beide Entwickler fokussieren sich auf Multi-Tenancy

### 1.2 Strategie-Entscheidungen

| Entscheidung | Begruendung |
|---|---|
| **Expand-Contract** fuer DB-Migrationen | App bleibt zu jedem Zeitpunkt funktionsfaehig (loest C-5) |
| **Long-Lived Feature Branch** | Production-Tenant wird nicht gefaehrdet bis zum finalen Cut-Over |
| **Phasenweises Arbeiten** | Shared Infrastructure zuerst (sequenziell), dann Domain-Verticals (parallel) |
| **RLS als Letztes aktivieren** | Verhindert stille Zero-Row-Results bei unvollstaendiger Migration |

---

## 2. Migrations-Strategie: Expand-Contract

### 2.1 Warum Expand-Contract?

Der bisherige 13-Schritt-Plan (02-datenbank.md §8) hat eine Luecke: Nach `SET NOT NULL` (Schritt 6) aber vor Code-Deploy (Schritt 11) schlagen INSERTs des alten Codes fehl. Expand-Contract loest das durch einen `DEFAULT 1`:

```
EXPAND:    tenant_id NULLABLE + DEFAULT 1   →  alter Code funktioniert weiter
MIGRATE:   neuer Code schreibt tenant_id    →  beide Code-Versionen funktionieren
CONTRACT:  NOT NULL, DEFAULT weg, RLS an    →  erst wenn 100% migriert
```

Zu **keinem Zeitpunkt** ist die App kaputt.

### 2.2 DB-Migration pro Tabelle (revidierte Schritte)

```sql
-- Schritt 1: Spalte hinzufuegen (instant in PG 17, kein Lock)
ALTER TABLE {schema}.{table}
    ADD COLUMN tenant_id BIGINT DEFAULT 1
    REFERENCES platform.schools(id);

-- Schritt 2: Bestehende Daten backfillen (idempotent)
UPDATE {schema}.{table} SET tenant_id = 1 WHERE tenant_id IS NULL;

-- Schritt 3: CHECK Constraint (instant, kein Table-Scan)
ALTER TABLE {schema}.{table}
    ADD CONSTRAINT {table}_tenant_id_not_null
    CHECK (tenant_id IS NOT NULL) NOT VALID;

-- Schritt 4: Constraint validieren (Scan, aber kein Exclusive Lock)
ALTER TABLE {schema}.{table}
    VALIDATE CONSTRAINT {table}_tenant_id_not_null;

-- Schritt 5: NOT NULL setzen (instant in PG 12+ wenn valider CHECK existiert)
ALTER TABLE {schema}.{table}
    ALTER COLUMN tenant_id SET NOT NULL;

-- Schritt 6: DEFAULT entfernen (erst wenn ALLER Code tenant_id explizit setzt)
ALTER TABLE {schema}.{table}
    ALTER COLUMN tenant_id DROP DEFAULT;
```

**Schritte 1-5** passieren in Phase 2. **Schritt 6** passiert in Phase 4 (nach Code-Migration).

---

## 3. Git-Workflow

### 3.1 Branch-Struktur

```
development / main  ← Production (1 Tenant live)
  │
  └── feature/multi-tenancy  (Integrations-Branch)
        ├── mt/infrastructure       (kurzlebig → merge in feature/)
        ├── mt/db-migrations        (kurzlebig → merge in feature/)
        ├── mt/tenant-package       (kurzlebig → merge in feature/)
        ├── mt/domain-users         (kurzlebig → merge in feature/)
        ├── mt/domain-education     (kurzlebig → merge in feature/)
        ├── mt/domain-active        (kurzlebig → merge in feature/)
        ├── mt/frontend-routing     (kurzlebig → merge in feature/)
        └── ...
```

### 3.2 Regeln

| Regel | Begruendung |
|---|---|
| Sub-Branches (`mt/*`) leben **max 5 Tage** | Laengere Branches erzeugen Branch-Divergenz auf zwei Ebenen |
| Merge in `feature/multi-tenancy` alle **2-3 Tage** | Konflikte frueh erkennen, Integration testen |
| `feature/multi-tenancy` von `development` **woechentlich rebasen** | Falls Hotfixes in Production landen, muessen sie in den Feature-Branch |
| Kein direkter Commit auf `feature/multi-tenancy` | Immer ueber Sub-Branch + Review |
| Finaler Merge `feature/multi-tenancy → development` erst wenn **alles fertig + getestet** | Production-Tenant wird nicht gefaehrdet |

### 3.3 Merge-Konflikt-Hotspots

| Datei | Risiko | Mitigation |
|---|---|---|
| `repositories/factory.go` | Beide Devs fuegen Methoden hinzu | Append-only, ein Dev merged zuerst |
| `services/factory.go` | Wie oben | Wie oben |
| `database/migrations/*` | Versions-Reihenfolge | Nur 1 Dev schreibt Migrations, Nummern vorab festlegen |
| `base/model.go` | Kern-Infrastruktur | Nur Phase 1, ein Entwickler |
| `api/` Router-Setup | Beide Devs fuegen Routes hinzu | Domain-Split minimiert Ueberlappung |

### 3.4 Rebase-Workflow bei Hotfixes

Wenn waehrend der Migration ein Hotfix in `development` landet:

```bash
# 1. Feature-Branch aktualisieren
git checkout feature/multi-tenancy
git rebase development

# 2. Alle aktiven Sub-Branches rebasen
git checkout mt/domain-users
git rebase feature/multi-tenancy
```

**Wichtig:** Rebase statt Merge verwenden, damit die Git-History linear bleibt und der finale Merge sauber ist.

---

## 4. Phasen-Plan

### Phase 1: Fundament (~1 Woche)

**Modus:** Sequenziell — 1 Entwickler baut, 1 reviewt

**Begruendung:** Alles andere haengt von diesen Primitiven ab. Paralleles Arbeiten hier erzeugt taegliche Merge-Konflikte.

#### Arbeitspakete

| # | Paket | Dateien | Abhaengigkeiten |
|---|---|---|---|
| 1.1 | `backend/tenant/` Package (Context Helpers, WithTenantTx, WithAdminTx) | ~5 neue Dateien | Keine |
| 1.2 | `models/base/tenant.go` (TenantModel Mixin) | 1 neue Datei | Keine |
| 1.3 | `platform.organizations` + `platform.schools` Tabellen + Models | 2 Migrations, 2 Models | Keine |
| 1.4 | `auth.account_tenants` Tabelle + Model | 1 Migration, 1 Model | 1.3 |
| 1.5 | JWT Claims erweitern (tenant_id, org_id, scope) | ~3 Dateien | 1.1 |
| 1.6 | Tenant-Middleware (Context aus JWT befuellen) | 1 Datei | 1.1, 1.5 |
| 1.7 | Test-Fixtures erweitern (CreateTestTenant, CreateTestStudentInTenant etc.) | ~2 Dateien | 1.3 |
| 1.8 | `getDB(ctx)` Helper im Base-Repository | 1 Datei | 1.1 |
| 1.9 | `assertRowsAffected` Helper | 1 Datei | Keine |

#### Ergebnis

- Neuer Code, nichts Bestehendes geaendert
- `WithTenantTx`/`WithAdminTx` sind verfuegbar aber noch nirgends aufgerufen
- Platform-Tabellen existieren
- Test-Fixtures koennen Tenants erstellen

#### Abnahme-Kriterien

- [ ] `go build ./...` erfolgreich
- [ ] `go test ./...` gruen (bestehende Tests unveraendert)
- [ ] Neuer Test: `WithTenantTx` setzt `current_setting('app.current_tenant_id')`
- [ ] Neuer Test: `WithAdminTx` hat BYPASSRLS
- [ ] Neuer Test: Query ohne Transaktion schlaegt fehl (phoenix_auth NOINHERIT)

---

### Phase 2: Datenbank Expand (~1 Woche)

**Modus:** Sequenziell — Pair-Programming (beide Entwickler zusammen)

**Begruendung:** Hoechstes Risiko. Migrations-Reihenfolge ist kritisch. Fehler hier betreffen alle folgenden Phasen. Vier Augen sehen mehr.

#### Arbeitspakete

| # | Paket | Umfang | Abhaengigkeiten |
|---|---|---|---|
| 2.1 | PostgreSQL-Rollen erstellen (phoenix_auth, phoenix_tenant, phoenix_admin) | 1 Migration | 1.3 |
| 2.2 | `tenant_id DEFAULT 1` auf 58 Tabellen | 1-3 Migrations | 2.1 |
| 2.3 | Backfill: `UPDATE SET tenant_id = 1` | Teil von 2.2 | 2.2 |
| 2.4 | `CHECK NOT VALID` + `VALIDATE` + `SET NOT NULL` | Teil von 2.2 | 2.3 |
| 2.5 | UNIQUE Constraints migrieren (31 Constraints) | 1-2 Migrations | 2.2 |
| 2.6 | `UNIQUE(tenant_id, id)` auf 18 Ziel-Tabellen (Vorbereitung fuer Composite FKs) | 1 Migration | 2.2 |
| 2.7 | Indexes erstellen (`idx_{table}_tenant`) | 1 Migration | 2.2 |
| 2.8 | BUN Model Tags anpassen (5 Models: `unique` entfernen) | 5 Dateien | 2.5 |
| 2.9 | `account_tenants` befuellen (alle bestehenden Accounts → Tenant 1) | 1 Migration | 1.4, 2.2 |

**NICHT in Phase 2:**
- Composite FKs (64 Stk.) — kommen in Phase 4 (erfordern dass aller Code tenant_id korrekt setzt)
- RLS Policies — kommen in Phase 4
- `DROP DEFAULT` — kommt in Phase 4

#### Ergebnis

- Alle Tabellen haben `tenant_id NOT NULL DEFAULT 1`
- Composite UNIQUE Constraints stehen
- PostgreSQL-Rollen existieren
- **Alter Code funktioniert unveraendert** (DEFAULT 1 faengt fehlende tenant_id ab)
- Ab jetzt: Tests mit 2 Tenants moeglich

#### Abnahme-Kriterien

- [ ] `go test ./...` gruen (bestehende Tests unveraendert, DEFAULT 1 wirkt)
- [ ] Manueller Test: INSERT ohne tenant_id bekommt DEFAULT 1
- [ ] Manueller Test: 2 Tenants angelegt, Daten in beiden sichtbar
- [ ] `go run main.go migrate validate` erfolgreich
- [ ] Kein bestehender Bruno-Test bricht

---

### Phase 3: Domain-Verticals (~2-3 Wochen)

**Modus:** Parallel — 2 Entwickler an verschiedenen Domains

**Begruendung:** Nachdem das Fundament steht, koennen Domains unabhaengig voneinander migriert werden. Jeder Entwickler besitzt seine Domains komplett (Model → Repo → Service → Handler).

#### Domain-Aufteilung

| Dev A | Dev B |
|---|---|
| **users/** (persons, students, staff, teachers, guests, guardians, rfid_cards, privacy_consents) | **education/** (groups, group_teacher, substitutions, grade_transitions) |
| **auth/** (tokens, roles, account_roles, account_permissions, invitation_tokens, guardian_invitations) | **facilities/** (rooms) |
| **active/** (groups, visits, attendance, supervisors, combined_groups, work_sessions, scheduled_checkouts, staff_absences) | **activities/** (categories, groups, schedules, supervisors, enrollments) |
| **audit/** (data_deletions, auth_events, data_imports, work_session_edits) | **schedule/** (timeframes, dateframes, recurrence_rules, pickup_schedules, pickup_exceptions, pickup_notes) |
| **suggestions/** (posts, votes, comments, reads) | **iot/** (devices) |
| **config/** (settings) | **feedback/** (entries) |
| SSE Hub (Tenant-prefixed Keys) | |

**Warum diese Aufteilung:**
- `active/` ist das groesste und komplexeste Domain — braucht einen vollen Dev
- `users/` + `auth/` haengen eng zusammen (Login-Flow, JWT, Rollen)
- `education/` + `activities/` + `schedule/` sind inhaltlich verwandt
- Die Aufteilung minimiert Cross-Domain-Abhaengigkeiten waehrend der Migration

#### Pro Domain: Migrations-Checkliste

Fuer jede Domain fuehrt der zustaendige Entwickler folgende Schritte durch:

```
1. Model: base.TenantModel einbetten
2. Repository: r.db → r.getDB(ctx) (alle Query-Methoden)
3. Repository: WHERE tenant_id = ? in alle Queries (Defense-in-Depth)
4. Repository: RowsAffected-Checks bei UPDATE/DELETE
5. Service: SetTenantID(tenant.FromContext(ctx)) vor Create-Calls
6. Service: RunInTx entfernen (Transaction-Ownership → Handler)
7. Service: WithTx-Patterns entfernen (tx kommt aus Context)
8. Handler: WithTenantTx/WithAdminTx wrappen
9. Tests: Isolation-Tests (Tenant A sieht Tenant B nicht)
10. Tests: RowsAffected-Tests (Cross-Tenant Update schlaegt fehl)
```

#### Aufwand pro Domain-Typ

| Domain-Typ | Geschaetzter Aufwand | Beispiele |
|---|---|---|
| Einfach (wenig Custom-Queries) | 1-2 Tage | rooms, settings, feedback, config |
| Mittel (Custom-Queries + Joins) | 2-4 Tage | students, groups, education, suggestions |
| Komplex (Cross-Schema-Joins, Business-Logic) | 4-7 Tage | active (visits + attendance + sessions), auth (Login-Flow + Rollen) |

#### Reihenfolge innerhalb Phase 3

Nicht alle Domains koennen gleichzeitig gestartet werden. Empfohlene Reihenfolge:

```
Woche 1:  Dev A: users/ (Basis fuer fast alles)
          Dev B: facilities/ + education/ (einfach, schnelle Erfolge)

Woche 2:  Dev A: auth/ (Login-Flow, JWT, Rollen-Migration)
          Dev B: activities/ + schedule/ + iot/

Woche 3:  Dev A: active/ + SSE Hub (komplex, braucht users/ + auth/)
          Dev B: suggestions/ + feedback/ + config/ + audit/
```

#### Merge-Rhythmus in Phase 3

```
Mo: Sub-Branch erstellen (mt/domain-X)
Mi: Zwischenstand committen, ggf. Review
Fr: Merge in feature/multi-tenancy, naechsten Sub-Branch starten

Freitag-Ritual: Beide Devs rebasen feature/multi-tenancy,
                lassen go test ./... laufen, besprechen Konflikte
```

#### Ergebnis

- Alle Repositories nutzen `getDB(ctx)` statt `r.db`
- Alle Services setzen `tenant_id` explizit
- Alle Handler wrappen mit `WithTenantTx`/`WithAdminTx`
- Isolation-Tests fuer jede Domain vorhanden
- `DEFAULT 1` faengt noch immer fehlende tenant_id ab (Safety Net)

#### Abnahme-Kriterien

- [ ] `go test ./...` gruen (alle alten + neuen Tests)
- [ ] Jede Domain hat mindestens 1 Isolation-Test
- [ ] Kein `r.db.NewSelect()` mehr in tenant-scoped Repositories (nur noch `r.getDB(ctx)`)
- [ ] Kein `s.db.RunInTx()` mehr in Services (Transaction-Ownership bei Handlern)
- [ ] Bruno-Tests gruen mit Tenant-Context

---

### Phase 4: Frontend, RLS & Finalisierung (~1-2 Wochen)

**Modus:** Mixed — Frontend parallel aufteilbar, RLS-Aktivierung sequenziell

#### Arbeitspakete

| # | Paket | Modus | Abhaengigkeiten |
|---|---|---|---|
| 4.1 | Next.js Middleware (Subdomain-Rewrite) | 1 Dev | Keine |
| 4.2 | `[tenant]/layout.tsx` + TenantProvider + resolveTenant | 1 Dev | 4.1 |
| 4.3 | Login-Page: tenant_slug im Body | 1 Dev | 4.2, Auth-Backend aus Phase 3 |
| 4.4 | useTenantSWR Migration (821 Stellen) | Beide parallel | 4.2 |
| 4.5 | useTenantRouter Migration (40+ Stellen) | 1 Dev | 4.2 |
| 4.6 | Session-Cache Tenant-Awareness | 1 Dev | 4.2 |
| 4.7 | Tenant-Switcher Komponente | 1 Dev | 4.3 |
| 4.8 | Cookie-Domain Konfiguration | 1 Dev | 4.1 |
| 4.9 | RLS Policies aktivieren (alle 58+1 Tabellen) | Pair-Programming | Phase 3 komplett |
| 4.10 | Composite FKs erstellen (64 FKs) | Pair-Programming | 4.9 |
| 4.11 | `DEFAULT 1` entfernen (alle 58 Tabellen) | Pair-Programming | 4.9, 4.10 |
| 4.12 | Views mit `security_invoker = true` | 1 Dev | 4.9 |
| 4.13 | Advisory Locks auf Zwei-Argument-Form | 1 Dev | Phase 3 |
| 4.14 | Avatar-Upload Tenant-Namespacing | 1 Dev | Phase 3 |
| 4.15 | Scheduler/Background-Jobs Tenant-Strategie | 1 Dev | Phase 3 |
| 4.16 | Seed-Daten fuer 2 Test-Tenants | 1 Dev | Phase 2 |
| 4.17 | End-to-End Testing (Bruno Multi-Tenant Suite) | Beide | Alles |

**Kritische Reihenfolge:**
1. Zuerst 4.9 (RLS) — das ist der "Flip the Switch"-Moment
2. Dann 4.10 (Composite FKs) — erst moeglich wenn aller Code tenant_id korrekt setzt
3. Dann 4.11 (DEFAULT weg) — erst wenn RLS + FKs stabil

#### Warum RLS als Letztes?

Wenn RLS aktiviert wird bevor aller Code tenant-aware ist, geben Queries **leise 0 Rows** zurueck statt einen Fehler zu werfen. Das sieht aus wie "keine Daten vorhanden", nicht wie ein Bug. Deswegen:

```
FALSCH:  RLS an → Code migrieren → Bugs als "leere Daten" sehen
RICHTIG: Code migrieren → RLS an → Bugs als "Permission Denied" sehen
```

#### Ergebnis

- Frontend tenant-aware (Subdomain-Routing, TenantProvider, Tenant-Switch)
- RLS aktiv auf allen Tabellen
- Composite FKs erzwingen Cross-Tenant-Integritaet
- DEFAULT 1 entfernt
- End-to-End getestet mit 2 Tenants

---

## 5. Cut-Over: feature/multi-tenancy → development → Production

### 5.1 Voraussetzungen

- [ ] Alle Phasen abgeschlossen
- [ ] `go test ./...` gruen
- [ ] `pnpm run check` gruen (Zero Warnings)
- [ ] Bruno Multi-Tenant Suite gruen
- [ ] Isolation-Tests fuer alle Domains vorhanden
- [ ] Bestehender Tenant als `platform.schools` ID=1 konfiguriert
- [ ] Alle bestehenden Accounts in `auth.account_tenants` (Tenant 1)
- [ ] Seed-Daten fuer lokale Entwicklung funktionieren
- [ ] PostgreSQL-Version >= 17.1 auf Production verifiziert

### 5.2 Merge-Reihenfolge

```
1. feature/multi-tenancy von development rebasen (letztes Mal)
2. Alle Tests laufen lassen
3. PR: feature/multi-tenancy → development (Review durch beide Devs)
4. Merge
5. development auf Staging deployen + verifizieren
6. development → main (Production-Deploy)
```

### 5.3 Production-Deploy Ablauf

```
1. Wartungsfenster ankuendigen (30-60 Minuten)
2. Maintenance-Mode aktivieren
3. Datenbank-Backup erstellen
4. Migrations ausfuehren:
   a. Platform-Tabellen erstellen (organizations, schools)
   b. PostgreSQL-Rollen erstellen
   c. tenant_id auf 58 Tabellen (DEFAULT 1 + Backfill + NOT NULL)
   d. UNIQUE Constraints migrieren
   e. Indexes erstellen
   f. account_tenants befuellen
   g. RLS Policies aktivieren
   h. Composite FKs erstellen
   i. DEFAULT 1 entfernen
   j. Views mit security_invoker aktualisieren
5. Neuen Code deployen (Backend + Frontend)
6. Smoke-Test: Login, Dashboard, Schueler-Liste
7. Maintenance-Mode deaktivieren
```

### 5.4 Rollback-Plan

Falls nach dem Deploy Probleme auftreten:

```
Sofort (< 5 Min):
  → Code-Rollback auf vorherige Version
  → RLS Policies auf USING(true) setzen
  → ALTER COLUMN tenant_id SET DEFAULT 1
  → Alter Code funktioniert wieder (DEFAULT 1 + keine RLS)

Danach (in Ruhe analysieren):
  → Fehler identifizieren und fixen
  → Erneuter Deploy-Versuch planen
```

---

## 6. Risiken & Mitigationen

| Risiko | Schwere | Mitigation |
|---|---|---|
| Merge-Konflikte bei finalem Merge | MITTEL | Woechentlich von development rebasen, Sub-Branches kurz halten |
| Vergessener tenant_id Filter in Repository | HOCH | RLS als Safety-Net, Isolation-Tests, CI-Check (`grep` fuer `r.db.New` in tenant-scoped Repos) |
| Production-Migration schlaegt fehl | HOCH | Datenbank-Backup, Rollback-Plan, Staging-Verifizierung |
| Stille Zero-Rows durch zu fruehe RLS | HOCH | RLS erst in Phase 4 nach Code-Migration aktivieren |
| DEFAULT 1 zu frueh entfernt | MITTEL | DEFAULT 1 bleibt bis Phase 4, expliziter Schritt |
| Cross-Domain-Abhaengigkeiten in Phase 3 | MITTEL | Reihenfolge innerhalb Phase 3 beachten (users/ vor active/) |
| Bestehende Tests brechen | NIEDRIG | DEFAULT 1 + bestehende Fixtures nutzen Default-Tenant |

---

## 7. Kommunikation waehrend der Migration

### Taegliches Sync (5 Min)

- Was habe ich gestern gemerged?
- Woran arbeite ich heute?
- Gibt es Konflikte oder Blocker?

### Freitags-Ritual (30 Min)

1. Beide Devs rebasen `feature/multi-tenancy` von `development`
2. `go test ./...` + `pnpm run check`
3. Offene Sub-Branches reviewen und mergen
4. Naechste Woche planen: Welche Domains stehen an?

### Entscheidungsprinzip

Bei Unklarheiten waehrend der Implementierung:
- **Architektur-Fragen:** Antwort in DEBATE.md suchen. Wenn nicht vorhanden → besprechen und DEBATE.md ergaenzen
- **Implementierungs-Details:** Der Domain-Owner entscheidet
- **Cross-Domain-Fragen:** Im Taeglichen Sync klaeren

---

## 8. Definition of Done

Die Multi-Tenancy-Migration ist abgeschlossen wenn:

- [ ] Alle 58 Tabellen haben `tenant_id NOT NULL` (kein DEFAULT)
- [ ] RLS-Policies aktiv auf allen tenant-scoped Tabellen
- [ ] 64 Composite FKs erstellt
- [ ] 31 UNIQUE Constraints migriert
- [ ] Alle Repositories nutzen `getDB(ctx)` + Defense-in-Depth WHERE
- [ ] Alle Services setzen tenant_id explizit via SetTenantID()
- [ ] Alle Handler wrappen mit WithTenantTx/WithAdminTx
- [ ] JWT enthaelt tenant_id, org_id, scope
- [ ] Frontend: Subdomain-Routing funktioniert
- [ ] Frontend: useTenantSWR auf allen SWR-Calls
- [ ] Isolation-Tests fuer jede Domain vorhanden
- [ ] Bruno Multi-Tenant Suite gruen
- [ ] Ein zweiter Test-Tenant kann angelegt und genutzt werden
- [ ] Bestehender Production-Tenant funktioniert nach Migration

---

## 9. Aenderungshistorie

| Datum | Aenderung |
|---|---|
| 2026-02-11 | Initiale Version basierend auf Architektur-Dokumenten (00-05), DEBATE.md, Review-Findings und Best-Practice-Research (Expand-Contract Pattern, Vertical Slices, Git-Workflow) |
