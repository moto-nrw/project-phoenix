# Devil's Advocate Review: Multi-Tenancy Plan

> Systematische Analyse der Dokumente 00-anforderungen.md, 01-architektur.md, 02-datenbank.md, 03-backend.md, 04-frontend.md, 05-testing.md, 06-offene-punkte.md und DEBATE.md (D1-D21).
>
> 7 Analyse-Durchgaenge: Anforderungs-Abdeckung, Inter-Dokument-Widersprueche, Intra-Dokument-Widersprueche, DEBATE.md-Konsistenz, fehlende Spezifikationen, Architektur-Risiken/Angriffsvektoren, Migrations-Sicherheit.
>
> Stand: 2026-02-10

---

## Zusammenfassung

| Severity | Anzahl | IDs |
|----------|--------|-----|
| CRITICAL | 6 | C-1 bis C-6 |
| HIGH | 9 | H-1 bis H-9 |
| MEDIUM | 9 | M-1 bis M-9 |
| LOW | 5 | L-1 bis L-5 |

**Gesamtbewertung:** Die Architektur-Grundlage ist solide — Shared Schema + RLS, Drei-Rollen-Architektur, Defense-in-Depth mit vier Schichten. Das Plan-Fundament (D7, D8) ist branchenueblich und korrekt. **Aber der Plan ist NICHT implementierungsbereit.** Drei kritische Luecken (C-1, C-4, C-5) muessen vor Implementierungsbeginn geschlossen werden. C-3 (OrgScope) widerspricht der eigenen Defense-in-Depth-Philosophie des Plans.

---

## CRITICAL

### C-1: Transaction-Ownership — Widerspruch zwischen Code-Beispielen

**Dokument:** 03-backend.md §1.3 vs. §8.4

**Problem:** 03-backend.md §1.3 definiert klar: "Transaction-Ownership wandert von Service auf Handler." Der Handler startet `WithTenantTx`, Services arbeiten darin, kein Service startet eigene Transaktionen. Code-Beispiel in §1.3:

```go
// Handler startet Transaktion
err := tenant.WithTenantTx(r.Context(), rs.db, tenantID,
    func(ctx context.Context, tx bun.Tx) error {
        return rs.activeService.CheckIn(ctx, studentID, groupID)
    })
```

Aber §8.4 (Policy Engine) zeigt das exakte Gegenteil — der Service startet die Transaktion:

```go
func (s *ActiveService) GetVisit(ctx context.Context, visitID int64) (*Visit, error) {
    err := tenant.WithTenantTx(ctx, s.db, tenant.FromContext(ctx),
        func(ctx context.Context, tx bun.Tx) error {
            visit, err := s.visitRepo.FindByID(ctx, visitID)
            // ...
        })
}
```

Das ist ein direkter Widerspruch. Wenn der Handler bereits `WithTenantTx` gestartet hat UND der Service nochmals `WithTenantTx` aufruft, entsteht eine **zweite Transaktion auf einer anderen Pool-Connection** — genau das Problem, das §1.3 als Fehlerzustand beschreibt (Zeile 106: "BUN startet eine zweite Transaktion auf einer anderen Pool-Connection — ohne SET LOCAL ROLE").

**Handlungsbedarf:** Entscheiden: Startet der Handler ODER der Service die Transaktion? Alle Code-Beispiele in 03-backend.md vereinheitlichen. Empfehlung: Handler-Pattern wie in §1.3 beschrieben, dann muss §8.4 umgeschrieben werden (Service empfaengt tx via Context, startet keine eigene).

---

### C-2: Keine CI-Pruefung fuer useSWR → useTenantSWR Migration

**Dokument:** 04-frontend.md §10, §16

**Problem:** 821 SWR-Calls muessen von `useSWR` auf `useTenantSWR` migriert werden. Ein einziger vergessener `useSWR`-Call bedeutet: Daten von Tenant A werden im SWR-Cache unter einem Key ohne Tenant-Prefix gespeichert. Bei Tenant-Switch bleibt dieser Cache-Eintrag bestehen und wird bei Tenant B angezeigt — **Cross-Tenant-Datenleck im Frontend**.

§17 definiert eine ESLint-Regel fuer `router.push` mit hardcoded Paths. Aber fuer den deutlich kritischeren `useSWR`-Case fehlt jede CI-Pruefung. 04-frontend.md erwaehnt nur "Alle 821 SWR-Calls muessen `useTenantSWR` nutzen" — keine Durchsetzung.

**Handlungsbedarf:** ESLint-Regel oder Custom-Lint die `useSWR` (ohne Tenant-Prefix) in Frontend-Code blockiert. Beispiel:

