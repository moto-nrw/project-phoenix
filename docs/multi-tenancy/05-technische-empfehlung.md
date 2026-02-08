# Technische Empfehlung: Multi-Tenancy Implementierung

Dieses Dokument ist die validierte technische Empfehlung basierend auf:
- Finalisierte Business-Anforderungen (00-anforderungen.md)
- Vollstaendige Codebase-Analyse (55 Tabellen, 54 Repositories, 100+ Frontend-Routes)
- Best-Practice-Research (Supabase, Shopify, Slack, PostgreSQL RLS, BUN ORM, Next.js)

---

## 1. Architektur-Entscheidungen

### 1.1 Isolationsstrategie: Shared Schema + RLS

**Entscheidung:** Alle Tenants teilen sich dasselbe Datenbank-Schema. Row Level Security (RLS) erzwingt die Datenisolation auf DB-Ebene.

**Begruendung:**
- 100-500 Tenants ist klar im Bereich einer Single-DB-Loesung (Shopify, Slack, Supabase bestaetigen das)
- Schema-per-Tenant ist mit BUN ORM nicht praktikabel (59 Models mit Hardcoded Schema-Tags, 55 BeforeAppendModel-Hooks)
- RLS bietet DSGVO-konforme Isolation auf Datenbankebene, nicht nur im Application-Code

### 1.2 Defense-in-Depth

**Entscheidung:** RLS als Safety Net + explizite `WHERE tenant_id = ?` Clauses in allen Repositories.

**Begruendung:**
- Doppelte Absicherung: Selbst wenn ein Repository die tenant_id vergisst, blockiert RLS den Zugriff
- Waehrend der Migration kann RLS schrittweise aktiviert werden (permissiv -> logging -> enforced)
- BUN-Maintainer empfiehlt explizite Filterung: "Better to write a little bit more code rather than relying on hooks & hacks"

### 1.3 Tenant = OGS (School)

- `platform.schools.id` ist die `tenant_id` auf allen Tabellen
- `platform.organizations.id` ist die `org_id` (Traeger-Umbrella)
- Die OGS ist die Daten-Isolationsgrenze, der Traeger ist die organisatorische Klammer

---

## 2. Datenbank-Aenderungen

### 2.1 Neue Tabellen

```sql
-- Traeger (Organizations)
CREATE TABLE platform.organizations (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(100) NOT NULL UNIQUE,
    active          BOOLEAN NOT NULL DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- OGS / Tenants (Schools)
CREATE TABLE platform.schools (
    id                BIGSERIAL PRIMARY KEY,
    organization_id   BIGINT NOT NULL REFERENCES platform.organizations(id),
    name              VARCHAR(200) NOT NULL,
    slug              VARCHAR(100) NOT NULL,
    subdomain         VARCHAR(100) NOT NULL UNIQUE,
    active            BOOLEAN NOT NULL DEFAULT true,
    settings          JSONB DEFAULT '{}',
    address           TEXT,
    city              VARCHAR(100),
    zip               VARCHAR(20),
    phone             VARCHAR(50),
    email             VARCHAR(255),
    device_pin_hash   VARCHAR(255),  -- Argon2id-Hash des OGS-spezifischen Device-PINs
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, slug)
);
CREATE INDEX idx_schools_subdomain ON platform.schools(subdomain);
CREATE INDEX idx_schools_organization ON platform.schools(organization_id);

-- Account -> Tenant Zuordnung (N:M)
-- Ermoeglicht: Betreuer an mehreren OGS, Buero-Mitarbeiter mit Zugriff auf N OGS
CREATE TABLE auth.account_tenants (
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
    is_primary        BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, tenant_id)
);

-- Cross-Tenant-Zugriff (zeitlich begrenzt, z.B. Ferienbetreuung)
CREATE TABLE platform.cross_tenant_access (
    id                BIGSERIAL PRIMARY KEY,
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    source_tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
    target_tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
    granted_by        BIGINT NOT NULL REFERENCES auth.accounts(id),
    valid_from        TIMESTAMPTZ NOT NULL,
    valid_until       TIMESTAMPTZ NOT NULL,
    reason            VARCHAR(200),
    active            BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source_tenant_id != target_tenant_id),
    CHECK (valid_until > valid_from)
);
CREATE INDEX idx_cross_tenant_account ON platform.cross_tenant_access(account_id);
CREATE INDEX idx_cross_tenant_active ON platform.cross_tenant_access(active, valid_until);

-- Operator -> Organization Zuordnung
CREATE TABLE platform.operator_organizations (
    operator_id       BIGINT NOT NULL REFERENCES platform.operators(id) ON DELETE CASCADE,
    organization_id   BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
    role              VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (operator_id, organization_id)
);
```

