# 13 — Plan Review: Empfehlungen nach Devil's Advocate Audit

> Cross-Reference der 30 Findings aus `12-devils-advocate-4-agents.md`
> gegen den neuen Implementierungsplan `11-implementierungsplan.md`
> und die 5 Konsistenz-Fixes (`83d52279`).
>
> Stand: 2026-02-11

---

## Zusammenfassung

| Kategorie | Adressiert | Teilweise | **Offen** |
|-----------|-----------|-----------|-----------|
| CRITICAL (10) | 6 | 2 | **2** |
| HIGH (14) | 7 | 6 | **1** |
| MEDIUM (5) | 4 | 1 | 0 |
| LOW (1) | 1 | 0 | 0 |
| **Gesamt** | **18** | **9** | **3** |

Der neue Plan (Expand-Contract, RLS-als-Letztes, Feature-Branch) ist eine deutliche Verbesserung. Die verbleibenden Luecken konzentrieren sich auf **Cross-Tenant-Szenarien** (Ferienbetreuung, IoT) und **operationelle Randfaelle** (Scheduler, Goroutines, Traeger-Deaktivierung).

---

## Teil 1: Vollstaendig adressierte Findings (18)

Diese Findings sind durch den neuen Plan abgedeckt und brauchen keine weitere Aktion:

| Finding | Wie adressiert |
|---------|---------------|
| CRIT-4: Base-Repo `r.DB` | Phase 1.8: `getDB(ctx)` Helper im Base-Repository |
| CRIT-5: 18x `s.db.` in Services | Phase 3 Checkliste Schritt 2+5: alle `r.db`/`s.db` → `getDB(ctx)` |
| CRIT-7: `admin:*` Wildcard | Phase 1.5: JWT Claims erweitern; Phase 3: Handler-Migration mit Tenant-Scoping |
| HIGH-2: Cross-Schema JOINs bei RLS-Rollout | Eliminiert durch "RLS als Letztes"-Strategie (Phase 4.9) |
| HIGH-5: DEFAULT 1 Race Condition | Expand-Contract: DEFAULT 1 bleibt bis Phase 4.11 |
| HIGH-8: Session-Cache + SWR Tenant-Awareness | Phase 4.4 (useTenantSWR) + Phase 4.6 (Session-Cache) |
| HIGH-9: Redirect-Count (62 vs 6) | Phase 4.5: useTenantRouter (40+ Stellen) |
| HIGH-11: WithTx Cross-Service-Propagation | Phase 3 Schritt 7: WithTx-Patterns entfernen, tx aus Context |
| HIGH-13: Active-Domain Timing-Konflikt | Geloest: Dev A besitzt sowohl `active/` als auch `auth/` |
| MED-1: Raw ExecContext SQL | Phase 3 Checkliste Schritt 2: alle Queries migrieren |
| MED-3: Advisory Lock | Phase 4.13: Zwei-Argument-Form |
| MED-4: Tenant-Slug Kollision | Phase 4.2: Subdomain-Routing mit Reserved-List |
| MED-5: Connection Pool Poisoning | Akzeptiertes Restrisiko (fail-closed durch NULLIF) |
| LOW-1: Sequence/ID Leakage | Akzeptiertes Risiko (DEBATE) |
| CRIT-1: BUN vs base.TxFromContext Keys | Phase 1.1 + 1.8: neues `getDB(ctx)` nutzt BUN-nativen Key; Phase 3 entfernt alle `base.TxFromContext` |
| CRIT-2: Trigger bypasses RLS | Phase 2: Trigger-Funktionen auf SECURITY INVOKER umschreiben |
| CRIT-3: View ohne security_invoker | Phase 4.12 (Reihenfolge-Fix noetig, siehe Teil 2) |
| HIGH-14: Cross-Carrier AVV | Rechtliche/geschaeftliche Entscheidung (kein Code-Problem) |

---

## Teil 2: Teilweise adressierte Findings — Konkrete Fixes (9)

### FIX-1: Phase 4.12 VOR Phase 4.9 ausfuehren (CRIT-3)

**Problem:** `users.expired_privacy_consents` View ohne `security_invoker = true` leakt PII aller Tenants. Phase 4.12 fixt das, aber Phase 4.9 (RLS aktivieren) kommt vorher laut "Kritischer Reihenfolge".

