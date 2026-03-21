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

### 1.3 Transaction-Ownership Migration (09-C1)

**Problem:** Die Codebase hat 51 `RunInTx`- und 110 `WithTx`-Calls. Services starten aktuell eigene Transaktionen (`s.db.RunInTx(...)`). Wenn `WithTenantTx` bereits eine Transaktion laufen hat und ein Service `s.db.RunInTx(...)` aufruft, startet BUN eine **zweite Transaktion auf einer anderen Pool-Connection** — ohne `SET LOCAL ROLE` → `phoenix_auth` (NOINHERIT) → Permission Denied.

**Loesung: Transaction-Ownership wandert von Service auf Handler.**

Es gibt pro Request genau EINE Transaktion, gestartet im Handler via `WithTenantTx`. Services und Repositories arbeiten innerhalb dieser Transaktion. Kein Service startet eigene Transaktionen.

```
VORHER (Service startet Transaktion):
Handler → Service.DoWork() → s.db.RunInTx() → Repo.Create()

NACHHER (Handler startet Transaktion):
Handler → WithTenantTx() → Service.DoWork(ctx) → Repo.Create(ctx)
                ↑                                        ↑
        SET LOCAL ROLE hier                    getDB(ctx) = gleiche tx
```

**Handler-Beispiel:**

```go
func (rs *Resource) handleCheckIn(w http.ResponseWriter, r *http.Request) {
    tenantID := tenant.FromContext(r.Context())
    err := tenant.WithTenantTx(r.Context(), rs.db, tenantID,
        func(ctx context.Context, tx bun.Tx) error {
            return rs.activeService.CheckIn(ctx, studentID, groupID)
        })
    // HTTP response...
}
```

**Service-Beispiel (kein eigener RunInTx mehr):**

```go
// VORHER:
func (s *ActiveService) CheckIn(ctx context.Context, studentID, groupID int64) error {
    return s.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        visit, err := s.visitRepo.Create(ctx, tx, ...)
        if err != nil { return err }
        return s.attendanceRepo.Create(ctx, tx, ...)
    })
}

// NACHHER:
func (s *ActiveService) CheckIn(ctx context.Context, studentID, groupID int64) error {
    // tx ist schon im ctx (von WithTenantTx im Handler)
    visit, err := s.visitRepo.Create(ctx, ...)  // getDB(ctx) → tx
    if err != nil { return err }
    return s.attendanceRepo.Create(ctx, ...)     // gleiche tx
}
```

**Migrationstabelle fuer bestehende Patterns:**

| Pattern | Anzahl | Migration |
|---------|--------|-----------|
| `s.db.RunInTx(...)` in Service | ~51 | **Entfernen** — Service ist bereits in tx. Code ohne Wrapper ausfuehren |
| `WithTx(tx)` Repository-Chains | ~110 | **Ersetzen** durch `getDB(ctx)` — tx kommt aus Context statt Parameter |
| Cross-Service `tx`-Passing | ~5 | **Entfernen** — beide Services nutzen gleiche tx aus Context |
| Echte Sub-Transaktionen (selten) | ~2-3 | `bun.TxFromContext(ctx).RunInTx(...)` — erzeugt SAVEPOINT innerhalb der laufenden tx |

**Warum das kein Workaround ist:** PostgREST, Supabase und jedes middleware-first Framework nutzt dieses Pattern: Transaktion startet am Request-Boundary (wo der Tenant bekannt ist), alles darunter arbeitet innerhalb dieser Transaktion. Eine Transaktion pro Request, ein `SET LOCAL ROLE`, ein `set_config`.

**Fail-Modes:**

