# 06 — Offene Punkte & Handlungsbedarf

> Ergebnis einer technischen Review der Dokumente 00–05 gegen den aktuellen Codebase-Stand.
> Aktualisiert nach Review der Entscheidungen D1–D17 aus DEBATE.md.
> Ergaenzt um Codebase-Audit-Findings (UNIQUE Constraints, SSE, Scheduler, FKs).
> Stand: 2026-02-09

---

## Kritisch (Blocker vor Implementierungsbeginn)

### 1. "org"-Scope hat keine technische Umsetzung

**Status:** OFFEN

**Problem:** Die Anforderungen (00, Abschnitt 3.2) definieren Traeger-Buero-Mitarbeiter, die ALLE OGS ihres Traegers sehen. Die Architektur (01, Abschnitt 3) definiert einen `"org"`-Scope. Aber:

- Die RLS-Policy (02, Abschnitt 4.2) behandelt nur `tenant_id = 0` (Platform) oder `tenant_id = X` (einzelner Tenant). Es gibt keinen RLS-Pfad fuer "alle Tenants der Organisation Y".
- Das Backend (03) setzt eine einzelne `tenant_id` im Context und in der RLS-Session-Variable. Es gibt keinen Mechanismus fuer mehrere Tenant-IDs oder eine `org_id` fuer RLS.
- Kein Repository-Code zeigt org-Scope-Query-Erweiterung.

**Hinweis:** D4 (Cross-Tenant Mechanismus) erwaehnt `scope: "org"` als "eigener Mechanismus" in der Situationstabelle, loest aber die technische Umsetzung nicht. D7/D8 aendern die RLS-Architektur (Drei-Rollen mit `SET LOCAL ROLE`), aber die org-Scope-Frage bleibt davon unberuehrt.

**Auswirkung:** Traeger-Buero-Rolle ist spezifiziert aber nicht implementierbar.

**Handlungsbedarf:** Entscheidung als D18 in DEBATE.md aufnehmen. Optionen:
- (a) Org-aware RLS-Policy: `tenant_id IN (SELECT id FROM platform.schools WHERE org_id = current_setting('app.current_org_id')::bigint)` — performant mit initPlan, aber org_id muss zusaetzlich zu tenant_id gesetzt werden
- (b) Application-Layer: Service fuehrt Queries via `WithAdminTx` (D8) aus und filtert per `WHERE tenant_id IN (SELECT id FROM platform.schools WHERE org_id = ?)` — nutzt bestehende Admin-Rolle
- (c) Org-Scope als eingeschraenkter Admin: `SET LOCAL ROLE phoenix_admin` + org-Filter im Service — einfachste Loesung, aber BYPASSRLS fuer Nicht-Platform-User fragwuerdig

---

### 11. UNIQUE Constraints brechen bei zweitem Tenant

**Status:** OFFEN

**Problem:** 10 single-column UNIQUE Constraints nehmen Single-Tenancy an. Ab dem zweiten Tenant schlaegt jeder INSERT fehl, wenn beide OGS denselben Namen/Key verwenden — was in der Praxis garantiert vorkommt (jede OGS hat "1a", "Turnhalle", "Sport").

**Betroffene Constraints (Codebase-Audit verifiziert):**

| Tabelle | Spalte | Beispiel-Konflikt | Migration-File |
|---------|--------|-------------------|----------------|
| `education.groups` | `name UNIQUE` | Zwei OGS mit Gruppe "1a" | `001002007_education_groups.go:55` |
| `facilities.rooms` | `name UNIQUE` | Zwei OGS mit "Turnhalle" | `001001001_facilities_rooms.go:55` |
| `activities.categories` | `name UNIQUE` | Zwei OGS mit "Sport" | `001003001_activities_categories.go:55` |
| `config.settings` | `key UNIQUE` | Zwei OGS mit "session_timeout" | `001006001_config_settings.go:56` |
| `auth.accounts` | `username UNIQUE` | Zwei User mit "admin" | `001000001_auth_accounts.go:62` |
| `auth.accounts_parents` | `email UNIQUE` | Elternteil an 2 OGS | `001000009_auth_accounts_parents.go:70` |
| `auth.accounts_parents` | `username UNIQUE` | Zwei Eltern mit "mueller" | `001000009_auth_accounts_parents.go:59` |
| `users.guardian_profiles` | `email UNIQUE` | Elternteil an 2 OGS | `001003005001_users_guardian_profiles.go:61` |
| `iot.devices` | `device_id UNIQUE` | Recycelte RFID-Geraete | `001003009_iot_devices.go:72` |
| `users.persons` | `tag_id UNIQUE` | RFID-Karte an anderer OGS neu vergeben | `001002001_users_persons.go:57` |