### 2.2 tenant_id zu bestehenden Tabellen

**Alle 49 Nicht-Platform-Tabellen** brauchen eine `tenant_id` Spalte. Migration in 3 Schritten:

```sql
-- Schritt 1: Spalte hinzufuegen (nullable, non-blocking in PostgreSQL)
ALTER TABLE {schema}.{table}
    ADD COLUMN tenant_id BIGINT REFERENCES platform.schools(id);

-- Schritt 2: Bestehende Daten dem Default-Tenant zuweisen
UPDATE {schema}.{table} SET tenant_id = 1;

-- Schritt 3: NOT NULL Constraint setzen
ALTER TABLE {schema}.{table}
    ALTER COLUMN tenant_id SET NOT NULL;
```

**Betroffene Tabellen nach Schema (49 total):**

| Schema | Tabellen | Anzahl |
|--------|----------|--------|
| auth | accounts, tokens, password_reset_tokens, roles, permissions, role_permissions, account_roles, account_permissions, accounts_parents, password_reset_rate_limits, invitation_tokens, guardian_invitations | 12 |
| users | rfid_cards, persons, profiles, staff, teachers, guests, persons_guardians, students, guardian_profiles, students_guardians, privacy_consents, guardian_phone_numbers | 12 |
| education | groups, group_teacher, group_substitution, grade_transitions, grade_transition_mappings, grade_transition_history | 6 |
| facilities | rooms | 1 |
| activities | categories, groups, schedules, supervisors, student_enrollments | 5 |
| active | groups, visits, group_supervisors, combined_groups, group_mappings, attendance, scheduled_checkouts, work_sessions, work_session_breaks, staff_absences | 10 |
| schedule | timeframes, dateframes, recurrence_rules, student_pickup_schedules, student_pickup_exceptions, student_pickup_notes | 6 |
| iot | devices | 1 |
| feedback | entries | 1 |
| config | settings | 1 |
| suggestions | posts, votes, comments, comment_reads, post_reads | 5 |
| audit | data_deletions, auth_events, data_imports, work_session_edits | 4 |

**Sonderfaelle:**
- `auth.roles`, `auth.permissions`, `auth.role_permissions`: Koennten global bleiben (systemweite Rollen). **Empfehlung:** tenant_id hinzufuegen fuer Custom-Rollen pro OGS, aber System-Rollen (`is_system = true`) sind global (tenant_id = NULL oder 0).
- `suggestions.comment_reads`, `suggestions.post_reads`, `platform.announcement_views`: Composite-PKs muessen um tenant_id erweitert werden.
- `audit.*` Tabellen: tenant_id fuer Zuordnung, aber RLS-Policy erlaubt Operator-Zugriff auf alle Audit-Daten.

### 2.3 Indexes

Fuer jede Tabelle mit tenant_id:

```sql
-- Standard-Index fuer tenant_id Filterung
CREATE INDEX idx_{table}_tenant ON {schema}.{table}(tenant_id);

-- Composite-Index fuer haeufige Queries (tenant + PK)
CREATE INDEX idx_{table}_tenant_id ON {schema}.{table}(tenant_id, id);
```

**Spezielle Composite-Indexes fuer Performance:**

```sql
-- Haeufigste Queries: Studenten einer OGS, aktive Besuche einer OGS
CREATE INDEX idx_students_tenant_class ON users.students(tenant_id, school_class);
CREATE INDEX idx_visits_tenant_active ON active.visits(tenant_id, exit_time) WHERE exit_time IS NULL;
CREATE INDEX idx_attendance_tenant_date ON active.attendance(tenant_id, date);
CREATE INDEX idx_devices_tenant ON iot.devices(tenant_id, status);
CREATE INDEX idx_accounts_tenant_email ON auth.accounts(tenant_id, email);
```

