# 10 — Codebase Audit & Devil's Advocate Review (Round 3)

> Cross-reference of all multi-tenancy review documents (06, 07, 08, 09, DEBATE D1-D21)
> against the current codebase state and documentation consistency.
>
> Stand: 2026-02-11

---

## Zusammenfassung

Drei Review-Runden haben insgesamt 68+ Findings produziert. Hier der aktuelle Stand:

| Review-Dokument | Findings | Geloest | **OFFEN** |
|---|---|---|---|
| 06-offene-punkte.md (16 Findings) | 16 | 16 | **0** |
| 09-devils-advocate-yonnock (23+ Findings) | 23+ | 23+ | **0** |
| **07-devils-advocate-review.md (29 Findings)** | 29 | 2 | **27** |
| **10-codebase-audit (dieses Dokument)** | 7 | — | **7** |

**06 + 09**: Vollstaendig geloest durch DEBATE.md D1-D21 und Doku-Updates.

**07 (neueste Review)**: Nur C-1 und C-2 gefixt (Commit `94ac0cdb`). **27 Findings bleiben offen** — darunter 4 CRITICAL, 9 HIGH.

**10 (dieses Dokument)**: 7 neue Findings aus frischer Codebase-Analyse.

---

## Teil 1: Status der 07-Review Findings

### CRITICAL (4 von 6 offen)

| ID | Finding | Status | Kommentar |
|---|---|---|---|
| C-1 | Transaction-Ownership Widerspruch §1.3 vs §8.4 | **GELOEST** | `94ac0cdb` — §8.4 nutzt jetzt tx aus Context |
| C-2 | Keine CI-Pruefung fuer useSWR → useTenantSWR | **GELOEST** | `94ac0cdb` — ESLint-Regel ergaenzt |
| **C-3** | **OrgScope BYPASSRLS = nur 1 Sicherheitsschicht** | **OFFEN** | Plan hat 4 Defense-Layers ueberall AUSSER OrgScope. Kein DB-Level-Schutz. Ein Bug in `WHERE org_id = ?` leaked ALLE Tenant-Daten |
| **C-4** | **Tenant-Loeschung komplett unspezifiziert** | **OFFEN** | GDPR Art. 17 erfordert es. Keine FK-Reihenfolge, kein Account-Handling, kein Cooling-Off, kein Subdomain-Recycling. D21 behandelt nur einzelne Account-Loeschung |
| **C-5** | **Migrations-Luecke: Schritte 4-11 = kaputte App** | **OFFEN** | Nach `SET NOT NULL` (Schritt 6) aber vor Code-Deploy (Schritt 11) kann alter Code nicht INSERT. Kein `DEFAULT 1` Workaround dokumentiert |
| **C-6** | **D12 vs 02-datenbank.md: auth.tokens Widerspruch** | **OFFEN** | D12 sagt "kein tenant_id auf auth.tokens", 02-datenbank.md listet es als "MIT tenant_id NOT NULL" |

### HIGH (alle 9 offen)

| ID | Finding | Status |
|---|---|---|
| **H-1** | Provisioning-Spec fehlt komplett | **OFFEN** — Kein `POST /operator/schools` Endpoint, kein Service-Flow |
| **H-2** | CREATE INDEX CONCURRENTLY innerhalb BUN-Transaktionen | **OFFEN** — Schlaegt fehl oder verursacht Table-Locks |
| **H-3** | UNIQUE/FK-Timing in Migrationsstrategie unklar | **OFFEN** — 31 Constraints + 64 FKs keinem Schritt zugeordnet |
| **H-4** | Rollback-Plan unvollstaendig | **OFFEN** — Nur Phase 1/2 abgedeckt, nicht 13 Einzelschritte |
| **H-5** | Cross-Tenant Enrollment-Autorisierung unklar | **OFFEN** — Wie findet Host-OGS Admin Quell-Kinder? |
| **H-6** | Announcements Per-OGS-Targeting fehlt im Schema | **OFFEN** — Keine `target_tenant_ids` Spalte |
| **H-7** | Subdomain-Takeover-Monitoring undefiniert | **OFFEN** — Wildcard-Cookie-Theft moeglich |
| **H-8** | Kein Konzept fuer inkrementelles Deployment | **OFFEN** — Big-Bang-Deploy von 100+ Endpoints riskant |
| **H-9** | account_roles in falscher Doku-Kategorie | **OFFEN** — Als NULLABLE gelistet, Spec sagt NOT NULL |

