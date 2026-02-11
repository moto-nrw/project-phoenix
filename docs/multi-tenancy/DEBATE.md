# Multi-Tenancy: Offene Diskussionspunkte

Vergleich Legacy-Entwuerfe (01-04) vs. validierte neue Dokumente (01-05).
Zusaetzlich: Findings aus Deep-Dive Codebase-Analyse und Best-Practice-Research.

Jeder Punkt wird einzeln besprochen und entschieden.

**Legende Severity:**
- KRITISCH = Sicherheitsrisiko oder Korrektheitsproblem
- HOCH = Architektur-Entscheidung mit weitreichenden Konsequenzen
- MITTEL = Design-Entscheidung mit begrenztem Impact

---

## D1: Operator Impersonation

**Status:** ENTSCHIEDEN | **Severity:** MITTEL

**Entscheidung:** Keine Impersonation im initialen Rollout. Die Architektur verhindert ein spaeteres Nachruestung nicht.

**Begruendung:**
- Operator-Dashboard deckt alle aktuellen Anforderungen ab (OGS-Filter, gezielte Announcements)
- Die geplante Architektur (JWT + tenant_id + scope + Wildcard-Cookie + RLS) ist die exakte Grundlage fuer Impersonation — spaeteres Nachruestung erfordert nur:
  - `POST /operator/impersonate` Endpoint (generiert tenant-scoped JWT mit `impersonated_by` Claim)
  - Frontend: Button im Operator-Dashboard + visueller Banner
  - Audit-Logging via `audit.auth_events.metadata` (JSONB Feld existiert bereits)
- Geschaetzter Aufwand fuer spaeteres Nachruestung: ~2-3 Tage
- Kein architektureller Vorbereitungsaufwand noetig

**Vermerkt in:** 00-anforderungen.md (Sektion 7)

---

## D2: base.Model vs TenantModel Mixin

**Status:** ENTSCHIEDEN | **Severity:** HOCH

**Entscheidung:** TenantModel Mixin. Separates `base.TenantModel` Struct mit `GetTenantID()` und `SetTenantID()`.

**Begruendung:**
1. **BUN-Kompatibilitaet:** `tenant_id` in `base.Model` wuerde BUN dazu bringen, `tenant_id` fuer Platform-Tabellen (operators, announcements) zu SELECT/INSERT — diese Spalte existiert dort nicht → SQL-Fehler. Workarounds (Field-Shadowing, Sentinel-Values) sind fragil.
2. **Vergesslichkeits-Richtung:** Vergessenes `TenantModel` Embed → DB-Error (`NOT NULL` violation, laut + sicher). Vergessener Platform-Ausschluss bei base.Model → stilles Datenleck (gefaehrlich).
3. **Compile-Time Interface:** `TenantScoped` Interface ermoeglicht generisches Auto-Setting im Base Repository ohne Runtime-Checks auf Sentinel-Values.
4. **Go-Idiomatik:** Composition ("embed nur was du brauchst") statt Vererbung. Standard Go-Pattern.
5. **Selbst-dokumentierend:** `base.TenantModel` eingebettet = RLS + WHERE-Clauses. Fehlt = Platform-Scope.

**Konkretes Design:**
```go
// models/base/tenant.go
type TenantModel struct {
    TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}
func (t *TenantModel) GetTenantID() int64   { return t.TenantID }
func (t *TenantModel) SetTenantID(id int64) { t.TenantID = id }
```

**Kein BeforeAppendModel auf TenantModel** (siehe D10). Service-Layer setzt tenant_id explizit, RLS ist Safety-Net.

**Impact:** 55 Models bekommen `base.TenantModel` als zweites Embed. 7+ Platform-Models (Operator, Announcement, OperatorAuditLog, Organization, School, OperatorOrganization, AnnouncementView) bleiben unberuehrt.

**Vermerkt in:** 03-backend.md (Sektion 2), 01-architektur.md (Sektion 4)

---

## D3: Deployment/Infrastructure

**Status:** ZURUECKGESTELLT | **Severity:** MITTEL

**Legacy 04 hat:**
```
DNS:     *.{TENANT_DOMAIN}  -> A Record -> Server IP
SSL:     Wildcard Cert fuer *.{TENANT_DOMAIN}
Caddy:   *.{TENANT_DOMAIN} { reverse_proxy frontend:3000 }
         api.{TENANT_DOMAIN} { reverse_proxy backend:8080 }
Docker:  TENANT_DOMAIN env var im Frontend-Container
```

**Neue Docs:**
Nur kurze URL-Struktur in 04-frontend.md. Keine Deployment-Details.

**Frage:**
Sollen wir eine eigene `06-deployment.md` erstellen mit DNS, SSL, Caddy und Docker-Konfiguration? Oder kommt das erst im Phasen-Plan (ganz am Ende)?

**Entscheidung:** _ausstehend_

---

## D4: Cross-Tenant-Access Mechanismus

**Status:** ENTSCHIEDEN | **Severity:** HOCH

**Entscheidung:** Tenant-Switch (Option C) als Primaer-Mechanismus + gezielter Service-Level Cross-Tenant-Read (Option B) nur fuer Ferienbetreuung.

**Begruendung:**
- **Option A verworfen:** `additional_tenant_ids` im JWT gibt Zugriff auf ALLE Daten der verknuepften OGS — zu breit fuer den Use Case (Betreuer soll nur die 30 Feriengruppen-Kinder sehen, nicht alle Kinder aller OGS)
- **Option C allein reicht nicht:** Ferienbetreuung hat gemischte Gruppen (z.B. 30 Kinder aus 5 OGS an einem Standort). Betreuer muesste 5x switchen → unbenutzbar
- **Kernproblem:** Kind "Max" hat `tenant_id=1` (Heimat-OGS), Betreuer hat `tenant_id=3` (Host-OGS) → RLS blockiert. Irgendwas muss die Tenant-Grenze ueberqueren

**Mechanismen nach Situation:**

| Situation | Mechanismus |
|-----------|------------|
| Normaler Alltag | Tenant-Switch — ein Tenant pro Session |
| Betreuer an 2 OGS (Mo-Mi/Do-Fr) | Tenant-Switch — wechselt je nach Tag |
| Vertretung/Aushilfe | Tenant-Switch — wechselt fuer den Tag |
| Ferienbetreuung | Tenant-Switch + Service-Level Cross-Tenant-Read |
| Traeger-Buero Uebersicht | `scope: "org"` — eigener Mechanismus |

**Ferienbetreuung-Ablauf:**
1. Admin erstellt Feriengruppe an Host-OGS
2. Admin enrollt Kinder aus anderen OGS (via `platform.cross_tenant_access`)
3. Betreuer switcht zu Host-OGS, oeffnet Feriengruppe
4. Active-Service erkennt Cross-Tenant-Enrollments
5. Service holt nur die eingeschriebenen Kinder via privilegierten Read (Admin-Connection, kein RLS)

**RLS bleibt simpel:** Ein tenant_id pro Request. Kein Array-Support, kein `additional_tenant_ids`. Cross-Tenant nur als gezielter Read im Feriengruppen-Service.

**Vermerkt in:** 00-anforderungen.md (Sektion 4.1), 03-backend.md

---

## D5: Frontend Tenant-Context Interface

**Status:** ENTSCHIEDEN | **Severity:** MITTEL

**Entscheidung:** `useTenant()` Hook mit Identitaet + Branding/Settings. Daten kommen aus Login-Response, kein separater API-Call im Betrieb.

**Interface:**
```typescript
interface TenantInfo {
    tenantId: string;
    tenantSlug: string;       // aus Subdomain/Route-Param
    tenantName: string;       // aus Login-Response
    orgId: string;
    orgName: string;
    scope: string;
    settings: TenantSettings; // aus platform.schools.settings JSONB
}

interface TenantSettings {
    logoUrl?: string;
    primaryColor?: string;
    [key: string]: unknown;   // Beliebig erweiterbar (JSONB)
}
```

**Datenquellen:**

| Zeitpunkt | Was passiert |
|-----------|-------------|
| Login-Response | Backend liefert `tenantName`, `orgName`, `settings` — cached im TenantContext Provider |
| Tenant-Switch | Neuer Token → neue Tenant-Info kommt automatisch mit |
| Pre-Login (public) | `GET /api/tenant/resolve?slug=...` fuer Login-Page Branding (Logo VOR dem Login) |

**Begruendung:**
- Session allein reicht nicht: UI braucht `tenantName` (Header), `tenantSlug` (Routing), Branding (Logo, Farben)
- White-Label vorbereitet: `platform.schools.settings` JSONB ist beliebig erweiterbar ohne Migration
- Public Endpoint noetig: Login-Seite soll OGS-Logo zeigen koennen bevor JWT existiert
- Kein Extra-Roundtrip im Betrieb: Alles kommt beim Login mit

**Vermerkt in:** 04-frontend.md

---

## D6: Login Handler Pattern

**Status:** ENTSCHIEDEN | **Severity:** MITTEL

**Entscheidung:** `tenant_slug` als explizites Feld im Login-Request-Body. Kein Custom-Header, kein dedizierter `createLoginHandler`.

**Begruendung:**
Best-Practice-Research zeigt drei gaengige Muster:
1. **Subdomain = Tenant** (Slack, Shopify): Backend leitet Tenant aus Subdomain ab
2. **Expliziter Parameter** (Auth0, WorkOS): Client sendet Tenant-Identifier im Request-Body
3. **Globaler Login** (GitHub, Vercel, Stripe): Login tenant-unabhaengig, danach Tenant-Auswahl

Phoenix nutzt Muster 1+2 kombiniert: Die Subdomain identifiziert den Tenant visuell (UX), aber der Login-Request sendet den Slug explizit im Body (technisch sauber). Das vermeidet Custom-Headers und funktioniert mit der bestehenden Next.js-Proxy-Architektur.

**Konkreter Flow:**
```
1. User oeffnet altenberge.moto-app.de/login
2. Next.js Middleware extrahiert "altenberge" aus Subdomain
3. Login-Page liest Slug aus Route-Param oder Middleware-Header
4. POST /api/auth/login { email, password, tenant_slug: "altenberge" }
5. Next.js API Route leitet an Backend weiter (Body unveraendert)
6. Backend: slug -> platform.schools.id -> tenant_id -> JWT Claims
```

**Vorteile:**
- Kein X-Tenant-Slug Header noetig (Header-Basierte Loesungen sind fragil bei Proxies/CDNs)
- Body-Parameter ist Standard-REST, leicht testbar (curl, Bruno, Postman)
- Bestehender Route-Wrapper braucht keine "kein JWT"-Variante — der Login-Endpoint hat ohnehin keine Auth-Middleware
- Auth0 und WorkOS nutzen exakt dieses Pattern

**Vermerkt in:** 03-backend.md (Sektion 5), 04-frontend.md

---

## D7: tenant_id=0 RLS-Bypass ist ein Sicherheitsrisiko

**Status:** ENTSCHIEDEN | **Severity:** KRITISCH

**Entscheidung:** Zwei-Rollen-Architektur. Kein `tenant_id=0` Bypass in RLS-Policies.