### 2.4 RLS-Policies

**Datenbank-Rollen Setup:**

```sql
-- Migrations-Rolle (besitzt Tabellen, nur fuer DDL)
-- Dies ist der aktuelle DB-User - bleibt wie er ist

-- Applikations-Rolle (fuer den Go-Server)
CREATE ROLE phoenix_app WITH LOGIN PASSWORD '...' NOBYPASSRLS;
GRANT USAGE ON ALL SCHEMAS TO phoenix_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA
    auth, users, education, facilities, activities, active,
    schedule, iot, feedback, config, suggestions, audit, platform
    TO phoenix_app;
```

**RLS-Policy Template (fuer JEDE tenant-scoped Tabelle):**

```sql
ALTER TABLE {schema}.{table} ENABLE ROW LEVEL SECURITY;
ALTER TABLE {schema}.{table} FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_{table}
    ON {schema}.{table}
    FOR ALL
    USING (
        -- Platform scope (tenant_id=0) bypasses RLS (fuer Operator)
        (SELECT current_setting('app.current_tenant_id', true))::bigint = 0
        OR tenant_id = (SELECT current_setting('app.current_tenant_id', true))::bigint
    );
```

**Wichtig:** Der `(SELECT ...)` Wrapper erzwingt, dass PostgreSQL `current_setting()` als initPlan (einmalig) evaluiert statt pro Zeile. Das bringt laut Supabase >100x Performance-Verbesserung auf grossen Tabellen.

**Platform-Tabellen (kein RLS):**
- `platform.organizations` - kein RLS, nur Operator-Zugriff
- `platform.operators` - kein RLS, nur Operator-Zugriff
- `platform.schools` - kein RLS, aber App liest nur eigenen Tenant

### 2.5 RLS Rollout in 3 Phasen

```
Phase 1 (Woche 1-2): ENABLE RLS + permissive Policy USING (true)
    → Alle Queries funktionieren weiterhin
    → Verifiziert: Kein Query bricht durch RLS-Aktivierung

Phase 2 (Woche 3-4): Echte Policy + Logging bei Violations
    → SET LOCAL wird in allen Code-Paths gesetzt
    → Violations werden geloggt (nicht blockiert)
    → Ziel: 100% der Requests haben Tenant-Context

Phase 3 (Woche 5+): Strikte Enforcement
    → current_setting(..., false) statt true
    → Fehlender Tenant-Context = Error statt leeres Ergebnis
    → Permissive Fallbacks entfernt
```

---

## 3. Backend-Aenderungen

### 3.1 Neues Package: `backend/tenant/`

```go
package tenant

import (
    "context"
    "fmt"
    "net/http"

    "github.com/uptrace/bun"
)

// ---- Context Helpers ----

type contextKey string

const (
    tenantKey contextKey = "tenant_id"
    orgKey    contextKey = "org_id"
    scopeKey  contextKey = "scope"
)

// WithTenantID stores the tenant ID in context.
func WithTenantID(ctx context.Context, id int64) context.Context {
    return context.WithValue(ctx, tenantKey, id)
}

// FromContext retrieves the tenant ID. Returns 0 if not set.
func FromContext(ctx context.Context) int64 {
    id, _ := ctx.Value(tenantKey).(int64)
    return id
}

// MustFromContext retrieves the tenant ID or panics.
// Use ONLY in tenant-scoped routes (never in operator routes).
func MustFromContext(ctx context.Context) int64 {
    id := FromContext(ctx)
    if id == 0 {
        panic("tenant.MustFromContext: no tenant in context")
    }
    return id
}

// IsPlatformScope returns true for operator/platform requests.
func IsPlatformScope(ctx context.Context) bool {
    scope, _ := ctx.Value(scopeKey).(string)
    return scope == "platform"
}

// ---- BUN Query Hook ----

// RLSHook sets SET LOCAL app.current_tenant_id before each query.
// Must be registered on the bun.DB instance at startup.
type RLSHook struct{}

func (h *RLSHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
    if event.DB == nil {
        return ctx
    }

    tenantID := FromContext(ctx)
    // 0 = platform scope (bypasses RLS)
    _, _ = event.DB.ExecContext(ctx,
        "SELECT set_config('app.current_tenant_id', ?, true)",
        fmt.Sprintf("%d", tenantID),
    )
    return ctx
}

func (h *RLSHook) AfterQuery(ctx context.Context, event *bun.QueryEvent) {}
```