**Fix:** In `11-implementierungsplan.md` die Reihenfolge aendern:

```
Kritische Reihenfolge:
1. Zuerst 4.12 (Views mit security_invoker)    ← VOR RLS
2. Dann 4.9 (RLS Policies aktivieren)
3. Dann 4.10 (Composite FKs)
4. Dann 4.11 (DEFAULT weg)
```

**Aufwand:** 0 — nur Dokument-Aenderung.

---

### FIX-2: CSP-Headers in Phase 4.1 ergaenzen (CRIT-6)

**Problem:** Wildcard-Cookie `.moto-app.de` ohne Content-Security-Policy. Jede XSS-Luecke auf irgendeiner Subdomain stiehlt alle Sessions.

**Fix:** In Phase 4.1 (Next.js Middleware) ergaenzen:

```typescript
// middleware.ts — neben Subdomain-Rewrite
const cspHeader = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline'`,  // Next.js benoetigt unsafe-inline
  `connect-src 'self' ${request.nextUrl.origin}`,
  "frame-ancestors 'none'",
  "base-uri 'self'",
].join("; ");

response.headers.set("Content-Security-Policy", cspHeader);
response.headers.set("X-Content-Type-Options", "nosniff");
response.headers.set("X-Frame-Options", "DENY");
```

**Aufwand:** ~2h — Teil von Phase 4.1 Middleware-Arbeit.

---

### FIX-3: Goroutine-Context-Pattern dokumentieren (HIGH-4)

**Problem:** 3 Goroutines (`logAuthEvent`, `sendGuardianEmail`, `sendInvitationEmail`) nutzen `context.WithoutCancel(ctx)` oder `context.Background()`. Nach Multi-Tenancy fehlt der Tenant-Context.

**Fix:** In Phase 3 Domain-Checkliste als Schritt 8.5 ergaenzen:

```
8.5 Goroutines: tenant_id VOR go func() extrahieren

// FALSCH — ctx ist nach Handler-Return tot
go func() {
    s.repo.Create(ctx, event)  // ctx hat toten tx
}()

// RICHTIG — frischen Context mit tenant_id
tenantID := tenant.FromContext(ctx)
go func() {
    bgCtx := tenant.NewContext(context.Background(), tenantID)
    tenant.WithTenantTx(bgCtx, s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {
        return s.repo.Create(ctx, event)
    })
}()
```

**Betroffene Stellen:**

| Datei | Zeile | Goroutine |
|-------|-------|-----------|
| `services/auth/token_cleanup.go` | 79-89 | `logAuthEvent` — `context.WithoutCancel(ctx)` |
| `services/users/guardian_service.go` | 315, 333 | `sendInvitationEmail` — `context.Background()` |
| `services/users/guardian_service.go` | 365, 373 | `sendInvitationEmail` — `context.Background()` |

**Aufwand:** ~1h Dokumentation + ~2h Implementierung pro Domain.

---

### FIX-4: SSE Hub Tenant-Design spezifizieren (HIGH-3)

**Problem:** SSE Hub ist ein globaler Singleton ohne Tenant-Awareness. `verifyGroupTenant()` existiert nicht, Hub hat keinen DB-Zugang, SSE-Connections sind langlebig (keine Transaction moeglich).

**Fix:** Konkretes Design fuer Phase 3 (Dev A, Woche 3):

```go
// hub.go — Tenant-prefixed Keys
type Hub struct {
    clients      map[*Client]bool
    groupClients map[string][]*Client  // "tenantID:groupID" -> subscribers
    mu           sync.RWMutex
    logger       *slog.Logger
}

type Client struct {
    Channel          chan Event
    UserID           int64
    TenantID         int64              // NEU
    SubscribedGroups map[string]bool
}

// Subscribe validiert Tenant-Zugehoerigkeit
func (h *Hub) Subscribe(client *Client, tenantID int64, groupID string) {
    key := fmt.Sprintf("%d:%s", tenantID, groupID)
    // Keine DB-Abfrage noetig: tenantID kommt aus JWT (bereits validiert)
    h.mu.Lock()
    defer h.mu.Unlock()
    h.groupClients[key] = append(h.groupClients[key], client)
    client.SubscribedGroups[key] = true
}