**Hinweis:** `auth.accounts.email` ist ebenfalls global UNIQUE, aber das ist per D15 korrekt (ein Account, mehrere Tenants). Die BUN-Model-Tags (`bun:"name,notnull,unique"`) muessen ebenfalls angepasst werden.

**Zusaetzlich:** `models/base/base.go` definiert ein `NameableUnique` Struct mit `bun:"name,notnull,unique"` — eine Falle fuer zukuenftige Models.

**Auswirkung:** DB-Error bei jedem zweiten Tenant der identische Namen nutzt. Betrifft Basis-Operationen (Gruppen anlegen, Raeume anlegen, Settings setzen).

**Handlungsbedarf:** Alle betroffenen Constraints zu `UNIQUE(tenant_id, name)` bzw. `UNIQUE(tenant_id, key)` migrieren. BUN-Model-Tags anpassen. In 02-datenbank.md dokumentieren.

---

### 12. SSE Hub: Cross-Tenant Event-Leakage

**Status:** OFFEN

**Problem:** Der SSE Hub (`realtime/hub.go:16-21`) nutzt `active_group_id` (stringified int64) als Map-Key fuer Event-Broadcasting. Da Auto-Increment-IDs datenbank-weit vergeben werden (nicht tenant-weit), koennen zwei Tenants Gruppen mit derselben numerischen ID haben. Ein Betreuer der Events fuer Gruppe `1` subscribed, bekommt Events von ALLEN Tenants deren Gruppe `1` heisst.

```go
type Hub struct {
    groupClients map[string][]*Client // active_group_id -> subscribers
}
```

**Auswirkung:** Cross-Tenant-Datenleck in Echtzeit. Betreuer von OGS A sieht Check-In/Check-Out Events von OGS B.

**Handlungsbedarf:** Map-Keys muessen tenant-prefixed werden (`"tenant_1:group_42"`) oder ein Hub-per-Tenant Pattern. Alternativ: Client-Registration muss Tenant-Context enthalten und Broadcasting filtert nach Tenant.

---

### 13. Background-Jobs/Scheduler ohne Tenant-Context

**Status:** OFFEN

**Problem:** Alle Scheduler-Jobs nutzen `context.Background()` ohne Tenant-Context. Mit der D8-Architektur (`SET LOCAL ROLE` pro Transaktion) fuehrt das zu einem Dilemma:
- Ohne `WithTenantTx`/`WithAdminTx` → Verbindung als `phoenix_auth` (NOINHERIT) → **Permission Denied**
- Mit `WithAdminTx` → BYPASSRLS → sieht alle Tenants (moeglicherweise gewollt, aber nicht spezifiziert)

**Betroffene Jobs:**

| Job | Datei | Problem |
|-----|-------|---------|
| `CleanupExpiredVisits` | `cleanup_service.go:52-89` | Iteriert global ueber alle Students |
| `EndDailySessions` | `session_service.go:951` | `groupRepo.List(ctx, nil)` — alle Gruppen aller Tenants |
| `CleanupStaleAttendance` | `cleanup_service.go:363-368` | Kein Tenant-Filter |
| `CleanupStaleSupervisors` | `cleanup_service.go:498-503` | Kein Tenant-Filter |
| `CleanupOpenSessions` | `work_session_service.go:661-691` | Alle offenen Sessions |
| `AutoEndExpiredBreaks` | `work_session_service.go:925-962` | Alle abgelaufenen Breaks |
| `CleanupExpiredTokens` | `token_cleanup.go:15-21` | Global (evtl. OK) |

**Auswirkung:** Jobs funktionieren nach D8-Migration nicht mehr (Permission Denied) oder laufen unkontrolliert ueber alle Tenants.

**Handlungsbedarf:** Scheduler-Strategie definieren:
- (a) Jobs iterieren ueber alle Tenants und nutzen pro Tenant `WithTenantTx` — sauberste Loesung, RLS aktiv
- (b) Jobs nutzen `WithAdminTx` und arbeiten bewusst tenant-uebergreifend — einfacher, aber RLS ist nicht aktiv
- (c) Hybrid: Cleanup-Jobs als Admin (tenant-uebergreifend sinnvoll), Session-Jobs per Tenant

---

## Hoch (Vor oder waehrend der Implementierung klaeren)

### 2. Tabellen-Anzahl in 02-datenbank.md ist falsch

**Status:** OFFEN

