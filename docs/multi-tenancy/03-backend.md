# Multi-Tenancy: Backend-Implementierung

Dieses Dokument beschreibt alle Backend-Aenderungen: Tenant-Package, Models, Repositories, JWT, Login-Flow, Middleware, Policy Engine, IoT Device-Auth, SSE-Isolation und Factory-Pattern.

**Verwandte Dokumente:**
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen
- [02-datenbank.md](02-datenbank.md) - Datenbank-Schema das hier genutzt wird
- [05-testing.md](05-testing.md) - Test-Strategie fuer Backend-Code
- [DEBATE.md](DEBATE.md) - Alle Diskussionspunkte und Entscheidungen

---

## 1. Neues Package: `backend/tenant/`

### 1.1 Context Helpers

```go
package tenant

import (
    "context"
    "fmt"

    "github.com/uptrace/bun"
)

type contextKey string

const (
    tenantKey contextKey = "tenant_id"
    orgKey    contextKey = "org_id"
    scopeKey  contextKey = "scope"
)

func WithTenantID(ctx context.Context, id int64) context.Context {
    return context.WithValue(ctx, tenantKey, id)
}

func FromContext(ctx context.Context) int64 {
    id, _ := ctx.Value(tenantKey).(int64)
    return id
}

func MustFromContext(ctx context.Context) int64 {
    id := FromContext(ctx)
    if id == 0 {
        panic("tenant.MustFromContext: no tenant in context")
    }
    return id
}

func WithOrgID(ctx context.Context, id int64) context.Context {
    return context.WithValue(ctx, orgKey, id)
}

func WithScope(ctx context.Context, scope string) context.Context {
    return context.WithValue(ctx, scopeKey, scope)
}

func IsPlatformScope(ctx context.Context) bool {
    scope, _ := ctx.Value(scopeKey).(string)
    return scope == "platform"
}
```

### 1.2 Transaktions-Wrapper (D8)

Alle Queries laufen in expliziten Transaktionen. `SET LOCAL ROLE` bestimmt die PostgreSQL-Rolle pro Transaktion. Kein QueryHook (D9 — eliminiert durch D8).

```go
// WithTenantTx wraps fn in a transaction with tenant-scoped RLS.
// Used for 99% of all requests (Betreuer, OGS-Buero, etc.).
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

// WithAdminTx wraps fn in a transaction with admin-scoped access (BYPASSRLS).
// Used for Operator-Routes, Migrations, Seeds, GDPR Cleanup, Cross-Tenant (D4).
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

**Sicherster Default:** Connection Pool verbindet als `phoenix_auth` (NOINHERIT, keine eigenen Rechte). Vergessene Transaktion → Hard-Fail (Permission Denied), nicht stiller Bypass.

---

## 2. TenantModel Mixin (D2, D10)

```go
// models/base/tenant.go
package base