| Szenario | Verhalten |
|----------|-----------|
| Service wird ohne `WithTenantTx` aufgerufen | `getDB(ctx)` findet keine tx → faellt auf `r.DB` zurueck → `phoenix_auth` (NOINHERIT) → Permission Denied |
| Service ruft versehentlich `s.db.RunInTx(...)` statt ctx zu nutzen | Neue tx ohne `SET LOCAL ROLE` → Permission Denied |
| Zwei Handler-Calls in einer Go-Routine (z.B. Scheduler) | Jeder Call bekommt eigene tx via `WithTenantTx` — korrekt isoliert |

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
2. Tenant-Lookup: slug -> platform.schools.id (= tenantID)
3. Finde Account by email (global UNIQUE — D15)
4. Pruefe: account_tenants WHERE account_id=? AND tenant_id=? AND status='active'
5. Verifiziere Passwort
6. Lade Rollen fuer DIESEN Tenant (D13 revidiert):
   - System-Rollen (tenant_id IS NULL) + Tenant-Rollen (tenant_id = tenantID)
   - via account_roles WHERE account_id=? AND tenant_id=tenantID
7. Lade effektive Permissions fuer DIESEN Tenant:
   - Permissions aus Rollen (role_permissions, gefiltert durch Rollen aus Schritt 6)
   - Direct Permissions (account_permissions WHERE tenant_id=tenantID)
   - Deny-Override: granted=false ueberschreibt Role-Grants
8. Lade Organization-Info (schools.organization_id -> organizations)
9. Lade Tenant-Settings (schools.settings JSONB) fuer Frontend-Branding (D5)
10. Generiere JWT MIT tenant_id + org_id + scope + tenant-spezifische Permissions
```

**Wichtig:** Permissions im JWT sind jetzt **tenant-spezifisch**. Bei Tenant-Switch (6.2) werden Rollen und Permissions fuer den Ziel-Tenant neu geladen.

**Warum Body statt Header:** `tenant_slug` im Body ist Standard-REST (Auth0, WorkOS Pattern), leicht testbar mit curl/Bruno/Postman, keine Header-Fragilitaet bei Proxies/CDNs (D6).

### 6.2 Tenant-Switch (D4, D15)

```
1. User klickt "Zu OGS Greven wechseln"
2. POST /auth/switch-tenant { tenant_slug: "greven" }
3. Backend: Zugriffspruefung (siehe 6.3)
4. Backend: Lade Rollen + Permissions fuer Ziel-Tenant (D13 revidiert)
5. Backend: Neues JWT mit neuem tenant_id + tenant-spezifischen Permissions
6. Frontend: SWR-Cache invalidieren (Tenant-prefixed Keys)
```

**Wichtig:** Schritt 4 ist der Kern der D13-Revision. Maria kann Admin bei OGS Altenberge sein und Betreuerin bei OGS Greven — beim Switch aendert sich nicht nur der Tenant, sondern auch ihre Permissions.

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

**Hinweis:** Die TenantMiddleware setzt nur den Context. Die eigentliche Transaktion (`WithTenantTx`/`WithAdminTx`) wird im Handler gestartet (§1.3). Keine Transaktion in der Middleware — abgelehnte Requests verschwenden keine DB-Connections.

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
    // tx ist schon im ctx (von WithTenantTx im Handler, siehe §1.3)
    // 1. Resource laden (RLS aktiv, tenant-scoped)
    visit, err := s.visitRepo.FindByID(ctx, visitID)
    if err != nil { return nil, err }

    // 2. Policy pruefen (Resource.TenantID aus geladener Entity)
    authCtx := &policy.Context{
        Subject:  policy.SubjectFromContext(ctx),
        Resource: policy.Resource{
            Type: "visit", ID: visitID, TenantID: visit.TenantID,
        },
        Action: policy.ActionView,
    }
    if allowed, err := s.policyEngine.Authorize(ctx, authCtx); !allowed || err != nil {
        return nil, authorize.ErrForbidden
    }

    return visit, nil
}
```

**Warum im Service statt Middleware:** `phoenix_auth` (NOINHERIT) hat keine Query-Rechte. Policy-Evaluation mit DB-Zugriff MUSS innerhalb der Transaktion laufen — die bereits vom Handler via `WithTenantTx` gestartet wurde (§1.3).