**Wichtige Design-Entscheidung:** Der `RLSHook` setzt `set_config()` mit `true` (transaction-local). Das ist kompatibel mit Connection-Pooling (PgBouncer Transaction-Mode) und BUN's eigener Pool-Verwaltung. Jeder Query in derselben Transaktion erbt den Tenant-Context.

### 3.2 TenantModel Mixin

```go
// models/base/tenant.go
package base

// TenantModel is embedded by all tenant-scoped models.
// Platform-scope models (Operator, Announcement, etc.) do NOT embed this.
type TenantModel struct {
    TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

// GetTenantID returns the tenant ID for this entity.
func (t *TenantModel) GetTenantID() int64 {
    return t.TenantID
}
```

**Verwendung in Models:**

```go
// models/users/student.go - VORHER
type Student struct {
    base.Model
    PersonID    int64  `bun:"person_id,notnull"`
    SchoolClass string `bun:"school_class,notnull"`
    // ...
}

// models/users/student.go - NACHHER
type Student struct {
    base.Model
    base.TenantModel  // NEU: Fuegt tenant_id hinzu
    PersonID    int64  `bun:"person_id,notnull"`
    SchoolClass string `bun:"school_class,notnull"`
    // ...
}
```

**Models die TenantModel NICHT bekommen (Platform-Scope):**
- `platform.Operator`
- `platform.Announcement`
- `platform.AnnouncementView`
- `platform.OperatorAuditLog`
- `platform.Organization` (NEU)
- `platform.School` (NEU)
- `platform.OperatorOrganization` (NEU)

### 3.3 Repository-Aenderungen

Jedes Repository muss `tenant_id` in Queries einbauen. Das Base-Repository wird erweitert:

```go
// database/repositories/base/base.go - Create erweitert
func (r *Repository[T]) Create(ctx context.Context, entity T) error {
    // Tenant-ID aus Context setzen (Defense-in-Depth, RLS faengt es auch ab)
    if tm, ok := any(entity).(interface{ GetTenantID() int64 }); ok {
        if tm.GetTenantID() == 0 {
            tenantID := tenant.FromContext(ctx)
            if tenantID > 0 {
                // Set tenant_id via reflection or interface method
            }
        }
    }
    // ... existing create logic
}

// FindByID erweitert: tenant_id als zusaetzlicher Filter
func (r *Repository[T]) FindByID(ctx context.Context, id int64) (T, error) {
    query := r.DB.NewSelect().
        Model(&entity).
        ModelTableExpr(fmt.Sprintf("%s AS \"%s\"", r.TableName, r.entityName)).
        Where(fmt.Sprintf(`"%s".id = ?`, r.entityName), id)

    // Defense-in-Depth: tenant_id Filter
    tenantID := tenant.FromContext(ctx)
    if tenantID > 0 {
        query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, r.entityName), tenantID)
    }

    err := query.Scan(ctx)
    // ...
}
```

**Aufwand pro Repository:**
- Einfache CRUD-Repos (nur Base-Methoden nutzen): ~5 Minuten (nur TenantModel zum Model hinzufuegen)
- Repos mit Custom-Queries: 30-60 Minuten pro Repo (jede Query braucht tenant_id WHERE)
- Repos mit Cross-Schema-Joins: 60-120 Minuten (beide Seiten des JOINs brauchen tenant_id)

### 3.4 JWT Claims erweitern