// Broadcast nur an Clients des gleichen Tenants
func (h *Hub) BroadcastToGroup(tenantID int64, groupID string, event Event) {
    key := fmt.Sprintf("%d:%s", tenantID, groupID)
    h.mu.RLock()
    defer h.mu.RUnlock()
    for _, client := range h.groupClients[key] {
        select {
        case client.Channel <- event:
        default:
            // Client kann nicht mithalten, ueberspringen
        }
    }
}
```

**SSE-Connection Lifecycle** (keine Transaction noetig):

```
1. SSE-Handler: tenantID aus JWT extrahieren (KEIN WithTenantTx)
2. Hub.Subscribe(client, tenantID, groupID)
3. Event-Loop: events aus client.Channel lesen und senden
4. Bei Disconnect: Hub.Unsubscribe(client)
```

**Frontend:** `useGlobalSSE` muss bei Tenant-Switch reconnecten:

```typescript
// use-global-sse.ts
const { tenantId } = useTenantContext();

useEffect(() => {
    const eventSource = new EventSource(`/api/sse/events?tenant=${tenantId}`);
    // ... event handlers ...
    return () => eventSource.close();  // Cleanup bei Tenant-Wechsel
}, [tenantId]);  // Re-connect bei Tenant-Aenderung
```

**Aufwand:** ~1 Tag Backend, ~0.5 Tag Frontend.

---

### FIX-5: Frontend/Backend Backward-Compatibility (HIGH-6)

**Problem:** Backend aendert JWT/Login-Endpoint in Phase 3, Frontend startet erst Phase 4. Dazwischen ist die App kaputt.

**Fix:** Login-Endpoint bleibt backward-compatible:

```go
// api/auth/login.go — Phase 3
type LoginRequest struct {
    Email      string `json:"email"`
    Password   string `json:"password"`
    TenantSlug string `json:"tenant_slug,omitempty"` // Optional waehrend Migration
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    // ...
    if req.TenantSlug == "" {
        // Fallback: Default-Tenant (ID=1) waehrend Migration
        req.TenantSlug = "default"
    }
    // ... normaler Login-Flow mit Tenant-Resolution
}
```

**In `11-implementierungsplan.md` ergaenzen:**

```
Phase 3 Regel: Alle Backend-API-Aenderungen MUESSEN backward-compatible sein.
- Login: tenant_slug ist optional (Default: Tenant 1)
- JWT: neue Claims (tenant_id, org_id) werden hinzugefuegt, alte bleiben
- Bestehende Endpoints bleiben funktional
- Erst in Phase 4 (nach Frontend-Migration) werden alte Pfade entfernt
```

**Aufwand:** 0 extra — nur eine Design-Regel fuer Phase 3.

---

### FIX-6: Zwei-Tab-Szenario dokumentieren (HIGH-7)

**Problem:** Wildcard-Cookie wird zwischen Tabs geteilt. Tab A refresht Token fuer Tenant A, Tab B bekommt falschen JWT.

**Fix:** Als bekannte Limitation dokumentieren + Mitigation:

```
Bekannte Limitation: Zwei-Tab Multi-Tenant

Szenario: User oeffnet Tab A (OGS-A) und Tab B (OGS-B).
- Wildcard-Cookie `.moto-app.de` wird geteilt
- Token-Refresh in Tab A ueberschreibt Session fuer Tab B
- Tab B macht API-Calls mit OGS-A JWT

Mitigation (Phase 4.6):
1. Session-Cache prueft tenant_id bei jedem getSession()-Call
2. Bei Mismatch: automatischer Re-Login (stiller Token-Refresh fuer den richtigen Tenant)
3. Backend RLS verhindert Datenzugriff auf falschen Tenant (Defense-in-Depth)
4. Frontend zeigt Tenant-Mismatch-Banner: "Seite wird aktualisiert..."

