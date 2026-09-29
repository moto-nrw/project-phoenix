# Einrichtungs-Assistent: Zustand bei Settings, Fortschritt als Projektion

Status: proposed. Gilt für den Einrichtungs-Assistenten neuer Schulen aus
[#2832](https://github.com/moto-nrw/project-phoenix/issues/2832). Führt den
Projection-Owner `school-setup` ein, den
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) nur mit einer
Architekturentscheidung zulässt.

## Kontext

Eine neue OGS soll nach der Anlage durch den Operator ohne moto startklar
werden. Ein Assistent führt Schul-Admins durch sechs Schritte:

1. Team einladen
2. Räume anlegen (nur bei Anwesenheit pro Raum)
3. Gruppen anlegen (nur bei festen Gruppen)
4. Kinder anlegen
5. Eltern einladen
6. Abschluss

Jeder Schritt lässt sich überspringen. Schritte 1 bis 5 gelten als erledigt,
sobald es mindestens einen passenden Datensatz gibt. Der Fortschritt gilt für
die ganze Schule, das Ausblenden des Assistenten nur für die Person.

Der Assistent fragt nicht, wie die OGS arbeitet. Eine neue Schule startet mit
den Standardwerten der Einstellungen; Gruppenmodus und Betreuungsplan ändert
sie in den Einstellungen, die Anwesenheitsart ändert moto. Ein erster Schritt
„So arbeitet Ihre OGS“ mit Anwesenheitsart, Betreuungsplan, Gruppenmodus und
Eltern-App war vorgesehen und wurde vor dem Merge wieder gestrichen.

Daraus entstehen zwei Arten von Daten:

- **Zustand, den der Assistent schreibt:** übersprungene Schritte, Abschluss
  der Schule, Ausblenden pro Person.
- **Fortschritt, den der Assistent liest:** ob es Räume, Einladungen, Gruppen,
  Kinder und Eltern-Einladungen gibt. Diese Tabellen gehören fünf Ownern.

## Entscheidung

1. **Der Zustand gehört `settings-platform`.** Zwei neue Tabellen:
   `config.school_setups` (eine Zeile pro Schule: übersprungene Schritte,
   Abschlusszeitpunkt) und
   `config.school_setup_dismissals` (eine Zeile pro Schule und Konto). Beides
   ist Konfiguration der Schule wie `config.home_block_policies` und
   `config.home_layouts`, die `settings-platform` schon besitzt. Ein eigener
   Domain-Owner lohnt für zwei kleine Tabellen ohne eigene Fachregeln nicht.
2. **Der Fortschritt ist eine Projektion des Owners `school-setup`** (Kind
   `projection`, Paket `modules/schoolsetupview`, Read-Projection
   `school-setup-progress`). Sie liest mit festen `SELECT`-Anweisungen:

   | Tabelle | Write-Owner | Frage |
   |---|---|---|
   | `facilities.rooms` | `facilities` | Gibt es einen Raum? |
   | `auth.invitation_tokens` | `identity-access` | Hat die Schule selbst jemanden eingeladen? (`created_by IS NOT NULL`; Einladungen des Operators haben keinen Ersteller) |
   | `education.groups` | `school-structure` | Gibt es eine Gruppe? |
   | `users.student_school_memberships` | `school-membership` | Gibt es ein aktives Kind? |
   | `auth.guardian_invitations` | `identity-access` | Gibt es eine Eltern-Einladung? |

   Jede Frage ist ein `EXISTS`. Alle fünf laufen in einer Anweisung, damit das
   Query-Budget bei eins bleibt. Die Alternative wären fünf Owner-Queries hinter
   fünf Consumer-Ports für eine reine Ja/Nein-Anzeige. Das kostet mehr, als es
   schützt; dieselbe Abwägung wie in
   [ADR 0033](0033-operator-dashboard-view-is-a-projection-owner.md).
   Die Projektion besitzt keine Tabelle und schreibt nichts. Sie ist kein
   persistentes Read-Model.