// TenantModel is embedded by all tenant-scoped models.
// Platform-scope models (Operator, Announcement, etc.) do NOT embed this.
type TenantModel struct {
    TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

func (t *TenantModel) GetTenantID() int64   { return t.TenantID }
func (t *TenantModel) SetTenantID(id int64) { t.TenantID = id }
```

### 2.1 Verwendung in Models

```go
// models/users/student.go - NACHHER
type Student struct {
    base.Model
    base.TenantModel  // NEU: Fuegt tenant_id hinzu
    PersonID    int64  `bun:"person_id,notnull"`
    SchoolClass string `bun:"school_class,notnull"`
    // ...
}
```

### 2.2 Kein BeforeAppendModel auf TenantModel (D10)

55 von 57 Models haben eigene `BeforeAppendModel` Hooks — ein Hook auf TenantModel wuerde bei fast allen stumm ignoriert (Go Embedding Shadowing). Zusaetzlich umgehen 18+ Repositories mit `NewInsert()` direkt den Base-Create.

**Stattdessen: Service-Layer setzt tenant_id explizit:**
```go
// Im Service — VOR dem Repository-Call
student.SetTenantID(tenant.FromContext(ctx))
err := s.studentRepo.Create(ctx, student)
```

**Defense-in-Depth Schichtung:**

| Schicht | Mechanismus | Bei fehlender tenant_id |
|---------|-------------|------------------------|
| 1. Service-Layer | `SetTenantID(tenant.FromContext(ctx))` | Erster Checkpoint |
| 2. Base Repository | TenantScoped-Check in `base.Create()` | Bonus fuer die 45 Repos die base nutzen |
| 3. DB-Constraint | `tenant_id NOT NULL` | INSERT schlaegt sofort fehl (laut) |
| 4. RLS-Policy | `WHERE tenant_id = current_setting(...)` | Selbst bei falschem Wert: zero rows |

Ein CI-Check warnt wenn ein TenantScoped-Model ohne vorheriges `SetTenantID()` an ein Repository uebergeben wird.

### 2.3 Models die TenantModel NICHT bekommen (Platform-Scope)

- `platform.Operator`
- `platform.Announcement`
- `platform.AnnouncementView`
- `platform.OperatorAuditLog`
- `platform.Organization` (NEU)
- `platform.School` (NEU)
- `platform.OperatorOrganization` (NEU)

---

## 3. Repository-Aenderungen

### 3.1 getDB() — Transaktion aus Context (D8)

```go
// database/repositories/base/base.go
// Repositories nutzen Transaktion aus Context wenn verfuegbar
func (r *Repository[T]) getDB(ctx context.Context) bun.IDB {
    if tx := bun.TxFromContext(ctx); tx != nil {
        return tx
    }
    return r.DB
}
```

Alle Repository-Methoden nutzen `r.getDB(ctx)` statt `r.DB` direkt. Dadurch laufen Queries automatisch in der Transaktion mit dem korrekten SET LOCAL ROLE.

### 3.2 Defense-in-Depth WHERE Clauses

Jedes Repository muss `tenant_id` in Queries einbauen (redundant zu RLS, aber zusaetzliche Sicherheit):

```go
func (r *Repository[T]) FindByID(ctx context.Context, id int64) (T, error) {
    var entity T
    query := r.getDB(ctx).NewSelect().
        Model(&entity).
        ModelTableExpr(fmt.Sprintf(`%s AS "%s"`, r.TableName, r.entityName)).
        Where(fmt.Sprintf(`"%s".id = ?`, r.entityName), id)

    // Defense-in-Depth: tenant_id Filter
    tenantID := tenant.FromContext(ctx)
    if tenantID > 0 {
        query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, r.entityName), tenantID)
    }

    err := query.Scan(ctx)
    return entity, err
}
```

### 3.3 RowsAffected-Checks (D16)

72% aller UPDATE/DELETE Operationen pruefen `RowsAffected()` nicht. Mit RLS bedeutet das stille Nicht-Aenderungen statt Fehler.

```go
// database/repositories/base/helpers.go
func assertRowsAffected(result sql.Result, expected int64) error {
    n, err := result.RowsAffected()
    if err != nil {
        return fmt.Errorf("rows affected: %w", err)
    }
    if n != expected {
        return fmt.Errorf("expected %d rows affected, got %d", expected, n)
    }
    return nil
}
```

Standardmaessig in allen Repository UPDATE/DELETE-Methoden nutzen.

### 3.4 Aufwand pro Repository-Typ

| Repository-Typ | Aufwand | Beispiele |
|----------------|---------|-----------|
| Einfache CRUD (nur Base-Methoden) | ~5 Min | rooms, settings |
| Custom-Queries | 30-60 Min | students, groups, visits |
| Cross-Schema-Joins | 60-120 Min | active (visits + attendance + groups) |

---

## 4. JWT Claims erweitern

```go
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

| Scope | Bedeutung | DB-Rolle | Zugriff |
|-------|-----------|----------|---------|
| `""` (leer) | Normaler User (Betreuer, OGS-Buero) | `phoenix_tenant` | Nur eigener Tenant |
| `"org"` | Traeger-Buero | `phoenix_tenant` (Haupt-Tenant) | Alle Tenants der Organization via Tenant-Switch |
| `"platform"` | Operator | `phoenix_admin` (BYPASSRLS) | Alles |

**Wichtig:** Operators nutzen `phoenix_admin` (BYPASSRLS) — nicht `tenant_id=0`. Kein Magic-Value-Bypass (D7).

---

## 5. RefreshClaims erweitern (D12)