### MEDIUM (alle 9 offen)

| ID | Finding | Status |
|---|---|---|
| **M-1** | Cross-Tenant-Audit unvollstaendig spezifiziert | **OFFEN** |
| **M-2** | Eltern OGS-Switcher technisches Design fehlt | **OFFEN** |
| **M-3** | settings JSONB ohne Validierung (XSS-Risiko) | **OFFEN** |
| **M-4** | Performance-Benchmarks nicht definiert | **OFFEN** |
| **M-5** | Doppelte Sektion 6 in 05-testing.md | **OFFEN** |
| **M-6** | router.push vs window.location.href Inkonsistenz | **OFFEN** |
| **M-7** | Origin-Header kann fehlen oder gespooft werden | **OFFEN** |
| **M-8** | Enrollment-Atomizitaet bei Eltern Auto-Provisioning | **OFFEN** |
| **M-9** | Phase 2 (Logging) ohne Monitoring-Tooling | **OFFEN** |

### LOW (alle 5 offen)

| ID | Finding | Status |
|---|---|---|
| **L-1** | D12-Text erwaehnt veralteten Kontext | **OFFEN** |
| **L-2** | Aufwand-Schaetzung veraltet | **OFFEN** |
| **L-3** | Operator-OGS-Auswahl ohne UI-Spec | **OFFEN** |
| **L-4** | cross_tenant_access ohne automatischen Cleanup | **OFFEN** |
| **L-5** | meta.migration_metadata Zaehlung | **OFFEN** |

---

## Teil 2: Neue Findings aus Codebase-Audit (Devil's Advocate Round 3)

### DA-1: CRITICAL — Compile-Time Tenant-Safety fehlt

**Quelle:** Codebase-Analyse aller Repository-Files

**Problem:** Der Plan definiert ein `TenantScoped` Interface (D2) und `base.TenantModel` Mixin. Aber es gibt keinen Mechanismus der zur Compile-Time sicherstellt, dass Repository-Methoden fuer tenant-scoped Entities tatsaechlich den Tenant-Filter nutzen.

Bei 538 `r.db.`-Calls, 150+ Query-Methoden in 32 Repository-Files reicht ein einziger vergessener Filter fuer ein Datenleck. Der Plan verlaesst sich auf:
1. RLS (DB-Level) — korrekt, aber Admin-Scope Jobs umgehen RLS
2. Code-Review — manuell, fehleranfaellig bei 538 Stellen
3. Tests — muessen geschrieben werden, koennten Faelle uebersehen

**Was fehlt:** Ein generischer Repository-Base-Layer der tenant-scoped Queries erzwingt:

```go
// Konzept: Base-Repository mit Compile-Time Enforcement
type TenantRepository[T TenantScoped] struct {
    db *bun.DB
}

func (r *TenantRepository[T]) FindByID(ctx context.Context, id int64) (*T, error) {
    tenantID := tenant.MustFromContext(ctx) // panics wenn kein Tenant
    var entity T
    err := r.getDB(ctx).NewSelect().
        Model(&entity).
        Where("id = ?", id).
        Where("tenant_id = ?", tenantID). // IMMER automatisch
        Scan(ctx)
    return &entity, err
}
```

**Handlungsbedarf:** Generischen Tenant-Base-Repository in die Architektur aufnehmen oder CI-Check der alle `NewSelect()`/`NewInsert()`/`NewUpdate()`/`NewDelete()` auf tenant-scoped Tabellen prueft ob `tenant_id` in der WHERE-Clause vorkommt.

---

### DA-2: HIGH — Globale Authorization-Registry koennte Cross-Tenant cachen

**Datei:** `backend/auth/authorize/resource_middleware.go`

**Problem:** `defaultResourceAuthorizer` ist ein globaler Singleton:
```go
var defaultResourceAuthorizer *ResourceAuthorizer

func SetResourceAuthorizer(ra *ResourceAuthorizer) {
    defaultResourceAuthorizer = ra
}
```

Der Authorizer selbst ist aktuell stateless (kein Caching). Aber wenn zukuenftig Performance-Caching hinzugefuegt wird (z.B. "Ist User X Betreuer von Gruppe Y?" cachen), koennte stale Cache von Tenant A Requests fuer Tenant B autorisieren.

**Handlungsbedarf:** Explizites "No-Caching"-Constraint in Policy-Engine-Dokumentation. Oder: Falls Caching kommt, muss der Cache-Key immer `(tenant_id, ...)` enthalten.