**Kernproblem:**
Der aktuell geplante `=0` Bypass ist **fail-open**: Ein einziger Bug (vergessene Middleware, Default-Wert, gefaelschter JWT) gibt ALLE Tenant-Daten frei. Das ist der gefaehrlichste Failure-Mode den eine Multi-Tenancy-Architektur haben kann.

**Begruendung:**
Umfassende Best-Practice-Research (Supabase, PostgREST, AWS, Citus, Crunchy Data, Nile) zeigt: Kein ernstzunehmender Multi-Tenant-PostgreSQL-Anbieter verwendet Magic-Value-Bypasses (`=0`, `=-1`, `=NULL`) in RLS-Policies. Alle nutzen separate Rollen mit unterschiedlichen RLS-Attributen.

**PostgreSQL-Rollen:**
```sql
-- App-Rolle: Fuer alle Tenant-scoped Queries (RLS IMMER aktiv)
CREATE ROLE phoenix_app WITH LOGIN PASSWORD '...' NOBYPASSRLS;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
  auth, users, education, facilities, activities, active,
  schedule, iot, feedback, config, meta, audit TO phoenix_app;

-- Admin-Rolle: Fuer Operator-Dashboard, Migrations, Seeds, Cross-Tenant
CREATE ROLE phoenix_admin WITH LOGIN PASSWORD '...' BYPASSRLS;
GRANT ALL ON ALL TABLES IN SCHEMA
  auth, users, education, facilities, activities, active,
  schedule, iot, feedback, config, meta, audit, platform TO phoenix_admin;

-- WICHTIG: RLS auch fuer Table-Owner erzwingen
ALTER TABLE users.students FORCE ROW LEVEL SECURITY;
-- (fuer alle 55 Tenant-Tabellen wiederholen)
```

**RLS-Policy (ohne =0 Bypass):**
```sql
CREATE POLICY tenant_isolation ON users.students
  USING (
    tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
  );
-- NULLIF verhindert Cast-Error bei leerem String → NULL → false → zero rows
-- Fail-closed: Vergessenes set_config → kein Match → zero rows (sicher)
```

**Wer nutzt phoenix_admin:**
- Operator-Routes (`/operator/*`)
- Migrations + Seeds (DDL + Testdaten)
- Cleanup-Jobs (GDPR-Loeschungen)
- Ferienbetreuung Cross-Tenant-Read (D4)

**Fail-Mode-Vergleich:**
| Szenario | Mit `=0` Bypass | Mit Zwei-Rollen |
|----------|----------------|-----------------|
| Vergessenes `set_config` | Alle Daten sichtbar (fail-open) | Zero rows (fail-closed) |
| JWT mit `tenant_id=0` | Alle Daten sichtbar | Kein Bypass moeglich |
| Bug in Middleware | Vollzugriff | Kein Zugriff (NOBYPASSRLS) |

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| `tenant_id=0` Bypass | Fail-open, Single-Point-of-Failure, kein Industriestandard |
| Tenant-Listen fuer Operators | Performance degeneriert bei 500 Tenants (`ANY(ARRAY[...])`) |
| SECURITY DEFINER Funktionen | Business-Logik wandert in DB, verletzt Handler→Service→Repo Architektur |

**Verbindungs-Mechanismus (in D8 entschieden):**
Ein Connection Pool als `phoenix_auth` (NOINHERIT) + `SET LOCAL ROLE` pro Transaktion. Drei Rollen statt zwei: `phoenix_auth` (Verbindung), `phoenix_tenant` (RLS), `phoenix_admin` (BYPASSRLS). Siehe D8 fuer Details.

