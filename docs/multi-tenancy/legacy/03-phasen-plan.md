# Phasenplan: Multi-Tenancy Migration

## Uebersicht: 6 Phasen

```
Phase 0: Fundament (1-2 Wochen)         <- Alle zusammen
Phase 1: Datenbank-Schema (2-3 Wochen)  <- Backend-Developer
Phase 2: Backend-Core (3-4 Wochen)      <- Backend-Developer
Phase 3: Frontend-Core (2-3 Wochen)     <- Frontend-Developer
Phase 4: Operator Dashboard (2-3 Wochen)<- Full-Stack Developer
Phase 5: Migration & Launch (2-3 Wochen)<- Alle zusammen
```

**Gesamt-Zeitrahmen: ~14-18 Wochen (3.5-4.5 Monate)**

**Wichtig:** Jede Phase hat klare Deliverables und Schnittstellen. Die Phasen 1-3 koennen teilweise parallel laufen.

---

## Phase 0: Fundament & Interfaces definieren (Woche 1-2)

### Ziel
Alle Developer haben das gleiche Verstaendnis. Schnittstellen sind definiert. Feature-Branch existiert.

### Tasks

- [x] **0.1** Feature-Branch `feature/multi-tenancy` von `development` erstellen
- [ ] **0.2** Tenant-Context Package definieren (`backend/tenant/`)
  - `TenantFromContext(ctx) int64`
  - `WithTenantID(ctx, id) context.Context`
  - `OrgFromContext(ctx) int64`
- [ ] **0.3** JWT Claims Interface definieren (neues `TenantID` + `OrgID` Feld)
- [ ] **0.4** API Contract definieren: Wie wird Tenant an Backend uebermittelt?
  - Header: `X-Tenant-ID` oder `X-Tenant-Slug`
  - JWT Claim: `tenant_id`
- [ ] **0.5** RLS-Strategie dokumentieren und vom Team reviewen lassen
- [ ] **0.6** Test-Strategie: Wie werden Multi-Tenant-Tests geschrieben?
- [ ] **0.7** Migration-Strategie fuer bestehende Production-Daten

### Deliverables
- `docs/multi-tenancy/` (diese Dokumente)
- `backend/tenant/` Package (leerer Skeleton mit Interfaces)

### Wer
**Alle Developer zusammen** (Kickoff-Meeting + gemeinsames Design)

---

## Phase 1: Datenbank-Schema (Woche 3-5)

### Ziel
Alle Tabellen haben `tenant_id`. RLS-Policies sind aktiv. Bestehende Daten sind migriert.

### Tasks

- [ ] **1.1** Migration: `platform.organizations` Tabelle erstellen
- [ ] **1.2** Migration: `platform.schools` Tabelle erstellen (= Tenants)
- [ ] **1.3** Migration: `tenant_id BIGINT` zu ALLEN bestehenden Tabellen hinzufuegen
  - `auth.accounts`
  - `users.persons`, `users.staff`, `users.students`, `users.teachers`, `users.guardians`, `users.rfid_cards`, `users.profiles`, `users.privacy_consents`
  - `education.groups`, `education.group_substitutions`
  - `facilities.rooms`
  - `activities.*` (categories, groups, schedules, supervisors_planned, student_enrollments)
  - `active.*` (groups, visits, group_supervisors, combined_groups, group_mappings, attendance)
  - `schedule.*` (timeframes, dateframes, recurrence_rules)
  - `iot.devices`
  - `feedback.entries`
  - `config.settings`
  - `suggestions.*` (posts, comments, comment_reads, post_reads)
  - `audit.*`
- [ ] **1.4** Default-Tenant erstellen und allen bestehenden Rows zuweisen
  ```sql
  INSERT INTO platform.organizations (name, slug) VALUES ('Default Org', 'default');
  INSERT INTO platform.schools (organization_id, name, slug, subdomain)
      VALUES (1, 'Aktuelle OGS', 'default', 'default');
  UPDATE auth.accounts SET tenant_id = 1;
  UPDATE users.persons SET tenant_id = 1;
  -- ... fuer alle Tabellen
  ```