```go
// auth/jwt/claims.go - VORHER
type AppClaims struct {
    ID          int      `json:"id,omitempty"`
    Sub         string   `json:"sub,omitempty"`
    Roles       []string `json:"roles,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
    IsAdmin     bool     `json:"is_admin,omitempty"`
    Scope       string   `json:"scope,omitempty"`
    CommonClaims
}

// auth/jwt/claims.go - NACHHER
type AppClaims struct {
    ID          int      `json:"id,omitempty"`
    Sub         string   `json:"sub,omitempty"`
    TenantID    int64    `json:"tenant_id"`            // NEU
    OrgID       int64    `json:"org_id"`               // NEU
    FirstName   string   `json:"first_name,omitempty"`
    LastName    string   `json:"last_name,omitempty"`
    Roles       []string `json:"roles,omitempty"`
    Permissions []string `json:"permissions,omitempty"`
    IsAdmin     bool     `json:"is_admin,omitempty"`
    Scope       string   `json:"scope,omitempty"`
    CommonClaims
}
```

**Scope-Werte:**

| Scope | TenantID | Bedeutung | Zugriff |
|-------|----------|-----------|---------|
| `""` (leer) | > 0 | Normaler User (Betreuer, OGS-Buero) | Nur eigener Tenant |
| `"org"` | > 0 (Haupt-Tenant) | Traeger-Buero | Alle Tenants der Organization |
| `"platform"` | 0 | Operator | Alles (kein RLS) |

### 3.5 Login-Flow Aenderungen

```
VORHER:
1. POST /auth/login {email, password}
2. Finde Account by email
3. Verifiziere Passwort
4. Lade Rollen/Permissions
5. Generiere JWT (ohne tenant_id)

NACHHER:
1. POST /auth/login {email, password}
   + Header: X-Tenant-Slug: "altenberge" (aus Subdomain extrahiert)
2. Tenant-Lookup: slug -> platform.schools.id
3. Finde Account by email
4. Pruefe: Account hat Zugriff auf diesen Tenant (via auth.account_tenants)
5. Verifiziere Passwort
6. Lade Rollen/Permissions
7. Lade Organization-Info (schools.organization_id -> organizations)
8. Generiere JWT MIT tenant_id + org_id + scope
```

**Wichtig:** Die `X-Tenant-Slug` wird nur beim Login benoetigt. Danach kommt die `tenant_id` aus dem JWT. Der Slug wird aus der Subdomain im Frontend extrahiert.

### 3.6 Tenant-Middleware

```go
// Wird NACH dem JWT-Authenticator in die Middleware-Chain eingefuegt
func TenantMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        claims := jwt.ClaimsFromCtx(r.Context())
        if claims == nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        ctx := r.Context()
        ctx = tenant.WithTenantID(ctx, claims.TenantID)
        ctx = tenant.WithOrgID(ctx, claims.OrgID)
        ctx = tenant.WithScope(ctx, claims.Scope)

        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Middleware-Chain Reihenfolge:**
```
Request -> SecurityHeaders -> CORS -> JWT Verifier -> JWT Authenticator
        -> TenantMiddleware (NEU) -> PermissionCheck -> Handler
```

### 3.7 IoT Device-Auth Aenderungen

```go
// VORHER: Globaler PIN aus Env-Var
pin := os.Getenv("OGS_DEVICE_PIN")
if subtle.ConstantTimeCompare([]byte(staffPIN), []byte(pin)) != 1 {
    return error
}

// NACHHER: Per-Tenant PIN aus platform.schools
func (a *DeviceAuthenticator) authenticate(r *http.Request) error {
    // 1. Device by API key finden (wie bisher)
    device, err := a.iotService.GetDeviceByAPIKey(apiKey)

    // 2. NEU: Tenant-Context aus Device ableiten
    tenantID := device.TenantID
    ctx = tenant.WithTenantID(r.Context(), tenantID)

    // 3. NEU: PIN gegen Tenant-spezifischen PIN pruefen
    school, err := a.schoolService.FindByID(ctx, tenantID)
    if !userpass.CheckPassword(staffPIN, school.DevicePINHash) {
        return ErrInvalidPIN
    }
}
```

### 3.8 SSE/Realtime: Tenant-Isolation