Akzeptiertes Restrisiko:
- Kurzzeitiges Flackern (max 10 Sekunden Session-Cache TTL)
- Kein Datenleck dank RLS als Safety-Net
```

**Aufwand:** ~4h Implementierung in Phase 4.6.

---

### FIX-7: Scheduler Tenant-Strategie konkretisieren (HIGH-10)

**Problem:** `scheduler.go` nutzt `context.Background()` an 5 Stellen. Geplante Jobs haben keinen Tenant-Context.

**Fix:** Konkretes Design fuer Phase 4.15:

```go
// scheduler.go — Tenant-Iteration Pattern
func (s *Scheduler) runDailyCleanup() {
    ctx := context.Background()

    // 1. Tenant-Liste holen (Admin-Scope)
    var tenants []platform.School
    tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
        return tx.NewSelect().Model(&tenants).
            Where("active = true").Scan(ctx)
    })

    // 2. Pro Tenant ausfuehren
    for _, t := range tenants {
        err := tenant.WithTenantTx(ctx, s.db, t.ID, func(ctx context.Context, tx bun.Tx) error {
            return s.cleanupService.CleanupExpiredVisits(ctx)
        })
        if err != nil {
            s.logger.Error("cleanup failed", "tenant_id", t.ID, "error", err)
            // Weiter mit naechstem Tenant — ein Fehler blockiert nicht alle
        }
    }
}
```

**Job-Klassifikation (aus 03-backend.md Sektion 11, revidiert):**

| Job | Scope | Begruendung |
|-----|-------|-------------|
| `CleanupExpiredVisits` | **Tenant-Scope** (Iteration) | Braucht Privacy-Consent pro Student, RLS-geschuetzt |
| `CleanupExpiredTokens` | Admin-Scope | Systemweite Token-Bereinigung, kein PII |
| `EndDailySessions` | **Tenant-Scope** (Iteration) | Beendet Sessions pro OGS, braucht Tenant-Context |
| `CleanupInvitations` | Admin-Scope | Abgelaufene Tokens, kein PII |
| `SendEmailDigests` | **Tenant-Scope** (Iteration) | Braucht Schuelerdaten fuer E-Mail-Inhalt |

**Aufwand:** ~1 Tag Refactoring des Schedulers.

---

### FIX-8: Ferienbetreuung Data Minimization (HIGH-12)

**Problem:** `WithAdminTx` fuer Cross-Tenant-Read gibt volle Student-Profile zurueck inkl. HealthInfo, GuardianContact etc. Verstoss gegen DSGVO Art. 5(1)(c).

**Fix:** Dedizierte Cross-Tenant-Query mit minimalen Feldern:

```go
// services/active/ferienbetreuung.go — NEU
type CrossTenantStudent struct {
    ID        int64  `bun:"id"`
    FirstName string `bun:"first_name"`
    LastName  string `bun:"last_name"`
    Class     string `bun:"school_class"`
    // KEIN HealthInfo, GuardianContact, SupervisorNotes etc.
}

func (s *Service) GetCrossTenantEnrolledStudents(ctx context.Context, groupID int64) ([]CrossTenantStudent, error) {
    var students []CrossTenantStudent
    err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
        return tx.NewSelect().
            TableExpr("users.students AS s").
            ColumnExpr("s.id, p.first_name, p.last_name, s.school_class").
            Join("INNER JOIN users.persons AS p ON p.id = s.person_id").
            Join("INNER JOIN activities.enrollments AS e ON e.student_id = s.id").
            Where("e.group_id = ?", groupID).
            Scan(ctx, &students)
    })
    return students, err
}
```

**In DEBATE.md als D22 ergaenzen:**

```
D22: Ferienbetreuung Cross-Tenant Data Minimization

Entscheidung: Cross-Tenant-Reads liefern NUR minimale Felder:
- Person: first_name, last_name
- Student: school_class, id
- KEINE: HealthInfo, GuardianName, GuardianContact, ExtraInfo, SupervisorNotes

Begruendung: DSGVO Art. 5(1)(c) — Datenminimierung.
Ferienbetreuungs-Personal braucht nur Name + Klasse fuer Anwesenheit.
Bei medizinischem Notfall: Rueckfrage an Home-OGS.
```

**Aufwand:** ~0.5 Tage (neue Query + Tests).

---

### FIX-9: IoT UNIQUE Constraint (MED-2)

**Problem:** `iot.devices` hat `UNIQUE(device_id)` global. Geraet kann nach Umzug nicht an neuem Tenant registriert werden.

**Fix:** In Phase 2.5 (UNIQUE Constraints migrieren) aufnehmen:

```sql
-- iot.devices: device_id nur pro Tenant unique, api_key bleibt global unique
ALTER TABLE iot.devices DROP CONSTRAINT IF EXISTS devices_device_id_key;
ALTER TABLE iot.devices ADD CONSTRAINT devices_device_id_tenant_unique
    UNIQUE(tenant_id, device_id);