- [ ] **1.5** `tenant_id NOT NULL` Constraint setzen (nach Daten-Migration)
- [ ] **1.6** Foreign Key: `tenant_id -> platform.schools(id)` fuer alle Tabellen
- [ ] **1.7** Composite Indexes: `(tenant_id, id)` fuer alle Tabellen
- [ ] **1.8** RLS-Policies erstellen fuer ALLE Tabellen
- [ ] **1.9** RLS aktivieren: `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`
- [ ] **1.10** Migration: `platform.cross_tenant_access` Tabelle
- [ ] **1.11** Migration: `platform.operator_organizations` Tabelle
- [ ] **1.12** Migration: `auth.account_tenants` Junction-Table (Account -> Tenant Mapping)
- [ ] **1.13** Tests: RLS-Policies mit verschiedenen Tenants verifizieren

### Deliverables
- Neue Migration-Dateien (ca. 5-8 Dateien)
- RLS-Policy SQL-Scripts
- Migration-Test-Suite

### Wer
**Backend-Developer 1** (DB-Spezialist)

### Abhaengigkeiten
- Phase 0 muss abgeschlossen sein

### Risiken
- Production-Migration muss Downtime-frei sein -> `ALTER TABLE ADD COLUMN` ist non-blocking in PG
- `NOT NULL` Constraint kann erst nach Data-Backfill gesetzt werden

---

## Phase 2: Backend Core (Woche 4-8)

### Kann parallel zu Phase 1 beginnen (nach 1.1-1.3)

### Ziel
Alle Backend-Layers sind tenant-aware. JWT enthaelt `tenant_id`. Middleware extrahiert Tenant.

### Sub-Phase 2a: Core-Infrastructure (Woche 4-5)

- [ ] **2a.1** `backend/tenant/` Package implementieren
  - Context-Helpers
  - Middleware
  - BUN Query-Hook fuer RLS `SET LOCAL`
- [ ] **2a.2** JWT Claims erweitern (`TenantID`, `OrgID`)
- [ ] **2a.3** Token-Generierung erweitert: `tenant_id` aus Account-Tenant-Mapping laden und in JWT packen
- [ ] **2a.4** Tenant-Middleware: JWT -> Context -> `tenant_id`
- [ ] **2a.5** Login-Flow anpassen:
  - Subdomain/Header -> Tenant-Lookup
  - Account-Suche mit Tenant-Filter (ueber `auth.account_tenants`)
  - JWT mit `tenant_id` generieren
- [ ] **2a.6** Refresh-Flow anpassen: `tenant_id` aus altem Token uebernehmen
- [ ] **2a.7** Factory-Pattern erweitern: Tenant-Hook in DB injizieren
- [ ] **2a.8** Base-Model erweitern: `TenantID int64` Feld

### Sub-Phase 2b: Repository-Layer (Woche 5-7)

- [ ] **2b.1** `base.Model` um `TenantID` erweitern
- [ ] **2b.2** `QueryOptions` um Tenant-Filter erweitern (automatisch)
- [ ] **2b.3** Alle Repositories: `tenant_id` in Create-Operationen setzen
- [ ] **2b.4** Alle Repositories: `tenant_id` in WHERE-Clauses einfuegen (Defense-in-Depth)
  - `auth/` Repositories
  - `users/` Repositories
  - `education/` Repositories
  - `facilities/` Repositories
  - `activities/` Repositories
  - `active/` Repositories
  - `schedule/` Repositories
  - `iot/` Repositories
  - `feedback/` Repositories
  - `config/` Repositories
  - `suggestions/` Repositories
- [ ] **2b.5** IoT Device-Auth: Device -> Tenant Mapping

### Sub-Phase 2c: Service & Handler Layer (Woche 6-8)

- [ ] **2c.1** Services: Tenant-Context durchreichen
- [ ] **2c.2** Handlers: Tenant aus JWT-Claims extrahieren
- [ ] **2c.3** Operator-Endpoints: Tenant-Bypass fuer Platform-Scope
- [ ] **2c.4** SSE/Realtime: Tenant-Isolation in Hub
- [ ] **2c.5** Seed-Data: Multi-Tenant-faehig machen
- [ ] **2c.6** Alle bestehenden Tests auf Multi-Tenant umstellen
- [ ] **2c.7** Neue Tests: Cross-Tenant-Isolation verifizieren

### Deliverables
- `backend/tenant/` Package
- Erweiterte JWT Claims
- Tenant-Middleware
- Alle Repositories tenant-aware
- Alle Services tenant-aware
- Erweiterte Test-Suite

### Wer
**Backend-Developer 1 + 2** (aufgeteilt nach Domains)

**Vorschlag fuer Aufteilung:**
- Dev A: `tenant/`, `auth/`, `users/`, `education/`, `active/` (Kern-Domains)
- Dev B: `facilities/`, `activities/`, `schedule/`, `iot/`, `feedback/`, `config/`, `suggestions/`