```go
// RefreshClaims — NEU: tenant_id fuer Re-Validierung
type RefreshClaims struct {
    ID       int    `json:"id,omitempty"`
    Sub      string `json:"sub,omitempty"`
    TenantID int64  `json:"tenant_id"`  // NEU
    CommonClaims
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

**Worst-Case bei Zugriffsentzug:** Max 15 Minuten (bis Access Token ablaeuft). Fuer sofortige Sperrung muesste jeder Request gegen die DB geprueft werden — widerspricht JWT-Prinzip.

---

## 6. Login-Flow Aenderungen (D6, D15)

### 6.1 Login

```
1. POST /auth/login { email, password, tenant_slug: "altenberge" }
   (tenant_slug als Body-Parameter, KEIN X-Tenant-Slug Header)
2. Tenant-Lookup: slug -> platform.schools.id
3. Finde Account by email (global UNIQUE — D15)
4. Pruefe: account_tenants WHERE account_id=? AND tenant_id=? AND status='active'
5. Verifiziere Passwort
6. Lade Rollen/Permissions
7. Lade Organization-Info (schools.organization_id -> organizations)
8. Lade Tenant-Settings (schools.settings JSONB) fuer Frontend-Branding (D5)
9. Generiere JWT MIT tenant_id + org_id + scope
```

**Warum Body statt Header:** `tenant_slug` im Body ist Standard-REST (Auth0, WorkOS Pattern), leicht testbar mit curl/Bruno/Postman, keine Header-Fragilitaet bei Proxies/CDNs (D6).

### 6.2 Tenant-Switch (D4, D15)

```
1. User klickt "Zu OGS Greven wechseln"
2. POST /auth/switch-tenant { tenant_slug: "greven" }
3. Backend: Zugriffspruefung (siehe 6.3)
4. Backend: Neues JWT mit neuem tenant_id
5. Frontend: SWR-Cache invalidieren (Tenant-prefixed Keys)
```

### 6.3 Tenant-Switch Zugriffspruefung (nach Scope)

| Scope | Pruefung | Begruendung |
|-------|----------|-------------|
| `""` (normal) | `account_tenants WHERE account_id=? AND tenant_id=? AND status='active'` | Explizit zugewiesene Tenants |
| `"org"` | `platform.schools WHERE id=? AND organization_id=user.org_id` | Traeger-Buero sieht **automatisch alle OGS** des Traegers, auch neue (00-anforderungen Sektion 3.2a) |

**Warum unterschiedliche Pruefung:** 00-anforderungen fordert, dass Traeger-Buero-Mitarbeiter "automatisch Zugriff auf ALLE OGS des Traegers, auch neue die spaeter hinzukommen" haben. Wuerde man nur `account_tenants` pruefen, muessten bei jeder neuen OGS-Provisionierung manuell Eintraege fuer alle Traeger-Buero-User erstellt werden. Die direkte Pruefung gegen `schools.organization_id` ist wartungsfrei.

### 6.4 Eltern Auto-Provisioning

Eltern-Accounts werden **nicht manuell** einem Tenant zugewiesen. Stattdessen verwaltet der Service-Layer die `account_tenants`-Eintraege automatisch:

| Event | Aktion |
|-------|--------|
| Kind wird an OGS eingeschrieben | `account_tenants` fuer alle Erziehungsberechtigten des Kindes erstellen (`status='active'`) |
| Kind verlaesst OGS | Pruefen ob Elternteil noch andere Kinder an dieser OGS hat. Falls nein: `account_tenants.status = 'inactive'` |
| Kind wechselt OGS | Alter Tenant deaktivieren (falls kein anderes Kind), neuer Tenant aktivieren |

**Implementierung:** Im `EnrollmentService` bei `Enroll()`/`Disenroll()`. Nutzt `WithAdminTx` fuer Cross-Tenant-Zugriff auf Erziehungsberechtigte.

---

## 7. Tenant-Middleware

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
        -> TenantMiddleware (NEU) -> PermissionCheck (Tier 1) -> Handler
```

**Hinweis:** Die TenantMiddleware setzt nur den Context. Die eigentliche Transaktion (`WithTenantTx`/`WithAdminTx`) wird im Service-Layer gestartet (D8). Keine Transaktion in der Middleware — abgelehnte Requests verschwenden keine DB-Connections.

---

## 8. Policy Engine: Two-Tier Authorization (D14)

### 8.1 Zwei Auth-Schichten

