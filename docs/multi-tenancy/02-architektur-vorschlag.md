# Architektur-Vorschlag: Multi-Tenancy fuer Project Phoenix

## 1. Strategie-Entscheidung: Shared Database, Shared Schema

### Optionen (mit Bewertung)

| Strategie | Beschreibung | Pro | Contra | Empfehlung |
|-----------|-------------|-----|--------|------------|
| **Database per Tenant** | Jede OGS bekommt eigene PostgreSQL-DB | Maximale Isolation | Migrations-Albtraum bei 100 DBs, Connection-Pool-Explosion | NEIN |
| **Schema per Tenant** | Jede OGS bekommt eigenes PG-Schema | Gute Isolation, einfaches Backup | Migration fuer 100+ Schemas, BUN ORM unterstuetzt kein dynamisches Schema-Routing | NEIN |
| **Shared Schema + RLS** | Alle Tenants in gleichen Tabellen, `tenant_id` Spalte + RLS | Einfachste Migration, beste Skalierung, ein Deployment | Erfordert sorgfaeltige RLS-Policies | **JA** |

### Warum Shared Schema + RLS?

1. **BUN ORM:** Alle Models haben hardcodierte Schema-Qualifizierung (`schema:auth,table:accounts`). Schema-per-Tenant wuerde JEDES Model und JEDE Query dynamisch machen.
2. **Migrations:** Eine Migration laeuft gegen eine DB, nicht gegen 100.
3. **Connection Pooling:** Ein Pool, nicht 100.
4. **Queries:** RLS wird auf DB-Ebene erzwungen - selbst wenn Code fehlerhaft ist, lecken keine Daten.
5. **Ferienbetreuung-Szenario:** Kinder mischen sich zwischen OGS -> einfach durch temporaere Cross-Tenant-Berechtigungen loesbar, nicht durch DB-Schranken.

---

## 2. Datenmodell: Tenant-Hierarchie

### 2.1 Neue Entitaeten

```
platform.organizations (Traeger)
|- id: BIGSERIAL PK
|- name: VARCHAR(200) NOT NULL
|- slug: VARCHAR(100) NOT NULL UNIQUE  -- URL-Slug fuer Subdomain
|- active: BOOLEAN DEFAULT true
|- settings: JSONB  -- Org-weite Einstellungen
|- created_at, updated_at
|
|- platform.schools (OGS / Einrichtung = TENANT)
|   |- id: BIGSERIAL PK
|   |- organization_id: BIGINT FK -> organizations
|   |- name: VARCHAR(200) NOT NULL
|   |- slug: VARCHAR(100) NOT NULL  -- z.B. "altenberge"
|   |- subdomain: VARCHAR(100) NOT NULL UNIQUE  -- z.B. "altenberge"
|   |- active: BOOLEAN DEFAULT true
|   |- settings: JSONB  -- OGS-spezifische Einstellungen
|   |- address, city, zip, phone, email
|   |- created_at, updated_at
|   |
|   +-- (Alle bestehenden Tabellen bekommen: tenant_id FK -> schools.id)
|
+-- platform.operator_organizations (Operator-Zuordnung)
    |- operator_id: BIGINT FK -> platform.operators
    |- organization_id: BIGINT FK -> organizations
    |- role: VARCHAR(50)  -- "owner", "admin", "viewer"
    |- PRIMARY KEY (operator_id, organization_id)
```

### 2.2 Naming-Entscheidung: `tenant_id`

Die Spalte heisst ueberall `tenant_id` (nicht `school_id` oder `ogs_id`), weil:
- Generisch und verstaendlich fuer alle Developer
- RLS-Policies koennen konsistent auf `tenant_id` filtern
- Referenziert `platform.schools.id` (die tatsaechliche Tenant-Einheit)

### 2.3 Beziehung: Traeger -> Buero -> OGS

```
Traeger (Organization):
  - Kann mehrere OGS verwalten
  - Buero-Accounts sehen alle OGS des Traegers
  - Ferienbetreuung: Temporaere Cross-Tenant-Zugriffe

Buero (Organization Member):
  - Hat Admin-Zugang auf Org-Ebene
  - Kann zwischen OGS wechseln ("Impersonate" / "Switch Context")
  - Sieht aggregierte Daten ueber alle OGS

OGS (School = Tenant):
  - Die eigentliche Daten-Isolationsgrenze
  - Jede OGS hat eigene Students, Staff, Groups, Rooms, etc.
  - Eigene Subdomain: altenberge.{TENANT_DOMAIN}
```