3. **Derselbe Owner `school-setup` setzt den Assistenten zusammen.** Er liest
   die Projektion und hält den eigenen Zustand; die Tabellen bleiben bei
   `settings-platform`. In die Einstellungen schreibt er nicht. Die Routen
   unter `/api/school-setup` (Paket `modules/schoolsetup/http`, Rolle `http`)
   verlangen `config:update`:

   | Route | Zweck |
   |---|---|
   | `GET /` | Zustand für die aufrufende Person |
   | `PUT /steps/{step}` | Schritt überspringen oder zurücknehmen |
   | `POST /complete` | Einrichtung für die Schule abschließen |
   | `PUT /dismissal` | Assistent für die Person aus- oder einblenden |

   Der Service (`modules/schoolsetup/internal/application`) liest den eigenen
   Zustand und fragt die Projektion über den Consumer-Port `Progress` des
   öffentlichen Vertrags. Er liefert je Schritt „gilt“, „erledigt“ und
   „übersprungen“. Welche Schritte gelten, folgt aus den Einstellungen
   `operations.presence_mode` und `operations.group_mode`; die übrigen
   Schritte gelten immer. Das Paket `modules/schoolsetup/compose`
   (Rolle `compose`) verdrahtet Speicher, Projektion und Service; die
   Root-Komposition baut daraus die Routen und hängt sie über eine Liste von
   Modul-Routen ein, nicht über ein neues Feld in `api.API`. Der Speicher
   liegt bei den Tabellen, die er schreibt: `database/repositories/config`
   (`settings-platform`, Rolle `postgres`) erfüllt den Port `Store` des
   öffentlichen Vertrags.
4. **Die Anwesenheitsart bleibt beim Operator.** `operations.presence_mode`
   bleibt `AccessOperatorOnly`, auch während der Einrichtung. Der Assistent
   liest den Wert nur, um zu entscheiden, ob der Schritt „Räume“ gilt. Die
   Anwesenheitsart klärt moto mit der Schule beim Onboarding. NFC und
   Web-Anwesenheit bleiben ebenfalls ausschließlich beim Operator.
5. **Bestehende Schulen sehen den Assistenten nicht.** Die Migration legt für
   jede vorhandene Schule eine Zeile mit gesetztem `completed_at` an. Eine
   Schule ohne Zeile gilt als neu. Die Zeile entsteht beim ersten Schreiben
   des Assistenten.

## Was `tenant_safe` hier bedeutet

Alle Lesezugriffe der Projektion laufen in der Tenant-Transaktion
(`tenant.WithTenantTx`), also unter RLS, und jede Anweisung filtert zusätzlich
auf `tenant_id`. Das ist die Invariante: Ein Aufruf außerhalb der
Tenant-Transaktion bricht diese Entscheidung.

## Policy-Registrierung

Neu: Owner `school-setup` (Kind `projection`) mit den Paketen
`modules/schoolsetupview` (Rolle `postgres`, die Projektion),
`modules/schoolsetup` (`public`), `modules/schoolsetup/internal/application`
(`application`), `modules/schoolsetup/compose` (`compose`, externe Tests
`workflow-integration-test`) und `modules/schoolsetup/http` (`http`). Dazu die
Read-Projection `school-setup-progress` mit den fünf Tabellen oben und
`"tenant_safe": true`. Owner-Id und Projection-Id unterscheiden sich wie bei
`operator-dashboard-view` / `operator-dashboard`.

Die Data-Objects `config.school_setups` und `config.school_setup_dismissals`
behalten den Write-Owner `settings-platform`; ein Projection-Owner besitzt
keine Tabelle. Deshalb liegt der Speicher in
`database/repositories/config` und erfüllt dort den Port `Store` des
öffentlichen Vertrags.

Dass der Assistent ein eigener Owner ist und nicht ein weiteres Paket von
`settings-platform`, ist keine Geschmacksfrage: Regeln gelten je Owner und
Rolle. Neue Kanten für die Rolle `compose` von `settings-platform` würden auch
dem schon vorhandenen Paket `modules/settings/compose` (#2736) erlaubt und
wären damit eine Lockerung der Policy. Die Regeln `school-setup.*`,
`settings-platform.postgres.school-setup-public` und
`root-composition.to.school-setup-*` hängen nur an den neuen Paketen und
Rollen. Architektur-Altlasten und Composition-Surface bleiben unverändert.

## Konsequenzen

- Die Projektion wiederholt Spaltennamen fremder Owner als SQL-Literale.
  Benennt ein Owner eine der fünf Tabellen oder Spalten um, muss er die
  Projektion im selben Change anpassen. Die Integrationstests der Projektion
  schlagen dann fehl.
- Der Schritt „Eltern einladen“ gilt für jede Schule. Nutzt eine Schule die
  Eltern-App nicht, überspringt sie ihn. Wird die Eltern-App später eine echte
  Einstellung (z. B. abhängig vom Tarif), kann sie den Schritt ausblenden.
- Der Assistent erscheint nur Konten mit `config:update`. Das Ausblenden ist
  keine Berechtigungsgrenze: Jede Seite, auf die ein Schritt führt, prüft ihre
  Rechte selbst.
