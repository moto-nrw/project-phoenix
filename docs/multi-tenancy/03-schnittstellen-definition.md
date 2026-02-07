# Schnittstellen-Definition: Multi-Tenancy

Dieses Dokument definiert alle Interfaces zwischen den Layern, damit Developer unabhaengig arbeiten koennen.

---

## 1. Tenant-Context Package (Backend)

**Package:** `backend/tenant/`
**Verantwortlich:** Backend-Dev A (Phase 2a)
**Konsumenten:** Alle Backend-Developer

```go
package tenant

import "context"

// ---- Context Helpers ----

type contextKey string

const (
    tenantKey contextKey = "tenant_id"
    orgKey    contextKey = "org_id"
)

// WithTenantID stores the tenant ID in context
func WithTenantID(ctx context.Context, id int64) context.Context

// TenantFromContext retrieves the tenant ID (0 = platform/no tenant)
func TenantFromContext(ctx context.Context) int64

// MustTenantFromContext panics if no tenant in context (for non-platform routes)
func MustTenantFromContext(ctx context.Context) int64

// WithOrgID stores the organization ID in context
func WithOrgID(ctx context.Context, id int64) context.Context

// OrgFromContext retrieves the organization ID
func OrgFromContext(ctx context.Context) int64

// IsPlatformContext returns true if this is a platform/operator request
func IsPlatformContext(ctx context.Context) bool

// ---- Middleware ----

// Middleware extracts tenant_id from JWT claims and sets it in context.
// For platform-scope tokens, tenant_id is 0.
// Must be placed AFTER JWT authentication middleware.
func Middleware(next http.Handler) http.Handler

// ---- BUN Hook ----

// RLSHook sets `SET LOCAL app.current_tenant_id` before each query
// Must be registered on the bun.DB instance at startup
type RLSHook struct{}
func (h *RLSHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context
func (h *RLSHook) AfterQuery(ctx context.Context, event *bun.QueryEvent)
```

---

## 2. JWT Claims (Backend <-> Frontend)

**Definiert von:** Backend-Dev A
**Konsumenten:** Frontend-Dev, alle Backend-Developer

### Access Token Claims

```json
{
  "id": 42,
  "sub": "user@example.com",
  "username": "optional_username",
  "first_name": "Max",
  "last_name": "Mustermann",
  "tenant_id": 1,
  "org_id": 1,
  "roles": ["user"],
  "permissions": ["users:read", "groups:list", "..."],
  "is_admin": false,
  "scope": "",
  "exp": 1738900000,
  "iat": 1738899100
}
```

### Scope-Werte

| Scope | Bedeutung | tenant_id | Zugriff |
|-------|-----------|-----------|---------|
| `""` (leer) | Normaler Tenant-User | > 0 | Nur eigener Tenant |
| `"org"` | Buero-Admin | > 0 (Haupt-Tenant) | Alle Tenants der Org |
| `"platform"` | Operator | 0 | Alles (kein RLS) |

### Refresh Token Claims (unveraendert)

```json
{
  "id": 123,
  "token": "uuid-v4-string",
  "exp": 1738986000,
  "iat": 1738899100
}
```

---

## 3. HTTP Headers (Frontend -> Backend)

### Standard-Request (eingeloggter User)

```http
GET /api/students HTTP/1.1
Host: altenberge.{TENANT_DOMAIN}
Authorization: Bearer eyJ...{jwt_with_tenant_id}
Content-Type: application/json
```

**Kein extra Header noetig** - `tenant_id` kommt aus dem JWT.

### Login-Request (noch kein JWT)

```http
POST /auth/login HTTP/1.1
Host: altenberge.{TENANT_DOMAIN}
X-Tenant-Slug: altenberge
Content-Type: application/json

{
  "email": "lehrer@example.com",
  "password": "..."
}
```

**`X-Tenant-Slug` Header** wird nur beim Login benoetigt (Frontend extrahiert Subdomain -> Header).

### Operator-Request

```http
GET /operator/suggestions HTTP/1.1
Host: operator.{TENANT_DOMAIN}
Authorization: Bearer eyJ...{jwt_with_scope_platform}
Content-Type: application/json
```

**Kein Tenant-Header.** JWT `scope: "platform"` + `tenant_id: 0` signalisiert Platform-Access.

### IoT Device Request

```http
POST /api/iot/checkin HTTP/1.1
Host: api.{TENANT_DOMAIN}
Authorization: Bearer {device_api_key}
X-Staff-PIN: 1234
Content-Type: application/json
```