-- api_key bleibt UNIQUE(api_key) — Sicherheitsanforderung
```

**Aufwand:** Teil der Phase 2.5 Migration, ~10 Minuten.

---

## Teil 3: Nicht adressierte Findings — Neue DEBATE-Eintraege noetig (3)

Diese Findings erfordern **Architektur-Entscheidungen**, nicht nur Code-Aenderungen. Sie sollten als D22-D24 in DEBATE.md aufgenommen werden.

### OPEN-1: Cross-Tenant Consent-Widerruf bei Ferienbetreuung (CRIT-8)

**Das Problem:**
- Elternteil widerruft Einwilligung bei OGS-A (Home-OGS)
- Kind ist in Ferienbetreuungsgruppe bei OGS-B eingeschrieben
- OGS-B hat keinen Mechanismus, um den Widerruf zu erfahren
- DSGVO Art. 7(3): "Widerruf muss jederzeit moeglich sein"

**Optionen:**

| Option | Beschreibung | Aufwand | Risiko |
|--------|-------------|---------|--------|
| A: Event-basiert | Home-OGS sendet Event bei Consent-Aenderung, Host-OGS reagiert | Mittel | Event kann verloren gehen |
| B: Real-time Check | Host-OGS prueft Consent bei jedem Zugriff via Admin-Read | Niedrig | Performance (zusaetzliche Query pro Zugriff) |
| C: Consent-Kopie | Bei Cross-Tenant-Enrollment wird Consent-Record bei Host-OGS erstellt | Hoch | Zwei Quellen der Wahrheit |

**Empfehlung:** Option B (Real-time Check) — einfachste Implementierung, keine Race Conditions:

```go
// Vor jedem Cross-Tenant-Datenzugriff:
func (s *Service) checkCrossTenantConsent(ctx context.Context, studentID, homeTenantID int64) (bool, error) {
    var consent users.PrivacyConsent
    err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
        return tx.NewSelect().Model(&consent).
            Where("student_id = ? AND tenant_id = ? AND accepted = true", studentID, homeTenantID).
            Scan(ctx)
    })
    return consent.Accepted, err
}
```

**Entscheidung noetig von:** Product Owner / Datenschutzbeauftragter

---

### OPEN-2: Traeger-Deaktivierung (Cascade) (CRIT-9)

**Das Problem:**
- Traeger geht insolvent, betreibt 10 OGS
- Alle 10 Tenants muessen deaktiviert werden
- Hunderte Accounts, aktive Sessions, Cross-Tenant-Referenzen betroffen

**Optionen:**

| Option | Beschreibung | Aufwand |
|--------|-------------|---------|
| A: Soft-Delete Cascade | `platform.organizations.active = false` → alle Schools + Accounts deaktivieren | Mittel |
| B: Manuell pro Tenant | Operator deaktiviert jede OGS einzeln | Niedrig (aber fehleranfaellig) |
| C: Grace Period | 30 Tage Cooling-Off, dann automatische Deaktivierung | Hoch |

**Empfehlung:** Option A mit Grace Period:

```sql
-- platform.organizations
ALTER TABLE platform.organizations ADD COLUMN deactivated_at TIMESTAMPTZ;

-- Cascade-Trigger (oder Service-Logik):
-- 1. SET platform.schools.active = false WHERE org_id = ?
-- 2. SET auth.account_tenants.status = 'inactive' WHERE tenant_id IN (betroffene Schools)
-- 3. DELETE FROM auth.tokens WHERE account_id IN (betroffene Accounts) — sofortige JWT-Invalidierung
-- 4. Log in audit.data_deletions
```

**Entscheidung noetig von:** Geschaeftsfuehrung / Rechtsabteilung

---

### OPEN-3: IoT Device Tenant-Isolation (HIGH-1)

**Das Problem:**
- IoT-Geraete authentifizieren sich per API-Key + PIN
- Aktuell: ein globaler `OGS_DEVICE_PIN` aus Environment
- Kein Tenant-Context im IoT-Pfad (kein JWT, kein Cookie)
- RFID-Tag von Tenant B an Geraet von Tenant A → Cross-Tenant Check-in

**Optionen:**

| Option | Beschreibung | Aufwand |
|--------|-------------|---------|
| A: Two-Phase Lookup (D20) | Phase 1: Admin-Read → device.tenant_id; Phase 2: WithTenantTx | Mittel |
| B: Device-Header | Geraet sendet `X-Tenant-ID` Header, Backend validiert | Niedrig (aber unsicher) |
| C: Tenant-in-API-Key | API-Key ist tenant-spezifisch, Lookup ergibt automatisch Tenant | Niedrig |

**Empfehlung:** Option A (Two-Phase Lookup) mit eingeschraenktem Admin-Scope:

```go
// Phase 1: Nur iot.devices lesen (KEIN volles BYPASSRLS)
func resolveDeviceTenant(ctx context.Context, db *bun.DB, apiKey string) (int64, error) {
    var device iot.Device
    // Spezielle GRANT: phoenix_auth darf SELECT auf iot.devices(api_key, tenant_id)
    err := db.NewSelect().Model(&device).
        Column("tenant_id").
        Where("api_key = ?", apiKey).
        Scan(ctx)
    return device.TenantID, err
}