Der aktuelle SSE-Hub (`realtime/hub.go`) broadcastet an alle Subscriber ohne Tenant-Filter.

```go
// NACHHER: Hub partitioniert nach Tenant
type Hub struct {
    // Subscriptions pro Tenant
    tenantSubscriptions map[int64]map[*Subscriber]bool
    // ...
}

func (h *Hub) Broadcast(tenantID int64, event Event) {
    // Nur an Subscriber des gleichen Tenants senden
    subs := h.tenantSubscriptions[tenantID]
    for sub := range subs {
        sub.Send(event)
    }
}
```

### 3.9 Factory-Pattern Aenderungen

```go
// VORHER
func NewFactory(db *bun.DB) *Factory

// NACHHER: RLS-Hook wird bei DB-Initialisierung registriert
func SetupDatabase(db *bun.DB) {
    db.AddQueryHook(&tenant.RLSHook{})
}

// Factory bleibt gleich - ein Set von Repos fuer alle Tenants
// Tenant-Scoping passiert ueber Context, nicht ueber separate Factory-Instanzen
func NewFactory(db *bun.DB) *Factory
```

---

## 4. Frontend-Aenderungen

### 4.1 Next.js Middleware: Subdomain-Extraktion

```typescript
// middleware.ts - KOMPLETT NEU
import { NextRequest, NextResponse } from 'next/server';

const TENANT_DOMAIN = process.env.TENANT_DOMAIN || 'localhost:3000';
const RESERVED_SUBDOMAINS = ['operator', 'api', 'www'];

export function middleware(request: NextRequest) {
    const hostname = request.headers.get('host') || '';
    const pathname = request.nextUrl.pathname;

    // Subdomain extrahieren
    let subdomain: string | null = null;

    if (hostname.includes('localhost')) {
        // Dev: subdomain.localhost:3000
        const match = hostname.match(/^([^.]+)\.localhost/);
        subdomain = match ? match[1] : null;
    } else {
        // Prod: subdomain.{TENANT_DOMAIN}
        const parts = hostname.replace(`.${TENANT_DOMAIN}`, '');
        if (parts !== hostname) {
            subdomain = parts;
        }
    }

    // Operator-Subdomain: eigene Route-Group
    if (subdomain === 'operator') {
        // Operator Cookie-Check (wie bisher)
        if (!pathname.startsWith('/operator/login')) {
            const token = request.cookies.get('phoenix-operator-token');
            if (!token?.value) {
                return NextResponse.redirect(new URL('/operator/login', request.url));
            }
        }
        return NextResponse.next();
    }

    // Root-Domain (kein Subdomain): Landing/Tenant-Auswahl
    if (!subdomain || subdomain === 'www') {
        return NextResponse.next();
    }

    // Reservierte Subdomains blocken
    if (RESERVED_SUBDOMAINS.includes(subdomain)) {
        return NextResponse.next();
    }

    // Tenant-Subdomain: Slug als Header weiterreichen
    const response = NextResponse.next();
    response.headers.set('x-tenant-slug', subdomain);
    return response;
}

export const config = {
    matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
};
```

### 4.2 NextAuth: Cookie-Domain fuer Subdomains

```typescript
// server/auth/config.ts - Cookie-Konfiguration erweitern
const isProduction = process.env.NODE_ENV === 'production';
const rootDomain = process.env.TENANT_DOMAIN; // z.B. "moto-app.de"

export const authConfig = {
    // ...existing config...
    cookies: {
        sessionToken: {
            name: isProduction
                ? '__Secure-next-auth.session-token'
                : 'next-auth.session-token',
            options: {
                httpOnly: true,
                sameSite: 'lax' as const,
                path: '/',
                // Wildcard-Domain fuer alle Subdomains
                domain: rootDomain ? `.${rootDomain}` : undefined,
                secure: isProduction,
            },
        },
    },
};
```

### 4.3 Session um Tenant-Info erweitern

```typescript
// VORHER
session.user = {
    id: string,
    name: string,
    email: string,
    token: string,
    refreshToken: string,
    roles: string[],
    firstName: string,
    isAdmin: boolean,
}

// NACHHER
session.user = {
    ...existing,
    tenantId: string,      // NEU: "1" (int64 -> string)
    orgId: string,         // NEU: "1"
    scope: string,         // NEU: "" | "org" | "platform"
}
```

