# Multi-Tenancy: Backend-Implementierung

Dieses Dokument beschreibt alle Backend-Aenderungen: Tenant-Package, Models, Repositories, JWT, Login-Flow, Middleware, IoT Device-Auth, SSE-Isolation und Factory-Pattern.

**Verwandte Dokumente:**
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema das hier genutzt wird
- [05-testing.md](05-testing.md) - Test-Strategie fuer Backend-Code

---

## 1. Neues Package: `backend/tenant/`

```go
package tenant

import (
    "context"
    "fmt"

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
```

### 1.1 BUN Query Hook (RLS)

```go
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

**Design-Entscheidung:** `set_config()` mit `true` (transaction-local). Das ist kompatibel mit Connection-Pooling (PgBouncer Transaction-Mode) und BUN's eigener Pool-Verwaltung. Jeder Query in derselben Transaktion erbt den Tenant-Context.

---

## 2. TenantModel Mixin

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

### 2.1 Verwendung in Models

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

### 2.2 Models die TenantModel NICHT bekommen (Platform-Scope)

- `platform.Operator`
- `platform.Announcement`
- `platform.AnnouncementView`
- `platform.OperatorAuditLog`
- `platform.Organization` (NEU)
- `platform.School` (NEU)
- `platform.OperatorOrganization` (NEU)

---

## 3. Repository-Aenderungen

Jedes Repository muss `tenant_id` in Queries einbauen (Defense-in-Depth). Das Base-Repository wird erweitert:

### 3.1 Create mit automatischer tenant_id

```go
// database/repositories/base/base.go
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
```

### 3.2 FindByID mit tenant_id Filter

```go
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

### 3.3 Aufwand pro Repository-Typ

| Repository-Typ | Aufwand | Beispiele |
|----------------|---------|-----------|
| Einfache CRUD (nur Base-Methoden) | ~5 Min | rooms, settings |
| Custom-Queries | 30-60 Min | students, groups, visits |
| Cross-Schema-Joins | 60-120 Min | active (visits + attendance + groups) |

---

## 4. JWT Claims erweitern

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

---

## 5. Login-Flow Aenderungen

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

---

## 6. Tenant-Middleware

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

---

## 7. IoT Device-Auth Aenderungen

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

**PyrePortal-Seite (separates Repository):** Keine Aenderungen noetig. PyrePortal sendet seinen API-Key, der Backend-Server leitet daraus den Tenant ab. Das Device muss nicht wissen, zu welchem Tenant es gehoert.

---

## 8. SSE/Realtime: Tenant-Isolation

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

---

## 9. Factory-Pattern Aenderungen

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

## 10. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