// Phase 2: Alle weiteren Queries mit Tenant-Context
func handleCheckin(ctx context.Context, db *bun.DB, tenantID int64, tagID string) error {
    return tenant.WithTenantTx(ctx, db, tenantID, func(ctx context.Context, tx bun.Tx) error {
        student, err := findStudentByTag(ctx, tagID)
        if err != nil {
            return err  // RLS: Student nicht gefunden = nicht an diesem Tenant
        }
        return createVisit(ctx, student.ID)
    })
}
```

**Wichtig:** Phase 1 sollte NICHT `WithAdminTx` nutzen. Stattdessen bekommt `phoenix_auth` ein minimales `GRANT SELECT(api_key, tenant_id) ON iot.devices TO phoenix_auth`.

**In Phase 3 (Dev B, Woche 2) aufnehmen:**
- [ ] `GRANT SELECT(api_key, tenant_id, pin_hash) ON iot.devices TO phoenix_auth`
- [ ] Device-Auth Middleware: `resolveDeviceTenant()` vor Handler
- [ ] Checkin-Workflow: `WithTenantTx(ctx, db, deviceTenantID, ...)`
- [ ] Test: RFID-Tag von Tenant B an Geraet von Tenant A → Fehler
- [ ] Per-Tenant PIN: `iot.devices.pin_hash` statt globalem `OGS_DEVICE_PIN`

**Entscheidung noetig von:** Entwickler-Team (technische Entscheidung)

---

## Teil 4: Zusammenfassung der Aenderungen am Plan

### Sofort (vor Phase 1):

| # | Aenderung | Wo | Aufwand |
|---|-----------|-----|---------|
| 1 | Phase 4.12 VOR Phase 4.9 verschieben | `11-implementierungsplan.md` | 0 |
| 2 | CSP-Headers in Phase 4.1 ergaenzen | `11-implementierungsplan.md` | 0 |
| 3 | Backward-Compatibility Regel fuer Phase 3 | `11-implementierungsplan.md` | 0 |
| 4 | Goroutine-Context-Pattern in Checkliste | `11-implementierungsplan.md` | 0 |
| 5 | IoT UNIQUE Constraint in Phase 2.5 | `11-implementierungsplan.md` | 0 |

### Vor Phase 3:

| # | Aenderung | Wo | Aufwand |
|---|-----------|-----|---------|
| 6 | SSE Hub Design spezifizieren | `03-backend.md` Sektion 10 | ~2h Doku |
| 7 | Scheduler Tenant-Strategie spezifizieren | `03-backend.md` Sektion 11 | ~2h Doku |
| 8 | IoT Two-Phase Lookup spezifizieren | `03-backend.md` Sektion 9 | ~2h Doku |

### Parallel (Business/Legal-Entscheidungen):

| # | Aenderung | Wer | Aufwand |
|---|-----------|-----|---------|
| 9 | OPEN-1: Cross-Tenant Consent | Product Owner + DSB | 1-2 Meetings |
| 10 | OPEN-2: Traeger-Deaktivierung | Geschaeftsfuehrung | 1 Meeting |
| 11 | OPEN-3: IoT Tenant-Isolation | Dev-Team | 1 Meeting |
| 12 | D22: Ferienbetreuung Data Minimization | Dev-Team + DSB | 1 Meeting |

---

## Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-11 | Initiale Version: Cross-Reference 30 Findings vs. neuer Implementierungsplan |