```javascript
"no-restricted-imports": ["error", {
    "name": "swr",
    "importNames": ["default"],
    "message": "Use useTenantSWR from @/lib/tenant-swr instead"
}]
```

---

### C-3: OrgScope mit BYPASSRLS widerspricht Defense-in-Depth

**Dokument:** DEBATE.md D18, 03-backend.md §8.4

**Problem:** D18 definiert: OrgScopeService nutzt `WithAdminTx` (BYPASSRLS) + Application-Layer `WHERE organization_id = ?`. Das bedeutet: Fuer Traeger-Buero-Zugriff gibt es **nur eine Sicherheitsschicht** — den `org_id`-Filter im Application-Code.

Der gesamte Rest des Plans definiert vier unabhaengige Sicherheitsschichten (01-architektur.md §3):
1. RLS (DB-Level)
2. Repository WHERE (Application-Level)
3. Policy Engine (Application-Level)
4. RowsAffected (Application-Level)

Beim OrgScope fallen Schicht 1 (RLS — deaktiviert durch BYPASSRLS), Schicht 2 (Repository WHERE — OrgScopeService hat eigene Queries, nicht die standard Repos) und Schicht 4 (RowsAffected — OrgScope ist read-only) komplett weg. Ein einziger Bug im `WHERE organization_id = ?` Filter des OrgScopeService gibt **alle Tenant-Daten aller Organisationen** frei.

D18 listet Guardrails auf: "Dedizierter OrgScopeService", "Eigene Permission", "Nur Read/Aggregate", "Org-Filter als Pflicht", "Audit-Logging". Aber keiner dieser Guardrails ist eine DB-Level-Sicherung. Das widerspricht der eigenen Architektur-Philosophie: "Application-only Filtering: Kein DB-Level-Schutz, ein vergessener Filter = Datenleck" (01-architektur.md §1, verworfene Alternative).

**Handlungsbedarf:** Mindestens einen DB-Level-Schutz hinzufuegen. Optionen:
- (a) PostgreSQL-Funktion `current_setting('app.current_org_id')` + DB-Check-Constraint
- (b) Separate RLS-Policy fuer OrgScope: `tenant_id IN (SELECT id FROM platform.schools WHERE organization_id = current_setting('app.current_org_id')::bigint)`
- (c) Akzeptiertes Risiko explizit dokumentieren (warum Defense-in-Depth hier nicht gilt)

---

### C-4: Tenant-Loeschung komplett unspezifiziert

**Dokument:** 00-anforderungen.md §5 fordert: "Ein Traeger kann verlangen, dass alle Daten einer OGS geloescht werden. Dies muss vollstaendig und nachweisbar sein (Audit-Log)."

**Problem:** Kein einziges Dokument (01-05, DEBATE D1-D21) spezifiziert, wie ein kompletter Tenant geloescht wird. D21 behandelt nur die Loeschung einzelner Accounts (Art. 17 fuer Shared Accounts), nicht die Loeschung eines gesamten Tenants.

Fehlende Spezifikation:
- Welche Tabellen muessen in welcher Reihenfolge geleert werden? (FK-Abhaengigkeiten!)
- Was passiert mit Accounts die nur bei diesem Tenant aktiv waren? (Grace Period? Sofort loeschen?)
- Was passiert mit Accounts die bei mehreren Tenants aktiv sind? (Nur `account_tenants` deaktivieren?)
- Was passiert mit `platform.cross_tenant_access` Records die diesen Tenant referenzieren?
- Wie wird sichergestellt, dass keine Daten in Audit-Tabellen verbleiben? (Art. 17 vs. Aufbewahrungspflicht)
- Wer darf die Loeschung ausloesen? (Operator? Traeger-Admin?)
- Gibt es eine Cooling-Off-Period?
- Was passiert mit der Subdomain nach Loeschung? (Sofort frei? Gesperrt?)
- Was passiert mit Avatar-Dateien? (03-backend.md §14 erwaehnt `os.RemoveAll`, aber kein orchestrierter Prozess)

**Handlungsbedarf:** Eigene Sektion in 03-backend.md oder eigenes Dokument mit vollstaendiger Loeschungs-Orchestrierung, FK-Reihenfolge, Account-Handling und Audit-Trail.

---

### C-5: Migrations-Luecke — Neue Inserts zwischen Schritt 4 und 11

**Dokument:** 02-datenbank.md §8.1 (Migrationsstrategie)

**Problem:** Die 13-Schritt-Migrationsstrategie hat eine kritische Luecke:

```
Schritt  4: tenant_id (NULLABLE) zu allen Tabellen hinzufuegen
Schritt  5: UPDATE SET tenant_id = 1 fuer bestehende Rows
Schritt  6: tenant_id NOT NULL Constraint setzen
...
Schritt 11: Code-Deploy mit Tenant-Middleware + WithTenantTx/WithAdminTx
```

Zwischen Schritt 6 (NOT NULL erzwungen) und Schritt 11 (neuer Code deployed) laeuft der **alte Code** — ohne `SetTenantID()`, ohne `WithTenantTx`. Jeder INSERT des alten Codes schlaegt fehl, weil `tenant_id NOT NULL` gesetzt ist, aber der alte Code keinen Wert liefert.

Die Migration behauptet "Zero-Downtime", aber zwischen Schritt 6 und 11 ist die Anwendung effektiv kaputt.

**Handlungsbedarf:** Entweder:
- (a) `DEFAULT 1` auf `tenant_id` Spalte setzen (ALTER COLUMN SET DEFAULT 1), damit alter Code weiterhin funktioniert. Default nach Code-Deploy entfernen.
- (b) Schritt 6 (NOT NULL) NACH Schritt 11 (Code-Deploy) verschieben. Alte Rows ohne tenant_id werden dann vom neuen Code mit Default-Tenant befuellt.
- (c) Schritt 6 und 11 im selben Deployment-Fenster ausfuehren (kein Zero-Downtime, aber konsistent).

---

### C-6: D12 vs. 02-datenbank.md Widerspruch bei auth.tokens

**Dokument:** DEBATE.md D12 vs. 02-datenbank.md §2.1

**Problem:** D12 entscheidet explizit: "Option B (DB-Spalte) ueberfluessig: Der DB-Lookup auf `auth.tokens` passiert bereits beim Refresh. Die `tenant_id` kommt aus den RefreshClaims — kein Schema-Change auf `auth.tokens` noetig."

Aber 02-datenbank.md §2.1 listet `auth.tokens` unter "MIT tenant_id NOT NULL" und erklaert sogar den Zweck: "Die DB-Spalte `tenant_id` ermoeglicht gezieltes Revoken aller Tokens fuer einen bestimmten Tenant (z.B. bei Zugriffsentzug)."

Das ist ein direkter Widerspruch. Entweder bekommt `auth.tokens` eine `tenant_id` Spalte (02-datenbank.md) oder nicht (D12).

**Handlungsbedarf:** Entscheidung treffen und beide Dokumente synchronisieren. Empfehlung: `tenant_id` auf `auth.tokens` beibehalten (02-datenbank.md), weil Massen-Revoke bei Tenant-Deaktivierung ein realer Use Case ist. D12-Text korrigieren.

---

## HIGH

### H-1: Provisioning-Spec fehlt komplett

**Dokument:** 00-anforderungen.md §4.5

**Problem:** Die Anforderungen definieren klar: Operator erstellt neue OGS, System erstellt automatisch Subdomain, Default-Rollen, Default-Settings, Admin-Account. Aber kein technisches Dokument spezifiziert den Provisioning-Service — keinen Endpoint, keinen Service-Flow, keine Transaktions-Reihenfolge, keine Fehlerbehandlung.

03-backend.md beschreibt nur den Login-Flow und bestehendes Tenant-Switching. Es gibt keinen `ProvisioningService` oder `POST /operator/schools` Endpoint.

**Handlungsbedarf:** Provisioning-Flow spezifizieren: Endpoint, Transaktions-Reihenfolge (Organization → School → Default Roles → Admin Account → account_tenants), Fehler-Rollback, idempotente Retry-Moeglichkeit.

---

### H-2: CREATE INDEX CONCURRENTLY innerhalb von BUN-Transaktionen

**Dokument:** 02-datenbank.md §2.5.3, §8.1 Schritt 7

**Problem:** §2.5.3 zeigt:
```sql
CREATE UNIQUE INDEX CONCURRENTLY idx_students_tenant_pk
    ON users.students(tenant_id, id);
```

06-offene-punkte.md erwaehnt: "`CREATE INDEX CONCURRENTLY` muss ausserhalb von BUN-Transaktionen laufen." Aber die Migrationsstrategie (§8.1 Schritt 7) sagt nur "Indexes erstellen (CONCURRENTLY)" ohne zu erklaeren WIE dies ausserhalb der BUN-Migrations-Transaktionen passiert.