**Problem:** Ueberschrift sagt "49 Tabellen", die tatsaechliche Auflistung summiert sich auf **64 Tabellen**: auth(12) + users(12) + education(6) + facilities(1) + activities(5) + active(10) + schedule(6) + iot(1) + feedback(1) + config(1) + suggestions(5) + audit(4).

**Auswirkung:** Tabellen koennten bei der Migration vergessen werden.

**Handlungsbedarf:** Zaehlung korrigieren und gegen die aktuelle Datenbank abgleichen. Pruefen ob das `meta`-Schema (in CLAUDE.md erwaehnt, in 02 nicht) Tabellen hat die `tenant_id` brauchen.

---

### 3. Eltern-Datenisolation ist unter-spezifiziert

**Status:** OFFEN

**Problem:** Anforderungen (00, Abschnitt 3.4) sagen: Eltern "sehen nur Daten des eigenen Kindes". Aber RLS operiert auf OGS-Ebene (Tenant), nicht auf Kind-Ebene. Sobald ein Elternteil Zugang zu einem Tenant hat, wuerde die RLS-Policy Zugriff auf ALLE Zeilen in diesem Tenant gewaehren.

D14 (Policy Engine) fuehrt einen Tenant-Assert in `Engine.Authorize()` ein, aber dieser prueft nur Tenant-Zugehoerigkeit, nicht Kind-Zugehoerigkeit. Die bestehenden Policies (z.B. `TeacherGroupPolicy`) pruefen Gruppen-Zuordnung — ein aehnliches Pattern koennte fuer Eltern-Kind-Zuordnung genutzt werden, ist aber nicht spezifiziert.

**Handlungsbedarf:** Klaeren ob Eltern-Isolation:
- Per Policy Engine (neue `ParentChildPolicy` analog zu `TeacherGroupPolicy`) oder
- Per Service-Layer (Application-Code filtert nach Kind-Zuordnung) umgesetzt wird

---

### 14. Cross-Tenant Foreign Keys nicht abgesichert

**Status:** OFFEN

**Problem:** Foreign Keys zwischen tenant-scoped Tabellen pruefen nur die ID, nicht den Tenant. Ein FK-Constraint `education.groups.room_id → facilities.rooms.id` verhindert nicht, dass eine Gruppe in Tenant A auf einen Raum in Tenant B zeigt.

**Betroffene FK-Beziehungen (Auszug):**

| Von | Nach | FK-Spalte |
|-----|------|-----------|
| `education.groups` | `facilities.rooms` | `room_id` |
| `activities.groups` | `facilities.rooms` | `planned_room_id` |
| `active.groups` | `facilities.rooms` | `room_id` |
| `active.groups` | `iot.devices` | `device_id` |
| `users.students` | `education.groups` | `group_id` |
| `active.visits` | `users.students` + `active.groups` | `student_id`, `active_group_id` |
| `active.group_supervisors` | `users.staff` + `active.groups` | `staff_id`, `group_id` |

**Hinweis:** RLS schuetzt teilweise (Room aus anderem Tenant ist unsichtbar bei SELECT), aber bei Admin-Operationen (BYPASSRLS) oder direkten DB-Zugriffen gibt es keinen Schutz.

**Handlungsbedarf:** Optionen:
- (a) Composite FKs: `FOREIGN KEY (tenant_id, room_id) REFERENCES facilities.rooms(tenant_id, id)` — erfordert `UNIQUE(tenant_id, id)` auf Ziel-Tabellen
- (b) Service-Layer-Validation: Pruefen dass beide Entities zum selben Tenant gehoeren
- (c) RLS als ausreichend akzeptieren (wenn keine Admin-Operationen betroffen sind)

---

### 15. Aggregate-Queries ohne Tenant-Scope

**Status:** OFFEN

**Problem:** Mehrere Repository- und Service-Methoden fuehren Aggregate-Queries (COUNT, SUM) ohne expliziten Tenant-Filter aus. Mit RLS wuerde die Filterung automatisch greifen, aber nur innerhalb von `WithTenantTx`. Queries die `r.db` direkt nutzen (statt `r.getDB(ctx)`) koennten am RLS vorbeilaufen.

**Betroffene Stellen:**

| Datei | Query | Problem |
|-------|-------|---------|
| `facility_service.go:97-123` | Room Occupancy Subqueries | Zaehlt Studenten/Supervisoren |
| `cleanup_service.go:176-202` | Retention Statistics | Aggregiert Visits |
| `data_deletion.go:160-192` | Deletion Stats | `COUNT(*)`, `SUM()` |
| `announcement_view_repository.go:163` | Target Audience | `COUNT(DISTINCT acc.id)` aller Accounts |
| `post_repository.go:110-158` | Vote/Comment Counts | Subqueries auf suggestions |
| `device.go:228-254` | Device Count by Type | Zaehlt alle Geraete |