---

## 3. RLS-Strategie (Row Level Security)

### 3.1 Grundprinzip

```sql
-- Session-Variable setzen (bei jeder DB-Verbindung aus dem Pool)
SET app.current_tenant_id = '42';

-- RLS-Policy (Beispiel fuer auth.accounts)
ALTER TABLE auth.accounts ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON auth.accounts
    USING (tenant_id = current_setting('app.current_tenant_id')::bigint);
```

### 3.2 Implementierung mit BUN ORM + Connection Pool

**Problem:** BUN nutzt `database/sql` Connection Pooling. `SET` Befehle gelten nur fuer eine Connection, nicht fuer einen Request.

**Loesung: Query-Hook oder Middleware, die vor JEDER Query den Tenant setzt.**

```go
// Ansatz 1: BUN QueryHook (empfohlen)
type TenantHook struct{}

func (h *TenantHook) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
    tenantID := TenantFromContext(ctx)
    if tenantID > 0 {
        _, _ = event.DB.ExecContext(ctx, "SET LOCAL app.current_tenant_id = ?", tenantID)
    }
    return ctx
}

// Ansatz 2: Transaction-basiert (sicherer)
func WithTenant(ctx context.Context, db *bun.DB, tenantID int64, fn func(ctx context.Context, tx bun.Tx) error) error {
    return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
        _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant_id = $1", tenantID)
        if err != nil {
            return err
        }
        return fn(ctx, tx)
    })
}
```

**`SET LOCAL` ist entscheidend:** Es gilt nur innerhalb der aktuellen Transaction, nicht fuer die gesamte Connection. Das verhindert Tenant-Leaks zwischen Requests die zufaellig die gleiche Pool-Connection bekommen.

### 3.3 RLS-Policy Kategorien

| Kategorie | Tabellen | Policy |
|-----------|----------|--------|
| **Standard-Isolation** | users.*, education.*, facilities.*, activities.*, active.*, schedule.*, iot.*, feedback.*, config.*, suggestions.* | `tenant_id = current_setting('app.current_tenant_id')::bigint` |
| **Auth mit Tenant** | auth.accounts, auth.tokens, auth.invitation_tokens | Gleiche Policy + Login-Endpoint ist Sonderfall (kein Tenant beim Login-Versuch) |
| **Platform (kein RLS)** | platform.* | Kein RLS - Operators sehen alles |
| **Cross-Tenant (Ferienbetreuung)** | Spezielle Queries | Bypass via `SET LOCAL app.current_tenant_id = 0` oder eigene Policy |
| **Audit** | audit.* | Tenant-isoliert, aber Operator kann cross-tenant lesen |

### 3.4 Bypass fuer Operator/Platform

```sql
-- Operators umgehen RLS komplett
CREATE POLICY operator_bypass ON auth.accounts
    USING (
        current_setting('app.current_tenant_id', true) = '0'  -- Platform scope
        OR tenant_id = current_setting('app.current_tenant_id')::bigint
    );
```

---

## 4. Backend-Architektur

### 4.1 Neuer Request-Flow

```
HTTP Request
    |
Chi Router
    |
NEUE: TenantMiddleware (extrahiert Tenant aus JWT oder Header)
    |
JWT Middleware (existierend)
    |
Permission Middleware (existierend)
    |
Handler -> Service -> Repository
    |                    |
    |               BUN Query mit tenant_id Filter
    |                    |
    |               PostgreSQL RLS erzwingt Isolation
    |
Response
```

### 4.2 JWT Token (Erweitert)

```go
type AppClaims struct {
    ID          int      `json:"id"`
    Sub         string   `json:"sub"`
    TenantID    int64    `json:"tenant_id"`    // NEU
    OrgID       int64    `json:"org_id"`       // NEU (optional, fuer Buero-User)
    Roles       []string `json:"roles"`
    Permissions []string `json:"permissions"`
    IsAdmin     bool     `json:"is_admin"`
    Scope       string   `json:"scope"`        // "platform", "org", "" (tenant)
}
```

### 4.3 Tenant-Context

```go
// Neues Package: tenant/context.go
type contextKey string
const tenantKey = contextKey("tenant_id")

func WithTenantID(ctx context.Context, id int64) context.Context {
    return context.WithValue(ctx, tenantKey, id)
}

func TenantFromContext(ctx context.Context) int64 {
    id, _ := ctx.Value(tenantKey).(int64)
    return id
}
```

### 4.4 Repository-Layer Aenderung