PostgreSQL verbietet `CREATE INDEX CONCURRENTLY` innerhalb einer Transaktion. BUN-Migrations laufen standardmaessig in Transaktionen. Ohne explizite Anleitung wird der Entwickler den Index innerhalb der Migration-Transaktion erstellen — was mit einem Fehler abbricht oder (schlimmer) ein regulaeres `CREATE INDEX` mit Table Lock ausfuehrt.

**Handlungsbedarf:** In 02-datenbank.md dokumentieren: CONCURRENTLY-Indexes muessen in separaten Go-Dateien laufen die `db.Exec()` direkt aufrufen, nicht `db.RunInTx()`. Oder BUNs `init()` mit `tx.Exec()` umgehen.

---

### H-3: UNIQUE/FK-Timing in der Migrationsstrategie unklar

**Dokument:** 02-datenbank.md §8.1 vs. §2.4

**Problem:** Die Migrationsstrategie (§8.1) hat 13 Schritte, aber UNIQUE-Constraint-Migration (§2.4 — 31 Constraints) und Composite-FK-Migration (§2.5 — 64 FKs) sind keinem Schritt zugeordnet. Diese muessen zwischen Schritt 5 (Daten-Backfill) und Schritt 7 (Indexes) passieren, aber die Reihenfolge ist kritisch:

1. Alte UNIQUE droppen
2. Neue Composite UNIQUE erstellen
3. Alte FK droppen
4. Neue Composite FK erstellen

Wenn die Reihenfolge falsch ist (z.B. FK vor UNIQUE), schlaegt die FK-Erstellung fehl weil der referenzierte UNIQUE-Index nicht existiert.

**Handlungsbedarf:** Expliziten Schritt "6a: UNIQUE-Constraints migrieren" und "6b: Composite FKs migrieren" in §8.1 einfuegen. Reihenfolge: Ziel-UNIQUE(tenant_id, id) zuerst, dann FKs.

---

### H-4: Rollback-Plan unvollstaendig

**Dokument:** 02-datenbank.md §8.2

**Problem:** Der Rollback-Plan behandelt nur zwei Szenarien: "Nach Phase 1" und "Nach Phase 2". Aber die Migration hat 13 Schritte und 3 RLS-Phasen. Was passiert bei Rollback nach:
- Schritt 3 (PostgreSQL-Rollen erstellt)? Rollen bleiben — aber Connection-String muss auch zurueckgesetzt werden
- Schritt 7 (Indexes erstellt)? 31 neue UNIQUE-Constraints und 64 neue FKs muessen zurueck
- Schritt 9 (RLS-Policies erstellt)? Policies muessen entfernt werden (nicht nur auf USING(true) gesetzt)
- Schritt 13 (Strict RLS)? Zurueck zu Permissive oder komplett entfernen?

Der Rollback-Plan schweigt zu UNIQUE-Constraints, Composite FKs, und PostgreSQL-Rollen.

**Handlungsbedarf:** Pro Schritt einen Rollback-Befehl dokumentieren. Mindestens: Rollen-Rollback, UNIQUE-Rollback, FK-Rollback, RLS-Rollback.

---

### H-5: Cross-Tenant Enrollment-Autorisierung unklar

**Dokument:** 03-backend.md §6.4, DEBATE.md D4

**Problem:** D4 beschreibt: "Admin erstellt Feriengruppe an Host-OGS, enrollt Kinder aus anderen OGS via `platform.cross_tenant_access`." Aber: Welcher Admin? Der Admin der Host-OGS? Wie bekommt er Zugriff auf die Kinder-IDs der Quell-OGS, um sie einschreiben zu koennen?

Der Admin der Host-OGS arbeitet mit `WithTenantTx(hostTenantID)` — RLS blockiert alle Daten anderer Tenants. Um Kinder aus der Quell-OGS zu finden, braeuchte er `WithAdminTx` (BYPASSRLS). Aber welche Autorisierungs-Pruefung stellt sicher, dass er nur Kinder der richtigen Quell-OGS zugreifen kann?

Die `platform.cross_tenant_access` Tabelle registriert den Grant, aber der Workflow VOR dem Grant (wie findet der Admin die Kinder?) ist nicht spezifiziert.

**Handlungsbedarf:** Enrollment-Workflow Schritt fuer Schritt spezifizieren: Wer initiiert, welche Autorisierung, wie werden Quell-Kinder gefunden (suche per Name? Per Liste vom Quell-Admin?), welche Daten werden kopiert vs. referenziert.

---

### H-6: Announcements Per-OGS-Targeting fehlt im Schema

**Dokument:** 00-anforderungen.md §3.3 vs. 02-datenbank.md §1