---

### DA-3: HIGH — GDPR-Cleanup ohne Tenant-Filter bei Admin-Scope

**Datei:** `backend/database/repositories/active/visits.go:221`

**Problem:** `DeleteExpiredVisits` filtert nur nach `student_id`, nicht nach `school_id`:

```go
func (r *VisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, ...) {
    r.db.NewDelete().
        Where(`"visit".student_id = ?`, studentID).
        Where(`"visit".created_at < ?`, cutoffDate)
    // KEIN school_id Filter
}
```

Das ist im Single-Tenant-Modus korrekt. Aber der Scheduler-Plan (03-backend.md §11) klassifiziert GDPR-Cleanup als "Admin-Scope" — d.h. `WithAdminTx` (BYPASSRLS). Damit umgeht der Job alle RLS-Policies und operiert auf ALLEN Tenants gleichzeitig. Ein falscher `student_id` (ID-Kollision zwischen Tenants, z.B. Student 42 bei OGS A und Student 42 bei OGS B) wuerde die Daten des falschen Tenants loeschen.

**Handlungsbedarf:** GDPR-Cleanup MUSS entweder:
- (a) Tenant-Scope nutzen (`WithTenantTx` pro Tenant iterierend), oder
- (b) Expliziten `WHERE tenant_id = ?` Filter haben auch im Admin-Scope

---

### DA-4: HIGH — Audit-Logging-Model ohne Tenant-Context

**Datei:** `backend/models/audit/data_deletion.go`

**Problem:** Das `DataDeletion` Model hat kein `SchoolID`/`TenantID` Feld:
```go
type DataDeletion struct {
    ID          int64
    EntityType  string
    EntityID    int64
    DeletedBy   int64
    DeletedAt   time.Time
    Reason      string
    // KEIN TenantID — GDPR-Compliance-Reports pro OGS nicht moeglich
}
```

Fuer GDPR-Compliance muss ein Traeger fragen koennen: "Zeige mir alle Loeschungen fuer OGS Altenberge." Ohne `tenant_id` im Audit-Log muesste man ueber EntityType + EntityID JOINen — fragil und bei geloeschten Entities unmoeglich.

**Handlungsbedarf:** `TenantID int64` zum DataDeletion Model hinzufuegen. In 02-datenbank.md als tenant_id NOT NULL markieren.

---

### DA-5: MEDIUM — Race Condition bei Tenant-Switch + Ferienbetreuung

**Dokument:** 03-backend.md §6.2 + D4

**Problem:** Ablauf:
1. Lehrer switched zu Host-OGS (neuer JWT mit tenant_id = Host)
2. Oeffnet Feriengruppe
3. Active-Service liest Cross-Tenant-Kinder via `WithAdminTx`

Was passiert wenn `account_tenants` Record des Lehrers fuer die Host-OGS **zwischen** Tenant-Switch und Cross-Tenant-Read deaktiviert wird (z.B. durch Admin)? Der JWT hat noch den alten `tenant_id`, aber die Berechtigung ist entzogen. Refresh-Token Check passiert stuendlich, nicht pro Request.

**Handlungsbedarf:** Entweder:
- (a) Pro-Request Validierung von `account_tenants.status` (Performance-Impact)
- (b) Akzeptiertes Risiko mit max. 1h Fenster dokumentieren
- (c) Event-basierte JWT-Invalidierung bei `account_tenants` Statusaenderung

---

### DA-6: MEDIUM — Kein Rate-Limiting auf Tenant-Switch Endpoint

**Dokument:** 03-backend.md §6.2

**Problem:** `POST /auth/switch-tenant` generiert einen neuen JWT. Ohne Rate-Limiting kann ein Angreifer mit kompromittiertem Account alle Tenant-Slugs proben. Selbst wenn jeder Call "nicht autorisiert" zurueckgibt, koennten Timing-Unterschiede verraten welche Tenants existieren (IDOR/Enumeration).

**Handlungsbedarf:** Rate-Limiting auf `/auth/switch-tenant` (z.B. 10 Requests/Minute pro Account). Einheitliche Response-Zeit unabhaengig von Tenant-Existenz.

---

### DA-7: MEDIUM — resolveTenant erlaubt Tenant-Enumeration

**Dokument:** 04-frontend.md §3, D17

