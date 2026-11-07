---
status: proposed
---

# Gleiche Transaktionsrahmen werden gebündelt, Admin geht nur einseitig in den Tenant

Status: Vorgeschlagen. Vorschlag vom 07.11.2026 (Entscheidung E6) zu
[#5881](https://github.com/moto-nrw/project-phoenix/issues/5881),
[#5795](https://github.com/moto-nrw/project-phoenix/issues/5795),
[#5878](https://github.com/moto-nrw/project-phoenix/issues/5878),
[#5894](https://github.com/moto-nrw/project-phoenix/issues/5894) und
[#5862](https://github.com/moto-nrw/project-phoenix/issues/5862).
Umgesetzt wird erst nach Freigabe. Der Vorschlag ändert keinen Code und
kein Register. Er legt fest, wann mehrere Transaktionsrahmen eines Ablaufs
zu einem werden dürfen und welche Registereinträge dafür entstehen.

## Kontext

Ein Transaktionsrahmen ist hier eine Transaktion der Unit of Work
(`modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go`):

- **Admin-Rahmen:** `BEGIN`, `SET LOCAL ROLE phoenix_admin`, Abfragen,
  `COMMIT` (`withinGlobal`, Zeile 154–174). RLS gilt nicht.
- **Tenant-Rahmen:** `BEGIN`, `SELECT set_config('role', 'phoenix_tenant', true),
  set_config('app.current_tenant_id', …, true)`, Abfragen, `COMMIT`
  (`WithinTenant`, Zeile 71–103). RLS gilt für genau diese Schule.
- **Eltern-Scope:** Eltern haben keine Schule in der Sitzung. Ihre Abläufe
  finden Kinder und Rechte in einem Admin-Rahmen und lesen dann im
  Tenant-Rahmen der Schule des Kindes.

Ein ambienter Rahmen wird nur bei gleichem Scope mitbenutzt: Tenant nur bei
gleicher Schule (Zeile 79–84), Admin nur in Admin (Zeile 158–162). Ein Wechsel
zwischen Admin und Tenant braucht heute immer einen neuen Rahmen. Jeder Rahmen
kostet drei Statements Verwaltung (BEGIN, Rolle, COMMIT) und eine Verbindung
aus dem Pool.

`TestTransactionBoundaryRatchet` (`backend/test/transaction_boundaries_ratchet_test.go:27`)
zählt jede Admin-Startstelle und jede Startstelle außerhalb von Roots und
Workflows mit ihrem genauen Schlüssel `Datei:Funktion:Aufruf#n`. Das Register
`backend/test/transaction_boundaries.json` darf nur schrumpfen
(`backend/test/transaction_register.go:19`). Neue Stellen sind nur über eine
benannte, exakte Ausnahmeliste mit ADR erlaubt, etwa
`backend/test/parent_document_inbox_transaction.go` (ADR 0126). Erkannt
werden nur die Operationsnamen in `transactionOperation`
(`transaction_boundaries_ratchet_test.go:167`).

### Messung

Gemessen am 07.11.2026 auf `development` @ `09ac66d70d` mit
`testpkg.CaptureQueries` und `PHX_QUERY_DUMP=1`, gegen einen eigenen
Test-Postgres. Für #5881 (unbekannter Token) und #5878 liefen temporäre
Mess-Tests, die nicht eingecheckt sind. Die Spalte „erwartet“ ist aus dem
gemessenen Dump abgeleitet: Es entfallen genau die Verwaltungsstatements der
zusammengelegten Rahmen, alle anderen Statements bleiben in Reihenfolge.

| Issue | Szenario | Rahmen heute (Statements) | heute | erwartet |
|---|---|---|---|---|
| #5881 | `api.calendar.public_feed.staff` (N3, N8) | Admin 4 (Eltern-Token), Admin 5 (Personal-Token, Konto), Tenant 17 | 26 | 23 |
| #5881 | unbekannter Token, 404 | Admin 4, Admin 4 (endet mit ROLLBACK) | 8 | 5 |
| #5795 | 19 Szenarien `operator.lists.*`, je N3 und N8 (38 Anfragen) | je Anfrage Admin 4 (Operator), Admin 4 (Sitzungsfamilie), dann die Route | 678 | 564 |
| #5795 | einzeln, z. B. `operator.lists.roles` | Admin 4, Admin 4, Admin 4 | 12 | 9 |
| #5878 | Logout, eine Familie, eine Schule | Revocation 7, Push Admin 4 (Familie), Push Admin 4 (Schule × Portal), Audit 4 | 19 | 16 |
| #5878 | `role_changed`, drei Familien | Tenant 7, Push Admin 4 × 4 | 23 | 14 |
| #5878 | `role_changed`, fünf Familien (Sitzungsgrenze) | Tenant 9, Push Admin 4 × 6 | 33 | 18 |
| #5894 | `communication18.attachments.parent.n3`, `.n5` | Admin 6 (Zielgruppe), Tenant 5 (Schalter), Tenant 4 (Dateizeilen), Admin 6 (erneute Prüfung) | 21 | 18, mit Schritt 5: 16 |
| #5862 | 14 Szenarien `parent.children.*` | je Admin 7 (Schulen, Beziehung, Kind), dann Tenant 4–14 | 205 | 177 |

Die 14 Szenarien von #5862: `related_accounts` (N3, N8), `master_data`,
`master_data_requests`, `sharing_options`, `sharing`, `events`, `consents`,
`care_exception`, `excused_requests`, `pickup_requests`, `care_offerings`,
`offering_catalog`, `meal_participation`. Heute 12, 17, 13, 14, 17, 12, 11,
13, 14, 13, 21, 17, 19, 12 Statements, erwartet je zwei weniger.

### Befunde am Code

- **#5881:** `PublicFeed` ruft erst `ParentCalendarFeedByToken`, bei
  `ErrNotFound` dann `StaffCalendarFeedByToken`
  (`modules/schoolcalendar/http/public_feed.go:33–36`). Der Eltern-Token öffnet
  über `FindParentCalendarFeedOwner` (`RunPlatform`) einen eigenen Admin-Rahmen
  (`modules/schoolcalendar/portal/internal/application/feed.go:519–541`), der
  Personal-Token einen zweiten in `staffCalendarOwnerByToken`
  (`caldav.go:140–158`). `RunPlatform` und die Kontolesung treten einem
  ambienten Admin-Rahmen bei
  (`modules/identityaccess/internal/application/command_transactions.go:47–56`).
  Der Befund gilt.
- **#5795:** `validateActiveOperatorSession`
  (`modules/identityaccess/inbound/operator/middleware.go:69–87`) ruft
  `FindOperator` (`operator.go:42`, `s.tx.RunAdmin`) und
  `LatestOperatorSessionInFamily` (`operator.go:297`, `s.tx.RunAdmin`) einzeln.
  Beide treten einem ambienten Rahmen bei (`command_transactions.go:58–67`).
  Der Befund gilt; heute 678 statt der im Issue genannten 680 Statements.
- **#5878:** Der Push-Cleanup läuft nach dem Commit ohne ambiente Transaktion
  (`session_revocation.go:172–189`). `deletePushForFamilies` (Zeile 404) und
  `deletePushUnboundAtTenants` (Zeile 479) öffnen je Familie und je Schule ×
  Portal über `withPushAdminTx` (Zeile 495) einen Admin-Rahmen und brechen beim
  ersten Fehler ab. Es gibt keinen Wiederholungsversuch: Der Fehler wird nur
  protokolliert (Zeile 174–179). Der Befund gilt.
- **#5894:** `GuardianAnnouncementTenant` prüft die Zielgruppe im Admin-Rahmen
  und danach den Schulschalter über `ResolveBoolForTenant` in einem eigenen
  Tenant-Rahmen (`workflows/parentportal/internal/messaging/parent_announcement_attachments.go:30–67`,
  `parent_announcement_service.go:343–362`). File Storage liest die Zeilen in
  einem weiteren Tenant-Rahmen (`modules/filestorage/internal/application/attachments.go:301–321`).
  Die erneute Prüfung nach dem Lesen (`announcement_attachment_entry.go:54–69`,
  `93–106`) liest den Schalter aus dem Einstellungs-Cache der Anfrage und
  öffnet nur den Admin-Rahmen. Der Befund gilt.
- **#5862:** `ResolvePermittedChild` prüft Beziehung und `parent_portal.*`-Recht
  im Admin-Rahmen (`workflows/parentportal/internal/care/child.go:52–84`); die
  Route öffnet danach ihren Tenant-Rahmen. 39 Aufrufer. Der Befund gilt.

## Entscheidung

1. **Muster A, gleicher Scope:** Folgen in einem Ablauf zwei oder mehr Rahmen
   mit gleichem Scope (Admin/Admin, gleiche Schule/gleiche Schule) direkt
   aufeinander, öffnet der Owner oder Workflow des Ablaufs einen Rahmen, und
   die bestehenden Fähigkeiten treten ihm bei. Reihenfolge, Guards und SQL der
   Fähigkeiten bleiben gleich. Erlaubt nur, wenn die früheren Rahmen nichts
   schreiben oder ein Teil-Commit nachweislich nicht beobachtbar ist (Punkt 5).
2. **Muster B, Admin geht in den Tenant:** Ein Ablauf darf in einer
   Transaktion erst als Admin entdecken und dann als Tenant lesen. Die
   Laufzeit bietet dafür eine Operation `WithinAdminThenTenant(ctx, discover,
   fn)`: `discover` läuft im Admin-Kontext und gibt nur die Schul-ID und die
   entdeckten Werte zurück. Danach setzt die Laufzeit in derselben Transaktion
   `set_config('role', 'phoenix_tenant', true)` und
   `set_config('app.current_tenant_id', …, true)` und ruft `fn` mit einem
   Tenant-Kontext, der aus dem ursprünglichen Kontext gebaut ist. Der
   Admin-Kontext ist danach nicht mehr erreichbar; ein Beitritt als Admin
   scheitert wie heute. Der Weg zurück (Tenant zu Admin) ist ausgeschlossen.
   Die Demo-Laufzeit lehnt die Operation ab, wie `WithinTraeger`.
3. **Nicht gebündelt** werden: Rahmen, deren Commit vor einer Freigabe liegen
   muss (Audit-Commit vor Datenfreigabe und die Admin-Einheit der Route bei
   #5795), die erneute Prüfung nach dem Lesen
   (#5894) und die Tenant-Projektion nach der Token-Entdeckung (#5881 verlangt,
   die Entdeckung vorher freizugeben). Eine ganze Anfrage als Admin bleibt
   verboten.
4. **Register:** Jede neue Startstelle steht mit ADR 0161 in einer exakten
   Ausnahmeliste `backend/test/transaction_frame_bundle_transaction.go`
   (Vorschlag unten). Die Namen `WithinAdminThenTenant` und
   `withinAdminThenTenant` kommen in derselben Änderung als Admin-Operationen
   in `transactionOperation`, bevor sie benutzt werden; sonst sähe der Ratchet
   die neue Startstelle nicht. Bestehende Einzelstellen bleiben, solange andere Aufrufer sie
   brauchen; fällt eine weg, wird ihr Eintrag gelöscht.
5. **Teil-Commit und Rollback:** Ein gebündelter Rahmen committet alles oder
   nichts. Für reine Lesungen ändert das nichts Beobachtbares: Unter READ
   COMMITTED liest jedes Statement seinen eigenen Snapshot, Lesungen halten
   keine Sperren. Ein SQL-Fehler bricht die Transaktion ab; darum muss jeder
   Zweig, der heute nach einem Fehler weiterläuft, vor dem nächsten Statement
   enden. Für #5878 ändert sich der Teil-Commit: Scheitert heute die dritte
   Familie, sind die ersten zwei schon gelöscht; gebündelt bleibt keine
   gelöscht. Das braucht eine Entscheidung (offene Frage 1).
6. **Fehlervertrag (E4):** 4xx-Antworten bleiben nach Status, Code und Body
   gleich, und die Prüfungen laufen in der heutigen Reihenfolge. Jede 4xx
   entsteht an derselben Stelle des Ablaufs wie heute:
   - **Vor dem ersten Tenant-Statement** stehen die Ablehnungen aus der
     Entdeckung: unbekannter oder inaktiver Feed-Token (404, #5881, hier folgt
     gar kein Tenant-Rahmen), inaktiver Operator oder ungültige
     Sitzungsfamilie (#5795, ohne Tenant), Kind nicht verknüpft und fehlendes
     `parent_portal.*`-Recht (#5862) und das Anhang-Gate „Ankündigung nicht
     live oder Konto nicht in der Zielgruppe“ (404, #5894).
   - **Im Tenant-Rahmen** bleiben die Ablehnungen, die schon heute dort
     entstehen: abgeschalteter oder nicht lesbarer Schulschalter und fehlende
     Anhangzeile (404, #5894) sowie die fachlichen 4xx der Payload einer
     Kind-Route nach bestandener Rechteprüfung (#5862, etwa Schulschalter).
   - **Nach dem Tenant-Rahmen** bleibt die erneute Zielgruppenprüfung von
     #5894 (404, wenn die Ankündigung inzwischen zurückgezogen ist oder die
     Schule wechselt).

   Bündeln verschiebt keine Ablehnung über eine dieser Grenzen. Details einer
   500 dürfen sich ändern, etwa durch einen Rollback statt eines Commits.
7. **Ambiente Einstiege:** Ein gebündelter Rahmen (Muster A) verhält sich in
   einem schon offenen Rahmen wie die Einzelstellen, die er zusammenfasst.
   `WithinAdmin` der Laufzeit tritt nur einem Admin-Rahmen bei
   (`tenant_runtime.go:158–162`); `RunAdmin` von Identity & Access tritt jedem
   ambienten Rahmen bei, außer unter `standaloneAdminScopeKey`
   (`command_transactions.go:58–67`). Ein gebündelter Tenant-Rahmen tritt nur
   einem Tenant-Rahmen derselben Schule bei. `WithinAdminThenTenant` lehnt jede
   ambiente Transaktion ab, weil es sonst die Rolle eines fremden Rahmens
   wechseln würde. #5878 behält den ambienten Zweig von `withPushAdminTx`.

### Invarianten

- **Tenant-Isolation:** Owner-Code einer Schule läuft nur im Tenant-Kontext
  mit gesetzter `app.current_tenant_id` unter RLS. Rolle und Schule sind
  transaktionslokal (`set_config(…, true)`, `SET LOCAL`). Nach dem Wechsel in
  Muster B ist kein Owner-Aufruf mit Admin-Rolle möglich. PostgreSQL plant
  RLS-Abfragen nach einem Rollenwechsel neu; ein vorher gecachter Plan gilt
  nicht weiter.
- **Sitzungen getrennt:** Tenant-, Operator-, Eltern- und Schul-Sitzung
  bleiben getrennt. Muster A und B ändern keine Sitzung, kein JWT und keine
  Portal-Prüfung. Die Operator-Prüfung (#5795) bleibt eine Operator-Lesung und
  öffnet nie einen Tenant.
- **Eltern-Kind-Recht:** Die Prüfung von Beziehung und `parent_portal.*`-Recht
  läuft in `discover`, vollständig vor dem ersten Tenant-Statement, mit der
  gleichen Konstante wie heute. Eine Mitgliedschaft allein öffnet nichts.
- **Keine neuen Rechte:** keine neue Rolle, keine Policy-Lockerung, kein
  fremdes SQL, kein Cache, keine `SECURITY DEFINER`-Funktion.

## Registereintrag-Vorschlag

Nicht aktiviert. Die Einträge kommen je Schritt mit der Umsetzung in
`backend/test/transaction_boundaries.json` und in die Ausnahmeliste, die
`transactionScopeSubset` (`backend/test/transaction_relocation.go:15–17`) wie
die übrigen `withoutReviewed…`-Filter entfernt. Ein Exaktheitstest wie
`parent_document_inbox_transaction_test.go` sichert, dass ein umformulierter
Grund wieder auffällt. Die genauen Schlüssel bestimmt der Scanner bei der
Umsetzung; weicht einer ab, gilt diese Entscheidung nur für die beschriebene
Stelle.

```go
// transactionFrameBundleSites are the boundaries ADR 0161 adds: one
// administrative read unit replaces consecutive same-scope units, and one
// one-way admin-to-tenant handoff replaces a discovery unit before a school
// read. A further site needs its own decision.
var transactionFrameBundleSites = map[string]string{
	// Admin, Schritt 1 (#5881)
	"modules/schoolcalendar/portal/internal/application/feed.go:*service.calendarFeedOwnerByToken:s.cfg.Runtime.WithinAdmin#1": "ADR 0161: the public calendar feed discovers the token owner in one administrative read unit, parent token first and the staff membership token with its account only on a miss; it writes nothing and ends before the school's tenant projection.",
	// Admin, Schritt 2 (#5795)
	"modules/identityaccess/internal/application/operator.go:*Service.ValidateActiveOperatorSession:s.tx.RunAdmin#1": "ADR 0161: the active-operator check reads the operator and the latest session of its family in one administrative read unit with unchanged order and guards; it writes nothing and the route keeps its own administrative unit.",
	// Admin, Schritt 4 (#5862): Laufzeit, Komposition, Aufrufer
	"tenant/runtime.go:WithinAdminThenTenant:runtime.withinAdminThenTenant#1":                                         "ADR 0161: the one-way handoff discovers as phoenix_admin and continues in the same transaction as phoenix_tenant for the discovered school only; no administrative context survives the switch.",
	"modules/transactionruntime/compose/tenant_runtime.go:NewTenantRuntime:unitOfWork.WithinAdminThenTenant#1":         "ADR 0161: binds the one-way admin-to-tenant handoff of the unit of work.",
	"workflows/parentportal/internal/care/child.go:*Service.WithPermittedChild:s.Runtime.WithinAdminThenTenant#1":     "ADR 0161: a parent child read discovers active schools and the relationship-level parent_portal permission before any tenant statement, then reads the child's school under RLS in the same transaction; it writes nothing.",
}
```

Für das Kommando-Register (`commands`) kommt in Schritt 4 hinzu:
`modules/transactionruntime/internal/adapters/postgres/tenant_runtime.go:*PostgresUnitOfWork.WithinAdminThenTenant:r.runInTx#1`
mit dem Grund „ADR 0161: implements the one-way admin-to-tenant handoff;
role and school are transaction-local“.

Ohne Registeränderung kommen aus: #5878 (der äußere Rahmen läuft über die
registrierte Stelle `session_revocation.go:*AccountAuthentication.withPushAdminTx:s.runtime.WithAdminTx#1`)
und #5894 Schritt 3 (eine Tenant-Startstelle im Workflow `parentportal`, die
der Ratchet nicht aufzählt).

## Umsetzungsreihenfolge

| Schritt | Issue | Änderung | Register | Ersparnis |
|---|---|---|---|---|
| 1 | #5881 | Methode `CalendarFeedByToken` im Kalender-Portal öffnet einen Admin-Rahmen für beide Token-Lesungen; `PublicFeed` ruft nur sie. Einzelne Eltern-, Personal- und CalDAV-Wege bleiben. | +1 Admin | 3 je Personal-Feed-Abruf (26 → 23), 3 je unbekanntem Token (8 → 5); Eltern-Treffer unverändert. Klasse A, Kalender-Clients fragen regelmäßig ab. |
| 2 | #5795 | `ValidateActiveOperatorSession` im Identity-Owner mit unveränderten Guards; die Middleware ruft nur sie. | +1 Admin | 3 je Operator-Anfrage (38 gemessene Anfragen: 678 → 564). |
| 3 | #5894 | Der Workflow trennt Zielgruppe (Admin) und Schalter; Schalter und Dateizeilen laufen in einem Tenant-Rahmen. Erneute Prüfung bleibt. | keine | 3 je Anhang-Aufruf (21 → 18). |
| 4 | #5862 | `WithinAdminThenTenant` in Laufzeit und Scanner; `WithPermittedChild` in `care`; zuerst die 14 Leserouten. Schreibende Routen bleiben bei `ResolvePermittedChild`. | +3 Admin, +1 Kommando | 2 je Kind-Route (14 Szenarien: 205 → 177). |
| 5 | #5894 | Zielgruppe und Tenant-Lesung über Muster B. | keine weitere | 2 weitere (18 → 16). |
| 6 | #5878 | Ein Admin-Rahmen um den ganzen Cleanup; die DELETEs bleiben je Familie und Schule × Portal in heutiger Reihenfolge. Nur nach Freigabe von Punkt 5. | keine | 3 je zusätzlichem Rahmen: Logout 19 → 16, drei Familien 23 → 14, fünf Familien 33 → 18. |

Jeder Schritt ist ein eigener PR. Er senkt die betroffenen Query-Budgets auf
den neuen gemessenen Wert. Für #5878 gibt es kein Budget; Schritt 6 legt eines
an (Regel 15). Jeder Schritt prüft:

- Erfolgs-, 4xx- und 500-Pfad, mit der Reihenfolge und Stelle der 4xx aus
  Punkt 6;
- Wiederverwendung der Pool-Verbindung nach Commit und nach Rollback des
  gebündelten Rahmens: Die nächste Transaktion auf derselben Verbindung sieht
  weder Rolle noch `app.current_tenant_id` des vorigen Rahmens
  (`current_user`, `current_setting('app.current_tenant_id', true)`);
- die ambienten Einstiege aus Punkt 7: Aufruf aus einem schon offenen
  Admin-Rahmen, aus einem Tenant-Rahmen derselben und einer anderen Schule;
- `TestTransactionBoundaryRatchet` und `scripts/backend-architecture.sh check`.

## Erwogene Optionen

- **Nichts ändern, nur Budgets halten:** keine Gefahr, aber die Kosten bleiben,
  am höchsten beim Kalender-Feed (Klasse A).
- **Je Issue eine eigene Ausnahme ohne gemeinsame Regel:** fünf Entscheidungen
  mit eigenen Begründungen; das Register wächst ohne Maßstab. Verworfen.
- **Ganze Anfrage als Admin:** spart am meisten, umgeht aber RLS für die Daten
  der Schule. Verworfen.
- **Entdeckung als `SECURITY DEFINER`-Funktion im Tenant-Rahmen (#5862):**
  käme ohne Admin-Rolle aus, schriebe aber SQL über Tabellen von People
  Directory und Identity & Access an der Owner-Grenze vorbei und verdoppelte
  `FindForAccount`. Verworfen.
- **Cache für Token oder Operator-Status:** widerspricht den Issues, verzögert
  Sperren und Abmeldungen. Verworfen.

## Folgen und Risiken

- Das Register wächst um fünf Admin-Stellen und eine Kommando-Stelle. Keine
  bestehende fällt weg, weil die Einzelwege weitere Aufrufer haben.
- Muster B ist eine neue Laufzeitfähigkeit. Ein Fehler darin hätte die Wirkung
  eines RLS-Fehlers. Darum: Scanner-Name vor Nutzung, ein Test, der nach dem
  Wechsel `current_user` und `app.current_tenant_id` prüft und einen
  Admin-Beitritt ablehnt, und ein Test, der Kinder einer fremden Schule nicht
  sieht.
- Ein gebündelter Rahmen hält seine Verbindung so lange wie die Summe der
  einzelnen; es entfallen aber Pool-Anfragen. Gemessen wird das nicht.
- Muster A verschiebt bei #5795 die Guards aus `inbound/operator` in den
  Identity-Owner. Die Middleware wird dünner; der Fehlervertrag der
  Middleware bleibt.
- Erwartete Werte sind aus Dumps abgeleitet, nicht an einer Umsetzung
  gemessen. Jeder Schritt misst neu, bevor er ein Budget senkt.

## Offene Fragen

1. #5878: Darf ein Fehler im Push-Cleanup die schon gelöschten Familien
   zurückrollen (alles oder nichts statt Präfix)? Folge: Nach einem Fehler
   bleiben mehr Push-Abos abgemeldeter Geräte stehen. Ob eine spätere
   Bereinigung sie entfernt, ist nicht geprüft.
2. #5862: Wird Muster B als neue Laufzeitfähigkeit freigegeben, zunächst nur
   für Leserouten?