**Problem:** §3.3 fordert: Operator "kann Announcements / Release Notes an alle OGS oder gezielt an bestimmte OGS senden." Das bestehende `platform.announcements` Schema (aus den existierenden Migrations) hat nur `target_roles` (rollenbasiertes Targeting). Es fehlt:
- `target_tenant_ids BIGINT[]` oder eine Junction-Tabelle `platform.announcement_tenants`
- Logik fuer "alle OGS" vs. "bestimmte OGS"

02-datenbank.md listet die neuen Tabellen (§1), aber keine Erweiterung von `platform.announcements`. Kein Dokument adressiert diesen Gap.

**Handlungsbedarf:** Schema-Erweiterung fuer Announcements: Entweder `target_tenant_ids` Array oder Junction-Tabelle. Dokumentieren ob "alle OGS" bedeutet: kein Filter (Broadcast) oder explizite Liste aller aktuellen Tenant-IDs.

---

### H-7: Subdomain-Takeover-Monitoring fehlt

**Dokument:** 04-frontend.md §13

**Problem:** §13.3 erwaehnt "Subdomain-Takeover-Monitoring als operationale Massnahme" als Mitigation fuer das Wildcard-Cookie-Risiko. Aber nirgendwo wird definiert:
- Was monitored wird
- Wie Monitoring implementiert wird
- Wer alertiert wird
- Was der Response-Plan ist

Wenn eine Subdomain (z.B. `abandoned-school.moto-app.de`) nicht mehr aktiv genutzt wird aber der DNS-Wildcard-Eintrag weiter existiert, koennte ein Angreifer diese Subdomain uebernehmen, XSS ausfuehren, und den Wildcard-Cookie (`.moto-app.de`) stehlen.

**Handlungsbedarf:** Entweder operationales Monitoring-Konzept erstellen ODER Subdomain-Deaktivierung implementieren: deaktivierte Tenants werden in der Middleware explizit blockiert (nicht nur `notFound()`, sondern auch Cookie-Clearing).

---

### H-8: Kein Konzept fuer inkrementelles Deployment

**Dokument:** 02-datenbank.md §8.1

**Problem:** Die Migrationsstrategie geht davon aus, dass der Code-Deploy (Schritt 11) atomar passiert — der GESAMTE neue Code geht gleichzeitig live. Bei einem System mit 100+ API-Endpoints, 54 Repositories, 29 Services, und 194 Frontend-Handlern ist ein Big-Bang-Deploy riskant.

Was passiert wenn der Deploy teilweise fehlschlaegt? Alte Pods mit altem Code laufen neben neuen Pods mit neuem Code. Alte Pods nutzen kein `WithTenantTx` → `phoenix_auth` (NOINHERIT) → Permission Denied fuer alle Requests auf alten Pods.

**Handlungsbedarf:** Feature-Flag-Strategie: Code kann `WithTenantTx` oder direkten DB-Zugriff nutzen (via Flag). Rollout schrittweise: erst lesen, dann schreiben. Oder: Backward-compatible Code der sowohl mit als auch ohne Transaktion funktioniert (`getDB(ctx)` Fallback auf `r.DB` mit temporaer aktiver `phoenix_auth` Berechtigung).

---

### H-9: account_roles in falscher Kategorie (02-datenbank.md)

**Dokument:** 02-datenbank.md §2.1

**Problem:** `auth.account_roles` steht in der Tabelle "MIT tenant_id NULLABLE" (direkt unter `auth.roles`), aber der Spaltentyp ist `BIGINT NOT NULL`. Die Tabelle gehoert in die Kategorie "MIT tenant_id NOT NULL" (58 Tabellen). Gleiches gilt fuer `auth.account_permissions`.

Dies ist nicht nur ein kosmetisches Problem: Die Zaehlung "58 NOT NULL, 1 nullable" stimmt nur wenn `account_roles` und `account_permissions` als NOT NULL gezaehlt werden. Wenn ein Entwickler die Tabelle in der "NULLABLE"-Sektion sieht, koennte er annehmen, dass NULL erlaubt ist.

**Handlungsbedarf:** `auth.account_roles` und `auth.account_permissions` in die NOT-NULL-Sektion verschieben. Nur `auth.roles` bleibt in der NULLABLE-Sektion.

---

## MEDIUM

### M-1: Cross-Tenant-Audit unvollstaendig spezifiziert

**Dokument:** 00-anforderungen.md §5 vs. 03-backend.md