**Problem:** Tenant-Validierung in `[tenant]/layout.tsx` via `resolveTenant()`. Unbekannte Slugs → `notFound()` (404). Valide Slugs → 200. Damit kann jeder valide Tenant-Slugs enumerieren:
- `school-a.moto-app.de` → 200 (existiert)
- `school-b.moto-app.de` → 404 (existiert nicht)

D17 erkennt das und nennt Rate-Limiting als Mitigation. Aber fuer ein Schulsystem kann es sensibel sein, welche Schulen die Platform nutzen (z.B. im Kontext von Datenschutz-Diskussionen).

**Handlungsbedarf:** Optionen:
- (a) Rate-Limiting + akzeptiertes Risiko dokumentieren (D17-Position)
- (b) Generische Login-Page ohne Tenant-Branding als Default, Branding erst nach Authentifizierung
- (c) CAPTCHA nach 5 ungültigen Subdomain-Aufrufen

---

## Teil 3: Handlungsempfehlung

### Sofort (Doku-Fixes, < 1 Tag je)

| Prioritaet | Finding | Aufwand |
|---|---|---|
| 1 | **C-5**: `DEFAULT 1` in Migrationsstrategie oder Schritte umordnen | 30min |
| 2 | **C-6**: auth.tokens tenant_id Ja/Nein entscheiden, D12 + 02-datenbank.md synchen | 30min |
| 3 | **H-9**: account_roles in korrekte Doku-Kategorie verschieben | 15min |
| 4 | **M-5**: 05-testing.md Sektionsnummern korrigieren | 15min |
| 5 | **M-6**: router.push auf window.location.href vereinheitlichen | 15min |
| 6 | **L-1**: D12-Text aktualisieren (abhaengig von C-6) | 15min |

### Vor Implementierungsbeginn (1-3 Tage je)

| Prioritaet | Finding | Aufwand |
|---|---|---|
| 1 | **C-3**: OrgScope DB-Level-Schutz entscheiden (RLS-Subquery oder akzeptiertes Risiko) | 2h |
| 2 | **C-4**: Tenant-Loeschungs-Orchestrierung spezifizieren | 1-2 Tage |
| 3 | **H-1**: Provisioning-Service Endpoint + Transaktions-Flow | 1 Tag |
| 4 | **H-2**: CONCURRENTLY-Index Approach ausserhalb BUN dokumentieren | 2h |
| 5 | **H-3**: UNIQUE/FK-Migration in 13-Schritt-Plan einordnen | 2h |
| 6 | **H-4**: Pro-Schritt Rollback-Befehle dokumentieren | 4h |
| 7 | **H-8**: Feature-Flag oder backward-compatible Deployment-Strategie | 4h |
| 8 | **DA-1**: Compile-Time Tenant-Safety Konzept festlegen | 2h |
| 9 | **DA-3**: GDPR-Cleanup Tenant-Scope vs Admin-Scope klaeren | 1h |
| 10 | **DA-4**: Audit-Model um TenantID erweitern | 30min |

### Waehrend Implementierung

Alle verbleibenden H, M, L Findings + DA-2, DA-5, DA-6, DA-7.

---

## Gesamtbewertung

### Staerken

1. **Architektur-Fundament ist solide** — Shared Schema + RLS, Drei-Rollen-Modell, Defense-in-Depth (4 Schichten)
2. **DEBATE.md ist exzellent** — 21 fundiert diskutierte Entscheidungen mit verworfenen Alternativen
3. **Erste zwei Review-Runden vollstaendig geloest** — Alle 39+ Findings aus 06 + 09 adressiert
4. **Codebase ist sauber** — Keine globalen Caches, keine shared State, stateless Repositories

### Schwaechen

1. **07-Review Findings weitgehend ignoriert** — 27 von 29 Findings offen, darunter 4 CRITICAL
2. **Tenant-Loeschung (C-4) ist ein GDPR-Blocker** — Ohne Spezifikation kann kein Traeger den Vertrag kuendigen
3. **Migrations-Luecke (C-5) verursacht Downtime** — Zero-Downtime-Claim ist falsch solange kein `DEFAULT 1` gesetzt wird
4. **OrgScope (C-3) widerspricht eigenem Sicherheitsmodell** — Plan definiert 4 Defense-Layers, OrgScope hat nur 1

**Empfehlung:** Die 6 "Sofort"-Fixes und die 10 "Vor Implementierung"-Items abarbeiten bevor Code geschrieben wird. Geschaetzter Aufwand: **3-5 Arbeitstage** reine Dokumentation.

---

## Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-11 | Initiales Codebase-Audit + Cross-Reference aller Review-Runden |