---

## 9. IoT Device-Auth Aenderungen (D20)

### 9.1 Two-Phase Lookup (analog D6 Login-Flow)

Das Chicken-and-Egg-Problem: Device-Lookup braucht Tenant-Context, Tenant-Context kommt aus dem Device. Loesung: Exakt das D6-Login-Pattern — Phase 1 als Admin, Phase 2 als Tenant.

```go
// auth/device/device_auth.go
func (a *DeviceAuthenticator) Authenticate(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        apiKey := extractBearerToken(r)

        // Phase 1: Device-Lookup via WithAdminTx (analog D6 Login)
        var dev *iot.Device
        err := tenant.WithAdminTx(r.Context(), a.db, func(ctx context.Context, tx bun.Tx) error {
            var err error
            dev, err = a.deviceRepo.FindByAPIKey(ctx, apiKey)
            return err
        })
        if err != nil || dev.Status != iot.DeviceStatusActive {
            http.Error(w, "device not found or inactive", http.StatusForbidden)
            return
        }

        // Phase 2: Per-Device PIN-Validierung (Argon2id)
        pin := r.Header.Get("X-Staff-PIN")
        if !verifyPINHash(dev.PINHash, pin) {
            http.Error(w, "invalid PIN", http.StatusUnauthorized)
            return
        }

        // Phase 3: Context mit Device + TenantID
        ctx := context.WithValue(r.Context(), CtxDevice, dev)
        ctx = context.WithValue(ctx, CtxTenantID, dev.TenantID)

        // Check-In Handler nutzt WithTenantTx(dev.TenantID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 9.2 Aenderungen gegenueber aktueller Implementierung

| Aspekt | Aktuell | Multi-Tenancy |
|--------|---------|---------------|
| Device-Lookup | Direkt `r.db` (kein Tx) | `WithAdminTx` (BYPASSRLS) |
| PIN-Validierung | Globale Env-Var `OGS_DEVICE_PIN` | Per-Device `pin_hash` (Argon2id) in `iot.devices` |
| Tenant-Context | Keiner | `dev.TenantID` → `WithTenantTx` fuer Check-In |
| Blast-Radius bei PIN-Leak | Alle OGS | Ein einzelnes Geraet |

### 9.3 Migration

1. `iot.devices` bekommt `tenant_id` (bereits in 02-datenbank.md)
2. `iot.devices` bekommt `pin_hash TEXT` Spalte
3. Bestehende Devices: PIN aus `OGS_DEVICE_PIN` hashen und in `pin_hash` migrieren
4. `OGS_DEVICE_PIN` Env-Var entfernen nach Migration
5. Device-Registration-Endpoint: PIN wird beim Registrieren gesetzt (Admin generiert)

**PyrePortal-Seite (separates Repository):** Keine Aenderungen noetig. PyrePortal sendet seinen API-Key, der Backend-Server leitet daraus den Tenant ab.

---

## 10. SSE/Realtime: Tenant-Isolation (06-#12, 08-H3)

### 10.1 Problem

Der aktuelle SSE-Hub (`realtime/hub.go`) nutzt `active_group_id` (stringified int64) als Map-Key fuer Event-Broadcasting. Mit Multi-Tenancy koennten zwei Tenants Groups mit derselben numerischen ID haben → Cross-Tenant Event-Leakage.

### 10.2 Korrektur: Group-Level bleibt, Tenant-Guard kommt dazu

**WICHTIG:** Der Hub bleibt group-level partitioniert — KEIN Tenant-Level-Broadcasting. Tenant-Level waere ein Rueckschritt: Jeder User einer OGS wuerde ALLE Events aller Groups bekommen, statt nur die Groups die ihn interessieren.

**Zwei Aenderungen:**

1. **Tenant-Validierung bei SSE-Connection:** JWT `tenant_id` muss zur Group's `tenant_id` passen
2. **Tenant-prefixed Map-Keys:** Verhindert ID-Kollisionen zwischen Tenants

```go
// realtime/hub.go — KORRIGIERT
type Hub struct {
    // Key: "tenantID:groupID" statt nur "groupID"
    groupClients map[string][]*Client
    mu           sync.RWMutex
}