**Problem:** §5 fordert: "Alle Cross-Tenant-Zugriffe muessen protokolliert werden. Wer hat wann auf welche OGS zugegriffen?" Aber die Audit-Spezifikation ist duenn:
- D18 (OrgScope) erwaehnt `s.logger.Info("org_scope_access", ...)` — das ist Logging, kein Audit-Trail
- D4 (Ferienbetreuung) erwaehnt `cross_tenant_access`-Tabelle als Grant-Record, aber kein Access-Logging
- 05-testing.md §7 PR-Checkliste: "Werden Audit-Logs mit der richtigen `tenant_id` geschrieben?" — aber wo?

Kein Dokument definiert eine `audit.cross_tenant_access_log` Tabelle oder einen strukturierten Audit-Trail fuer Cross-Tenant-Zugriffe.

**Handlungsbedarf:** Cross-Tenant-Audit-Tabelle definieren (wer, wann, welcher Quell-Tenant, welcher Ziel-Tenant, welche Aktion, welche Entity). In OrgScopeService und CrossTenantService integrieren.

---

### M-2: Eltern OGS-Switcher — technisches Design fehlt

**Dokument:** 00-anforderungen.md §3.4, §4.3

**Problem:** Eltern koennen Kinder bei verschiedenen OGS und sogar verschiedenen Traegern haben. Beim Login auf `altenberge.moto-app.de` soll ein "OGS-Switcher" verfuegbar sein. Aber:
- Wie wird die Liste der verfuegbaren Tenants fuer Eltern befuellt? (Bei Betreuern: `account_tenants`. Bei Eltern: automatisch via Kind-Einschreibung)
- Der Login passiert bei einer Subdomain (z.B. Altenberge). Wenn das Kind auch bei Greven (anderer Traeger!) ist — wie switched der Elternteil zu einer anderen Subdomain?
- 03-backend.md §6.4 beschreibt Auto-Provisioning fuer `account_tenants` bei Enrollment. Aber der Switch-Endpoint (§6.2) prueft nur `account_tenants`. Bei traeger-uebergreifendem Switch: Ist der `scope` leer? Kann ein Elternteil zum anderen Traeger wechseln?

**Handlungsbedarf:** Eltern-Switcher-Flow End-to-End dokumentieren, insbesondere den traeger-uebergreifenden Fall.

---

### M-3: settings JSONB ohne Validierung

**Dokument:** 02-datenbank.md §1.2 (`platform.schools.settings JSONB`)

**Problem:** `settings JSONB DEFAULT '{}'` ist beliebig erweiterbar. 04-frontend.md §4.1 definiert `TenantSettings` mit `logoUrl`, `primaryColor`, `[key: string]: unknown`. Aber:
- Keine Backend-Validierung des JSONB-Inhalts
- Kein Schema fuer gueltige Keys/Value-Types
- Ein Operator koennte beliebigen Content in `settings` schreiben
- XSS-Risiko wenn `logoUrl` oder `primaryColor` ungefiltert im Frontend gerendert werden

**Handlungsbedarf:** Backend-Validierung fuer `settings` JSONB: erlaubte Keys, Value-Types, URL-Sanitisierung fuer `logoUrl`, CSS-Value-Sanitisierung fuer `primaryColor`.

---

### M-4: Performance-Benchmarks nicht definiert

**Dokument:** 01-architektur.md §11, 05-testing.md §8

**Problem:** 01-architektur.md nennt "Benchmark bei 100 Tenants" als Mitigation fuer Performance-Regression. 05-testing.md §8.4 erwaehnt "Performance-Baseline-Test: < 10% Overhead". Aber:
- Kein konkreter Benchmark-Plan (welche Queries? Welche Metriken? Welche Baseline?)
- "< 10% Overhead" — gegenueber was? Aktuellem Single-Tenant? Mit welcher Datenmenge?
- Kein Performance-Budget fuer RLS-Policies

**Handlungsbedarf:** Konkreten Benchmark-Plan mit definierten Queries, Metriken (p50/p95/p99 Latency), Datenmenge (100 Tenants x 50 Students x 200 Visits), und akzeptablen Schwellenwerten.

---

### M-5: Doppelte Sektion 6 in 05-testing.md

**Dokument:** 05-testing.md

**Problem:** Es gibt zwei Sektionen mit der Nummer 6:
- §6 "RowsAffected-Tests (D16)" (Zeile 211)
- §6 "Advisory Lock Tenant-Isolation (D16)" (Zeile 236)

Die zweite sollte §7 sein. Die Nummerierung aller folgenden Sektionen ist verschoben.

**Handlungsbedarf:** Nummerierung korrigieren.

---

### M-6: router.push vs. window.location.href Inkonsistenz

**Dokument:** 04-frontend.md §9 vs. §16.3