```go
// VORHER:
func (r *studentRepo) FindByID(ctx context.Context, id int64) (*users.Student, error) {
    student := new(users.Student)
    err := r.db.NewSelect().Model(student).Where("id = ?", id).Scan(ctx)
    return student, err
}

// NACHHER (Option A: Expliziter Filter):
func (r *studentRepo) FindByID(ctx context.Context, id int64) (*users.Student, error) {
    student := new(users.Student)
    err := r.db.NewSelect().Model(student).
        Where("id = ?", id).
        Where("tenant_id = ?", tenant.TenantFromContext(ctx)).  // NEU
        Scan(ctx)
    return student, err
}

// NACHHER (Option B: RLS macht es automatisch):
// Code bleibt gleich! RLS filtert unsichtbar.
// ABER: RLS allein reicht nicht - wir brauchen BEIDE Schichten als Defense-in-Depth.
```

**Empfehlung: Option A + RLS als Safety Net.** Explizite Tenant-Filter im Code machen die Intention klar und sind testbar. RLS faengt Fehler ab, die durch den Code rutschen.

### 4.5 Login-Flow (Sonderfall)

Beim Login kennt der Server den Tenant noch nicht (der User gibt nur Email + Passwort ein):

```
1. User geht auf altenberge.{TENANT_DOMAIN}
2. Frontend sendet Login-Request MIT Subdomain-Info (Header oder im Body)
3. Backend: Subdomain -> tenant_id Lookup
4. Backend: Suche Account WHERE email = ? AND tenant_id IN (account_tenants)
5. Bei Erfolg: JWT wird MIT tenant_id generiert
```

---

## 5. Frontend-Architektur

### 5.1 Subdomain-Routing

```
{TENANT_DOMAIN}              -> Tenant-Auswahl-Seite ("Welche OGS?")
altenberge.{TENANT_DOMAIN}   -> Login-Seite fuer OGS Altenberge (Root = Login, wie aktuell)
altenberge.{TENANT_DOMAIN}/dashboard -> Dashboard fuer OGS Altenberge

operator.{TENANT_DOMAIN}     -> Operator-Dashboard (unveraendert)
```

**Entscheidung:** Login-URL bleibt `/` (Root ist Login, wie aktuell). Eingeloggte User werden zu `/dashboard` redirected. Die Subdomain-Domain wird ueber `TENANT_DOMAIN` Umgebungsvariable konfiguriert (kann spaeter entschieden werden).

### 5.2 Next.js Middleware (Subdomain-Extraktion)

```typescript
// middleware.ts - ERWEITERT
export function middleware(request: NextRequest) {
    const hostname = request.headers.get('host') || '';
    const tenantDomain = process.env.TENANT_DOMAIN || 'moto-app.de';
    const subdomain = hostname.replace(`.${tenantDomain}`, '');

    // Operator-Routes: Eigene Logik (unveraendert)
    if (subdomain === 'operator') {
        return handleOperatorMiddleware(request);
    }

    // Root-Domain: Tenant-Auswahl
    if (!subdomain || subdomain === tenantDomain || subdomain === 'www') {
        return NextResponse.rewrite(new URL('/tenant-select', request.url));
    }

    // Tenant-Subdomain: Tenant-Context setzen
    const response = NextResponse.next();
    response.headers.set('x-tenant-slug', subdomain);
    return response;
}
```

### 5.3 Tenant-Context (React)

```typescript
// lib/tenant-context.tsx - NEU
interface TenantContext {
    tenantId: string;
    tenantSlug: string;
    tenantName: string;
    organizationId: string;
}

const TenantProvider: React.FC = ({ children }) => {
    // Tenant aus Subdomain oder API-Response
    const tenant = useTenantFromSubdomain();
    return <TenantCtx.Provider value={tenant}>{children}</TenantCtx.Provider>;
};
```

### 5.4 API-Route-Wrapper (Tenant-Header)

```typescript
// lib/route-wrapper.ts - ERWEITERT
export function createGetHandler(handler) {
    return async (request, context) => {
        const session = await auth();
        const token = session?.user?.token;
        const tenantSlug = request.headers.get('x-tenant-slug');

        // Tenant-Header an Backend weiterleiten
        return handler(request, token, params, { tenantSlug });
    };
}
```

---

## 6. Operator Dashboard: Erweiterungen

### 6.1 Neue Faehigkeiten

Das Operator Dashboard braucht:

1. **Tenant-Verwaltung:** CRUD fuer Organizations und Schools
2. **Tenant-Uebersicht:** Dashboard mit allen OGS, Status, User-Zahlen
3. **Tenant-Wechsel:** Impersonation / "Als OGS X einloggen" fuer Support
4. **Cross-Tenant Reporting:** Aggregierte Statistiken ueber alle OGS
5. **Subdomain-Verwaltung:** Neue Subdomains anlegen/deaktivieren

### 6.2 Operator -> Organization Zuordnung

```sql
-- Operators koennen Orgs verwalten
CREATE TABLE platform.operator_organizations (
    operator_id BIGINT FK -> platform.operators,
    organization_id BIGINT FK -> platform.organizations,
    role VARCHAR(50),  -- "owner", "admin", "viewer"
    PRIMARY KEY (operator_id, organization_id)
);
```

---

## 7. PyrePortal: Minimale Aenderungen

| Was | Aenderung |
|-----|----------|
| `DEVICE_API_KEY` | Bleibt gleich - Backend mappt Key -> Tenant |
| API-Endpunkte | Bleiben gleich - Backend scoped via Device -> Tenant |
| Lokaler Cache | Bleibt gleich - ist bereits device-spezifisch |
| `.env` Config | Optional: `TENANT_ID` hinzufuegen (fuer Debugging) |

**Fazit: PyrePortal braucht fast keine Aenderungen.** Der Backend-IoT-Layer mappt bereits Device -> School. Dieses Mapping bekommt einfach eine `tenant_id`.

---

## 8. Ferienbetreuung (Cross-Tenant Szenario)

### Problem
In einem Traegerverband von 5 OGS mischen sich Kinder waehrend der Ferienbetreuung. Betreuer muessen temporaer Kinder von fremden OGS einsehen koennen.

### Loesung: Temporaere Cross-Tenant-Zugriffe

```sql
-- Neue Tabelle
CREATE TABLE platform.cross_tenant_access (
    id BIGSERIAL PK,
    account_id BIGINT NOT NULL,           -- Wer bekommt Zugriff
    source_tenant_id BIGINT NOT NULL,     -- Heimat-OGS
    target_tenant_id BIGINT NOT NULL,     -- Zugriff auf welche OGS
    granted_by BIGINT NOT NULL,           -- Wer hat es freigegeben
    valid_from TIMESTAMPTZ NOT NULL,
    valid_until TIMESTAMPTZ NOT NULL,
    reason VARCHAR(200),                  -- "Ferienbetreuung Sommer 2026"
    active BOOLEAN DEFAULT true
);
```

**Implementierung:**
1. Org-Admin erstellt temporaere Cross-Tenant-Zugriffe
2. JWT wird bei naechstem Login/Refresh um `additional_tenant_ids: [3, 5, 7]` erweitert
3. RLS-Policy beruecksichtigt die zusaetzlichen Tenant-IDs
4. Nach Ablauf werden die Zugriffe automatisch deaktiviert

---

## 9. Entscheidungen

### 9.1 Email-Eindeutigkeit -- ENTSCHIEDEN: Global Unique

**Entscheidung: Option A - Ein Account, mehrere Tenants.**

- `auth.accounts.email` bleibt `UNIQUE` (global einzigartig)
- Neue Junction-Table `auth.account_tenants` mappt einen Account auf mehrere Schools
- Ein Lehrer, der an zwei OGS arbeitet, hat EIN Passwort und loggt sich je nach Subdomain bei der richtigen OGS ein
- Ferienbetreuung: Cross-Tenant-Access funktioniert automatisch ueber die Account-Tenant-Zuordnung

**Begruendung:** Bessere UX, passt zum Traeger-Modell (gleiche Mitarbeiter an mehreren Standorten), weniger Passwort-Chaos.

### 9.2 ID-Strategie -- ENTSCHIEDEN: Behalten (int64)

Auto-increment IDs + `tenant_id` als Composite Key. Kein Grund fuer UUIDs wenn RLS den Tenant-Scope sicherstellt.

### 9.3 Subdomain-Domain -- OFFEN (wird spaeter entschieden)

Wird ueber `TENANT_DOMAIN` Umgebungsvariable konfiguriert. Ob `moto-app.de`, `moto.nrw` oder eine andere Domain ist eine reine Deployment-Entscheidung, die erst in Phase 5 relevant wird.

### 9.4 Login-Page URL -- ENTSCHIEDEN: Root (`/`) bleibt Login

Wie aktuell. Die Subdomain identifiziert den Tenant. `altenberge.{TENANT_DOMAIN}/` zeigt Login. Eingeloggte User werden zu `/dashboard` redirected.