// Subscribe prueft Tenant-Zugehoerigkeit bei Connection
func (h *Hub) Subscribe(tenantID int64, groupID int64, client *Client) error {
    // Guard: Group muss zu diesem Tenant gehoeren (einmalig bei Connect)
    // Verhindert: User mit JWT fuer Tenant A subscribed auf Group von Tenant B
    if !h.verifyGroupTenant(tenantID, groupID) {
        return ErrForbidden
    }

    key := fmt.Sprintf("%d:%d", tenantID, groupID)
    h.mu.Lock()
    h.groupClients[key] = append(h.groupClients[key], client)
    h.mu.Unlock()
    return nil
}

// Broadcast sendet nur an Clients der korrekten Tenant+Group Kombination
func (h *Hub) Broadcast(tenantID int64, groupID int64, event Event) {
    key := fmt.Sprintf("%d:%d", tenantID, groupID)
    h.mu.RLock()
    clients := h.groupClients[key]
    h.mu.RUnlock()

    for _, c := range clients {
        c.Send(event)
    }
}
```

### 10.3 SSE-Endpoint Aenderungen

```go
// api/sse/api.go
func (rs *Resource) handleSSE(w http.ResponseWriter, r *http.Request) {
    // Tenant aus JWT extrahieren (Middleware hat bereits validiert)
    tenantID := tenant.FromContext(r.Context())
    groupID := parseGroupID(r)

    client := NewClient(w)
    if err := rs.hub.Subscribe(tenantID, groupID, client); err != nil {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }
    defer rs.hub.Unsubscribe(tenantID, groupID, client)

    client.Listen(r.Context()) // Blockiert bis Disconnect
}
```

### 10.4 Migration bestehender Clients

Bestehende SSE-Connections nutzen nur `groupID` als Key. Nach der Migration:
- Alle bestehenden Connections werden getrennt (Hub-Restart)
- Clients reconnecten automatisch (SSE EventSource Retry)
- Neue Connections nutzen `tenantID:groupID` Keys

---

## 11. Scheduler/Background-Jobs: Tenant-Strategie (09-C2)

Alle Scheduler-Jobs nutzen aktuell `context.Background()` ohne Tenant-Context. Mit `phoenix_auth` (NOINHERIT) fuehrt das zu Permission Denied.

**Strategie: Hybrid — Cleanup als Admin, Business-Logic per Tenant.**

### 11.1 Admin-Scope Jobs (WithAdminTx)

Jobs die tenant-uebergreifend aufraeumen. RLS ist hier nicht noetig — die Jobs loeschen/bereinigen expired Daten systemweit.

| Job | Datei | Begruendung Admin-Scope |
|-----|-------|------------------------|
| `CleanupExpiredTokens` | `token_cleanup.go` | Tokens sind global (auth.tokens), kein Tenant-Bezug |
| `CleanupExpiredVisits` | `cleanup_service.go` | Loescht abgelaufene Daten systemweit, Tenant-Iteration waere ueberfluessig |
| `CleanupStaleAttendance` | `cleanup_service.go` | Bereinigung verwaister Datensaetze |
| `CleanupStaleSupervisors` | `cleanup_service.go` | Bereinigung verwaister Datensaetze |

```go
func (s *Scheduler) runCleanupExpiredVisits(ctx context.Context) {
    err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
        return s.cleanupService.CleanupExpiredVisits(ctx)
    })
    if err != nil {
        s.logger.Error("cleanup_expired_visits failed", "error", err)
    }
}
```

### 11.2 Tenant-Scope Jobs (Tenant-Iteration + WithTenantTx)

Jobs die Geschaeftsdaten aendern. RLS MUSS aktiv sein — ein Bug im Job darf nicht Cross-Tenant-Daten korrumpieren.

| Job | Datei | Begruendung Tenant-Scope |
|-----|-------|--------------------------|
| `EndDailySessions` | `session_service.go` | Aendert active.groups — Geschaetsdaten pro OGS |
| `ProcessScheduledCheckouts` | `scheduled_checkout.go` | Aendert active.visits — Geschaetsdaten pro OGS |
| `AutoEndExpiredBreaks` | `work_session_service.go` | Aendert work_sessions — Geschaetsdaten pro OGS |
| `CleanupOpenSessions` | `work_session_service.go` | Aendert work_sessions — Geschaetsdaten pro OGS |

```go
func (s *Scheduler) runEndDailySessions(ctx context.Context) {
    tenants, err := s.schoolRepo.ListActive(ctx) // WithAdminTx fuer Tenant-Liste
    if err != nil {
        s.logger.Error("failed to list tenants", "error", err)
        return
    }

    for _, t := range tenants {
        err := tenant.WithTenantTx(ctx, s.db, t.ID,
            func(ctx context.Context, tx bun.Tx) error {
                return s.sessionService.EndDailySessions(ctx)
            })
        if err != nil {
            // Fehler in einem Tenant blockiert NICHT den naechsten
            s.logger.Error("end_daily_sessions failed",
                "tenant_id", t.ID, "tenant_name", t.Name, "error", err)
            continue
        }
    }
}
```

### 11.3 Fehlerbehandlung

- Fehler in einem Tenant duerfen NICHT den naechsten blockieren (`continue` nach Error-Log)
- Jeder Tenant-Job laeuft in eigener Transaktion — Rollback betrifft nur diesen Tenant
- Metriken: Anzahl fehlgeschlagener Tenants pro Job-Run loggen
- Bei >50% Tenant-Failures: Alert (deutet auf systemisches Problem hin, nicht Einzel-Tenant-Bug)

---

## 12. Factory-Pattern Aenderungen

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

## 13. PostgreSQL-Anforderungen (D16)

### 13.1 Mindestversion: PostgreSQL 17.6

| CVE | Beschreibung | Gefixt in |
|-----|-------------|-----------|
| CVE-2024-10976 | Plan-Cache ignoriert Role-Wechsel bei Subqueries/CTEs + SET LOCAL ROLE | PG 17.1 |
| CVE-2025-8713 | Optimizer-Statistiken leaken RLS-versteckte Daten | PG 17.6 |

### 13.2 Advisory Lock: Zwei-Argument-Form

```go
// VORHER — Cross-Tenant-Blocking
_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?)", activityID)

