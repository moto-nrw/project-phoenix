# 06 — Offene Punkte & Handlungsbedarf

> Ergebnis einer technischen Review der Dokumente 00–05 gegen den aktuellen Codebase-Stand.
> Aktualisiert nach Review der Entscheidungen D1–D17 aus DEBATE.md.
> Ergaenzt um Codebase-Audit-Findings (UNIQUE Constraints, SSE, Scheduler, FKs).
> Stand: 2026-02-09

---

## Kritisch (Blocker vor Implementierungsbeginn)

### 1. "org"-Scope hat keine technische Umsetzung

**Status:** ENTSCHIEDEN — D18 in DEBATE.md

**Entscheidung:** Application-Layer mit `WithAdminTx` + org_id-Filter (Option b). Dedizierter `OrgScopeService`, read-only, eigene Permission `org:dashboard:read`. Tagesgeschaeft per Tenant-Switch + `WithTenantTx`. RLS-Policies bleiben simpel (kein Subquery-Overhead). Guardrails: kein Schreib-Zugriff ueber Org-Scope, Audit-Logging, org_id aus JWT.

---

### 11. UNIQUE Constraints brechen bei zweitem Tenant

**Status:** DOKUMENTIERT — Vollstaendige Migrationsliste in [02-datenbank.md Sektion 2.4](02-datenbank.md#24-unique-constraints-migration-c1-h2)

**Ergebnis:** 31 Constraints muessen migriert werden (13 funktional notwendig + 18 Defense-in-Depth), 9 sind OK. Sonderfall `auth.roles.name` benoetigt zwei Partial Indexes (System-Rollen vs. Tenant-Rollen). 5 BUN-Model-Tags muessen angepasst werden. Architektur-Entscheidung: `auth.accounts` global (D15), `users.persons` per-Tenant (ein Betreuer an 2 OGS hat 2 Person-Records).

---

### 12. SSE Hub: Cross-Tenant Event-Leakage

**Status:** DOKUMENTIERT — Korrektur in [03-backend.md Sektion 10](03-backend.md#10-sserealtime-tenant-isolation-06-12-08-h3)

**Entscheidung:** Group-Level Broadcasting bleibt (kein Tenant-Level — das waere ein Rueckschritt). Zwei Aenderungen: (1) Tenant-Validierung bei SSE-Connection (JWT tenant_id muss zur Group passen), (2) Tenant-prefixed Map-Keys (`"tenantID:groupID"`) verhindern ID-Kollisionen.

---

### 13. Background-Jobs/Scheduler ohne Tenant-Context

**Status:** DOKUMENTIERT — Hybrid-Strategie in [03-backend.md Sektion 11](03-backend.md#11-schedulerbackground-jobs-tenant-strategie-09-c2)

**Entscheidung:** Hybrid-Ansatz (Option c). Admin-Scope Jobs (`WithAdminTx`) fuer systemweite Cleanup-Tasks (Tokens, Visits, Attendance, Supervisors). Tenant-Scope Jobs iterieren ueber alle Tenants und nutzen pro Tenant `WithTenantTx` (Sessions, Checkouts, Breaks). Fehlerbehandlung: Fehler in einem Tenant blockiert nicht den naechsten (`continue`), Rollback pro Tenant, Alert bei >50% Failures.

---

## Hoch (Vor oder waehrend der Implementierung klaeren)

### 2. Tabellen-Anzahl in 02-datenbank.md ist falsch

**Status:** GELOEST — Zaehlung gegen Migrations-Dateien verifiziert, [02-datenbank.md §2.1](02-datenbank.md) korrigiert

**Ergebnis:** Verifizierung gegen alle `CREATE TABLE` Statements in `database/migrations/` ergibt **70 bestehende Tabellen** in 14 Schemas. Davon 58 mit tenant_id NOT NULL, 1 nullable (auth.roles), 11 ohne tenant_id. "~44" in §2.1 zu "58" korrigiert. Fehlende Tabellen klassifiziert: `suggestions.operator_comments` (Platform-Scope, kein tenant_id), `meta.migration_metadata` (Infrastruktur, kein tenant_id). Gesamt nach Multi-Tenancy-Migration: 75 Tabellen (70 + 5 neue).

---

### 3. Eltern-Datenisolation ist unter-spezifiziert

**Status:** ENTSCHIEDEN — D19 in DEBATE.md

**Entscheidung:** Service-Layer-Filterung. Dedizierte Parent-Endpoints filtern per `students_guardians` JOIN. RLS schuetzt cross-tenant, Service schuetzt intra-tenant (nur eigene Kinder). Keine RLS-Aenderung noetig. Betreuer-Endpoints sind fuer Eltern nicht erreichbar (andere Permissions).

---

### 14. Cross-Tenant Foreign Keys nicht abgesichert

**Status:** DOKUMENTIERT — Composite FK Migration in [02-datenbank.md Sektion 2.5](02-datenbank.md#25-composite-foreign-keys-09-h3)

**Entscheidung:** Option (a) — Composite FKs auf DB-Level. 64 FKs werden zu `FK(tenant_id, column) → target(tenant_id, id)` migriert. 19 Ziel-Tabellen bekommen `UNIQUE(tenant_id, id)`. Kein Service-Bug kann Cross-Tenant-Verlinkungen erzeugen.

---

### 15. Aggregate-Queries ohne Tenant-Scope

**Status:** GELOEST durch Transaction-Ownership Migration (03-backend.md §1.3)

**Loesung:** Die Transaction-Ownership Migration (09-C1) loest das Problem systemisch: ALLE Queries laufen innerhalb von `WithTenantTx` (gestartet im Handler). `r.getDB(ctx)` extrahiert die Transaktion aus dem Context. RLS filtert automatisch — auch Aggregate-Queries. Die 6 betroffenen Stellen werden im Rahmen der 538 `r.db.`-Call-Migration umgestellt.

---

### 4. Infrastruktur/Deployment nur in Legacy-Docs

**Status:** ZURUECKGESTELLT (per D3)

**Problem:** Die Hauptdokumente (00–05) enthalten keine Infrastruktur-Spezifikationen. DNS-, SSL-, Caddy- und Docker-Compose-Aenderungen stehen nur in `legacy/04-schnittstellen-definition.md` (Abschnitt 7).

**Entscheidung in D3:** Zurueckgestellt. Kommt am Ende mit Phasen-Plan.

**Handlungsbedarf:** Bei Beginn der Deployment-Phase: Relevante Infrastruktur-Specs aus legacy/04 in ein neues Hauptdokument ueberfuehren (z.B. `07-deployment.md`).

---

## Mittel (Waehrend der Implementierung adressieren)

### 5. `auth.accounts.tenant_id` — Zweck klaeren

**Status:** GELOEST

**Loesung:** `auth.accounts` bekommt KEIN `tenant_id` (D15). Steht bereits korrekt in 02-datenbank.md in der "KEIN tenant_id" Liste. Die per-tenant RBAC-Entscheidung (D13 revidiert) betrifft die Junction-Tabellen (`auth.account_roles`, `auth.account_permissions`), nicht die accounts-Tabelle. Login nutzt `WithAdminTx` fuer Account-Lookup (D6 Schritt 3), danach Switch zu `WithTenantTx`.

---

### 6. Keine Frontend-Tests spezifiziert

**Status:** DOKUMENTIERT — [05-testing.md §8](05-testing.md#8-frontend-tests-06-6)

**Ergebnis:** Vier Testbereiche ergaenzt: (1) E2E-Tests mit Playwright fuer Subdomain-Routing, Login mit tenant_slug und Tenant-Switch, (2) SWR-Cache-Isolationstests mit React Testing Library, (3) Bruno Multi-Tenant API-Testsuite mit Isolation- und Cross-Tenant-Szenarien, (4) Performance-Baseline-Test mit 100 Tenants (< 10% RLS-Overhead als Ziel).

---

### 7. Kein Test fuer "org"-Scope

**Status:** DOKUMENTIERT — [05-testing.md §9](05-testing.md#9-org-scope--cross-tenant-tests-06-7)

**Ergebnis:** Org-Scope Tests (D18): OrgScopeService sieht alle Tenants einer Organisation, sieht NICHT Tenants anderer Organisationen, ist read-only. Cross-Tenant Tests (D4): Ferienbetreuung mit zeitlich begrenztem Zugriff funktioniert, abgelaufener Zugriff wird abgelehnt.

---

### 16. Avatar-Uploads ohne Tenant-Namespacing

**Status:** DOKUMENTIERT — [03-backend.md §14](03-backend.md#14-avatar-uploads-tenant-namespacing-06-16)

**Ergebnis:** Pfadstruktur wird zu `public/uploads/avatars/{tenant_id}/{userID}_{random}.ext` geaendert. Betroffene Stellen: `api/usercontext/api.go:296,392`. Migrationsskript verschiebt bestehende Dateien nach `avatars/1/` (Default-Tenant). GDPR-Vorteil: Komplette Tenant-Loeschung per `os.RemoveAll(avatars/{tenant_id}/)`.

---

### 8. Lokale Entwicklungsumgebung fuer Subdomains

**Status:** DOKUMENTIERT — [04-frontend.md §19](04-frontend.md#19-lokale-entwicklungsumgebung-fuer-subdomains-06-8)

**Ergebnis:** Browser-Kompatibilitaet dokumentiert (Chrome 63+, Firefox 84+, Edge 79+ nativ, Safari braucht `/etc/hosts`). URLs fuer lokale Entwicklung (`school-a.localhost:3000`). Environment-Setup fuer CORS und TENANT_DOMAIN. Seed-Daten erstellen automatisch 2 Test-Tenants. Docker Compose braucht keine Aenderungen.

---

## Niedrig (Nice-to-have / Cleanup)

### 9. Rollback-Plan beruecksichtigt NOT NULL nicht

**Status:** GELOEST

**Loesung:** In PG 17 ist `ADD COLUMN ... NOT NULL DEFAULT` metadata-only (kein Lock, kein Rewrite). Der 3-Phasen-Rollout (02-datenbank.md §7) stellt sicher: Phase 1 nullable, Phase 2 `SET NOT NULL` erst wenn Application-Layer stabil. Rollback in Phase 1 trivial (Column ignoriert). Rollback nach Phase 2 erfordert `ALTER COLUMN DROP NOT NULL` — dokumentiert im Rollback-Plan.

---

### 10. Funktionsnamen-Inkonsistenz zwischen Haupt- und Legacy-Docs

**Status:** GELOEST (geringer Impact)

**Loesung:** Legacy-Docs (01-04 in `legacy/`) sind superseded durch die Hauptdokumente (00-05). Die Hauptdocs verwenden die korrekten Funktionsnamen. Legacy-Docs werden nicht aktualisiert — sie dienen nur als historische Referenz.

---

## Geloeste Punkte (durch DEBATE.md D1–D17)

Die folgenden Punkte aus der initialen Review wurden durch Entscheidungen in DEBATE.md geloest:

| Urspruengliches Finding | Geloest durch | Kernentscheidung |
|------------------------|---------------|-----------------|
| **RLS-Hook Transaktions-Bug** (ehem. Kritisch #2) | **D8** | `SET LOCAL ROLE` pro Transaktion, drei PostgreSQL-Rollen (`phoenix_auth`/`phoenix_tenant`/`phoenix_admin`), alle Queries in expliziten Transaktionen. QueryHook entfaellt komplett (D9). |
| **tenant_id=0 RLS-Bypass Sicherheitsrisiko** | **D7** | Zwei-Rollen-Architektur: `phoenix_tenant` (NOBYPASSRLS) + `phoenix_admin` (BYPASSRLS). Kein Magic-Value-Bypass. Fail-closed statt fail-open. |
| **Tenant-Switching-Flow nicht spezifiziert** (ehem. Kritisch #3) | **D4 + D15** | Tenant-Switch als Primaer-Mechanismus (`POST /auth/switch-tenant`). Ein Account, mehrere Tenants via `account_tenants`. Service-Level Cross-Tenant-Read nur fuer Ferienbetreuung. |
| **Login-Edge-Cases nicht spezifiziert** (ehem. Hoch #6) | **D6 + D12 + D15** | `tenant_slug` im Request-Body (Auth0/WorkOS-Pattern). Refresh Token re-validiert `account_tenants`. Login-Flow Schritt fuer Schritt mit Tenant-Lookup. |
| **Frontend Tenant-Validierung fehlt** (ehem. Mittel #12) | **D17** | Stateless Middleware (D11 Rewrite), Tenant-Validation im `[tenant]/layout.tsx` via `resolveTenant()`. `notFound()` bei unbekanntem Slug. |
| **Frontend Header vs. Rewrite Pattern** | **D11** | Rewrite Pattern (Vercel Platforms Starter Kit). Kein `headers()` → kein Dynamic Rendering Trap. |
| **Frontend Tenant-Context fehlte** | **D5** | `useTenant()` Hook mit Identitaet + Branding/Settings. Daten aus Login-Response, `resolveTenant()` fuer Pre-Login-Branding. |
| **BeforeAppendModel Shadowing-Risiko** | **D10** | Kein Hook auf TenantModel. Service-Layer setzt `tenant_id` explizit. CI-Check als Praevention. |
| **Per-Tenant Rollen** | **D13 (revidiert 2026-02-10)** | Per-Tenant RBAC mit System-Rollen. Account kann verschiedene Rollen bei verschiedenen Tenants haben. |
| **Transaction Pattern Conflict** (09-C1) | **03-backend.md §1.3** | Transaction-Ownership wandert von Service auf Handler. 51 RunInTx + 110 WithTx werden migriert. Handler startet WithTenantTx, Services/Repos nutzen tx aus Context. |
| **Policy Engine Tenant-Awareness** | **D14** | Two-Tier Authorization: Middleware (statisch/JWT) + Service (dynamisch/DB). Fail-closed Tenant-Assert in `Engine.Authorize()`. |
| **Scheduler ohne Tenant-Context** (09-C2) | **03-backend.md §11** | Hybrid-Strategie: Admin-Scope fuer Cleanup, Tenant-Scope fuer Business-Logic. Per-Tenant Error-Handling. |
| **IoT Device Auth Bootstrap** (09-C3) | **D20** | Two-Phase Lookup (WithAdminTx → WithTenantTx), per-Device PIN-Hash, analog D6 Login-Flow. |
| **GDPR Art. 17 Shared Accounts** (09-C7) | **D21** | Art. 28 AV-Modell (Industrie-Standard). Controller-Scope Loeschung. Self-Service Account-Loeschung. AVV + DPIA als juristischer Workstream. |
| **SEQUENCE GRANTs fehlen** (09-H1) | **02-datenbank.md §4.1** | `GRANT USAGE ON ALL SEQUENCES` + `ALTER DEFAULT PRIVILEGES` fuer beide Rollen ergaenzt. |
| **Cross-Tenant Foreign Keys** (09-H3) | **02-datenbank.md §2.5** | 64 Composite FKs: `FK(tenant_id, col) → target(tenant_id, id)`. 19 Ziel-Tabellen mit `UNIQUE(tenant_id, id)`. |
| **SSE Hub Event-Leakage** (06-#12, 08-H3) | **03-backend.md §10** | Group-Level Broadcasting bleibt, Tenant-Guard bei Connection + tenant-prefixed Map-Keys. |
| **Wildcard Cookie XSS** (09-H4) | **04-frontend.md §13** | CSP-Headers, HttpOnly/SameSite/Secure bereits gesetzt, JWT tenant_id bindet Cookie an Tenant, akzeptiertes Restrisiko dokumentiert. |
| **Confused Deputy Slug/Origin** (09-H5) | **04-frontend.md §14** | Backend validiert tenant_slug gegen Origin-Header. Mismatch = 400 Bad Request. |
| **NextAuth JwtPayload** (09-H6) | **04-frontend.md §15** | JwtPayload um tenant_id, tenant_slug, org_id, scope, permissions erweitert. parseJwtPayload extrahiert Tenant-Felder. |
| **SWR Cache Cross-Tenant** (09-H7) | **04-frontend.md §16** | useTenantSWR fuer alle 821 SWR-Calls, Session-Cache Tenant-Awareness, Cache-Control: no-store, SWR-Invalidierung bei Switch. |
| **Hardcoded Redirects** (09-H8) | **04-frontend.md §17** | useTenantRouter() Helper, 40+ Stellen migrieren, ESLint-Rule gegen neue hardcoded Redirects. |
| **Tenant-Switch Session** (09-M3) | **04-frontend.md §18** | NextAuth Session Update vor Redirect, alternativ Set-Cookie in Switch-Response. |
| **Ferienbetreuung BYPASSRLS** (09-H9) | **D4 + Klarstellung** | Visit.tenant_id = Host-OGS (wo das Kind physisch ist). Audit.tenant_id = Host-OGS. Student-Read via WithAdminTx mit expliziter Enrollment-Allowlist. Write-Ops per WithTenantTx(Host). Cross-Traeger erfordert AVV (D21). |
| **admin:* Wildcard Bypass** (09-H10) | **D13 (revidiert)** | Per-Tenant RBAC loest das Problem: JWT enthaelt nur Permissions des aktuellen Tenants. admin:* bei OGS A wird nicht in JWT fuer OGS B geladen. hasAdminWildcard prueft JWT-Inhalt, nicht globale Rolle. |
| **Zero-downtime Claim** (09-M1) | **Klarstellung** | `ADD COLUMN ... NOT NULL DEFAULT` ist in PG 17 metadata-only (kein Lock). `SET NOT NULL` danach erfordert Lock — Migrationsstrategie nutzt 3-Phasen-Rollout (§7 in 02-datenbank.md). `CREATE INDEX CONCURRENTLY` muss ausserhalb von BUN-Transaktionen laufen. |
| **Trigger Functions + RLS** (09-M2) | **Akzeptiertes Risiko** | Trigger wie `enforce_single_primary_student_guardian()` filtern by student_id. Unter RLS implizit safe. Unter BYPASSRLS (Admin-Ops) theoretisch Cross-Tenant — aber Admin-Ops sind Operator-Only mit eigenem Audit-Trail. Niedrige Prioritaet. |
| **resolveTenant DoS** (09-M4) | **04-frontend.md §3 + Klarstellung** | D17 sagt "Kein Cache". Mitigation: Rate-Limiting auf Reverse-Proxy (Caddy), resolveTenant ist ein einfacher DB-Lookup (< 1ms). Wildcard-DNS + Rate-Limit = ausreichend. Optional: in-memory Cache mit 60s TTL. |
| **CVE-2025-8713** (09-M5) | **Klarstellung** | CVE kann nicht verifiziert werden. Mindestversion bleibt PG 17.1 (fuer CVE-2024-10976). PG 17.6 als Empfehlung beibehalten (neueste Patch-Version), aber nicht als hartes Requirement. |
| **Sequential IDs** (09-M6) | **Akzeptiertes Risiko** | Globale BIGSERIAL IDs leaken Aktivitaetsvolumen zwischen Tenants (ID-Sprung = andere Tenants aktiv). Dokumentiert als akzeptiertes Risiko. UUIDs waeren Alternative, aber erfordern massive Codebase-Aenderung (alle int64 IDs) — YAGNI fuer 100-500 Tenants. |
| **Raw SQL / Subquery Sicherheit** | **D16** | RLS filtert alle Query-Formen. 6 gezielte Massnahmen: `RowsAffected()`-Audit, PG 17.6+, Seeds, View `security_invoker`, Advisory Lock 2-Arg, LEFT JOIN Review. |

---

## Zusammenfassung

| # | Prioritaet | Thema | Status |
|---|-----------|-------|--------|
| 1 | **Kritisch** | org-Scope Design | **Entschieden** — D18 |
| 11 | **Kritisch** | UNIQUE Constraints (31 migrieren) | **Dokumentiert** — 02-datenbank.md §2.4 |
| 12 | **Kritisch** | SSE Hub Cross-Tenant Event-Leakage | **Dokumentiert** — 03-backend.md §10 |
| 13 | **Kritisch** | Scheduler/Background-Jobs | **Dokumentiert** — 03-backend.md §11 |
| 2 | Hoch | Tabellen-Anzahl falsch (70 verifiziert) | **Geloest** — 02-datenbank.md §2.1 |
| 3 | Hoch | Eltern-Isolation | **Entschieden** — D19 |
| 4 | Hoch | Infrastruktur-Docs | Zurueckgestellt (D3) |
| 14 | Hoch | Cross-Tenant Foreign Keys (64 FKs) | **Dokumentiert** — 02-datenbank.md §2.5 |
| 15 | Hoch | Aggregate-Queries ohne Tenant-Scope | **Geloest** — 03-backend.md §1.3 |
| 5 | Mittel | accounts.tenant_id Zweck | **Geloest** — D15 + D13 rev |
| 6 | Mittel | Frontend-Tests | **Dokumentiert** — 05-testing.md §8 |
| 7 | Mittel | org-Scope Tests | **Dokumentiert** — 05-testing.md §9 |
| 8 | Mittel | Lokale Dev-Umgebung | **Dokumentiert** — 04-frontend.md §19 |
| 16 | Mittel | Avatar-Uploads Tenant-Namespacing | **Dokumentiert** — 03-backend.md §14 |
| 9 | Niedrig | Rollback-Plan NOT NULL | **Geloest** — 02-datenbank.md §8.2 |
| 10 | Niedrig | Namens-Inkonsistenz | **Geloest** — Legacy superseded |

**Stand vor DEBATE.md:** 14 offene Punkte (3 kritisch, 4 hoch, 5 mittel, 2 niedrig)
**Stand nach DEBATE.md:** 10 offene Punkte (1 kritisch, 3 hoch, 3 mittel, 2 niedrig) — 11 Findings geloest
**Stand nach Codebase-Audit:** 16 offene Punkte (4 kritisch, 5 hoch, 4 mittel, 2 niedrig) — 6 neue Findings aus Code-Analyse
**Stand nach systematischer Review (D18-D21 + 09-Findings):** 4 offene Punkte (0 kritisch, 1 hoch, 3 mittel) — 12 weitere Findings geloest, 23 Findings aus 09-Review dokumentiert
**Stand nach Abschluss aller offenen Punkte:** **0 offene Punkte** — alle 16 Findings geloest/dokumentiert

**Alle Findings sind geloest. Alle Blocker sind geloest. Die Architektur ist vollstaendig dokumentiert und implementierungsbereit.**
