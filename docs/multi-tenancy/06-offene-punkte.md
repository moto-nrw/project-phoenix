# 06 — Offene Punkte & Handlungsbedarf

> Ergebnis einer technischen Review der Dokumente 00–05 gegen den aktuellen Codebase-Stand.
> Aktualisiert nach Review der Entscheidungen D1–D17 aus DEBATE.md.
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
| 2 | Hoch | Tabellen-Anzahl falsch (64 vs. 49) | Offen |
| 3 | Hoch | Eltern-Isolation | Offen |
| 4 | Hoch | Infrastruktur-Docs | Zurueckgestellt (D3) |
| 5 | Mittel | accounts.tenant_id Zweck | Teilweise geloest (D15) |
| 6 | Mittel | Frontend-Tests | Offen |
| 7 | Mittel | org-Scope Tests | Offen (abh. von #1) |
| 8 | Mittel | Lokale Dev-Umgebung | Offen |
| 9 | Niedrig | Rollback-Plan NOT NULL | Offen |
| 10 | Niedrig | Namens-Inkonsistenz | Offen |

**Empfehlung:** Punkt 1 (org-Scope) ist der einzige verbleibende Blocker vor Implementierungsbeginn. Punkte 2–3 sollten parallel zur ersten Implementierungsphase adressiert werden. Der Rest kann waehrend der Entwicklung iterativ geloest werden.

**Stand vor DEBATE.md:** 14 offene Punkte (3 kritisch, 4 hoch, 5 mittel, 2 niedrig)
**Stand nach DEBATE.md:** 10 offene Punkte (1 kritisch, 3 hoch, 3 mittel, 2 niedrig) — 11 Findings geloest