### 4.4 Route-Wrapper: Tenant-Header automatisch

```typescript
// lib/route-wrapper.ts - X-Tenant-Slug automatisch setzen
async function forwardToBackend(request: Request, token: string, path: string) {
    const headers = new Headers({
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
    });

    // NEU: Tenant-Slug aus Request-Header (gesetzt von Middleware)
    const tenantSlug = request.headers.get('x-tenant-slug');
    if (tenantSlug) {
        headers.set('X-Tenant-Slug', tenantSlug);
    }

    return fetch(`${getServerApiUrl()}${path}`, { headers });
}
```

### 4.5 SWR Cache-Keys: Tenant-Prefix

```typescript
// VORHER
useSWR('supervision-visits-room-1', fetcher)
useSWR('student-detail-42', fetcher)

// NACHHER: Tenant-ID als Prefix
const { tenantId } = useSession();
useSWR(`t${tenantId}:supervision-visits-room-1`, fetcher)
useSWR(`t${tenantId}:student-detail-42`, fetcher)
```

**Oder besser:** Ein Wrapper-Hook der automatisch prefixed:

```typescript
function useTenantSWR(key: string, fetcher: Fetcher) {
    const { data: session } = useSession();
    const tenantKey = session?.user?.tenantId
        ? `t${session.user.tenantId}:${key}`
        : key;
    return useSWR(tenantKey, fetcher);
}
```

### 4.6 Env-Variablen erweitern

```typescript
// env.js - Neue Variablen
const env = createEnv({
    server: {
        // ...existing...
        TENANT_DOMAIN: z.string().optional(),  // NEU: z.B. "moto-app.de"
    },
    client: {
        // ...existing...
    },
});
```

---

## 5. IoT/PyrePortal Aenderungen

### 5.1 Backend-Seite (in diesem Repository)

1. **`iot.devices`**: `tenant_id` Spalte hinzufuegen
2. **Device-Auth Middleware**: Nach Device-Lookup den `tenant_id` aus dem Device in den Context setzen
3. **Alle IoT Data-Endpoints**: Daten nach `tenant_id` des Devices filtern
4. **Per-Tenant PIN**: `device_pin_hash` in `platform.schools` statt globaler Env-Var

### 5.2 PyrePortal-Seite (separates Repository)

**Keine Aenderungen noetig.** PyrePortal sendet seinen API-Key, und der Backend-Server leitet daraus den Tenant ab. Das Device muss nicht wissen, zu welchem Tenant es gehoert.

---

## 6. Test-Strategie

### 6.1 Neue Test-Fixtures

```go
// test/fixtures.go - NEU
func CreateTestOrganization(t *testing.T, db *bun.DB, name string) *platform.Organization
func CreateTestTenant(t *testing.T, db *bun.DB, name string) *platform.School
func CreateTestTenantInOrg(t *testing.T, db *bun.DB, orgID int64, name string) *platform.School
func CreateTestStudentInTenant(t *testing.T, db *bun.DB, tenantID int64, first, last, class string) *users.Student
func CreateTestStaffInTenant(t *testing.T, db *bun.DB, tenantID int64, first, last string) *users.Staff
func CreateTestAccountInTenant(t *testing.T, db *bun.DB, tenantID int64, email string) *auth.Account
```

### 6.2 Isolation-Tests (PFLICHT fuer jeden PR)

```go
func TestTenantIsolation_Students(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Daten in Tenant A erstellen
    studentA := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")

    // Tenant B darf Student A NICHT sehen
    ctxB := tenant.WithTenantID(context.Background(), tenantB.ID)
    students, err := repo.List(ctxB)
    require.NoError(t, err)
    assert.Empty(t, students, "Tenant B must not see Tenant A's students")

    // Tenant A sieht eigene Daten
    ctxA := tenant.WithTenantID(context.Background(), tenantA.ID)
    students, err = repo.List(ctxA)
    require.NoError(t, err)
    assert.Len(t, students, 1)
    assert.Equal(t, studentA.ID, students[0].ID)
}
```