// NACHHER — Tenant-isoliert, kein Overflow-Risiko
_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(?, ?)", tenantID, activityID)
```

### 13.3 Views mit security_invoker

```sql
-- Alle Views auf Tenant-Tabellen MUESSEN security_invoker setzen
CREATE OR REPLACE VIEW users.expired_privacy_consents
WITH (security_invoker = true) AS ...
```

### 13.4 Praeventions-Checkliste

| Regel | Grund |
|-------|-------|
| Keine Materialized Views auf Tenant-Tabellen | Bypassen RLS komplett |
| Kein COPY FROM auf RLS-Tabellen | PostgreSQL blockiert es |
| Keine SECURITY DEFINER Funktionen mit BYPASSRLS-Owner | Bypassen RLS |
| Alle Views mit `security_invoker = true` | Views bypassen RLS sonst |
| RowsAffected() nach jedem UPDATE/DELETE | Silent Failures durch RLS |
| Advisory Locks mit tenant_id als Key1 | Sonst Cross-Tenant-Blocking |

---

## 14. Avatar-Uploads: Tenant-Namespacing (06-#16)

**Aktuell:** User-Avatare werden in einem globalen Verzeichnis gespeichert:
```
public/uploads/avatars/{userID}_{random}.ext
```

Funktional kein Konflikt (userID + random ist unique), aber bei Tenant-Loeschung oder GDPR Art. 17 Cleanup muessen alle Avatare eines Tenants identifizierbar sein.

### 14.1 Neue Pfadstruktur

```
public/uploads/avatars/{tenant_id}/{userID}_{random}.ext