| Tier | Frage | Schicht | DB noetig? |
|------|-------|---------|------------|
| Tier 1 (statisch) | "Hat User Permission `visits:read`?" | Middleware (`RequiresPermission`) | Nein (JWT) |
| Tier 2 (dynamisch) | "Ist Lehrer in der Gruppe dieses Schuelers?" | Service (Policy Engine) | Ja |

### 8.2 Subject + Resource mit TenantID

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

### 8.3 Fail-closed Tenant-Assert in Engine.Authorize()

```go
func (e *Engine) Authorize(ctx context.Context, authCtx *Context) (bool, error) {
    // Tenant-scoped User + Resource ohne TenantID = DENY (fail-closed)
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
        // ... existing policy evaluation
    }
}
```

### 8.4 Service-Layer Flow (Policy innerhalb WithTenantTx)

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

**Warum im Service statt Middleware:** D8 erzwingt es — `phoenix_auth` (NOINHERIT) hat keine Query-Rechte. Policy-Evaluation mit DB-Zugriff MUSS innerhalb von `WithTenantTx` laufen.

---

## 9. IoT Device-Auth Aenderungen

```go
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

**PyrePortal-Seite (separates Repository):** Keine Aenderungen noetig. PyrePortal sendet seinen API-Key, der Backend-Server leitet daraus den Tenant ab.

---

## 10. SSE/Realtime: Tenant-Isolation

Der aktuelle SSE-Hub (`realtime/hub.go`) broadcastet an alle Subscriber ohne Tenant-Filter.

```go
// NACHHER: Hub partitioniert nach Tenant
type Hub struct {
    tenantSubscriptions map[int64]map[*Subscriber]bool
}

func (h *Hub) Broadcast(tenantID int64, event Event) {
    subs := h.tenantSubscriptions[tenantID]
    for sub := range subs {
        sub.Send(event)
    }
}
```

---

## 11. Factory-Pattern Aenderungen

```go
// Connection Pool verbindet als phoenix_auth (D8)
db := connectAs("phoenix_auth")

// KEIN AddQueryHook — QueryHook entfaellt komplett (D9)
// set_config wird einmal pro Transaktion in WithTenantTx gesetzt

// Factory bleibt gleich — ein Set von Repos fuer alle Tenants
// Tenant-Scoping passiert ueber Context + SET LOCAL ROLE
func NewFactory(db *bun.DB) *Factory
```

---

## 12. PostgreSQL-Anforderungen (D16)

### 12.1 Mindestversion: PostgreSQL 17.6

| CVE | Beschreibung | Gefixt in |
|-----|-------------|-----------|
| CVE-2024-10976 | Plan-Cache ignoriert Role-Wechsel bei Subqueries/CTEs + SET LOCAL ROLE | PG 17.1 |
| CVE-2025-8713 | Optimizer-Statistiken leaken RLS-versteckte Daten | PG 17.6 |

### 12.2 Advisory Lock: Zwei-Argument-Form

```go
// VORHER — Cross-Tenant-Blocking
_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", activityID)

// NACHHER — Tenant-isoliert, kein Overflow-Risiko
_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?, ?)", tenantID, activityID)
```

### 12.3 Views mit security_invoker

```sql
-- Alle Views auf Tenant-Tabellen MUESSEN security_invoker setzen
CREATE OR REPLACE VIEW users.expired_privacy_consents
WITH (security_invoker = true) AS ...
```

### 12.4 Praeventions-Checkliste

| Regel | Grund |
|-------|-------|
| Keine Materialized Views auf Tenant-Tabellen | Bypassen RLS komplett |
| Kein COPY FROM auf RLS-Tabellen | PostgreSQL blockiert es |
| Keine SECURITY DEFINER Funktionen mit BYPASSRLS-Owner | Bypassen RLS |
| Alle Views mit `security_invoker = true` | Views bypassen RLS sonst |
| RowsAffected() nach jedem UPDATE/DELETE | Silent Failures durch RLS |
| Advisory Locks mit tenant_id als Key1 | Sonst Cross-Tenant-Blocking |

---

## 13. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: SET LOCAL ROLE (D8), kein QueryHook (D9), kein BeforeAppendModel (D10), kein tenant_id=0 Bypass (D7), Body statt Header (D6), RefreshClaims (D12), Two-Tier Auth (D14), RowsAffected/PG 17.6 (D16) |