**Handlungsbedarf:** Alle betroffenen Queries muessen innerhalb von `WithTenantTx` laufen und `r.getDB(ctx)` statt `r.db` nutzen. Alternativ: Explizite `WHERE tenant_id = ?` als Defense-in-Depth.

---

### 4. Infrastruktur/Deployment nur in Legacy-Docs

**Status:** ZURUECKGESTELLT (per D3)

**Problem:** Die Hauptdokumente (00–05) enthalten keine Infrastruktur-Spezifikationen. DNS-, SSL-, Caddy- und Docker-Compose-Aenderungen stehen nur in `legacy/04-schnittstellen-definition.md` (Abschnitt 7).

**Entscheidung in D3:** Zurueckgestellt. Kommt am Ende mit Phasen-Plan.

**Handlungsbedarf:** Bei Beginn der Deployment-Phase: Relevante Infrastruktur-Specs aus legacy/04 in ein neues Hauptdokument ueberfuehren (z.B. `07-deployment.md`).

---

## Mittel (Waehrend der Implementierung adressieren)

### 5. `auth.accounts.tenant_id` — Zweck klaeren

**Status:** TEILWEISE GELOEST

**Problem:** 02-datenbank.md fuegt `tenant_id` zu `auth.accounts` hinzu, aber D15 entscheidet klar: Email bleibt global UNIQUE, `account_tenants` ist die N:M-Junction-Tabelle. Der Zweck von `tenant_id` direkt auf `accounts` (vs. nur in `account_tenants`) ist weiterhin unklar.

**Moegliche Erklaerung:** `tenant_id` auf `accounts` koennte als RLS-Filter dienen (da `auth.accounts` auch `tenant_id` braucht fuer die RLS-Policy). In diesem Fall waere es der "Registrierungs-Tenant" oder "primaere Tenant". D15 adressiert das nicht explizit.

**Handlungsbedarf:** Entweder:
- Den Zweck dokumentieren (z.B. "RLS-Pflichtfeld, zeigt primaeren Tenant") oder
- Die Spalte entfernen und `auth.accounts` von RLS ausnehmen (Login muss Accounts ohne Tenant-Context finden koennen — siehe D6 Login-Flow Schritt 3)

---

### 6. Keine Frontend-Tests spezifiziert

**Status:** OFFEN

**Problem:** 05-testing.md definiert nur Go-Testmuster. Keine Spezifikation fuer:
- E2E-Tests (Subdomain-Routing, Login-Flow, Tenant-Switching)
- SWR-Cache-Isolationstests
- Bruno/API-Integrationstests mit Multi-Tenant-Szenarien
- Performance-Tests mit 100+ Tenants

**Handlungsbedarf:** Test-Strategie um Frontend- und API-Integrationstests erweitern.

---

### 7. Kein Test fuer "org"-Scope

**Status:** OFFEN (abhaengig von Punkt 1)

**Problem:** 05-testing.md hat Tests fuer Single-Tenant und Platform-Scope, aber keinen Test fuer den "org"-Scope (Traeger-Buero sieht alle OGS ihrer Organisation). Ebenso fehlt ein Test fuer `cross_tenant_access` (Ferienbetreuung).

**Handlungsbedarf:** Testmuster fuer org-Scope und Cross-Tenant-Access hinzufuegen (nach Klaerung von Punkt 1).

---

### 16. Avatar-Uploads ohne Tenant-Namespacing

**Status:** OFFEN

**Problem:** User-Avatare werden in einem globalen Verzeichnis gespeichert (`public/uploads/avatars/`) mit Dateinamen `{userID}_{random}.ext` (`api/usercontext/api.go:296,392`). Kein Tenant-Prefix im Pfad.

**Auswirkung:** Funktional kein direkter Konflikt (userID + random ist unique), aber bei Tenant-Loeschung oder GDPR-Cleanup muessen alle Avatare eines Tenants identifizierbar sein. Ohne Tenant-Prefix im Pfad ist das nur ueber DB-Lookup moeglich.

**Handlungsbedarf:** Pfadstruktur zu `public/uploads/avatars/{tenant_id}/` aendern. Bestehende Avatare bei Migration verschieben.

---

### 8. Lokale Entwicklungsumgebung fuer Subdomains

**Status:** OFFEN

**Problem:** D11 entschied das Rewrite Pattern (`subdomain.localhost:3000` → `/subdomain/...`). Die Middleware unterstuetzt dies, aber es gibt keine Dokumentation wie man das lokal einrichtet.