# Beispiel:
public/uploads/avatars/42/1337_a8f3c2.jpg
public/uploads/avatars/99/2048_b7e1d4.png
```

### 14.2 Code-Aenderungen

```go
// api/usercontext/api.go — Upload-Handler
func (rs *resource) uploadAvatar(w http.ResponseWriter, r *http.Request) {
    tenantID := tenant.FromContext(r.Context())

    // NEU: Tenant-Subdirectory
    uploadDir := filepath.Join("public", "uploads", "avatars", strconv.FormatInt(tenantID, 10))
    if err := os.MkdirAll(uploadDir, 0o755); err != nil {
        // ...
    }

    filename := fmt.Sprintf("%d_%s%s", userID, randomStr, ext)
    filepath := filepath.Join(uploadDir, filename)
    // ... save file ...
}
```

**Betroffene Stellen:**
- `api/usercontext/api.go:296` (Avatar-Upload)
- `api/usercontext/api.go:392` (Avatar-Loeschung)
- Avatar-URL in API-Responses muss `/{tenant_id}/` im Pfad haben

### 14.3 Migration bestehender Avatare

```bash
# Alle bestehenden Avatare in Default-Tenant-Verzeichnis verschieben
mkdir -p public/uploads/avatars/1/
mv public/uploads/avatars/*.{jpg,jpeg,png,gif,webp} public/uploads/avatars/1/ 2>/dev/null || true
```

### 14.4 GDPR-Vorteil

Tenant-Loeschung (Art. 17) kann Avatare per Verzeichnis loeschen statt per DB-Lookup:

```go
// Komplett-Loeschung aller Avatare eines Tenants
os.RemoveAll(filepath.Join("public", "uploads", "avatars", strconv.FormatInt(tenantID, 10)))
```

---

## 15. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: SET LOCAL ROLE (D8), kein QueryHook (D9), kein BeforeAppendModel (D10), kein tenant_id=0 Bypass (D7), Body statt Header (D6), RefreshClaims (D12), Two-Tier Auth (D14), RowsAffected/PG 17.6 (D16) |
| 2026-02-10 | D13 revidiert: Login-Flow und Tenant-Switch laden Rollen/Permissions pro Tenant. JWT enthaelt tenant-spezifische Permissions. |
| 2026-02-10 | Transaction-Ownership Migration (Sektion 1.3, 09-C1): Handler starten Transaktionen, Services/Repos nutzen tx aus Context. Migrationstabelle fuer 51 RunInTx + 110 WithTx Calls. |
| 2026-02-10 | Scheduler/Background-Jobs Tenant-Strategie (Sektion 11, 09-C2): Hybrid-Ansatz mit Admin-Scope und Tenant-Scope Jobs. |
| 2026-02-10 | IoT Device-Auth ueberarbeitet (Sektion 9, D20): Two-Phase Lookup analog D6, per-Device PIN-Hash statt globaler Env-Var. |
| 2026-02-10 | SSE Hub korrigiert (Sektion 10, 08-H3): Group-Level Broadcasting bleibt, Tenant-Guard bei Connection, tenant-prefixed Map-Keys statt falschem Tenant-Level-Partitioning. |
| 2026-02-10 | Avatar-Uploads Tenant-Namespacing (Sektion 14, 06-#16): Pfadstruktur zu `avatars/{tenant_id}/` aendern, Migrationsskript fuer bestehende Dateien, GDPR-Cleanup per Verzeichnis. |