### 6.3 Fehlender Tenant-Context Test

```go
func TestMissingTenantContext_Fails(t *testing.T) {
    db := testpkg.SetupTestDB(t)

    // Query OHNE Tenant-Context muss fehlschlagen (nach Phase 3 RLS)
    ctx := context.Background() // Kein tenant_id!
    _, err := repo.List(ctx)
    require.Error(t, err, "Query without tenant context must fail")
}
```

---

## 7. Migrations-Strategie fuer Production

### 7.1 Reihenfolge (Zero-Downtime)

```
1. Migration: platform.organizations + platform.schools erstellen
2. Migration: Default-Org und Default-School erstellen (ID=1)
3. Migration: tenant_id (NULLABLE) zu allen Tabellen hinzufuegen
   → Non-blocking in PostgreSQL (nur Metadaten-Aenderung)
4. Migration: UPDATE ... SET tenant_id = 1 fuer alle bestehenden Rows
   → In Batches fuer grosse Tabellen (LIMIT + OFFSET)
5. Migration: tenant_id NOT NULL Constraint setzen
6. Migration: Indexes erstellen (CONCURRENTLY)
7. Migration: RLS Policies erstellen (Phase 1: permissive)
8. Migration: auth.account_tenants befuellen (alle Accounts -> Tenant 1)
9. Code-Deploy mit Tenant-Middleware
10. Verification: Alle Queries funktionieren noch
11. Migration: RLS auf strict umstellen (Phase 3)
```

### 7.2 Rollback-Plan

```
Bei Problemen nach Schritt 9:
- Code-Rollback auf vorherige Version (ohne Tenant-Middleware)
- tenant_id Spalten bleiben (stoeren nicht, da nullable oder default)
- RLS-Policies auf USING(true) zuruecksetzen
```

---

## 8. Aufwand-Schaetzung

| Bereich | Dateien | Aufwand |
|---------|---------|--------|
| DB Migrations (neue Tabellen, tenant_id, Indexes, RLS) | ~8-10 | 2-3 Wochen |
| tenant/ Package + Middleware | ~5 | 2-3 Tage |
| JWT Claims + Login-Flow | ~10 | 1 Woche |
| base.TenantModel + alle Models | ~50 | 1 Woche |
| Repositories (Defense-in-Depth WHERE) | ~54 | 2-3 Wochen |
| Services (Context-Propagation) | ~29 | 1 Woche |
| IoT Device-Auth + Per-Tenant PIN | ~5 | 3-4 Tage |
| SSE/Realtime Tenant-Isolation | ~3 | 2-3 Tage |
| Frontend Middleware + Auth | ~10 | 1 Woche |
| Frontend API Routes | ~100+ | 1-2 Wochen |
| Frontend SWR Cache-Keys | ~20 | 3-4 Tage |
| Tests (Isolation + Fixtures) | ~40 | 2 Wochen |
| Production Migration + Testing | - | 2-3 Wochen |
| **TOTAL** | **~350 Dateien** | **~14-18 Wochen** |

---

## 9. Risiken

| Risiko | Schwere | Mitigation |
|--------|---------|------------|
| Datenleck zwischen Tenants | KRITISCH | Defense-in-Depth (RLS + WHERE), Isolation-Tests als Pflicht |
| Performance-Regression durch RLS | MITTEL | initPlan-Wrapper, Composite-Indexes, Benchmark bei 100 Tenants |
| Migration bricht Production | HOCH | Zero-Downtime-Strategie, nullable tenant_id, phased RLS |
| Vergessene tenant_id in neuem Code | HOCH | PR-Checkliste, Test-Template, RLS als Safety Net |
| Cross-Schema-Joins ohne tenant_id | MITTEL | Code-Review-Regel: Alle JOINs muessen tenant_id auf beiden Seiten haben |
| SWR-Cache Collision | NIEDRIG | Tenant-prefixed Cache-Keys |
| Cookie-Domain Probleme | NIEDRIG | Testen mit Wildcard-Domain auf Staging |

---

## 10. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse und Best-Practice-Research |