**Quellen:**
- [PostgreSQL Docs: Row Security Policies](https://www.postgresql.org/docs/current/ddl-rowsecurity.html)
- [PostgreSQL Docs: Role Attributes (BYPASSRLS)](https://www.postgresql.org/docs/current/role-attributes.html)
- [Supabase: Postgres Roles + RLS](https://supabase.com/docs/guides/database/postgres/roles)
- [PostgREST: Authentication + Role Switching](https://docs.postgrest.org/en/v14/references/auth.html)
- [AWS: Multi-tenant data isolation with RLS](https://aws.amazon.com/blogs/database/multi-tenant-data-isolation-with-postgresql-row-level-security/)
- [Crunchy Data: RLS for Tenants](https://www.crunchydata.com/blog/row-level-security-for-tenants-in-postgres)
- [Nile: Multi-tenant SaaS with RLS](https://www.thenile.dev/blog/multi-tenant-rls)
- [Bytebase: Common RLS Footguns](https://www.bytebase.com/blog/postgres-row-level-security-footguns/)
- CVE-2024-10976: RLS Policy Bypass via Query Plan Reuse (gefixt in PG 17.1+)
- CVE-2019-10130: Statistics Leakage durch RLS (gefixt seit 2019)

**Vermerkt in:** 01-architektur.md, 02-datenbank.md, 03-backend.md

---

## D8: set_config Transaction-Sicherheit + Base Repository Transaction Gap

**Status:** ENTSCHIEDEN | **Severity:** KRITISCH

**Entscheidung:** SET LOCAL ROLE pro Transaktion (PostgREST/Supabase-Pattern). Ein Connection Pool, drei PostgreSQL-Rollen, alle Queries in expliziten Transaktionen.

**Die drei Probleme die geloest werden:**
1. `set_config(..., true)` wirkt nur in expliziten Transaktionen — ohne `BEGIN...COMMIT` verfaellt der Wert sofort
2. Base Repository nutzt keine Transaktionen (`r.DB` direkt, nie `TxFromContext`)
3. BUN QueryHook ist fundamental kaputt mit Connection-Pooling: Hook bekommt Connection A, Query bekommt Connection B → `set_config` landet auf falscher Connection

**Konsequenz:** Option B (session-scoped, `is_local=false`) funktioniert NICHT — nicht wegen stale values, sondern weil Hook und Query verschiedene Pool-Connections nutzen. Explizite Transaktionen sind die einzige korrekte Loesung.

**Architektur (drei Rollen, ein Pool):**
```sql
-- Verbindungs-Rolle: LOGIN, aber KEINE eigenen Rechte (sicherster Default)
CREATE ROLE phoenix_auth LOGIN NOINHERIT PASSWORD '...';

-- Tenant-Rolle: Subject to RLS, alle CRUD-Rechte auf Tenant-Tabellen
CREATE ROLE phoenix_tenant NOLOGIN;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit TO phoenix_tenant;
GRANT SELECT ON platform.schools TO phoenix_tenant;

-- Admin-Rolle: Bypasses RLS (Operator, Migrations, Seeds, Cross-Tenant)
CREATE ROLE phoenix_admin NOLOGIN BYPASSRLS;
GRANT ALL ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform TO phoenix_admin;

-- Verbindungs-Rolle darf zu beiden switchen
GRANT phoenix_tenant TO phoenix_auth;
GRANT phoenix_admin TO phoenix_auth;
```

**Go-Implementierung:**
```go
// Ein Connection Pool beim Startup (verbunden als phoenix_auth)
db := connectAs("phoenix_auth")

// Tenant-scoped Wrapper (99% aller Requests)
func WithTenantTx(ctx context.Context, db *bun.DB, tenantID int64,
    fn func(ctx context.Context, tx bun.Tx) error) error {
    return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant"); err != nil {
            return fmt.Errorf("set role: %w", err)
        }
        if _, err := tx.ExecContext(ctx,
            "SELECT set_config('app.current_tenant_id', $1, true)",
            fmt.Sprintf("%d", tenantID),
        ); err != nil {
            return fmt.Errorf("set tenant: %w", err)
        }
        return fn(ctx, tx)
    })
}

// Admin-scoped Wrapper (Operator-Routes, Migrations, Cleanup)
func WithAdminTx(ctx context.Context, db *bun.DB,
    fn func(ctx context.Context, tx bun.Tx) error) error {
    return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_admin"); err != nil {
            return fmt.Errorf("set role: %w", err)
        }
        return fn(ctx, tx)
    })
}
```

**Base Repository Fix:**
```go
// Repositories nutzen Transaktion aus Context wenn verfuegbar
func (r *Repository[T]) getDB(ctx context.Context) bun.IDB {
    if tx := bun.TxFromContext(ctx); tx != nil {
        return tx
    }
    return r.DB
}
```

**Warum SET LOCAL ROLE statt Zwei Pools:**
- Gleicher Transaction-Refactoring-Aufwand bei beiden Optionen
- SET LOCAL ROLE loest zusaetzlich D9 (QueryHook wird obsolet)
- Kein Dual-Connection-Problem fuer AnnouncementService und OperatorSuggestionsService
- Sicherster Default: `phoenix_auth` hat NOINHERIT, null Rechte → Hard-Fail bei vergessener Transaktion
- PostgREST/Supabase nutzen exakt dieses Pattern seit 2014

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option B: `is_local=false` (session-scoped) | QueryHook und Query nutzen verschiedene Pool-Connections → set_config auf falscher Connection |
| Zwei Connection Pools | Gleicher Refactoring-Aufwand, aber Dual-Connection-Problem bei 2 Services, kein D9-Fix |
| QueryHook beibehalten | Fundamental kaputt mit Connection-Pooling (s.o.) |

**Voraussetzung:** PostgreSQL 17.1+ (CVE-2024-10976: Query-Plan Reuse nach SET ROLE gefixt).

**Refactoring-Umfang:**
- Base Repository: `getDB()` Methode hinzufuegen (~1 Datei)
- Services: Jede Public-Methode in `WithTenantTx`/`WithAdminTx` wrappen (~29 Services)
- Middleware: `TenantMiddleware` startet Transaktion statt nur Context zu setzen
- QueryHook: Entfaellt komplett (D9 geloest)
- Transaktionen werden bereits in 63 Files genutzt → Pattern ist etabliert

**Quellen:**
- [PostgREST: Authentication + SET LOCAL ROLE](https://docs.postgrest.org/en/v14/references/auth.html)
- [PostgreSQL: SET ROLE](https://www.postgresql.org/docs/current/sql-set-role.html)
- [BUN ORM: Transactions](https://bun.uptrace.dev/guide/transactions.html)
- [Go database/sql: Managing Connections](https://go.dev/doc/database/manage-connections)
- CVE-2024-10976 (gefixt in PG 17.1+)

**Vermerkt in:** 02-datenbank.md, 03-backend.md

---

## D9: BUN QueryHook Strategie (Performance)

**Status:** GELOEST DURCH D8 | **Severity:** HOCH

**Entscheidung:** QueryHook entfaellt komplett. `set_config()` wird einmal pro Transaktion im `WithTenantTx` Wrapper gesetzt (D8 Option B).

**Begruendung:**
D8 hat entschieden, dass alle Queries in expliziten Transaktionen laufen (`WithTenantTx`/`WithAdminTx`). Der `WithTenantTx` Wrapper setzt `set_config('app.current_tenant_id', ...)` einmal am Transaktionsbeginn. Alle Queries innerhalb der Transaktion erben den Wert automatisch — kein QueryHook noetig.

**Was entfaellt:**
- `RLSHook` Struct komplett (03-backend.md Sektion 1.1)
- `db.AddQueryHook(&tenant.RLSHook{})` beim Startup
- Alle drei D9-Optionen (A/B/C) sind obsolet

**Performance-Gewinn:**
Vorher (QueryHook): N+1 `set_config()` Calls pro Request (1 pro Query)
Nachher (D8): Genau 1 `set_config()` pro Transaktion, unabhaengig von Query-Anzahl

---

## D10: BeforeAppendModel Shadowing bei TenantModel

**Status:** ENTSCHIEDEN | **Severity:** HOCH

**Entscheidung:** Option A (kein BeforeAppendModel auf TenantModel) + Service-Layer Verantwortung + CI-Check.

**Begruendung:**

1. **Shadowing ist real aber nicht das Hauptargument:** 55 von 57 Models haben eigene `BeforeAppendModel` Hooks — ein Hook auf TenantModel wuerde bei fast allen stumm ignoriert.

2. **Das eigentliche Problem:** 63 custom Create-Methoden in 57 Repository-Files. Davon delegieren 45 an `base.Create()`, aber 18+ nutzen `NewInsert()` direkt (Upserts, Bulk-Inserts, Composite-Key-Tabellen). Ein BeforeAppendModel Hook wuerde bei den direkten NewInsert-Calls zwar greifen, gibt aber eine **falsche Sicherheit** — Entwickler denken "tenant_id wird automatisch gesetzt" und testen es nie.

3. **Service-Layer setzt tenant_id explizit (A2):**
```go
// Im Service — VOR dem Repository-Call
student.SetTenantID(tenant.FromContext(ctx))
err := s.studentRepo.Create(ctx, student)
```
Der Service weiss welcher Tenant aktiv ist. Das Repository muss nichts ueber Tenants wissen.

4. **Defense-in-Depth Schichtung (gemaess D8):**

| Schicht | Mechanismus | Bei fehlender tenant_id |
|---------|-------------|------------------------|
| 1. Service-Layer | `SetTenantID(tenant.FromContext(ctx))` | Erster Checkpoint |
| 2. Base Repository | TenantScoped-Check in `base.Create()` | Bonus fuer die 45 Repos die base nutzen |
| 3. DB-Constraint | `tenant_id NOT NULL` | INSERT schlaegt sofort fehl (laut) |
| 4. RLS-Policy | `WHERE tenant_id = current_setting(...)` | Selbst bei falschem Wert: zero rows |

5. **CI-Check als Praevention:** Ein grep-basierter CI-Check der warnt wenn ein TenantScoped-Model ohne vorheriges `SetTenantID()` an ein Repository uebergeben wird. Wirksamste Praevention gegen Vergesslichkeit — nicht ein Hook der still versagt.

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option B: Expliziter Aufruf in 55 Hooks | Riesiger Aufwand, leicht vergessbar, neue Hooks muessen es auch tun |
| Option C: Nur Defense-in-Depth | Korrekt, aber INSERT-Fehler sind unschoener als explizites Setzen |
| BeforeAppendModel auf TenantModel | Falsche Sicherheit: 55/57 Models shadowen es, 18+ Repos umgehen es |

**Vermerkt in:** 03-backend.md (Sektion 2)

---

## D11: Frontend Middleware: Rewrite Pattern vs. Header Pattern

**Status:** ENTSCHIEDEN | **Severity:** HOCH

**Entscheidung:** Rewrite Pattern. Middleware rewritet Subdomain-Requests zu `/[tenant]/...` Route-Segmenten. Vercel Platforms Starter Kit als Referenz-Implementierung.

**Begruendung:**

1. **Offizielles Vercel-Pattern:** Das Vercel Platforms Starter Kit (github.com/vercel/platforms) nutzt exakt dieses Pattern. Dub.co (Millionen Links) basiert darauf. Next.js Multi-Tenant Docs verweisen direkt darauf.

2. **`headers()` Dynamic Rendering Trap:** `headers()` erzwingt Dynamic Rendering in JEDER Server Component — bestaetigt durch GitHub Issues #44712, #58862, #85239. Kein Workaround ausser PPR (experimental). Das Header Pattern wuerde uns dauerhaft in Dynamic Rendering einschliessen.

3. **Minimaler Mehraufwand:** API Routes (194 Handler) bleiben wo sie sind — Tenant kommt aus JWT, nicht aus der URL. Nur 33 protected Pages + 1 public Page verschieben sich nach `app/[tenant]/`. Operator-Routes bleiben an Root.

4. **Zukunftssicher:** `next/root-params` (Next.js 15.5+) ermoeglicht tiefes Lesen von Root-Layout-Params ohne `headers()` — funktioniert nur mit Rewrite. Falls wir spaeter Server Components / PPR nutzen, blockiert uns das Rewrite Pattern nicht.

5. **Type-safe:** `params.tenant` statt `headers().get('x-tenant-slug') ?? ''`.

**Verzeichnisstruktur:**
```
app/
  [tenant]/                          ← Rewrite-Target
    (protected)/                     ← Bestehende Route-Group verschoben
      layout.tsx                     ← Liest params.tenant
      dashboard/page.tsx
      students/page.tsx
      rooms/page.tsx
      ...                            ← 33 Pages total
    (public)/
      invite/page.tsx
  (operator)/                        ← Bleibt an Root, kein [tenant]
    layout.tsx
    operator/login/page.tsx
    operator/announcements/page.tsx
    ...
  api/                               ← Bleibt an Root, kein [tenant]
    auth/[...nextauth]/route.ts
    students/route.ts
    active/*/route.ts
    operator/*/route.ts
    ...                              ← 194 Handler unveraendert
```

**Middleware:**
```typescript
// middleware.ts — Subdomain → Rewrite
if (subdomain && !RESERVED_SUBDOMAINS.includes(subdomain)) {
    return NextResponse.rewrite(new URL(`/${subdomain}${pathname}`, request.url));
}
```

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Header Pattern (x-tenant-slug) | `headers()` erzwingt Dynamic Rendering, gegen Vercel-Empfehlung, nicht type-safe |
| Hybrid (Rewrite + Header) | Request-Headers auf Rewrite triggern Browser-Reload (GitHub Discussion #45471) |

**Quellen:**
- [Vercel Platforms Starter Kit](https://github.com/vercel/platforms)
- [Next.js Multi-Tenant Guide](https://nextjs.org/docs/app/guides/multi-tenant)
- [GitHub Issue #44712: headers() forces dynamic rendering](https://github.com/vercel/next.js/issues/44712)
- [GitHub Discussion #58862: Middleware header trap for i18n/multi-tenancy](https://github.com/vercel/next.js/discussions/58862)
- [GitHub Discussion #45471: Rewrite + header causes reload](https://github.com/vercel/next.js/discussions/45471)

**Vermerkt in:** 04-frontend.md (Sektion 1)

---

## D12: Refresh Token Tenant-Validierung

**Status:** ENTSCHIEDEN | **Severity:** HOCH

**Entscheidung:** Option A + C. `tenant_id` in RefreshClaims speichern + Re-Validierung gegen `account_tenants` bei jedem Refresh.

**Begruendung:**

1. **Industrie-Standard:** Auth0, WorkOS, Firebase validieren Tenant/Org-Membership bei jedem Token-Refresh, nicht bei jedem Request. 15-Minuten-Fenster (Access Token Expiry) ist akzeptiert.

2. **Minimaler Aufwand:** Der Refresh-Handler laedt bereits den Token aus `auth.tokens` und re-validiert Account-Status + Permissions. Ein zusaetzlicher `account_tenants.Exists()` Check ist eine Zeile.

3. **Option B (DB-Spalte) fuer Validierung ueberfluessig:** Der DB-Lookup auf `auth.tokens` passiert bereits beim Refresh. Die `tenant_id` kommt aus den RefreshClaims — fuer die Refresh-Validierung ist kein Schema-Change noetig. **Hinweis:** `auth.tokens` bekommt dennoch `tenant_id NOT NULL` (siehe 02-datenbank.md) — nicht fuer Validierung, sondern fuer gezieltes Per-Tenant-Revoken bei Zugriffsentzug.

**Aenderungen:**
```go
// RefreshClaims erweitern
type RefreshClaims struct {
    ID       int    `json:"id,omitempty"`
    Sub      string `json:"sub,omitempty"`
    TenantID int64  `json:"tenant_id"`  // NEU
    CommonClaims
}

// Im Refresh-Handler (eine Zeile zusaetzlich)
hasAccess, err := accountTenantRepo.Exists(ctx, accountID, claims.TenantID)
if !hasAccess {
    return ErrTenantAccessRevoked
}
```

**Refresh-Flow:**
```
1. Client sendet Refresh Token (enthaelt tenant_id in Claims)
2. Server validiert JWT-Signatur
3. Server prueft: Token existiert in auth.tokens + nicht revoked
4. Server laedt Account + Permissions neu
5. NEU: Server prueft account_tenants.Exists(accountID, claims.TenantID)
6. Server generiert neuen Access Token MIT tenant_id + org_id + scope
```

**Worst-Case bei Zugriffsentzug:** Max 15 Minuten (bis Access Token ablaeuft und Refresh versucht wird). Fuer sofortige Sperrung muesste jeder Request gegen die DB geprüft werden — das widerspricht dem JWT-Prinzip und macht kein Auth-Provider.

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option B allein (DB-Spalte) | DB-Lookup passiert bereits, tenant_id aus Claims reicht |
| Check bei jedem Request | Widerspricht JWT-Prinzip, Performance-Problem |
| Nur Option A (ohne Re-Validierung) | Entzogener Zugriff wirkt erst nach Refresh Token Expiry (1h statt 15min) |

**Vermerkt in:** 03-backend.md (Sektion 4)

---

## D13: Per-Tenant Rollen vs. Globale Rollen

**Status:** REVIDIERT (2026-02-10) | **Severity:** HOCH

**Entscheidung:** Option A — Per-Tenant RBAC mit System-Rollen. Rollen- und Permission-Zuweisung erfolgt pro Tenant.

**Begruendung fuer Revision:**

Die urspruengliche Entscheidung (globale Rollen, YAGNI) wurde nach Review durch @yonnock (09-devils-advocate-findings C4, H10) und Codebase-Analyse revidiert. Drei Gruende:

1. **Realer Use Case, kein Edge Case:** OGS-Buero-Mitarbeiterin Maria ist Admin bei OGS Altenberge (`users:manage`, `config:update`) und Betreuerin bei OGS Greven (nur `students:read`, `visits:read`). Mit globalen Rollen hat Maria ueberall Admin-Rechte — ueber-privilegiert bei Greven. Das ist der Standard-Betrieb bei jedem Traeger mit mehreren OGS.

2. **`admin:*` Wildcard + globale Rollen = Master Key:** Der `hasAdminWildcard`-Check in `permission.go:86-89` bypassed ALLE Permission-Middleware. Combined mit globalen Rollen: Admin bei einer OGS = Admin ueberall. RLS schuetzt die Daten, aber die Autorisierung (was ein User TUN darf) ist komplett offen. (09-devils-advocate H10)

3. **OGS-Admin braucht Flexibilitaet:** Die urspruengliche Vision (Christian) war: Rollen als dynamische Permission-Bundles, verwaltet von OGS-Admins. Beispiel: "Vertretungs-Betreuer" mit Sonderrechten ab 12:30 wenn das Buero nicht mehr besetzt ist. Globale Rollen machen das unmoeglich.

**Design:**

```
System-Rollen (tenant_id = NULL, is_system = true):
  → admin, user, guardian, guest
  → Vordefiniert, nicht loeschbar, ueberall verfuegbar
  → Permission-Zuordnung global (role_permissions ohne tenant_id)

Tenant-spezifische Rollen (tenant_id = X, is_system = false):
  → Von OGS-Admins erstellte Rollen ("Vertretung", "Springer", etc.)
  → Nur innerhalb des Tenants sichtbar und zuweisbar
  → Permission-Zuordnung tenant-spezifisch
```

**Betroffene Tabellen:**

| Tabelle | Aenderung | RLS |
|---------|-----------|-----|
| `auth.roles` | `tenant_id BIGINT NULL` hinzufuegen | `tenant_id IS NULL OR tenant_id = current_setting(...)` |
| `auth.account_roles` | `tenant_id BIGINT NOT NULL` hinzufuegen | Standard-RLS |
| `auth.account_permissions` | `tenant_id BIGINT NOT NULL` hinzufuegen | Standard-RLS |
| `auth.permissions` | Bleibt global (keine Aenderung) | Kein RLS |
| `auth.role_permissions` | Bleibt global (Scoping durch Role-FK) | Kein RLS |

**Permission-Loading wird tenant-aware:**

```go
// Login/Refresh: Permissions fuer den aktuellen Tenant laden
func (r *PermissionRepo) FindByAccountAndTenant(ctx context.Context, accountID, tenantID int64) ([]*Permission, error) {
    // CTE 1: Direct permissions fuer diesen Tenant
    // CTE 2: Permissions aus Rollen die diesem Account bei diesem Tenant zugewiesen sind
    //         (System-Rollen mit tenant_id=NULL + Tenant-Rollen mit tenant_id=X)
    // UNION → DISTINCT
}
```

**JWT Claims aendern sich NICHT strukturell** — `Permissions []string` bleibt ein flaches Array. Aber der Inhalt ist jetzt pro Tenant unterschiedlich. Bei Tenant-Switch wird ein neuer Token mit den Permissions des Ziel-Tenants generiert.

**Migration bestehender Daten:**

```sql
-- System-Rollen markieren
UPDATE auth.roles SET is_system = true
    WHERE name IN ('admin', 'user', 'guardian', 'guest');

-- Bestehende account_roles dem Default-Tenant zuweisen
UPDATE auth.account_roles SET tenant_id = 1;

-- Bestehende account_permissions dem Default-Tenant zuweisen
UPDATE auth.account_permissions SET tenant_id = 1;
```

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option B: Globale Rollen (urspruengliche D13) | Admin bei einer OGS = Admin ueberall. OGS-spezifische Sonderrechte unmoeglich. Revidiert nach Review. |
| Option C: Hybrid (Global + per-Tenant Override) | Zwei Stellen fuer Rollen = verwirrend, Merge-Logik komplex |
| Per-Tenant Permissions ohne per-Tenant Roles | Deckt Sonderrechte ab, aber keine wiederverwendbaren Bundles. Halb-Loesung. |

---

## D14: Policy Engine Tenant-Awareness

**Status:** ENTSCHIEDEN | **Severity:** MITTEL

**Entscheidung:** Two-Tier Authorization mit automatischem Tenant-Assert in Engine.Authorize() (fail-closed). Policy-Evaluation wandert von Middleware in Service-Layer.

**Begruendung:**

1. **Zwei fundamental verschiedene Auth-Fragen gehoeren in verschiedene Schichten:**

| Tier | Frage | Schicht | DB noetig? |
|------|-------|---------|------------|
| Tier 1 (statisch) | "Hat dieser User Permission `visits:read`?" | Middleware (`RequiresPermission`) | Nein (JWT) |
| Tier 2 (dynamisch) | "Ist dieser Lehrer in der Gruppe dieses Schuelers?" | Service (Policy Engine) | Ja |

OWASP Microservices Security Cheat Sheet und NIST SP 800-204B empfehlen explizit dieses Pattern: "Coarse-grained at edge, fine-grained at service." Chris Richardson (microservices.io): "The primary responsibility for authorization rests with the services themselves."

2. **D8 erzwingt die Migration:** Die Resource Authorization Middleware (`resource_middleware.go`) ruft `educationService.GetTeacherGroups()` auf — das geht zum Repository, das `r.getDB(ctx)` nutzt. Ohne Transaktion im Context faellt es auf `r.DB` zurueck. `r.DB` verbindet als `phoenix_auth` (NOINHERIT, keine Permissions) → Permission Denied. Policy-Eval mit DB-Zugriff MUSS innerhalb von `WithTenantTx` laufen.

3. **Automatischer Tenant-Assert (fail-closed):**
```go
func (e *Engine) Authorize(ctx context.Context, authCtx *Context) (bool, error) {
    // Tenant-scoped User + Resource ohne TenantID-Tag = DENY (fail-closed)
    if authCtx.Subject.TenantID > 0 && authCtx.Resource.TenantID == 0 {
        return false, ErrMissingTenantContext
    }
    // Cross-Tenant = DENY
    if authCtx.Subject.TenantID > 0 && authCtx.Resource.TenantID > 0 {
        if authCtx.Subject.TenantID != authCtx.Resource.TenantID {
            return false, ErrCrossTenantAccess
        }
    }
    // Platform-User (TenantID=0) oder Tenant-Match → weiter zu Policies
    for _, p := range e.resourceToPolicy[authCtx.Resource.Type] {
        // ...
    }
}
```

Vergessenes `Resource.TenantID` setzen → Deny (nicht stilles Skip). Nur Platform-User (`TenantID=0`) koennen auf nicht-getaggte Resources zugreifen.

4. **Defense-in-Depth wird komplett (vier unabhaengige Schichten):**

| Schicht | Mechanismus | Fail-Mode |
|---------|-------------|-----------|
| 1. RLS | `WHERE tenant_id = current_setting(...)` | Zero rows (silent filter) |
| 2. Repository | `WHERE tenant_id = ?` (explicit) | Zero rows (explicit filter) |
| 3. Engine.Authorize() | `Subject.TenantID != Resource.TenantID` | Hard deny + error log |
| 4. Individual Policy | Relationship-Checks (tenant-scoped via RLS) | Hard deny |

5. **Service-Layer Flow (nachher):**
```go
func (s *ActiveService) GetVisit(ctx context.Context, visitID int64) (*Visit, error) {
    var result *Visit
    err := tenant.WithTenantTx(ctx, s.db, tenant.FromContext(ctx),
        func(ctx context.Context, tx bun.Tx) error {
            // 1. Resource laden (RLS aktiv, tenant-scoped)
            visit, err := s.visitRepo.FindByID(ctx, visitID)
            if err != nil { return err }

            // 2. Policy pruefen (Resource.TenantID aus geladener Entity)
            authCtx := &policy.Context{
                Subject:  policy.SubjectFromContext(ctx),
                Resource: policy.Resource{
                    Type: "visit", ID: visitID, TenantID: visit.TenantID,
                },
                Action: policy.ActionView,
            }
            if allowed, err := s.policyEngine.Authorize(ctx, authCtx); !allowed || err != nil {
                return authorize.ErrForbidden
            }

            result = visit
            return nil
        })
    return result, err
}
```

**Aenderungen an Subject + Resource:**
```go
type Subject struct {
    AccountID   int64
    TenantID    int64     // NEU — aus JWT Claims
    Roles       []string
    Permissions []string
}

type Resource struct {
    Type     string
    ID       interface{}
    TenantID int64         // NEU — aus geladener Entity
}
```

**Ferienbetreuung (Cross-Tenant, D4):** Platform-User oder spezielle Cross-Tenant Tokens haben `TenantID=0` → Engine-Assert wird uebersprungen → individuelle Policies regeln Zugriff.

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option A: Expliziter Check in jeder Policy | Vergessbar — gleiches Problem wie WHERE ohne RLS. Go-Policies haben keinen DSL-Zwang wie Casbin/Cerbos |
| Option B: Policy-Schicht bleibt tenant-blind | Widerspricht OWASP, AWS, Permit.io, Cerbos — "validate tenant at every layer" |
| Option C: TenantID verfuegbar ohne Pflicht | "Security by Wishful Thinking" — Feld ohne Check ist kein Security-Feature |
| Transaction-in-Middleware (PostgREST) | Jeder Request oeffnet Transaktion — auch abgelehnte. Verschwendete DB-Connections |
| JWT-Enrichment (group_ids im Token) | Dynamische Checks (aktive Supervision) unmoeglich, Staleness bis 15 Min, JWT aufgeblaeht |
| Cached Relationships | Staleness = Security-Window, Event-basierte Invalidierung komplex, GDPR-Bedenken |
| Separater Read-Only Pool fuer Auth | Kein Industriestandard, zweiter Pool umgeht SET LOCAL ROLE Sicherheitsmodell |

**Aufwand:**
| Aenderung | Aufwand |
|-----------|---------|
| `TenantID int64` in Subject + Resource | 2 Zeilen |
| `SubjectFromContext(ctx)` Helper | ~5 Zeilen |
| Fail-closed Assert in Engine.Authorize() | ~10 Zeilen |
| Policy-Eval von Middleware in Service (1 Route betroffen) | ~30 Zeilen |
| RequiresResourceAccess Middleware entfaellt | Entfernen |

**Quellen:**
- [OWASP Microservices Security Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Microservices_Security_Cheat_Sheet.html)
- [NIST SP 800-204B: PEP Placement](https://nvlpubs.nist.gov/nistpubs/SpecialPublications/NIST.SP.800-204B.pdf)
- [Microservices.io: JWT Authorization](https://microservices.io/post/architecture/2025/07/22/microservices-authn-authz-part-3-jwt-authorization.html)
- [AWS Prescriptive Guidance: SaaS Authorization](https://docs.aws.amazon.com/prescriptive-guidance/latest/saas-multitenant-api-access-authorization/introduction.html)
- [PostgREST: db-pre-request (inside transaction)](https://docs.postgrest.org/en/v12/references/transactions.html)
- [Permit.io: Multi-Tenant Authorization Best Practices](https://www.permit.io/blog/best-practices-for-multi-tenant-authorization)
- [Cerbos: Scalable Multi-Tenant Authorization](https://www.cerbos.dev/blog/how-to-implement-scalable-multitenant-authorization)

**Vermerkt in:** 03-backend.md

---

## D15: Email-Eindeutigkeit bei Skalierung

**Status:** ENTSCHIEDEN | **Severity:** HOCH

**Entscheidung:** Option A — Ein Account, mehrere Tenants. Email bleibt global UNIQUE. Tenant-Zugehoerigkeit ueber `auth.account_tenants` Junction-Tabelle mit Soft-Delete.

**Begruendung:**

1. **Industriestandard:** Auth0, WorkOS, Clerk, Microsoft Entra — alle nutzen ein Shared-Identity-Pool-Modell mit Membership-Tabelle. Kein moderner Identity-Provider empfiehlt Account-per-Tenant.

2. **Slack hat Option B ausprobiert und es war ein Fehler:** Separate Accounts pro Workspace fuehrten zu UX-Chaos (mehrere Passwoerter, kein einheitlicher Login). Enterprise Grid wurde explizit gebaut um auf Option A zu migrieren.

3. **UX:** Ein Login, ein Passwort, Tenant-Switcher. Betreuer muss sich nicht mehrere Passwoerter merken.

4. **GDPR ohne Ownership-Komplexitaet:**
   - Schule entfernt Betreuer → `account_tenants.status = inactive` + tenant-spezifische Daten loeschen
   - Betreuer will Account komplett loeschen → GDPR Request an Plattform → alle Memberships deaktivieren + Account loeschen
   - Letzte aktive Membership entfernt → Account nach Grace Period (30 Tage) zur Loeschung markiert
   - Kein Tenant "besitzt" den Account — jede Schule ist unabhaengiger Controller (EDPB-konform)

5. **Kein UNIQUE-Constraint-Change noetig:** `idx_accounts_email` bleibt global UNIQUE. Kein `UNIQUE(email, tenant_id)`.

**Schema:**
```sql
-- Bestehend (unveraendert):
-- auth.accounts.email hat globales UNIQUE INDEX

-- NEU: Junction-Tabelle
CREATE TABLE auth.account_tenants (
    id            BIGSERIAL PRIMARY KEY,
    account_id    BIGINT NOT NULL REFERENCES auth.accounts(id),
    tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id),
    status        TEXT NOT NULL DEFAULT 'active'
                  CHECK (status IN ('pending', 'active', 'inactive')),
    invited_at    TIMESTAMPTZ DEFAULT now(),
    activated_at  TIMESTAMPTZ,
    deactivated_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(account_id, tenant_id)
);

CREATE INDEX idx_account_tenants_account ON auth.account_tenants(account_id);
CREATE INDEX idx_account_tenants_tenant ON auth.account_tenants(tenant_id);
CREATE INDEX idx_account_tenants_active ON auth.account_tenants(account_id, tenant_id)
    WHERE status = 'active';
```

**Login-Flow (Zusammenspiel mit D6 + D11):**
```
1. User oeffnet altenberge.moto-app.de/login (D11: Rewrite Pattern)
2. POST /auth/login { email, password, tenant_slug: "altenberge" } (D6)
3. Backend: Finde Account by email (global UNIQUE)
4. Backend: Pruefe account_tenants WHERE account_id=? AND tenant_id=? AND status='active'
5. Backend: Verifiziere Passwort
6. Backend: Lade Rollen/Permissions fuer diesen Tenant
7. JWT mit tenant_id + org_id + scope
```

**Tenant-Switch (D4):**
```
1. User klickt "Zu OGS Greven wechseln"
2. POST /auth/switch-tenant { tenant_slug: "greven" }
3. Backend: Pruefe account_tenants fuer neuen Tenant
4. Backend: Neues JWT mit neuem tenant_id
5. Frontend: SWR-Cache invalidieren (Tenant-prefixed Keys)
```

**Tenant-Admin Rechte:**
| Aktion | Erlaubt? |
|--------|----------|
| Membership hinzufuegen (einladen) | Ja |
| Membership deaktivieren (entfernen) | Ja — `status = 'inactive'` |
| Rollen innerhalb Tenant aendern | Ja |
| Account-Passwort aendern | Nein — nur der User selbst |
| Account komplett loeschen | Nein — nur User (GDPR) oder Operator |
| Andere Tenant-Memberships sehen | Nein |

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option B: Account pro Tenant | Slack hat es versucht, war UX-Desaster. Passwort-Sync unlösbar, doppelte Datenhaltung |
| Option C: Hybrid mit Ownership | Unnoetige Komplexitaet — wer ist Owner bei gleichzeitigem Start? Was bei Tenant-Schliessung? Microsoft braucht es fuer B2B mit separaten Azure-ADs, Phoenix hat zentrales Auth |

**Quellen:**
- [Auth0: Multiple Organization Architecture](https://auth0.com/docs/get-started/architecture-scenarios/multiple-organization-architecture)
- [WorkOS: Users and Organizations](https://workos.com/docs/user-management/users-organizations)
- [Clerk: Multi-Tenant Architecture](https://clerk.com/docs/guides/how-to-design-multitenant-saas-architecture)
- [Microsoft Entra: Multi-Tenant Identity](https://learn.microsoft.com/en-us/azure/architecture/guide/multitenant/considerations/identity)
- [Slack Enterprise Grid (Migration von Option B zu A)](https://docs.slack.dev/enterprise/)

**Vermerkt in:** 02-datenbank.md, 03-backend.md

---

## D16: Raw SQL Subqueries + Seed-Daten Tenant-Filterung

**Status:** ENTSCHIEDEN | **Severity:** MITTEL

**Entscheidung:** Option A (RLS als primaere Absicherung) + 6 gezielte Massnahmen. Kein manuelles `WHERE tenant_id` in Raw-SQL-Subqueries noetig — RLS filtert alle Query-Formen innerhalb von `WithTenantTx` (D8).

**Begruendung:**

PostgreSQL RLS filtert **alle** Query-Formen innerhalb einer Transaktion mit gesetztem `SET LOCAL ROLE`:

| Query-Form | RLS filtert? | Verifiziert |
|------------|-------------|-------------|
| Subqueries (`WHERE IN (SELECT ...)`) | Ja | PG 17 Docs: Policies als unsichtbare WHERE-Clause |
| CTEs (`WITH ... AS`) | Ja | PG 17 Docs, CVE-2024-10976 vor PG 17.1 gefixt |
| JOINs (INNER + LEFT) | Ja (beide Seiten) | PG 17 Docs (LEFT JOIN: NULLs statt hidden rows) |
| INSERT...SELECT | Ja (SELECT + INSERT CHECK) | PG 17 Docs |
| UPDATE...FROM (cross-schema) | Ja (beide Tabellen) | PG 17 Docs |
| BUN Relation() Eager Loading | Ja (separate SELECTs) | 14 Nutzungen verifiziert, alle sicher |

**Codebase-Analyse: Raw SQL Patterns (verifiziert):**
- `suggestions/post_repository.go`: 6x ColumnExpr-Subqueries + JOIN zu `users.persons` (cross-schema) → RLS
- `active/visits.go:194`: WHERE IN (SELECT FROM active.groups) → RLS
- `education/grade_transition.go:520`: UPDATE users.students FROM education.grade_transition_mappings (cross-schema) → RLS
- `platform/announcement_view_repository.go:235`: JOIN ueber 3 Schemas (platform→auth→users) → RLS (platform-Tabellen haben kein RLS, das ist korrekt)
- `auth/password_reset_rate_limit.go:62`: CTE mit INSERT...ON CONFLICT → RLS

**BUN Relation(): 14 Nutzungen, 2 cross-schema — alle sicher.** BUN generiert separate SELECT-Queries (keine JOINs), jede wird unabhaengig durch RLS gefiltert. Das Team hat cross-schema Relation() bereits in 7 Stellen durch explizite JOINs ersetzt.

---

**6 gezielte Massnahmen:**

**1. PRIO-1: RowsAffected()-Checks (Silent Failure Prevention)**

72% aller UPDATE/DELETE Operationen (~70 von ~97 Call-Sites) pruefen `RowsAffected()` nicht. Mit RLS bedeutet das:
```
Tenant A: UPDATE visits SET exit_time = NOW() WHERE id = 42
→ RLS blockiert (Visit gehoert Tenant B)
→ UPDATE 0, kein Error
→ Code laeuft weiter als waere Checkout erfolgreich
→ Student bleibt "eingecheckt" — Silent Data Corruption
```

Loesung: `assertRowsAffected(result sql.Result, expected int64) error` Helper im Base-Package. Standardmaessig in allen Repository UPDATE/DELETE-Methoden nutzen.

Betroffene Stellen (Auszug):
- `users/student.go` — 2 Updates ohne Check
- `users/staff.go` — 2 Updates ohne Check
- `auth/accounts.go` — 2 Updates ohne Check
- `active/work_session.go` — 2 Updates ohne Check
- `users/privacy_consent.go` — 6 Updates ohne Check

**2. PRIO-1: PostgreSQL >= 17.6 als Mindestversion**

| CVE | Beschreibung | Gefixt in | Relevanz fuer Phoenix |
|-----|-------------|-----------|----------------------|
| CVE-2024-10976 | Plan-Cache ignoriert Role-Wechsel bei Subqueries/CTEs + SET LOCAL ROLE | PG 17.1 | Direkt: D8-Pattern mit SET LOCAL ROLE + Subqueries |
| CVE-2025-8713 | Optimizer-Statistiken leaken RLS-versteckte Daten | PG 17.6 | Indirekt: Statistik-basiertes Informationsleck |

Harte Anforderung: `PostgreSQL >= 17.6` in Architektur-Docs und Docker-Compose vermerken.

**3. PRIO-2: Seeds mit tenant_id**

23 Seed-Files, 42+ direkte `NewInsert()` Calls, null tenant-Awareness. Seeds laufen als `phoenix_admin` (BYPASSRLS) → RLS schuetzt hier nicht. `NOT NULL` Constraint auf `tenant_id` blockiert Inserts ohne Wert sofort.

Aenderungen:
- Seed-System bekommt `tenantID int64` Parameter
- Jeder Insert setzt `tenant_id` explizit
- `resetData()` (`TRUNCATE CASCADE`) bleibt global (nur Dev-Umgebung)
- Multi-Tenant Seeds: Mindestens 2 Tenants mit unterschiedlichen Daten

**4. PRIO-2: View `users.expired_privacy_consents` fixen**

```sql
-- Migration 001003007: View ohne security_invoker
CREATE VIEW users.expired_privacy_consents AS
SELECT pc.*, s.person_id ... FROM users.privacy_consents pc
JOIN users.students s ON pc.student_id = s.id WHERE ...
```

Views sind in PostgreSQL standardmaessig `SECURITY DEFINER` (Owner-Rechte). Wenn Owner BYPASSRLS hat → alle Tenant-Daten sichtbar.

Fix:
```sql
CREATE OR REPLACE VIEW users.expired_privacy_consents
WITH (security_invoker = true) AS ...
```

**5. PRIO-2: Advisory Lock mit Zwei-Argument-Form**

```go
// VORHER (session_service.go:168) — Cross-Tenant-Blocking
_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", activityID)

// NACHHER — Tenant-isoliert, kein Overflow-Risiko
_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?, ?)", tenantID, activityID)
```

PostgreSQL's Zwei-Argument-Form (`pg_advisory_xact_lock(key1, key2)`) ist sauberer als Multiplikation — kein Overflow bei grossen IDs.

**6. PRIO-3: LEFT JOIN Korrektheits-Review**

Bei LEFT JOINs mit RLS-geschuetzten Tabellen: Wenn RLS die rechte Seite versteckt → NULLs. Code der `IS NULL` als "existiert nicht" interpretiert, hat ein stilles Problem. 7+ Stellen mit expliziten LEFT JOINs im Code. Kein Sicherheitsproblem, aber Korrektheitsproblem (falsche UI-Anzeigen).

→ Niedrig-Prio Cleanup-Task waehrend Multi-Tenancy-Migration.

---

**Praeventions-Checkliste fuer zukuenftigen Code:**

| Regel | Grund |
|-------|-------|
| Keine Materialized Views auf Tenant-Tabellen | Bypassen RLS komplett |
| Kein COPY FROM auf RLS-Tabellen | PostgreSQL blockiert es |
| Keine SECURITY DEFINER Funktionen mit BYPASSRLS-Owner | Bypassen RLS |
| Alle Views mit `security_invoker = true` | Views bypassen RLS sonst |
| RowsAffected() nach jedem UPDATE/DELETE | Silent Failures durch RLS |
| Advisory Locks mit tenant_id als Key1 | Sonst Cross-Tenant-Blocking |

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option B: Manuelles WHERE tenant_id in jedem Raw SQL | Redundant zu RLS, fehleranfaellig, hoher Aufwand. RLS filtert bereits alle Query-Formen |

**Vermerkt in:** 02-datenbank.md (PG-Version), 03-backend.md (RowsAffected, Advisory Lock)

---

## D17: Tenant-Validierung in Frontend Middleware

**Status:** ENTSCHIEDEN | **Severity:** MITTEL

**Entscheidung:** Modifizierte Option B — Middleware bleibt stateless (nur Slug-Extraktion + Rewrite). Tenant-Validierung passiert im `[tenant]/layout.tsx` via `resolveTenant()` (D5-Endpoint). Invalid Tenants bekommen `notFound()` im Layout — vor der Login-Page.

**Begruendung:**

1. **Industriestandard (Vercel Platforms + Cal.com Pattern):** Beide Referenz-Implementierungen halten die Middleware stateless und validieren Tenants auf Page-/Layout-Ebene. Dub.co nutzt Middleware-Validation mit 3-Layer-Cache (LRU → Redis → PlanetScale HTTP), aber fuer Millionen Redirects/Tag — nicht fuer Dutzende OGS.

2. **D5 + D11 loesen das Problem bereits:** D11 entschied Rewrite zu `[tenant]/...`, D5 entschied `resolveTenant()` Endpoint + `TenantProvider` im Layout. Die Validation ist ein einzeiliges `if (!tenantData) notFound()` nach dem Data-Load der sowieso passiert:
```typescript
// app/[tenant]/layout.tsx — nutzt D5 (TenantProvider) + D11 (Rewrite)
export default async function TenantLayout({ params, children }) {
    const { tenant } = await params;
    const tenantData = await resolveTenant(tenant);  // D5-Endpoint

    if (!tenantData) {
        notFound();  // → app/[tenant]/not-found.tsx
    }

    return <TenantProvider value={tenantData}>{children}</TenantProvider>;
}
```

3. **Self-hosted = kein Edge-Vorteil:** Phoenix laeuft auf Docker Compose, nicht Vercel. Middleware laeuft im selben Node.js-Prozess — kein geographischer Vorteil fuer Edge-Validation. Der `[tenant]/layout.tsx` ist genauso schnell.

4. **Keine neue Infrastruktur:** Option A (Cached Allow-List) wuerde Redis/In-Memory-Cache in der Middleware erfordern — eine neue Abhaengigkeit fuer ein Problem das bei Dutzenden Tenants nicht existiert. Option C's Endpoint existiert bereits als D5-Entscheidung.

5. **Neuer Tenant sofort sichtbar:** Kein TTL-Cache in Middleware → neuer Tenant ist ab Erstellung erreichbar. Bei Option A wuerde ein neuer Tenant bis zu 5 Min unsichtbar sein.

**Konkreter Flow:**
```
User besucht altenberge.moto-app.de/dashboard

1. DNS: *.moto-app.de → Server IP
2. Middleware: Extrahiert "altenberge", rewritet zu /altenberge/dashboard (D11)
3. [tenant]/layout.tsx: resolveTenant("altenberge") → { id: 42, name: "OGS Altenberge", ... }
4. TenantProvider liefert Tenant-Daten an alle Children (D5)
5. (protected)/layout.tsx: Session-Check → Login-Redirect wenn noetig

User besucht nichtexistent.moto-app.de/dashboard

1. DNS: *.moto-app.de → Server IP
2. Middleware: Extrahiert "nichtexistent", rewritet zu /nichtexistent/dashboard
3. [tenant]/layout.tsx: resolveTenant("nichtexistent") → null
4. notFound() → app/[tenant]/not-found.tsx
5. User sieht: "Diese OGS existiert nicht" (OHNE Login-Page dazwischen)
```

**Middleware bleibt stateless:**
```typescript
// middleware.ts — kein I/O, kein Cache, kein fetch()
export function middleware(request: NextRequest) {
    const hostname = request.headers.get("host") || "";
    const subdomain = extractSubdomain(hostname);

    // Operator-Routes: bestehende Logik beibehalten
    if (pathname.startsWith("/operator")) { /* ... bestehend ... */ }

    // Tenant-Routes: Rewrite (D11)
    if (subdomain && !RESERVED_SUBDOMAINS.includes(subdomain)) {
        return NextResponse.rewrite(
            new URL(`/${subdomain}${pathname}`, request.url)
        );
    }

    return NextResponse.next();
}

const RESERVED_SUBDOMAINS = ["www", "api", "admin", "operator", "app"];
```

**Verworfene Alternativen:**
| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option A: Cached Allow-List (Redis/In-Memory) | Overengineering fuer Dutzende Tenants. Neue Infrastruktur-Abhaengigkeit (Redis). TTL-Lag bei neuen Tenants. Dub.co-Pattern fuer Millionen Requests, nicht OGS-Software |
| Option B (original): Validation erst beim Login | UX-Luecke: User sieht Login-Page fuer nicht-existierende OGS bevor Error kommt |
| Option C: Middleware ruft Backend-Endpoint auf | Edge Runtime kann kein `pg` (TCP). HTTP-Fetch in Middleware ist langsamer als Layout-Level Validation. D5-Endpoint wird im Layout genutzt, nicht in Middleware |

**Quellen:**
- [Vercel Platforms Starter Kit](https://github.com/vercel/platforms) — Middleware stateless, Page-Level `notFound()`
- [Cal.com orgDomains.ts](https://github.com/calcom/cal.com) — Middleware nur Reserved-Words, DB-Lookup im Page-Layer
- [Dub.co Middleware](https://github.com/dubinc/dub) — 3-Layer Cache Pattern (LRU → Redis → PlanetScale HTTP) fuer High-Traffic
- [Next.js Multi-Tenant Guide](https://nextjs.org/docs/app/guides/multi-tenant)

**Vermerkt in:** 04-frontend.md

---

## D18: Org-Scope (Traeger-Buero) Implementierung

**Status:** ENTSCHIEDEN | **Severity:** KRITISCH

**Entscheidung:** Option B — Application-Layer mit `WithAdminTx` + org_id-Filter. Kein org-aware RLS.

**Begruendung:**

1. **RLS bleibt simpel:** Jede RLS-Policy hat genau eine Bedingung (`tenant_id = X`). Kein Subquery-Overhead, keine Performance-Risiken bei Index-Scans, keine Komplexitaet in der kritischsten Sicherheitsschicht.

2. **Traeger-Buero ist eine Nischen-Nutzergruppe:** 3-10 User pro Traeger vs. Hunderte Betreuer. RLS fuer den 99%-Fall (Betreuer) optimieren, nicht fuer den 1%-Fall.

3. **Maximale Flexibilitaet:** Aenderungen am Org-Scope betreffen nur Application-Code, nicht DB-Policies. Wenn sich die Business-Anforderungen aendern (z.B. Org-Scope nur fuer bestimmte Daten, nicht alle), ist das ein Service-Layer-Change, keine Migration.

4. **Tagesgeschaeft per Tenant-Switch:** Traeger-Buero-User arbeiten im Regelfall an einer spezifischen OGS (Vertretungen planen, Krankmeldungen bearbeiten). Dafuer nutzen sie Tenant-Switch + `WithTenantTx` — voller RLS-Schutz wie jeder andere User.

**Design:**

```
Traeger-Buero Workflow:

1. Login → JWT mit tenant_id (primaere OGS) + org_id
2. Tagesgeschaeft: Tenant-Switch zu spezifischer OGS (WithTenantTx, RLS aktiv)
3. Aggregierte Ansichten: Dedizierte Org-Scope-Endpoints (WithAdminTx + org_id-Filter)
```

**Guardrails gegen BYPASSRLS-Missbrauch:**

| Guardrail | Mechanismus |
|-----------|------------|
| **Dedizierter OrgScopeService** | Eigener Service mit eigenen Queries — kein bestehendes Repo wird mit `WithAdminTx` aufgerufen |
| **Eigene Permission** | `org:dashboard:read` — nur Traeger-Buero-Rolle hat diese |
| **Nur Read/Aggregate** | Org-Scope ist read-only. Schreib-Operationen IMMER per Tenant-Switch + `WithTenantTx` |
| **Org-Filter als Pflicht** | Jede Query im OrgScopeService hat `WHERE tenant_id IN (SELECT id FROM platform.schools WHERE organization_id = ?)` |
| **Audit-Logging** | Jeder Org-Scope-Zugriff wird in `audit.auth_events` protokolliert |

**OrgScopeService-Konzept:**

```go
type OrgScopeService struct {
    db     *bun.DB
    logger *slog.Logger
}

// IMMER org-gefiltert, IMMER read-only, IMMER geloggt
func (s *OrgScopeService) StaffOverview(ctx context.Context, orgID int64) ([]StaffSummary, error) {
    var result []StaffSummary
    err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
        return tx.NewSelect().
            Model(&result).
            Where("tenant_id IN (SELECT id FROM platform.schools WHERE organization_id = ?)", orgID).
            Scan(ctx)
    })
    s.logger.Info("org_scope_access", "org_id", orgID, "action", "staff_overview")
    return result, err
}
```

**Was Org-Scope NICHT kann:**
- Keine Schreib-Operationen (kein UPDATE/DELETE/INSERT ueber Org-Scope)
- Kein Zugriff auf andere Organisationen (org_id kommt aus JWT, nicht aus Request)
- Keine Impersonation (Traeger-Buero bleibt Traeger-Buero, wird nie als OGS-Admin behandelt)

**Verworfene Alternativen:**

| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Option A: Org-aware RLS | Subquery in jeder Policy (`OR tenant_id IN (SELECT ...)`) — Performance-Risiko bei Index-Scans, erhoehte Komplexitaet der kritischsten Sicherheitsschicht, schwerer zu testen und zu auditieren |
| Option C: Nur Tenant-Switch | Keine aggregierten Ansichten moeglich, schlechte UX fuer Personalplanung/Lohnabrechnung die mehrere OGS ueberblicken muss |
| Org-Scope per RLS spaeter nachrüsten | Wenn (b) funktioniert, gibt es keinen Grund auf (a) umzusteigen. (b) → (a) Migration ist moeglich aber unwahrscheinlich noetig |

**Vermerkt in:** 03-backend.md, 06-offene-punkte.md

---

## D19: Eltern-Datenisolation (Kind-Level)

**Status:** ENTSCHIEDEN | **Severity:** KRITISCH

**Entscheidung:** Service-Layer-Filterung. Eltern-Endpoints filtern per `students_guardians`-Beziehung. Kein RLS auf Kind-Ebene.

**Problem:**

RLS isoliert auf Tenant-Ebene. Sobald ein Elternteil Zugang zu einem Tenant hat (Kind an OGS Altenberge eingeschrieben), sieht die RLS-Policy ALLE Rows des Tenants. Elternteil A koennte Kinderdaten von Familie B sehen — Name, Anwesenheit, Gesundheitsinfos, Notfallkontakte. Das ist ein GDPR Art. 33/34 meldepflichtiger Vorfall (Kinderdaten, Art. 8 besonderer Schutz).

**Begruendung:**

1. **Eltern-App hat eigene Endpoints:** Die Eltern-UI ist eine separate Ansicht (00-anforderungen.md Sektion 3.4). Eltern nutzen nicht die Betreuer-Endpoints. Dedizierte Parent-Endpoints koennen im Service-Layer per Guardian-Student-Beziehung filtern — kein RLS-Umbau noetig.

2. **RLS fuer Kind-Level waere massiv overengineered:** Man muesste `app.current_guardian_id` als Session-Variable setzen und jede RLS-Policy um `OR (current_setting('app.current_role') = 'guardian' AND student_id IN (SELECT ...))` erweitern. Das verkompliziert JEDE Policy fuer einen einzigen User-Typ.

3. **Bestehende Pattern als Vorlage:** Die `TeacherGroupPolicy` filtert bereits Lehrer auf ihre zugewiesenen Gruppen. Das gleiche Pattern funktioniert fuer Eltern → Kinder. Bewaehrtes Muster, kein neues Konzept.

4. **Defense-in-Depth bleibt intakt:** RLS schuetzt weiterhin cross-tenant (Elternteil sieht nur Daten der OGS seines Kindes). Service-Layer schuetzt intra-tenant (Elternteil sieht nur eigenes Kind). Zwei Schichten, klar getrennt.

**Design:**

```go
// Dedizierte Parent-Endpoints (NICHT die Betreuer-Endpoints!)
// api/parent/api.go
r.Route("/parent", func(r chi.Router) {
    r.Use(authorize.RequiresRole("guardian"))

    r.Get("/children", rs.listMyChildren)       // Nur eigene Kinder
    r.Get("/children/{id}/visits", rs.childVisits) // Nur Besuche eigener Kinder
    r.Get("/children/{id}/schedule", rs.childSchedule)
})

// services/parent/parent_service.go
func (s *ParentService) ListMyChildren(ctx context.Context, guardianAccountID int64) ([]Child, error) {
    var children []Child
    err := tenant.WithTenantTx(ctx, s.db, tenant.FromContext(ctx),
        func(ctx context.Context, tx bun.Tx) error {
            // RLS filtert auf Tenant, Service filtert auf Guardian
            return tx.NewSelect().
                Model(&children).
                Join("INNER JOIN users.students_guardians sg ON sg.student_id = student.id").
                Join("INNER JOIN users.guardian_profiles gp ON gp.id = sg.guardian_profile_id").
                Where("gp.account_id = ?", guardianAccountID).
                Scan(ctx)
        })
    return children, err
}
```

**Sicherheits-Schichten fuer Eltern:**

| Schicht | Was sie schuetzt | Fail-Mode |
|---------|-----------------|-----------|
| 1. RLS | Eltern sehen nur Daten ihres Tenants (OGS) | Zero rows cross-tenant |
| 2. `RequiresRole("guardian")` | Nur Eltern erreichen Parent-Endpoints | 403 Forbidden |
| 3. Service-Layer JOIN | Nur eigene Kinder zurueckgegeben | Leere Liste |
| 4. Policy Engine (optional) | `ParentChildPolicy` als zusaetzlicher Check | Deny + Audit-Log |

**Wichtig:** Betreuer-Endpoints (`/api/students`, `/api/visits` etc.) sind fuer Eltern NICHT erreichbar — andere Permission (`students:read` vs. `parent:children:read`). Selbst wenn ein Elternteil die URL kennt, blockiert die Permission-Middleware.

**Verworfene Alternativen:**

| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| RLS auf Kind-Ebene | Jede Policy wird um Guardian-Subquery erweitert. Massive Komplexitaet fuer einen einzigen User-Typ. Performance-Impact auf alle Queries, nicht nur Eltern-Queries |
| Separate DB-Rolle fuer Eltern | Vierte PostgreSQL-Rolle (`phoenix_guardian`) mit eigener RLS-Policy. Erhoehte Komplexitaet bei `SET LOCAL ROLE`, kaum Vorteil gegenueber Service-Layer |
| Eltern-Daten in separatem Schema | Physische Isolation. Overkill, dupliziert Daten, erschwert Betreuer-Zugriff auf Kind-Infos |

**GDPR-Compliance:**

- Art. 8 (Kinderdaten): Besonderer Schutz durch doppelte Isolation (RLS + Service-Layer)
- Art. 25 (Privacy by Design): Eltern-Endpoints sind Default-Deny, nur eigene Kinder per JOIN
- Art. 32 (Sicherheit): Zwei unabhaengige Schutzschichten, keine single-point-of-failure

**Vermerkt in:** 03-backend.md, 06-offene-punkte.md

---

## D20: IoT Device Auth Bootstrap (Chicken-and-Egg)

**Status:** ENTSCHIEDEN | **Severity:** KRITISCH

**Entscheidung:** Two-Phase Lookup (analog D6 Login-Flow). Device-Lookup via `WithAdminTx`, danach `WithTenantTx` fuer Check-In Operations. Globaler PIN wird durch per-Device PIN-Hash ersetzt.

**Problem:**

IoT-Check-In ist die Kernfunktion. Der aktuelle Flow:
1. Device sendet `Authorization: Bearer <api_key>` + `X-Staff-PIN`
2. Backend sucht Device via `FindByAPIKey()` — **kein Tenant-Context**
3. Device bestimmt den Tenant fuer den Check-In

Mit RLS braucht der Device-Lookup (Schritt 2) einen Tenant-Context. Den Tenant-Context kennt man erst nach dem Lookup (Schritt 3). Chicken-and-Egg.

Zusaetzlich: Der Staff-PIN ist aktuell eine einzelne Env-Var (`OGS_DEVICE_PIN`) — identisch fuer alle OGS. Mit Multi-Tenancy untragbar (PIN von OGS A funktioniert bei OGS B).

**Begruendung:**

1. **Exakt das D6-Login-Pattern:** Login sucht Account global via `WithAdminTx`, validiert Tenant-Membership, dann Switch zu `WithTenantTx`. IoT macht dasselbe — Device global suchen, `device.TenantID` extrahieren, dann `WithTenantTx` fuer den Check-In.

2. **Devices gehoeren physisch zu einer OGS:** Ein RFID-Reader ist in einem Raum einer spezifischen OGS installiert. `tenant_id` auf `iot.devices` ist semantisch korrekt — nicht nur technisch noetig.

3. **Per-Device PIN-Hash ist sicherer:** Aktuell teilen sich alle Devices und alle OGS einen einzigen PIN. Kompromittierung = alle OGS betroffen. Per-Device PIN (Argon2id-Hash in `iot.devices.pin_hash`) limitiert den Blast-Radius auf ein einzelnes Geraet.

4. **`device_id` und `api_key` bleiben global UNIQUE:** Keine Aenderung an der globalen Eindeutigkeit. Ein Device kann nicht bei zwei OGS gleichzeitig registriert sein.

**Design:**

```go
// auth/device/device_auth.go — Neuer Flow
func (d *DeviceAuthenticator) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := extractBearerToken(r)

        // Phase 1: Device-Lookup via WithAdminTx (analog D6 Login)
        var dev *iot.Device
        err := tenant.WithAdminTx(r.Context(), d.db, func(ctx context.Context, tx bun.Tx) error {
            var err error
            dev, err = d.deviceRepo.FindByAPIKey(ctx, apiKey)
            return err
        })
        if err != nil || dev.Status != iot.DeviceStatusActive {
            http.Error(w, "device not found or inactive", http.StatusForbidden)
            return
        }

        // Phase 2: PIN-Validierung (per-Device Hash)
        pin := r.Header.Get("X-Staff-PIN")
        if !verifyPINHash(dev.PINHash, pin) {
            http.Error(w, "invalid PIN", http.StatusUnauthorized)
            return
        }

        // Phase 3: Context mit Device + TenantID fuer nachfolgende Handler
        ctx := context.WithValue(r.Context(), CtxDevice, dev)
        ctx = context.WithValue(ctx, CtxTenantID, dev.TenantID)

        // Check-In Handler nutzt WithTenantTx(dev.TenantID) fuer alle Operationen
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Migrations-Aenderungen:**

```sql
-- 1. iot.devices bekommt tenant_id (bereits in 02-datenbank.md Migrationsliste)
ALTER TABLE iot.devices ADD COLUMN tenant_id BIGINT
    REFERENCES platform.schools(id);
UPDATE iot.devices SET tenant_id = (SELECT id FROM platform.schools LIMIT 1);
ALTER TABLE iot.devices ALTER COLUMN tenant_id SET NOT NULL;

-- 2. Per-Device PIN-Hash
ALTER TABLE iot.devices ADD COLUMN pin_hash TEXT;
-- Bestehende Devices: PIN aus OGS_DEVICE_PIN env-var hashen und migrieren
-- Danach: OGS_DEVICE_PIN env-var entfernen
```

**BUN Model Aenderung:**

```go
// models/iot/device.go
type Device struct {
    // ... bestehende Felder ...
    TenantID int64  `bun:"tenant_id,notnull" json:"-"`
    PINHash  string `bun:"pin_hash" json:"-"`
}
```

**Verworfene Alternativen:**

| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| `iot.devices` von RLS ausnehmen | Sonderfall, bricht das einheitliche Muster. Jede Tabelle unter RLS ist sicherer als Ausnahmen |
| Tenant in API-Key kodieren | Fragil, Key-Rotation wird komplex, Tenant-Wechsel eines Devices erfordert neuen Key |
| Globalen PIN beibehalten | Ein kompromittierter PIN betrifft alle OGS. Unakzeptabel fuer GDPR-reguliertes System mit Kinderdaten |

**Vermerkt in:** 03-backend.md

---

## D21: GDPR Art. 17 Erasure bei Shared Accounts (09-C7)

**Status:** ENTSCHIEDEN | **Severity:** KRITISCH

**Entscheidung:** Technisch geloest durch D15 (globale Accounts + `account_tenants`). Juristisch erfordert AVV + DPIA als separaten Workstream. Art. 28 Auftragsverarbeitung ist der korrekte und branchenuebliche Rechtsrahmen.

**Problem:**

D15 definiert: Ein Account, mehrere Tenants, Email global UNIQUE. Szenario:
- Betreuerin Lisa arbeitet bei OGS A (Caritas) und OGS B (AWO)
- Lisa kuendigt bei OGS A. Caritas muss auf Art. 17 Loeschantrag reagieren
- Der Account (Email, Name, Password-Hash) liegt in `auth.accounts` — global, kein `tenant_id`
- Caritas kann den Account nicht loeschen, weil AWO ihn noch braucht
- `account_tenants.status = 'inactive'` allein ist keine Loeschung (EuGH C-553/07)

**Warum Art. 28 (Auftragsverarbeitung) korrekt und branchenueblich ist:**

1. **Jeder Traeger ist eigenstaendiger Data Controller** (Art. 4 Nr. 7) fuer seine Tenant-Daten
2. **moto ist Auftragsverarbeiter** (Art. 28) — verarbeitet Daten im Auftrag der Controller
3. **Art. 17 Loeschpflicht gilt nur fuer den jeweiligen Controller.** Wenn Caritas "loesche Lisa" sagt, bezieht sich das nur auf Caritas' Datenbestand. AWO ist ein unabhaengiger Controller mit eigener Rechtsgrundlage (Art. 6(1)(b) — Vertragserfuellung)
4. **Kein Joint-Controller-Szenario** (Art. 26): Caritas und AWO bestimmen nicht gemeinsam Zweck und Mittel. Sie nutzen unabhaengig voneinander dieselbe SaaS-Plattform (vgl. EDPB Guidelines 07/2020, Reisebüro-Analogie)

**Industrie-Praxis (exakt D15-Pattern):**

| Plattform | Architektur | Loeschung |
|-----------|-------------|-----------|
| Auth0 | User global, Org-Membership per Junction | Member entfernen ≠ User loeschen |
| WorkOS | "Users can belong to multiple tenants" | Membership loeschen ≠ User loeschen |
| Microsoft Entra | "Use a single identity per person" | Zugang entziehen, Identity bleibt |
| Okta | DPA: Controller-spezifische Loeschung | Processor loescht pro Controller-Auftrag |

**Technische Loeschung bei Art. 17 Anfrage (Controller-Scope):**

```
Caritas sagt "Loesche Lisa":

1. HARD DELETE: account_tenants Record fuer OGS A
2. HARD DELETE: account_roles fuer OGS A (tenant_id = OGS_A)
3. HARD DELETE: account_permissions fuer OGS A
4. HARD DELETE: Alle Tenant-spezifischen Daten bei OGS A
   (Activity Logs, Visits, Work Sessions, Audit Records etc.)
5. auth.accounts Record BLEIBT — AWO hat eigene Rechtsgrundlage

Wenn Lisa auch OGS B verlaesst (letzter account_tenants Record):
6. Grace Period (30 Tage, D15)
7. HARD DELETE: auth.accounts Record
```

**Self-Service Account-Loeschung (Pflicht):**

Lisa muss ihren gesamten Account selbst loeschen koennen — unabhaengig von einzelnen Traegern. Dies ist ein direkter Art. 17 Anspruch gegen moto als Verantwortlichen fuer die Plattform-Identity-Daten.

```go
// api/account/api.go
r.With(authorize.RequiresAuthentication).Delete("/me", rs.deleteMyAccount)

// Loescht: auth.accounts + ALLE account_tenants + ALLE Tenant-Daten
// Erfordert: Passwort-Bestaetigung + 72h Cooling-Off Period
```

**Juristischer Workstream (NICHT in technischen Docs loesbar):**

| Dokument | Zweck | Verantwortlich |
|----------|-------|---------------|
| **AVV-Template** (Art. 28(3)) | Pro Traeger, definiert Loeschung = Controller-Scope | moto + Anwalt |
| **Datenschutzerklaerung** | Erklaert Shared-Identity-Modell, Self-Service Loeschung | moto |
| **DPIA** (Art. 35) | Wahrscheinlich Pflicht: Kinderdaten + RFID-Tracking + Skalierung | moto + DSB |
| **Eltern-Information** | Template fuer Traeger, erklaert Datenverarbeitung | moto bereitstellen, Traeger verteilen |

**Warum DPIA wahrscheinlich Pflicht:**
- Kinderdaten (Recital 38, Art. 8 — besonderer Schutz)
- Systematisches RFID-Standort-Tracking (Art. 35(3)(b) — Monitoring im grossen Umfang)
- DSK DPIA-Muss-Liste: "Systematische Ueberwachung oeffentlich zugaenglicher Bereiche"
- LDI NRW ist bei Bildungsdaten erfahrungsgemaess streng

**Grauzone (Anwalt/DSB klaeren):**

1. Welche Kinderdaten fallen unter SchulG NRW §120 (Schule als Controller) vs. Traeger-eigene Daten?
2. Ist RFID-Tracking ein "systematisches Monitoring" nach Art. 35(3)(c)?
3. Exakte Abgrenzung moto als Processor vs. Controller fuer Operator-Dashboard-Daten

**Verworfene Alternativen:**

| Alternative | Grund fuer Ablehnung |
|-------------|---------------------|
| Art. 26 Joint Controller | Traeger bestimmen nicht gemeinsam Zweck/Mittel. SaaS-Standard ist Art. 28, nicht Art. 26. Wuerde unnoetige vertragliche Komplexitaet schaffen |
| Account-Splitting bei Loeschung | Account duplizieren, alten loeschen, neuen fuer verbleibende Tenants erstellen. Technisch komplex, Email-Eindeutigkeit problematisch, kein Industrie-Standard |
| Soft-Delete als Loeschung deklarieren | EuGH C-553/07 interpretiert "erasure" als echte Loeschung. Soft-Delete allein reicht nicht. LDI NRW wuerde das beanstanden |

**Quellen:**
- EDPB Guidelines 07/2020 (Controller/Processor)
- DSK Kurzpapier Nr. 13 (Auftragsverarbeitung)
- EuGH C-553/07 (Erasure = echte Loeschung)
- EuGH C-210/16 (Joint Controller Abgrenzung)
- Auth0 GDPR Documentation
- LDI NRW Bildungsdatenschutz

**Vermerkt in:** 06-offene-punkte.md

---

## Entscheidungs-Log

| # | Punkt | Severity | Entscheidung | Datum |
|---|-------|----------|-------------|-------|
| D1 | Operator Impersonation | MITTEL | Keine Impersonation jetzt, Architektur kompatibel fuer spaeter | 2026-02-08 |
| D2 | base.Model vs TenantModel | HOCH | TenantModel Mixin (Compile-Time Safety, BUN-Kompatibilitaet) | 2026-02-08 |
| D3 | Deployment/Infrastructure | MITTEL | Zurueckgestellt (kommt am Ende mit Phasen-Plan) | 2026-02-08 |
| D4 | Cross-Tenant Mechanismus | HOCH | Tenant-Switch + gezielter Service-Level Read fuer Ferienbetreuung | 2026-02-08 |
| D5 | Frontend TenantContext | MITTEL | useTenant() Hook mit Identitaet + Settings/Branding aus Login-Response | 2026-02-08 |
| D6 | Login Handler Pattern | MITTEL | tenant_slug im Request-Body (Auth0/WorkOS Pattern) | 2026-02-08 |
| D7 | tenant_id=0 RLS-Bypass | KRITISCH | Zwei-Rollen: phoenix_app (NOBYPASSRLS) + phoenix_admin (BYPASSRLS), kein =0 Bypass | 2026-02-08 |
| D8 | set_config + Transaction Gap | KRITISCH | SET LOCAL ROLE pro Transaktion, ein Pool, drei Rollen (phoenix_auth/tenant/admin) | 2026-02-08 |
| D9 | QueryHook Strategie | HOCH | Geloest durch D8: QueryHook entfaellt, set_config einmal pro Transaktion | 2026-02-08 |
| D10 | BeforeAppendModel Shadowing | HOCH | Kein Hook auf TenantModel, Service setzt tenant_id explizit, CI-Check | 2026-02-08 |
| D11 | Rewrite vs. Header Pattern | HOCH | Rewrite Pattern (Vercel Platforms Starter Kit), kein headers() | 2026-02-08 |
| D12 | Refresh Token Tenant-Validierung | HOCH | tenant_id in RefreshClaims + Re-Validierung gegen account_tenants bei Refresh | 2026-02-08 |
| D13 | Per-Tenant Rollen | HOCH | ~~Globale Rollen~~ → Per-Tenant RBAC mit System-Rollen (revidiert 2026-02-10) | 2026-02-10 |
| D14 | Policy Engine Tenant-Awareness | MITTEL | Two-Tier Auth: Middleware (statisch/JWT) + Service (dynamisch/DB), fail-closed Tenant-Assert in Engine | 2026-02-08 |
| D15 | Email-Eindeutigkeit | HOCH | Ein Account, mehrere Tenants (Auth0/WorkOS-Pattern), Email global UNIQUE, account_tenants Junction | 2026-02-08 |
| D16 | Raw SQL + Seed Tenant-Filterung | MITTEL | RLS filtert alle Query-Formen (Raw SQL, Relation, CTE). 6 gezielte Massnahmen: RowsAffected-Audit, PG 17.6+, Seeds, View security_invoker, Advisory Lock 2-arg, LEFT JOIN Review | 2026-02-08 |
| D17 | Tenant-Validierung in Middleware | MITTEL | Middleware stateless (D11), Tenant-Validation im [tenant]/layout.tsx via resolveTenant() (D5), notFound() bei unbekanntem Slug | 2026-02-08 |
| D18 | Org-Scope (Traeger-Buero) | KRITISCH | Application-Layer mit `WithAdminTx` + org_id-Filter. Dedizierter OrgScopeService, read-only, eigene Permission `org:dashboard:read` | 2026-02-10 |
| D19 | Eltern-Datenisolation | KRITISCH | Service-Layer-Filterung per `students_guardians` JOIN. Eigene Parent-Endpoints, kein RLS auf Kind-Ebene | 2026-02-10 |
| D20 | IoT Device Auth Bootstrap | KRITISCH | Two-Phase Lookup (WithAdminTx → WithTenantTx), per-Device PIN-Hash, analog D6 Login-Flow | 2026-02-10 |
| D21 | GDPR Art. 17 Shared Accounts | KRITISCH | Art. 28 AV-Modell, Controller-Scope Loeschung, Self-Service Account-Loeschung, AVV + DPIA als juristischer Workstream | 2026-02-10 |

---

## Abhaengigkeiten zwischen Entscheidungen

```
D7 (tenant_id=0 Bypass) ──────────────┐
D8 (set_config + Transaction Gap) ─────┼──→ D9 (QueryHook Strategie)
                                       │
D2 (TenantModel Mixin) ───────────────┼──→ D10 (BeforeAppendModel)
                                       │
D8 (Transaction Gap) ─────────────────┼──→ D16 (Raw SQL Tenant-Filterung)
                                       │
D4 (Cross-Tenant Mechanismus) ────────┼──→ D13 (Per-Tenant Rollen)
                                       │     D14 (Policy Engine)
                                       │     D15 (Email-Eindeutigkeit)
                                       │
D11 (Rewrite vs. Header) ─────────────┼──→ D5 (Frontend TenantContext)
                                       │     D6 (Login Handler)
                                       │     D17 (Middleware Validierung)
                                       │
D12 (Refresh Token) ──────────────────┘

D7 (BYPASSRLS) + D8 (WithAdminTx) ───→ D18 (Org-Scope)
D14 (Policy Engine) + D15 (Accounts) ──→ D19 (Eltern-Isolation)
D6 (Login Pattern) + D8 (WithAdminTx) ─→ D20 (IoT Device Auth)
D15 (Email-Eindeutigkeit) ─────────────→ D21 (GDPR Art. 17 Erasure)
```

**Empfohlene Reihenfolge:**
1. D7, D8 (KRITISCH — bestimmen DB- und Transaction-Architektur)
2. D2, D9, D10 (Backend-Model + QueryHook — abhaengig von D7/D8)
3. D11, D13, D15 (Frontend + Rollen + Email — fundamentale Architektur)
4. D4, D12, D14 (Cross-Tenant + Auth — aufbauend)
5. D1, D3, D5, D6, D16, D17 (Restliche Punkte)