### Abhaengigkeiten
- Phase 1 (Migrations) muss fuer die entsprechenden Tabellen abgeschlossen sein
- Phase 0 (Interfaces) muss abgeschlossen sein

---

## Phase 3: Frontend Core (Woche 6-9)

### Kann parallel zu Phase 2b beginnen

### Ziel
Frontend kann mit Subdomains umgehen. Auth-Flow enthaelt Tenant. API-Calls leiten Tenant weiter.

### Sub-Phase 3a: Infrastructure (Woche 6-7)

- [ ] **3a.1** Next.js Middleware: Subdomain-Extraktion
- [ ] **3a.2** Tenant-Context Provider implementieren
- [ ] **3a.3** Tenant-Auswahl-Seite (Root-Domain ohne Subdomain)
- [ ] **3a.4** Environment-Variablen erweitern (`TENANT_DOMAIN`, etc.)
- [ ] **3a.5** `env.js` (t3-env) um Tenant-Variablen erweitern

### Sub-Phase 3b: Auth-Integration (Woche 7-8)

- [ ] **3b.1** NextAuth Config: `tenant_id` in Session/Token speichern
- [ ] **3b.2** Login-Page: Subdomain an Backend-Login senden
- [ ] **3b.3** Operator-Auth: Bleibt cross-tenant (kein Tenant-Binding)
- [ ] **3b.4** Token-Refresh: `tenant_id` beibehalten
- [ ] **3b.5** Session-Provider: Tenant in User-Object

### Sub-Phase 3c: API-Layer (Woche 8-9)

- [ ] **3c.1** Route-Wrapper: `X-Tenant-ID` Header automatisch setzen
- [ ] **3c.2** Operator Route-Wrapper: Kein Tenant-Header (Platform-Scope)
- [ ] **3c.3** Axios Interceptor: Tenant-Header bei Client-Side Calls
- [ ] **3c.4** SWR Cache-Keys: Tenant-Prefix hinzufuegen
- [ ] **3c.5** SSE-Connection: Tenant-Info mitgeben
- [ ] **3c.6** Alle API-Client-Libraries: Tenant-Context nutzen

### Deliverables
- Subdomain-Middleware
- Tenant-Provider
- Tenant-Auswahl-Seite
- Erweiterte Auth-Integration
- Alle API-Routes tenant-aware

### Wer
**Frontend-Developer** (1 Person)

### Abhaengigkeiten
- Phase 0 (Interfaces) muss abgeschlossen sein
- Phase 2a (JWT Changes) muss abgeschlossen sein (fuer Auth-Integration)

---

## Phase 4: Operator Dashboard Erweiterung (Woche 8-11)

### Kann parallel zu Phase 3 laufen

### Ziel
Operators koennen Tenants verwalten, neue OGS anlegen, Subdomains konfigurieren.

### Tasks

- [ ] **4.1** Backend: Organization CRUD API
- [ ] **4.2** Backend: School/Tenant CRUD API
- [ ] **4.3** Backend: Operator -> Organization Zuordnung
- [ ] **4.4** Frontend: Tenant-Management-Seite im Operator Dashboard
- [ ] **4.5** Frontend: Organization-Uebersicht (alle OGS eines Traegers)
- [ ] **4.6** Frontend: Neuen Tenant anlegen (Wizard: Name, Slug, Subdomain, Admin-Account)
- [ ] **4.7** Backend: Tenant-Provisioning (Schema-Seed fuer neuen Tenant)
- [ ] **4.8** Frontend: Cross-Tenant-Access Verwaltung (Ferienbetreuung)
- [ ] **4.9** Backend: OGS-spezifische Daten im Operator-Dashboard (Feedback, Statistiken per OGS)
- [ ] **4.10** Frontend: OGS-Auswahl im Operator-Dashboard (Dropdown/Filter fuer OGS-spezifische Ansichten)
- [ ] **4.11** Backend: Gezielte Announcements (an alle OGS oder bestimmte OGS)
- [ ] **4.12** Frontend: Announcement-Targeting UI (OGS-Auswahl bei Erstellung)

### Deliverables
- Tenant-Management CRUD
- Tenant-Provisioning-Workflow
- Cross-Tenant-Access UI
- OGS-spezifische Operator-Ansichten
- Gezieltes Announcement-Targeting