**Kein Tenant-Header.** Backend mappt `device_api_key -> iot.devices.id -> iot.devices.tenant_id`.

---

## 4. Frontend API Contract

### Tenant-Context (React)

```typescript
// lib/tenant-context.tsx
interface TenantInfo {
  tenantId: string;        // "1" (as string, int64 -> string mapping)
  tenantSlug: string;      // "altenberge"
  tenantName: string;      // "OGS Altenberge"
  organizationId: string;  // "1"
  organizationName: string; // "Traegerverein XY"
}

function useTenant(): TenantInfo;
```

### NextAuth Session (erweitert)

```typescript
// Aktuell:
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

// NEU (zusaetzliche Felder):
session.user = {
  ...existing,
  tenantId: string,        // NEU
  orgId: string,           // NEU
  scope: string,           // NEU ("" | "org" | "platform")
}
```

### Route-Wrapper (erweitert)

```typescript
// lib/route-wrapper.ts - Signature bleibt gleich
// Intern wird X-Tenant-Slug automatisch gesetzt

// Fuer Login-Endpunkt (speziell):
export function createLoginHandler(
  handler: (request: Request, tenantSlug: string) => Promise<Response>
): RouteHandler;
```

### API-URL Struktur

```
# Tenant-spezifisch (ueber Subdomain geroutet):
altenberge.{TENANT_DOMAIN}/api/students -> Next.js -> backend:8080/api/students

# Operator (eigene Subdomain):
operator.{TENANT_DOMAIN}/api/operator/* -> Next.js -> backend:8080/operator/*

# IoT (direkt, kein Next.js):
api.{TENANT_DOMAIN}/api/iot/* -> backend:8080/api/iot/*
```

---

## 5. Datenbank-Schema Contract

### Neue Tabellen (platform Schema)

```sql
-- platform.organizations
CREATE TABLE platform.organizations (
    id              BIGSERIAL PRIMARY KEY,
    name            VARCHAR(200) NOT NULL,
    slug            VARCHAR(100) NOT NULL UNIQUE,
    active          BOOLEAN NOT NULL DEFAULT true,
    settings        JSONB DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- platform.schools (= TENANTS)
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
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, slug)
);

-- platform.operator_organizations
CREATE TABLE platform.operator_organizations (
    operator_id       BIGINT NOT NULL REFERENCES platform.operators(id),
    organization_id   BIGINT NOT NULL REFERENCES platform.organizations(id),
    role              VARCHAR(50) NOT NULL DEFAULT 'viewer',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (operator_id, organization_id)
);

-- auth.account_tenants (Account -> Tenant Mapping)
-- Ein Account kann mehreren Tenants zugeordnet sein (z.B. Lehrer an 2 OGS)
CREATE TABLE auth.account_tenants (
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id),
    tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
    is_primary        BOOLEAN NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (account_id, tenant_id)
);

-- platform.cross_tenant_access
CREATE TABLE platform.cross_tenant_access (
    id                BIGSERIAL PRIMARY KEY,
    account_id        BIGINT NOT NULL REFERENCES auth.accounts(id),
    source_tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
    target_tenant_id  BIGINT NOT NULL REFERENCES platform.schools(id),
    granted_by        BIGINT NOT NULL,
    valid_from        TIMESTAMPTZ NOT NULL,
    valid_until       TIMESTAMPTZ NOT NULL,
    reason            VARCHAR(200),
    active            BOOLEAN NOT NULL DEFAULT true,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (source_tenant_id != target_tenant_id)
);
```

### tenant_id Spalte (fuer ALLE bestehenden Tabellen)

```sql
-- Muster fuer jede Tabelle:
ALTER TABLE {schema}.{table}
    ADD COLUMN tenant_id BIGINT REFERENCES platform.schools(id);

-- Nach Daten-Backfill:
ALTER TABLE {schema}.{table}
    ALTER COLUMN tenant_id SET NOT NULL;

-- Index:
CREATE INDEX idx_{table}_tenant ON {schema}.{table}(tenant_id);

-- Composite Index (fuer haeufige Queries):
CREATE INDEX idx_{table}_tenant_id ON {schema}.{table}(tenant_id, id);
```

### RLS-Policy Template