**Problem:** §9 (Tenant-Switch) zeigt:
```typescript
router.push(`https://${slug}.${TENANT_DOMAIN}/dashboard`);
```

§16.3 (Cache-Isolation) zeigt:
```typescript
window.location.href = `https://${slug}.${TENANT_DOMAIN}/dashboard`;
```

§16.3 erklaert sogar explizit: "Warum `window.location.href` statt `router.push`: Hard-Navigation zu einer anderen Subdomain ist ein Origin-Wechsel." Dann ist §9 falsch — `router.push` macht eine Soft-Navigation die den JS-Context behaelt, stale Cache inklusive.

**Handlungsbedarf:** §9 auf `window.location.href` aendern und §9/§16.3/§18 vereinheitlichen.

---

### M-7: Origin-Header kann fehlen oder gespooft werden

**Dokument:** 04-frontend.md §14

**Problem:** §14 implementiert Slug-Origin-Validierung: Backend vergleicht `tenant_slug` aus Body mit Subdomain aus `Origin`-Header. Fallback: "Wenn `Origin` fehlt (alte Browser, Proxy), Login trotzdem erlauben."

Aber: Ein Angreifer kann den `Origin`-Header in einem Server-to-Server-Request beliebig setzen. Die Validierung schuetzt nur gegen Browser-basierte Angriffe (CSRF). Der Fallback ("trotzdem erlauben") macht die gesamte Validierung optional.

**Handlungsbedarf:** Klarstellen dass diese Validierung UX-Schutz ist (verhindert versehentliches Einloggen am falschen Tenant), kein Security-Feature gegen gezielte Angriffe. Die echte Security kommt aus dem JWT: der Token enthaelt den tenant_id, und der Backend-Code arbeitet nur mit diesem Tenant.

---

### M-8: Enrollment-Atomizitaet bei Eltern Auto-Provisioning

**Dokument:** 03-backend.md §6.4

**Problem:** §6.4 beschreibt: Kind wird eingeschrieben → `account_tenants` fuer alle Erziehungsberechtigten erstellen. Kind verlaesst OGS → pruefen ob noch andere Kinder, falls nein: deaktivieren. Aber:
- Was wenn die Erziehungsberechtigten noch keinen Account haben? (Guardian-Invitation-Flow)
- Was wenn das Kind in einer Transaktion eingeschrieben wird, aber die `account_tenants`-Erstellung fehlschlaegt?
- "Nutzt `WithAdminTx` fuer Cross-Tenant-Zugriff auf Erziehungsberechtigte" — aber warum Cross-Tenant? Guardian-Profiles sind im gleichen Tenant wie das Kind

**Handlungsbedarf:** Edge-Cases dokumentieren: Account existiert noch nicht (Invitation pending), Race Condition bei gleichzeitigem Enrollment/Disenrollment, Fehlerbehandlung bei teilweisem Failure.

---

### M-9: Phase 2 (Logging bei Violations) ohne Monitoring-Tooling

**Dokument:** 02-datenbank.md §7

**Problem:** Phase 2 der RLS-Migration: "Violations werden geloggt (nicht blockiert). Ziel: 100% der Requests haben korrekten Transaktions-Context." Aber:
- Wie werden Violations geloggt? (PostgreSQL-Log? Application-Log? Metric?)
- Wie wird "100%" verifiziert? (Dashboard? Alerting?)
- Wie lange dauert Phase 2 bevor man auf Phase 3 wechselt?

Ohne Monitoring-Tooling wird Phase 2 zu "wir hoffen dass es funktioniert" statt "wir messen dass es funktioniert".

**Handlungsbedarf:** Monitoring-Anforderungen fuer Phase 2: Log-Aggregation, Dashboard mit Violation-Counter, Alert-Threshold (z.B. > 0 Violations = Phase 3 blockiert).

---

## LOW

### L-1: D12-Text erwaehnt veralteten Kontext

**Dokument:** DEBATE.md D12

**Problem:** D12 sagt: "Option B (DB-Spalte) ueberfluessig." Aber 02-datenbank.md fuegt trotzdem `tenant_id` zu `auth.tokens` hinzu (siehe C-6). Der D12-Text sollte aktualisiert werden, falls die Entscheidung ist, `tenant_id` auf `auth.tokens` beizubehalten.

**Handlungsbedarf:** D12-Text synchronisieren (abhaengig von C-6-Entscheidung).

---

### L-2: Aufwand-Schaetzung in 01-architektur.md veraltet

**Dokument:** 01-architektur.md §10

**Problem:** §10 schaetzt "~350 Dateien, ~14-18 Wochen". Seit der Erstellung wurden massive Ergaenzungen gemacht: 64 Composite FKs, 31 UNIQUE-Migrationen, Transaction-Ownership-Migration (161 Code-Stellen), 821 SWR-Calls, OrgScopeService, ParentService, Provisioning-Flow. Die tatsaechliche Komplexitaet uebersteigt die Schaetzung deutlich.

**Handlungsbedarf:** Schaetzung aktualisieren oder als "initialer Richtwert" markieren.

---

### L-3: Operator-OGS-Auswahl ohne UI-Spec

**Dokument:** 00-anforderungen.md §3.3

**Problem:** "Operator kann im Operator-Dashboard eine bestimmte OGS auswaehlen und deren Daten einsehen." Kein Frontend-Dokument beschreibt die UI fuer diese Auswahl (Dropdown? Suchfeld? Dashboard mit Karten?). 04-frontend.md behandelt nur Tenant-Subdomains und Tenant-Switch fuer regulaere User.

**Handlungsbedarf:** Minimale UI-Spec fuer Operator-Dashboard OGS-Auswahl.

---

### L-4: cross_tenant_access ohne automatischen Cleanup

**Dokument:** 02-datenbank.md §1.4

**Problem:** `platform.cross_tenant_access` hat `valid_until TIMESTAMPTZ NOT NULL` und `active BOOLEAN DEFAULT true`. Aber kein Scheduler-Job deaktiviert abgelaufene Grants automatisch. Die Ferienbetreuung endet am 12.08., aber der `active=true` Eintrag bleibt bestehen bis jemand manuell eingreift.

D4 sagt: "Nach Ablauf faellt der Zugriff automatisch weg." Aber der Access-Check-Code (03-backend.md §6.3) ist nicht spezifiziert — prueft er `valid_until`?

**Handlungsbedarf:** (a) Access-Check-Query muss `WHERE active = true AND valid_until > NOW()` enthalten, und/oder (b) Cleanup-Job in 03-backend.md §11 ergaenzen der abgelaufene Grants deaktiviert.

---

### L-5: meta.migration_metadata Zaehlung

**Dokument:** 02-datenbank.md §2.1

**Problem:** Die Zaehlung erwaehnt `meta.migration_metadata` als "Infrastruktur-Tabelle, kein tenant_id" unter den 11 Tabellen ohne tenant_id. Aber `meta.migration_metadata` ist keine regulaere Application-Tabelle — sie wird nur von BUN-Migrations intern genutzt. Die Zaehlung "70 bestehende Tabellen" inkludiert diese interne Tabelle, was die Zahl leicht aufblaeht.

**Handlungsbedarf:** Optional: Fussnote dass `meta.migration_metadata` eine BUN-interne Tabelle ist und nicht zum Application-Schema gehoert.

---

## Gesamtbewertung

### Staerken des Plans

1. **Drei-Rollen-Architektur (D7/D8)** — Branchenueblich, fail-closed, kein Magic-Value-Bypass
2. **Defense-in-Depth** — Vier unabhaengige Schichten (mit Ausnahme OrgScope, siehe C-3)
3. **DEBATE.md** — 21 fundiert diskutierte Entscheidungen mit Quellen und verworfenen Alternativen
4. **Vollstaendige Codebase-Analyse** — 70 Tabellen, 54 Repos, 821 SWR-Calls inventarisiert
5. **Per-Tenant RBAC (D13 rev)** — Korrektur einer urspruenglich schwachen Entscheidung zeigt Lernfaehigkeit

### Kritische Luecken vor Implementierung

| # | Finding | Aufwand | Blockiert |
|---|---------|---------|-----------|
| C-1 | Transaction-Ownership Widerspruch | 1h (Doku-Fix) | Alle Service-Implementierungen |
| C-4 | Tenant-Loeschung unspezifiziert | 1-2 Tage (Spec) | GDPR-Compliance |
| C-5 | Migrations-Luecke Schritt 4-11 | 1h (Doku-Fix + DEFAULT 1) | Production-Migration |
| C-6 | auth.tokens Widerspruch | 30min (Doku-Fix) | JWT/Token-Implementierung |

### Empfohlene Reihenfolge

1. **SOFORT:** C-1, C-5, C-6 fixen (reine Dokumentations-Korrekturen)
2. **VOR Implementierung:** C-3 entscheiden (akzeptiertes Risiko oder DB-Level-Schutz), C-4 spezifizieren
3. **PARALLEL zur Implementierung:** H-1 bis H-9 adressieren
4. **WAEHREND Implementierung:** M-1 bis M-9 als Cleanup-Tasks

---

## Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-10 | Initiale Devil's Advocate Review ueber alle 8 Dokumente |