### Wer
**Beliebiger Developer** (muss Backend + Frontend koennen)

### Abhaengigkeiten
- Phase 1 (DB-Schema) abgeschlossen
- Phase 2a (Backend-Core) abgeschlossen

---

## Phase 5: Migration, Testing & Launch (Woche 10-13)

### Ziel
Production-Migration durchfuehren. Zweiten Tenant anlegen. Alles testen.

### Tasks

- [ ] **5.1** Staging-Umgebung mit Multi-Tenant-Setup aufsetzen
- [ ] **5.2** Production-Datenmigration testen (auf Staging-Kopie)
- [ ] **5.3** Wildcard-SSL-Zertifikat fuer `*.{TENANT_DOMAIN}` einrichten
- [ ] **5.4** DNS: Wildcard-Record `*.{TENANT_DOMAIN} -> Server-IP`
- [ ] **5.5** Caddy/Reverse-Proxy: Subdomain-Routing konfigurieren
- [ ] **5.6** Production-Migration durchfuehren (geplante Wartung)
- [ ] **5.7** Bestehende Schule als Tenant "altenberge" konfigurieren
- [ ] **5.8** Zweiten Pilot-Tenant anlegen und testen
- [ ] **5.9** PyrePortal: Devices dem Tenant zuordnen
- [ ] **5.10** End-to-End-Tests mit zwei Tenants
- [ ] **5.11** Performance-Tests: 100 Tenants simulieren
- [ ] **5.12** Security-Audit: Cross-Tenant-Isolation verifizieren
- [ ] **5.13** Rollback-Plan dokumentieren

### Deliverables
- Migrationsskripte
- SSL + DNS Setup
- Production-Migration abgeschlossen
- Pilot mit 2 Tenants laeuft
- Security-Audit bestanden

### Wer
**Alle Developer** + DevOps

---

## Parallelisierungs-Matrix

```
Woche:  1  2  3  4  5  6  7  8  9  10 11 12 13
Phase:
  0     ----                                       <- Alle
  1        ----------                              <- Backend Dev A
  2              ------------------                 <- Backend Dev A + B
  3                    --------------              <- Frontend Dev
  4                          --------------        <- Full-Stack Dev
  5                                   ------------ <- Alle
```

### Wer macht was (3 Developer)?

| Developer | Phase 0 | Phase 1 | Phase 2 | Phase 3 | Phase 4 | Phase 5 |
|-----------|---------|---------|---------|---------|---------|---------|
| **Dev A (Backend-Lead)** | Design | Migrations, RLS | Core Auth, Users, Active | Review | -- | Migration |
| **Dev B (Backend)** | Design | -- | Facilities, Activities, IoT, Config | -- | Operator Backend | Testing |
| **Dev C (Frontend)** | Design | -- | -- | Alle Frontend-Tasks | Operator Frontend | SSL/DNS |

---

## Kommunikations-Schnittstellen zwischen Developern

### API Contract (Backend <-> Frontend)

```yaml
# Jeder Request vom Frontend ans Backend MUSS:
headers:
  Authorization: "Bearer {jwt_with_tenant_id}"
  X-Tenant-Slug: "{subdomain}"  # Fallback, falls JWT noch nicht da (Login)

# JWT Claims enthalten nach Login:
{
  "id": 42,
  "sub": "lehrer@example.com",
  "tenant_id": 1,
  "org_id": 1,
  "roles": ["user"],
  "permissions": ["users:read", ...],
  "scope": ""
}

# Operator JWT Claims:
{
  "id": 99,
  "sub": "operator:99",
  "tenant_id": 0,      # 0 = kein Tenant (Platform)
  "org_id": 0,
  "roles": ["operator"],
  "permissions": [],
  "scope": "platform"
}
```

### Event: "Neuer Tenant erstellt"

```
1. Operator erstellt Tenant ueber Dashboard
2. Backend: INSERT platform.schools
3. Backend: Seed Basis-Daten (Default-Rollen, Permissions, Config)
4. Backend: Erstelle Admin-Account fuer die neue Schule
5. Frontend: Zeige neue Subdomain + Admin-Zugangsdaten
```

### Code-Review-Regel

**Jeder PR der eine Repository-Methode aendert MUSS geprueft werden auf:**
1. Hat die Query einen `tenant_id` Filter?
2. Wird bei `Create` die `tenant_id` gesetzt?
3. Gibt es einen Test der verifiziert, dass Tenant A nicht Daten von Tenant B sieht?
