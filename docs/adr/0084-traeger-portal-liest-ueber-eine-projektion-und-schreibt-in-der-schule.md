---
status: proposed
---

# Träger-Portal liest über eine Projektion und schreibt in der Schule

Entscheidung vom 23.10.2026 zu [#4645](https://github.com/moto-nrw/project-phoenix/issues/4645),
Etappe 0 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611).
Sie beantwortet [#2809](https://github.com/moto-nrw/project-phoenix/issues/2809),
[#2810](https://github.com/moto-nrw/project-phoenix/issues/2810),
[#2811](https://github.com/moto-nrw/project-phoenix/issues/2811),
[#2821](https://github.com/moto-nrw/project-phoenix/issues/2821) und
[#2822](https://github.com/moto-nrw/project-phoenix/issues/2822).
Sie bleibt `proposed`, bis der Product Owner sie annimmt
([ADR 0081](0081-owner-granularity-follows-invariants-not-source-size.md)),
weil sie einen neuen Owner einführt und
[ADR 0066](0066-principle-every-module-table-and-cross-owner-read-has-one-owner.md)
Punkt 7 erweitert.

Die Schule bleibt der Tenant. Das Träger-Büro arbeitet in einem eigenen
Träger-Portal mit einer eigenen Träger-Sitzung. Es liest über Schulgrenzen nur
in einem einzigen Projektions-Owner, in einer Admin-Transaktion mit Filter auf
die Schulen des Trägers. Es schreibt immer in der Tenant-Transaktion der
Zielschule über deren Owner. Zugriff auf Kinder einer fremden Schule entsteht
nur aus einer bestätigten Buchung, als befristete Kopie in der ausrichtenden
OGS. Kein Token trägt mehr als eine `tenant_id` (D4).

## Kontext

Der Träger existiert heute nur als `platform.organizations`, gepflegt im
Operator-Portal. Für einen Org-Scope gibt es ein Gerüst, das nie lief: Claim
`org_id`, Scope `org`, Zweige in Slug-Auflösung, Tenant-Wechsel,
`TenantMiddleware`, drei Konto-Abfragen und eine automatische
Admin-Transaktion, wenn ein Org-Token ohne Tenant kommt
(`identityaccess/internal/adapters/postgres/composition_tenant_runtime_run_in_tx_binding.go`).
Kein Login stellt ein solches Token aus. Das Gerüst geht davon aus, dass ein
Org-Token zugleich eine `tenant_id` trägt und im Sitzungstopf des OGS-Portals
liegt. Eine Tabelle für Träger-Mitgliedschaften fehlt.

`GET /api/active/cross-tenant-students` liest heute Kinder anderer Schulen
live, geprüft nur über `groups:read` in der ausrichtenden Schule, ohne Bezug
auf Träger, Buchung oder Einwilligung.

Das Sitzungs-Cookie des OGS-Portals gilt für alle Schul-Subdomains gemeinsam.
Ein Schulwechsel in einem Tab wechselt auch alle anderen Tabs.

## Entscheidung

### 1. Lesen über Schulgrenzen (#2810 Option a)

- Ein neuer Owner `traeger-view` vom Typ `projection` ist der einzige Ort, der
  Daten mehrerer Schulen für das Träger-Portal liest. Vorbild ist
  [ADR 0033](0033-operator-dashboard-view-is-a-projection-owner.md).
- Jede Fläche (Übersicht, Anmeldeliste, Berichte, Zugriffsprotokoll) ist ein
  eigener `read_projections`-Eintrag mit ihren genauen Tabellen.
- Die Projektion holt die Schul-IDs des Trägers der Sitzung bei Organisation &
  Tenancy und liest in einer Admin-Transaktion. Jede Query im Paket filtert
  auf `tenant_id = ANY(<Schulen des Trägers>)`.
- Stichtagszahlen ruft die Projektion über die vorhandenen Zählfunktionen aus
  #2791 ab, damit Operator und Träger dieselbe Zahl sehen (#2813).
- **Ratchet:** Ein Test prüft jede Query im Postgres-Paket von `traeger-view`
  auf diesen Filter. `backend/test/transaction_boundaries.json` erhält für das
  Träger-Portal Admin-Einträge nur aus diesem Paket und aus Login und Sitzung
  in Identity & Access. Jeder andere Code, der eine Admin-Transaktion mit
  Träger-Bezug öffnet, fällt dort auf.
- **Erweiterung von ADR 0066 Punkt 7:** `tenant_safe` heißt bei einer
  Projektion für das Träger-Portal: Routen nur hinter der Träger-Sitzung,
  Admin-Transaktion, Filter auf die Schulen des Trägers, geprüft durch den
  Ratchet. Für installationsweite Lesezugriffe bleibt es beim Operator-Scope.

### 2. Schreiben

- Jede Schreiboperation auf Daten einer Schule prüft zuerst, ob die Schule zum
  Träger der Sitzung gehört und die Person das Recht hat. Dann läuft sie in
  einer normalen Tenant-Transaktion dieser Schule, mit RLS, über den
  zuständigen Owner.
- Wirkt eine Operation auf mehrere Schulen, schreibt ein Workflow je Schule
  einen eigenen Tenant-Schritt
  ([ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)).
- Objekte auf Träger-Ebene (Ferienzeitraum, Tarif, Vorlage) hängen an der
  Träger-ID und gehören ihrem Fach-Owner. Das Träger-Portal besitzt keine
  Schreibfähigkeit selbst.
- Schreibend gibt es keinen Weg an RLS vorbei.

### 3. Portal und Sitzung

- **Konto:** Eine Person hat ein Konto. Das Träger-Portal ist ein eigener
  Zugang zu diesem Konto, kein zweites Konto.
- **Host und Slug:** eigener Host `NEXT_PUBLIC_TRAEGER_HOSTNAME`, Basis-URL für
  Einladungen als Pflichtwert ohne Default. `traeger` wird in der
  Slug-Reservierung von Frontend und Backend eingetragen.
- **Token:** Der Scope bleibt `org`. Ein `org`-Token trägt nur `org_id`, nie
  eine `tenant_id`. Eine Sitzung gehört zu genau einem Träger. Wer Mitglied
  mehrerer Träger ist, wählt beim Login.
- **Töpfe:** eigener Sitzungstopf `org` (nicht mehr gemeinsam mit `tenant`),
  eigener Push-Topf `traeger`, eigenes Cookie `traeger.session-token`.
- **Middleware:** Eine `TraegerMiddleware` und eine geschützte Routengruppe
  akzeptieren nur `org`. `TenantMiddleware` und `SchoolMiddleware` lehnen
  `org` ab. Ein Scope-Matrix-Test nach dem Muster von
  `backend/api/school_scope_matrix_test.go` prüft das für alle Routen.
- **Altes Gerüst:** Alle bisherigen Org-Zweige werden entfernt, auch die
  automatische Admin-Transaktion für Org-Tokens und der Org-Zweig der
  Slug-Auflösung.
- **MFA:** Pflicht für jedes Konto mit Träger-Mitgliedschaft, für alle Rollen,
  unabhängig von `security.mfa_mode` der Schulen.
- **Freigabe:** Das moto-Team schaltet das Träger-Portal je Träger frei. Ohne
  Freigabe gibt es keinen Login und keine Spur des Portals, auch nicht für
  einen Träger mit einer einzigen OGS. Freigabe und Entzug stehen im
  Operator-Audit.

### 4. Mitgliedschaft, Rollen und Rechte

- **Mitgliedschaft:** eigene Tabelle bei Organisation & Tenancy, Status
  pending, active und inactive mit Zeitstempeln wie `account_tenants`. Ein
  Konto darf Mitglied mehrerer Träger sein.
- **Rollen und Rechte:** bei Identity & Access, getrennt von Schulrollen. Keine
  Schulrolle mit `tenant_id = NULL` dient als Träger-Rolle. Prüfungen laufen
  nur gegen Rechte, nie gegen Rollennamen.
- **Einladung:** über das bestehende Einladungssystem mit dem Portalwert
  `traeger`. Die erste Person lädt das moto-Team ein, weitere lädt ein, wer
  „Konten verwalten“ hat.
- **Ende:** Deaktivieren der Mitgliedschaft oder Entzug der Freigabe beendet
  sofort alle Träger-Sitzungen und alle daraus abgeleiteten Schulsitzungen.

Die Träger-Rollen sind in v1 feste Bündel. Eine Person kann mehrere haben.

| Recht | Verwaltung | Abrechnung | Personal | nur lesen |
|---|---|---|---|---|
| Übersicht lesen | ✓ | ✓ | ✓ | ✓ |
| Anmeldung verwalten | ✓ | | | |
| Verträge verwalten | ✓ | | | |
| Abrechnung | | ✓ | | |
| Finanz-Export | | ✓ | | |
| Personal | | | ✓ | |
| Berichte | ✓ | ✓ | ✓ | ✓ |
| Einstellungen | ✓ | | | |
| Konten verwalten | ✓ | | | |
| Zugriffsprotokoll lesen | ✓ | | | |
| In OGS wechseln | ✓ | | | |

„Berichte“ zeigt zusammengefasste Zahlen. Personenbezogene Zahlen gibt es nur
zusammen mit dem Fachrecht.

### 5. Absprung in eine OGS (#2822)

- Wer „In OGS wechseln“ hat, bekommt per Übergabe eine normale OGS-Sitzung für
  genau eine Schule des Trägers, ohne neue Anmeldung. Es entsteht kein
  Eintrag in `account_tenants`.
- Das Token trägt nur die `tenant_id` dieser Schule, die Admin-Rolle der
  Schule und einen Vermerk, dass der Zugang über den Träger kommt.
- Jeder Refresh prüft neu: Mitgliedschaft aktiv, Recht vorhanden, Schule
  gehört noch zum Träger, Portal freigegeben.
- Die Person handelt als sie selbst. Das ist keine Impersonation, die D18
  ausschließt. Jede Handlung trägt im Audit der Schule den Vermerk
  „Träger-Büro“.
- Die OGS sieht in ihrer Kontenliste getrennt, wer über den Träger Zugriff
  hat.
- **Eine Sitzung je Schule:** Jede Schul-Subdomain bekommt ein eigenes
  host-only Cookie, für alle OGS-Konten. Ein Schulwechsel erzeugt eine neue
  Sitzung auf dem Ziel-Host per Übergabe. Die Grenze von fünf Sitzungen gilt je
  Konto, Topf und Schule. Die Umstellung ist ein eigenes Ticket.

### 6. Zugriff über eine bestätigte Buchung

- Mit der Bestätigung einer Ferienbuchung entsteht in der ausrichtenden OGS
  ein Feriengast mit Mindestfeldern: Vorname, Nachname, Geburtsdatum,
  Heimschule falls vorhanden, gebuchte Tage mit Zeitmodell, Notfallkontakte,
  Abholberechtigte und Heimweg-Regel, Allergien und Unverträglichkeiten,
  Einwilligungen, soweit sie die Betreuung betreffen.
- Nicht übertragen werden Notizen, Förderdaten, sonstige Gesundheitsdaten,
  Geschwister und Vertragsdaten.
- Die ausrichtende OGS sieht den Feriengast vom ersten bis zum letzten
  gebuchten Tag, danach sieben Tage nur lesend. Gelöscht wird nach der
  Löschfrist für Ferien-Daten (drei Monate,
  [ADR 0073](0073-inhalte-der-schule-folgen-der-frist-ihrer-datenart.md)).
- Ändert die Heimschule Notfalldaten, Abholung oder eine Einwilligung, zieht
  ein Ereignis die Kopie nach.
- RLS, Check-in und Token bleiben unverändert auf einen Tenant beschränkt.
- `GET /api/active/cross-tenant-students` wird durch den Feriengast ersetzt.
  Bis dahin prüft der Endpunkt eine bestätigte Buchung für die ausrichtende
  OGS.

### 7. Audit

- Jeder Zugriff des Träger-Portals auf personenbezogene Daten schreibt in
  `audit.data_access_log` einen Eintrag je betroffener Schule, im Tenant dieser
  Schule, mit Person, Kanal „Träger-Portal“ und Träger.
- Zusammengefasste Zahlen werden nicht protokolliert.
- Das Zugriffsprotokoll im Träger-Portal liest diese Einträge über
  `traeger-view`.

### 8. Einträge in Policy und Registern

1. Owner `traeger-view`, Typ `projection`, mit Paketen der Rollen `postgres`
   und `http`.
2. Je Fläche ein `read_projections`-Eintrag mit `tenant_safe: true` im Sinn von
   Abschnitt 1.
3. `modules/traegerportal` als `http`-Paket von `identity-access` für Login und
   Sitzung, wie `modules/schoolportal`.
4. Schreibrouten in den `http`-Paketen der Fach-Owner, eingehängt in die
   Träger-Routengruppe.
5. `data_objects` für Träger-Mitgliedschaft (`write_owner`
   `organization-tenancy`) und Träger-Rollenzuordnung (`write_owner`
   `identity-access`).
6. Regeln für `traeger-view` nur auf `public`- und `contract`-Rollen der
   Fach-Owner, damit der Owner-Graph azyklisch bleibt
   ([ADR 0058](0058-owner-dependencies-form-a-dag.md),
   [ADR 0071](0071-principle-owner-dependencies-point-one-way.md)).
7. Admin-Einträge in `transaction_boundaries.json` nur nach Abschnitt 1.
8. Route-Golden, Analytics-Guard und Scope-Matrix-Test für `org` decken jede
   neue Route ab.

## Verworfene Optionen

- **Zweite RLS-Policy-Familie** über `app.current_org_id` (#2810 Option b):
  Postgres wäre ein stärkeres Netz als ein Ratchet. Die Familie müsste aber
  auf jede tenant-gebundene Tabelle, und jede neue Tabelle müsste sie
  mitbringen. Die Spec schließt sie aus.
- **Nächtliche, anonymisierte Verdichtung** (#2810 Option c): Die Übersicht
  „heute anwesend“ wäre nicht live, und Anmeldung, Abrechnung und Personal
  brauchen personenbezogene Daten.
- **Lesefunktionen je Fach-Owner** im Muster von #2791: Die Admin-Einträge
  lägen über viele Pakete verstreut, und kein einzelner Ratchet könnte den
  Filter prüfen.
- **Live-Lesen des Feriengasts** über eine Freigabe: Jeder Check-in-Bildschirm
  bräuchte einen Lesezugriff über Schulen, und Gäste ohne Schule hätten keine
  Heimschule, aus der man lesen könnte.
- **Mitgliedschaften automatisch in jeder Schule anlegen**: Sie müssten bei
  jeder neuen Schule, jedem Rollenwechsel und jedem Austritt nachgezogen
  werden. Die abgeleitete Sitzung prüft das bei jedem Refresh.
- **Eigene Träger-Konten**: Eine OGS-Leitung im Träger-Büro hätte zwei
  Passwörter und zwei MFA, und Audit und Löschung hingen an zwei Identitäten.
- **Das bestehende Org-Gerüst weiterverwenden**: Es lässt Org-Tokens durch die
  `TenantMiddleware` und öffnet für sie automatisch eine Admin-Transaktion.
  Das wäre ein allgemeiner Weg an RLS vorbei.

## Folgen

- Kein Träger-Code, bevor der neue Owner in `policy.json` steht. Dieser ADR ist
  dafür die Voraussetzung.
- Die Umstellung auf ein Cookie je Schul-Subdomain betrifft alle OGS-Konten mit
  mehreren Schulen und braucht ein eigenes Ticket.
- Wer das Feriengast-Modell besitzt, entscheidet eine spätere ADR. Hier steht
  nur die Zugriffsregel.
- Nicht entschieden sind hier die Träger-Ebene im Settings-System und Personal
  über Schulen (#1417, #2817). Dafür sind eigene ADRs der Spec vorgesehen.