```sql
-- Fuer JEDE tenant-scoped Tabelle:
ALTER TABLE {schema}.{table} ENABLE ROW LEVEL SECURITY;

-- Standard-Policy (Tenant-User sehen nur eigene Daten)
CREATE POLICY tenant_isolation_{table}
    ON {schema}.{table}
    FOR ALL
    USING (
        -- Platform scope (tenant_id=0) bypasses RLS
        current_setting('app.current_tenant_id', true)::bigint = 0
        OR tenant_id = current_setting('app.current_tenant_id', true)::bigint
    );

-- Superuser (DB admin) bypasses RLS automatically
-- Application user must NOT be superuser!
```

---

## 6. Test-Schnittstellen

### Hermetic Test Setup (erweitert)

```go
import testpkg "github.com/moto-nrw/project-phoenix/test"

func TestMultiTenant(t *testing.T) {
    db := testpkg.SetupTestDB(t)
    defer db.Close()

    // NEU: Test-Tenants erstellen
    tenantA := testpkg.CreateTestTenant(t, db, "School A")
    tenantB := testpkg.CreateTestTenant(t, db, "School B")

    // Fixtures MIT Tenant
    studentA := testpkg.CreateTestStudentInTenant(t, db, tenantA.ID, "Max", "A", "1a")
    studentB := testpkg.CreateTestStudentInTenant(t, db, tenantB.ID, "Anna", "B", "2b")

    // Context mit Tenant
    ctxA := tenant.WithTenantID(context.Background(), tenantA.ID)
    ctxB := tenant.WithTenantID(context.Background(), tenantB.ID)

    // Tenant A sieht nur eigene Daten
    students, _ := repo.List(ctxA)
    assert.Len(t, students, 1)
    assert.Equal(t, "Max", students[0].Person.FirstName)

    // Tenant B sieht nur eigene Daten
    students, _ = repo.List(ctxB)
    assert.Len(t, students, 1)
    assert.Equal(t, "Anna", students[0].Person.FirstName)
}
```

### Neue Test-Fixtures

```go
// test/fixtures.go - NEU
func CreateTestOrganization(t *testing.T, db *bun.DB, name string) *platform.Organization
func CreateTestTenant(t *testing.T, db *bun.DB, name string) *platform.School
func CreateTestTenantInOrg(t *testing.T, db *bun.DB, orgID int64, name string) *platform.School
func CreateTestStudentInTenant(t *testing.T, db *bun.DB, tenantID int64, first, last, class string) *users.Student
func CreateTestStaffInTenant(t *testing.T, db *bun.DB, tenantID int64, first, last string) *users.Staff
func CreateTestAccountInTenant(t *testing.T, db *bun.DB, tenantID int64, email string) *auth.Account
```

---

## 7. Deployment/Infrastructure Contract

### DNS

```
*.{TENANT_DOMAIN}    -> A Record -> Server IP
{TENANT_DOMAIN}      -> A Record -> Server IP
```

### SSL

```
Wildcard Cert fuer *.{TENANT_DOMAIN} (Let's Encrypt oder Cloudflare)
```

### Caddy/Reverse Proxy

```caddyfile
# Tenant-Subdomains -> Frontend
*.{TENANT_DOMAIN} {
    reverse_proxy frontend:3000
}

# Root-Domain -> Frontend
{TENANT_DOMAIN} {
    reverse_proxy frontend:3000
}

# API (direkt fuer IoT)
api.{TENANT_DOMAIN} {
    reverse_proxy backend:8080
}
```

### Docker Compose (erweitert)

```yaml
services:
  frontend:
    environment:
      API_URL: "http://server:8080"
      NEXT_PUBLIC_API_URL: "https://api.{TENANT_DOMAIN}"
      TENANT_DOMAIN: "{TENANT_DOMAIN}"  # NEU
```

---

## 8. Checkliste: "Ist mein Code Tenant-Safe?"

Jeder Developer sollte bei jedem PR diese Checkliste durchgehen:

- [ ] Hat meine DB-Query einen `WHERE tenant_id = ?` Filter?
- [ ] Setze ich bei `INSERT` die `tenant_id` aus dem Context?
- [ ] Kann ein User durch URL-Manipulation (z.B. ID erraten) an Daten eines anderen Tenants kommen?
- [ ] Habe ich einen Test der verifiziert, dass Tenant A die Daten von Tenant B NICHT sieht?
- [ ] Verwende ich `tenant.TenantFromContext(ctx)` statt Hardcoding?
- [ ] Ist mein SWR Cache-Key tenant-prefixed?
- [ ] Leake ich in Logs keine Daten von anderen Tenants?
