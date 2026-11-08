---
status: accepted
---

# Gleiche Transaktionsrahmen werden gebündelt, Admin geht nur einseitig in den Tenant

Status: Angenommen. Freigegeben von Yannick Mark Wenger am 07.11.2026.

Vorschlag vom 07.11.2026 (Entscheidung E6) zu
[#5881](https://github.com/moto-nrw/project-phoenix/issues/5881),
[#5795](https://github.com/moto-nrw/project-phoenix/issues/5795),
[#5878](https://github.com/moto-nrw/project-phoenix/issues/5878),
[#5894](https://github.com/moto-nrw/project-phoenix/issues/5894) und
[#5862](https://github.com/moto-nrw/project-phoenix/issues/5862).
Die Umsetzung folgt in späteren PRs, je Schritt einer. Die Entscheidung
selbst ändert keinen Code und kein Register. Sie legt fest, wann mehrere Transaktionsrahmen eines Ablaufs
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
| #5862 | 13 Routen, 14 Szenarien `parent.children.*` | je Admin 7 (Schulen, Beziehung, Kind), dann Tenant 4–14 | 205 | 177 |

Die 14 Szenarien der 13 Routen von #5862: `related_accounts` (N3, N8), `master_data`,
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

### 4xx-Ausgänge je Route

Stand `development` @ `b4b1950e5a`, je Zeile am Code gelesen. Pfade ohne
Verzeichnis liegen im Paket der Route: `workflows/parentportal/http/` für die
Eltern-Handler, `workflows/parentportal/internal/care/` für `child.go`,
`meal_plan.go` und `parent_care_offerings_service.go`,
`workflows/parentportal/internal/messaging/` für die Anhang- und
Freigabe-Dienste. Ein Fehler ohne eigenen Code bekommt den Klassencode aus
`api/common/problem.go:27–43`: 401 und 403 `general.permission`, 409
`general.business_rejection`, 400 und 404 `general.input`.

Phasen: **vor Rahmen** (vor dem ersten BEGIN), **Admin n** und **Tenant n**
(im n-ten Admin- bzw. Tenant-Rahmen), **zwischen** (nach dem COMMIT eines
Rahmens, vor dem BEGIN des nächsten), **nach Rahmen** (nach dem letzten
COMMIT). 5xx sind nicht aufgeführt.

**Gemeinsame Eltern-Kette** (#5862, #5894; `api/session_route_assembly.go:40–42`),
vor den Rahmen der Route und von der Bündelung nicht berührt: 401
`general.permission` bei fehlendem, ungültigem oder abgelaufenem Token
(`modules/identityaccess/sessiontoken/authenticator.go:85–100`, `216–228`) und
bei fehlendem Eltern-Scope (`sessiontoken/parent_middleware.go:21`, `30`);
403 `module.not_booked`, wenn keine Schule des Kontos die Eltern-App gebucht
hat (`api/module_booking_gate.go:188–189`). Diese Buchungsprüfung liest selbst
und ist kein Teil der hier gebündelten Rahmen.

**#5881 `GET /public/calendar/{token}`** (Admin 1 Eltern-Token, Admin 2
Personal-Token, Tenant 1 Projektion). Alle 404 rendert
`modules/schoolcalendar/http/public_feed.go:38–40` ohne Code (`general.input`).

| Status | Code | Ursache | Datei:Zeile | Phase |
|---|---|---|---|---|
| 404 | `general.input` | Feed nicht verdrahtet | `public_feed.go:28–30` | vor Rahmen |
| 404 | `general.input` | leerer Token (Eltern- und Personalzweig) | `portal/internal/application/feed.go:523–525`, `caldav.go:144–146` | vor Rahmen |
| – | – | Eltern-Token unbekannt oder Konto inaktiv: kein Ausgang, weiter zum Personalzweig | `feed.go:531–539`, `public_feed.go:34–36` | Admin 1, ausgewertet zwischen |
| 404 | `general.input` | Personal-Token unbekannt | `caldav.go:165–167` | Admin 2 |
| 404 | `general.input` | Konto fehlt oder inaktiv | `caldav.go:172–174` | Admin 2 |
| 404 | `general.input` | Person, Personal oder Kalenderzugang fehlt | `feed.go:263–264`, `268–269`, `273–274`, `289–290`, `296–297`, weitergereicht `221–222` | Tenant 1 |

**#5795 Operator-Kette** (`modules/identityaccess/inbound/operator/sessions.go:57–66`;
Admin 1 Operator, Admin 2 Sitzungsfamilie, danach die Rahmen der Route). Pfade
relativ zu `modules/identityaccess/`.

| Status | Code | Ursache | Datei:Zeile | Phase |
|---|---|---|---|---|
| 401 | `general.permission` | Token fehlt, ungültig, widerrufen | `sessiontoken/authenticator.go:85–100`, `216–228` | vor Rahmen |
| 403 | `identity.read_only_preview` | Schreibanfrage in der Lesevorschau | `api/common/readonly_preview.go:72` | vor Rahmen |
| 403 | `general.permission` | kein Operator-Scope | `inbound/operator/middleware.go:28–33` | vor Rahmen |
| 401 | `general.permission` | keine Konto-ID im Token | `middleware.go:46–48` | vor Rahmen |
| 401 | `identity.invalid_credentials` | Operator fehlt oder ID ≤ 0 | `internal/application/operator.go:48–49`, `56–57`; gerendert `middleware.go:91–93`, `inbound/operator/errors.go:82–83` | Admin 1 |
| 403 | `identity.account_inactive` | Operator inaktiv oder keine Familie im Token | `middleware.go:74–76`, `errors.go:80–81`, `100–102` | zwischen |
| 403 | `identity.account_inactive` | keine Sitzung der Familie | `operator.go:308–309`, umgesetzt `middleware.go:78–80` | Admin 2 |
| 403 | `identity.account_inactive` | Sitzung gehört einem anderen Operator oder ist abgelaufen | `middleware.go:84–86` | nach Rahmen |
| 4xx | je Route | Fehler der Route selbst | Handler der Route | nach Rahmen (eigene Rahmen der Route) |

**#5878 Push-Cleanup:** kein 4xx. Der Cleanup läuft nach dem Commit der
auslösenden Anfrage; sein Fehler wird nur protokolliert
(`modules/identityaccess/internal/application/session_revocation.go:172–189`).
Die kontoweiten Wege, die den Fehler zurückgeben (`214`, `233`, `239`, `535`),
löschen in einem Rahmen (`392–402`) und sind von der Bündelung nicht berührt;
ihr Fehler ist ein 5xx.

**#5894 Anhänge der Eltern** (`modules/filestorage/http/files/attachments.go:241–295`).
Jede Ablehnung der Prüfung wird zu `ErrAttachmentNotFound` und über
`roots/owners/parent_announcement_attachments.go:56–62` und
`attachments.go:135–148` zu 404 ohne Code (`general.input`). Rahmenfolgen:

- **Liste:** Admin 1 Zielgruppe, Tenant 1 Schulschalter (entfällt bei
  Systemankündigungen, `parent_announcement_service.go:344–346`), Tenant 2
  Dateizeilen, Admin 2 erneute Prüfung.
- **Ankündigungs-Download:** wie die Liste bis Admin 2, danach Tenant 3 Zeile
  und zuletzt die gespeicherte Datei (`announcement_attachment_entry.go:93–105`).
- **Nachweis-Download** (`?student_id=`, `entry.go:82–88`): Admin 1 Kind
  (`child.go:61` über `parent_declaration_feed.go:173`), Admin 2 Nachweis
  (`parent_declaration_feed.go:178–180`), Tenant 1 Zeile, Admin 3 und Admin 4
  erneute Prüfung (dasselbe Gate noch einmal, `entry.go:102–104`), Tenant 2
  Zeile, zuletzt die Datei. Kein Schalter.

Liste und Ankündigungs-Download:

| Status | Code | Ursache | Datei:Zeile | Phase |
|---|---|---|---|---|
| 400 | `general.input` | `announcementId` bzw. `attachmentId` keine Zahl | `attachments.go:242`, `261`, `265`, `api/common/request.go:81–88` | vor Rahmen |
| 401 | `general.permission` | keine Konto-ID im Token | `attachments.go:232–239` | vor Rahmen |
| 404 | `general.input` | Konto- oder Ankündigungs-ID ≤ 0 | `announcement_attachment_entry.go:109–110` | vor Rahmen |
| 404 | `general.input` | Ankündigung fehlt, nicht live oder Konto nicht in der Zielgruppe | `parent_announcement_attachments.go:42`, `49`, ausgewertet `59–60`, `entry.go:116–121` | Admin 1, ausgewertet zwischen |
| 404 | `general.input` | Schulschalter aus oder nicht lesbar (fail closed) | `parent_announcement_service.go:351–361`, `parent_announcement_attachments.go:64–65` | Tenant 1, ausgewertet zwischen |
| 404 | `general.input` | `attachmentId` ≤ 0 (Download) | `modules/filestorage/internal/application/attachments.go:358–359` | zwischen (nach Tenant 1, vor Tenant 2) |
| 404 | `general.input` | Anhangzeile fehlt (Download) | `filestorage/internal/application/attachments.go:368–369` | Tenant 2 |
| 404 | `general.input` | Ankündigung inzwischen zurückgezogen oder andere Schule | `announcement_attachment_entry.go:127–134` | Admin 2 (nach den Tenant-Rahmen) |
| 404 | `general.input` | Zeile oder gespeicherte Datei fehlt beim Öffnen (Download) | `filestorage/internal/application/attachments.go:345–346`, `service.go:366–370` | Tenant 3 bzw. nach Rahmen |

Nachweis-Download:

| Status | Code | Ursache | Datei:Zeile | Phase |
|---|---|---|---|---|
| 400 | `general.input` | `announcementId` bzw. `attachmentId` keine Zahl | `attachments.go:261`, `265` | vor Rahmen |
| 401 | `general.permission` | keine Konto-ID im Token | `attachments.go:232–239` | vor Rahmen |
| 400 | `general.input` | `student_id` mehrfach oder nicht positiv | `attachments.go:277–284` | vor Rahmen |
| 404 | `general.input` | eine der IDs ≤ 0 | `announcement_attachment_entry.go:84–85` | vor Rahmen |
| 404 | `general.input` | Kind nicht verknüpft oder ohne `parent_portal.access`; jeder Fehler der Kindprüfung wird zu „nicht gefunden“ | `parent_declaration_feed.go:173–175`, ausgewertet `parent_announcement_attachments.go:79–83`, `entry.go:116–121` | Admin 1, ausgewertet zwischen |
| 404 | `general.input` | Ankündigung fehlt, ist keine Erklärung oder gehört einer anderen Schule; keine eigene Abgabe | `parent_declaration_feed.go:192–193`, `205–206`, ausgewertet `parent_announcement_attachments.go:79–83` | Admin 2, ausgewertet nach Admin 2 |
| 404 | `general.input` | Anhang gehört zu keiner Version einer eigenen Abgabe | `parent_announcement_attachments.go:86–97` | nach Admin 2, vor Tenant 1 |
| 404 | `general.input` | Anhangzeile fehlt | `filestorage/internal/application/attachments.go:368–369` | Tenant 1 |
| 404 | `general.input` | Nachweis nicht mehr gültig oder andere Schule (alle Zeilen ab Admin 1 noch einmal) | `announcement_attachment_entry.go:102–104`, `127–134` | Admin 3, Admin 4 bzw. danach (nach Tenant 1) |
| 404 | `general.input` | Zeile oder gespeicherte Datei fehlt beim Öffnen | `filestorage/internal/application/attachments.go:345–346`, `service.go:366–370` | Tenant 2 bzw. nach Rahmen |

Die Schritte 3 und 5 bündeln nur Rahmen von Liste und Ankündigungs-Download.
Der Nachweis-Download hat keinen Schalter-Rahmen; seine Zeilen gelten bei
jedem Schritt unverändert mit den Phasen oben.

**#5862 Eltern-Kind-Leserouten** (`routes.go:122–229`; Admin 1 `child.go:61`,
dann Tenant 1 der Route). Gemeinsame Zeilen aller 13 Routen:

| ID | Status | Code | Ursache | Datei:Zeile | Phase |
|---|---|---|---|---|---|
| G1 | 401 | `general.permission` | keine Konto-ID im Token | `child_write_handlers.go:565–572` | vor Rahmen |
| G2 | 400 | `general.input` | `studentId` ungültig | `child_write_handlers.go:574`, `api/common/request.go:93–100` | vor Rahmen |
| G3 | 403 | `care.child_not_linked` | keine Verknüpfung mit `parent_portal.access`, Schule ohne Eltern-App, Kind entlassen oder gelöscht | `child.go:66–67`, gemappt `child_write_errors.go:14–17`; Filter `modules/careplan/internal/adapters/parentpostgres/child_repository.go:114–130`, `roots/owners/parent_portal_module_booking.go:43–58` | Admin 1 |
| G4 | 403 | `care.guardian_permission_denied` | Recht fehlt; nur erreichbar, wenn die Route ein anderes Recht als `parent_portal.access` verlangt | `child.go:69–70`, gemappt `child_write_errors.go:18–19` | Admin 1 |

Fehler, die kein Renderer kennt, werden 500 (`child_write_handlers.go:602–621`,
`request_sharing_handlers.go:138–154`). Die drei Freigabe-Routen mappen G4
auf 403 `care.guardian_permission_denied` (`request_sharing_handlers.go:151–153`);
beim Lesen verlangen sie nur `parent_portal.access`
(`parent_request_sharing_service.go:122–124`), G4 ist dort nicht erreichbar.

| Route (`/me/children/{studentId}/…`) | Zusätzliche 4xx: Status, Code, Ursache, Datei:Zeile, Phase |
|---|---|
| `consents` | nur G1–G3 (`consent_handlers.go:42`, `46`) |
| `meal-participation` | 400 `general.input`, `from`/`to` fehlt oder kein Datum, `child_write_handlers.go:433–437`, vor Rahmen. 400 `meals.participation_out_of_range`, Zeitraum außerhalb dieser Woche bis Montag + 25 Tage, `meal_plan.go:88–90`, gemappt `child_write_errors.go:41–42`, **zwischen**. 403 `meals.registration_disabled`, Speiseplan oder Anmeldung aus, `modules/mealplan/internal/application/service.go:447–448`, `461–462`, `meal_plan.go:215–216`, Tenant 1. 400 `meals.invalid_participation`, ungültige Bestellregeln, `meal_plan.go:219–220`, Tenant 1 |
| `excused-requests` | nur G1–G3 |
| `requests/{requestType}/{requestId}/events` | 400 `general.input`, `requestId` ungültig, `request_edit_handlers.go:322`, vor Rahmen. G3 wird hier 404 `care.request_not_found` (`request_sharing_handlers.go:148–150`), Admin 1. 400 `care.request_sharing_invalid`, unbekannter Typ, `parent_request_sharing_service.go:480`, Tenant 1. 404 `care.request_not_found`, Anfrage fehlt, falsche Art oder nicht vom Konto, `parent_request_sharing_service.go:464`, `502`, Tenant 1 |
| `request-sharing/{requestType}/{requestId}` | wie `events`: 400 `general.input` (`request_sharing_handlers.go:106`), G3 als 404 `care.request_not_found`, dann 400 `care.request_sharing_invalid` und 404 `care.request_not_found` in Tenant 1 (`parent_request_sharing_service.go:464`, `480`, `502`) |
| `request-sharing-options` | G1, G2; G3 als 404 `care.request_not_found` (`request_sharing_handlers.go:148–150`), Admin 1 |
| `care-exception` | 400 `general.input`, `from`/`to` ungültig oder `to` vor `from`, `child_write_handlers.go:237–249`, gerendert `care_exception_handlers.go:290–292`, vor Rahmen |
| `pickup-change-requests` | nur G1–G3 |
| `care-offerings` | nur G1–G3; Schalter enden ohne Fehler (`parent_care_offerings_service.go:527–579`) |
| `care-offerings/catalog` | 400 `care.offering_change_invalid`, `effective_from` kein Datum, `care_offerings_handlers.go:329–333`, vor Rahmen. G4 erreichbar (`parent_portal.request.submit`), Admin 1. 403 `care.child_care_ended`, `child.go:115–116` über `parent_care_offerings_service.go:225`, **zwischen**. 403 `care.offering_changes_disabled`, Änderungen aus oder Schalter nicht lesbar, `modules/careplan/internal/application/offering_changes.go:110`, `113`, Tenant 1. 403 `students.care_offerings_disabled`, `offering_changes.go:120`, `123`, Tenant 1. 400 `care.offering_change_invalid`, Datum vor der Frist, `offering_change_catalog.go:41`, Tenant 1. 403 `students.offering_changes_no_enrollment`, `offering_changes.go:178`, `200`, Tenant 1 |
| `master-data` | nur G1–G3 |
| `master-data/requests` | nur G1–G3 |
| `related-accounts` | nur G1–G3 |

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
   entdeckten Werte zurück. Prüfungen, die heute zwischen Admin- und
   Tenant-Rahmen laufen, stehen am Ende von `discover` (Punkt 6); ein Fehler
   dort beendet die Transaktion ohne Rollenwechsel. Danach setzt die Laufzeit in derselben Transaktion
   `set_config('role', 'phoenix_tenant', true)` und
   `set_config('app.current_tenant_id', …, true)` und ruft `fn` mit einem
   Tenant-Kontext, der aus dem ursprünglichen Kontext gebaut ist. Der
   Admin-Kontext ist danach nicht mehr erreichbar; ein Beitritt als Admin
   scheitert wie heute. Der Weg zurück (Tenant zu Admin) ist ausgeschlossen.
   Die Demo-Laufzeit lehnt die Operation ab, wie `WithinTraeger`.
   Freigegeben am 07.11.2026 (#5862) als neue Laufzeitfähigkeit, zunächst nur
   für Leserouten. Schreibende Eltern-Kind-Routen bleiben beim bisherigen
   Ablauf: `ResolvePermittedChild` im Admin-Rahmen, danach der eigene
   Tenant-Rahmen. Eine Ausweitung auf Schreibwege braucht eine eigene
   Entscheidung.
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
   gelöscht. Entschieden am 07.11.2026: Für #5878 gilt alles oder nichts. Ein
   Fehler im Push-Cleanup rollt alle Löschungen dieses Laufs zurück. Folge:
   Push-Abos abgemeldeter Geräte können bis zum nächsten erfolgreichen Lauf
   bestehen bleiben.
6. **Fehlervertrag (E4):** Phase und Reihenfolge jedes 4xx der Tabelle
   „4xx-Ausgänge je Route“ bleiben gleich, ebenso Status, Code und Body. Die
   Phase meint die Lage zwischen den fachlichen Statements; Bündeln entfernt
   nur BEGIN, Rollenwechsel und COMMIT zwischen zwei Teilen. Daraus folgt je
   Phase:
   - **vor Rahmen** und **nach Rahmen:** unverändert, auch die gemeinsamen
     Ketten und die Zeilen G1, G2 und die Parse-Fehler der Routen.
   - **Admin n, Tenant n:** Die Ablehnung bleibt im selben Teil nach
     denselben Statements. Werden Admin 1 und Admin 2 ein Rahmen (#5881,
     #5795), folgt sie weiter denselben SELECTs, etwa der unbekannte
     Personal-Token nach der Eltern-Token-Lesung.
   - **zwischen:** Die Prüfung bleibt zwischen denselben Statements. In
     Muster A läuft sie im gebündelten Rahmen an dieser Stelle: inaktiver
     Operator zwischen den zwei SELECTs (#5795), Schulschalter vor der
     Dateilesung (#5894 Schritt 3). In Muster B läuft sie am Ende von
     `discover`, nach den Admin-Lesungen und vor dem Rollenwechsel:
     `meals.participation_out_of_range` (`meal_plan.go:88–90`) und
     `care.child_care_ended` auf dem Katalog (`parent_care_offerings_service.go:225`),
     bei #5894 Schritt 5 die ausgewertete Zielgruppe. Sie wandert weder vor
     die Rechteprüfung (dann würde ein nicht verknüpftes Kind 400 statt 403
     bekommen) noch in den Tenant-Teil.
   - Eine Ablehnung, der heute ein COMMIT vorausgeht, beendet den gebündelten
     Rahmen mit ROLLBACK. Die Teile davor lesen nur; beobachtbar ist das
     nicht.

   Kein Bündeln vertauscht zwei Prüfungen oder schiebt eine über eine
   fachliche Lesung. 5xx-Status bleiben (etwa 503 bei gescheiterter
   Operator-Prüfung, `middleware.go:96–101`); ihre Details dürfen sich ändern.
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
| 4 | #5862 | `WithinAdminThenTenant` in Laufzeit und Scanner; `WithPermittedChild` in `care`; zuerst die 13 Leserouten (14 Szenarien). Schreibende Routen bleiben bei `ResolvePermittedChild`. | +3 Admin, +1 Kommando | 2 je Kind-Route (14 Szenarien: 205 → 177). |
| 5 | #5894 | Zielgruppe und Tenant-Lesung über Muster B. | keine weitere | 2 weitere (18 → 16). |
| 6 | #5878 | Ein Admin-Rahmen um den ganzen Cleanup; die DELETEs bleiben je Familie und Schule × Portal in heutiger Reihenfolge; alles oder nichts nach Punkt 5. | keine | 3 je zusätzlichem Rahmen: Logout 19 → 16, drei Familien 23 → 14, fünf Familien 33 → 18. |

Jeder Schritt ist ein eigener PR. Er senkt die betroffenen Query-Budgets auf
den neuen gemessenen Wert. Für #5878 gibt es kein Budget; Schritt 6 legt eines
an (Regel 15). Jeder Schritt prüft:

- Erfolgs- und 500-Pfad sowie jede Zeile der Tabelle „4xx-Ausgänge je Route“
  für die geänderten Routen, mit Status, Code, Phase und Reihenfolge nach
  Punkt 6; eine neue oder verschobene Zeile ist ein Fehler der Umsetzung;
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
- Nach einem Fehler im Push-Cleanup (#5878) bleiben die Push-Abos aller
  Familien dieses Laufs stehen, nicht nur die der gescheiterten. Sie können
  bis zum nächsten erfolgreichen Lauf bestehen bleiben.
- Erwartete Werte sind aus Dumps abgeleitet, nicht an einer Umsetzung
  gemessen. Jeder Schritt misst neu, bevor er ein Budget senkt.

## Nachtrag 08.11.2026: Kommando-Schlüssel des Eltern-Feeds (#5881)

Schritt 1 verlagert die Tenant-Startstelle
`modules/schoolcalendar/portal/internal/application/feed.go:*service.ParentCalendarFeedByToken:s.cfg.Runtime.WithinTenant#1`
nach `…/feed.go:*service.parentCalendarFeed:s.cfg.Runtime.WithinTenant#1`, damit
`CalendarFeedByToken` den Eltern-Feed ohne zweite Token-Lesung rendert. Es ist
derselbe Rahmen mit bytegleichem Rumpf und wörtlich gleichem Grund; die Zahl der
Tenant-Startstellen bleibt gleich, und der neue Schlüssel steht in
`transactionFrameBundleSites`.