**Hinweis:** Moderne Browser unterstuetzen `*.localhost` nativ (Chrome, Firefox, Edge). Kein `/etc/hosts`-Eintrag noetig. Aber das muss dokumentiert und getestet werden.

**Handlungsbedarf:** Dev-Setup-Anleitung fuer Subdomain-Entwicklung erstellen (welche Browser, welche URLs, ggf. Docker-Compose-Anpassungen).

---

## Niedrig (Nice-to-have / Cleanup)

### 9. Rollback-Plan beruecksichtigt NOT NULL nicht

**Status:** OFFEN

**Problem:** Der Rollback-Plan (02, Abschnitt 6.2) sagt "tenant_id Spalten bleiben (stoeren nicht)". Aber Schritt 5 der Migration setzt sie auf NOT NULL. Bei einem Rollback nach Schritt 5 wuerden Inserts ohne tenant_id fehlschlagen.

**Handlungsbedarf:** Rollback-Plan um `ALTER COLUMN DROP NOT NULL` erweitern.

---

### 10. Funktionsnamen-Inkonsistenz zwischen Haupt- und Legacy-Docs

**Status:** OFFEN (geringer Impact)

- Hauptdocs (03): `FromContext()`, `MustFromContext()`, `IsPlatformScope()`
- Legacy (04): `TenantFromContext()`, `MustTenantFromContext()`, `IsPlatformContext()`

**Handlungsbedarf:** Legacy-Docs als superseded markieren oder Namensgebung angleichen.

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
| **Per-Tenant Rollen Komplexitaet** | **D13** | Globale Rollen beibehalten. YAGNI fuer Per-Tenant Rollen. |
| **Policy Engine Tenant-Awareness** | **D14** | Two-Tier Authorization: Middleware (statisch/JWT) + Service (dynamisch/DB). Fail-closed Tenant-Assert in `Engine.Authorize()`. |
| **Raw SQL / Subquery Sicherheit** | **D16** | RLS filtert alle Query-Formen. 6 gezielte Massnahmen: `RowsAffected()`-Audit, PG 17.6+, Seeds, View `security_invoker`, Advisory Lock 2-Arg, LEFT JOIN Review. |

---

## Zusammenfassung

| # | Prioritaet | Thema | Status |
|---|-----------|-------|--------|
| 1 | **Kritisch** | org-Scope Design | **Offen** — Blocker |
| 11 | **Kritisch** | UNIQUE Constraints brechen (10 Stueck) | **Offen** — Blocker |
| 12 | **Kritisch** | SSE Hub Cross-Tenant Event-Leakage | **Offen** — Blocker |
| 13 | **Kritisch** | Scheduler/Background-Jobs ohne Tenant-Context | **Offen** — Blocker |
| 2 | Hoch | Tabellen-Anzahl falsch (64 vs. 49) | Offen |
| 3 | Hoch | Eltern-Isolation | Offen |
| 4 | Hoch | Infrastruktur-Docs | Zurueckgestellt (D3) |
| 14 | Hoch | Cross-Tenant Foreign Keys | Offen |
| 15 | Hoch | Aggregate-Queries ohne Tenant-Scope | Offen |
| 5 | Mittel | accounts.tenant_id Zweck | Teilweise geloest (D15) |
| 6 | Mittel | Frontend-Tests | Offen |
| 7 | Mittel | org-Scope Tests | Offen (abh. von #1) |
| 8 | Mittel | Lokale Dev-Umgebung | Offen |
| 16 | Mittel | Avatar-Uploads ohne Tenant-Namespacing | Offen |
| 9 | Niedrig | Rollback-Plan NOT NULL | Offen |
| 10 | Niedrig | Namens-Inkonsistenz | Offen |

**Empfehlung:** Die Punkte 1, 11, 12, 13 sind Blocker. Punkt 11 (UNIQUE Constraints) und 13 (Scheduler) muessen in der Migrations-Phase adressiert werden. Punkt 12 (SSE Hub) muss vor dem Go-Live mit mehreren Tenants gefixt sein. Punkte 14–15 sollten parallel zur Implementierung adressiert werden.

**Stand vor DEBATE.md:** 14 offene Punkte (3 kritisch, 4 hoch, 5 mittel, 2 niedrig)
**Stand nach DEBATE.md:** 10 offene Punkte (1 kritisch, 3 hoch, 3 mittel, 2 niedrig) — 11 Findings geloest
**Stand nach Codebase-Audit:** 16 offene Punkte (4 kritisch, 5 hoch, 4 mittel, 2 niedrig) — 6 neue Findings aus Code-Analyse
