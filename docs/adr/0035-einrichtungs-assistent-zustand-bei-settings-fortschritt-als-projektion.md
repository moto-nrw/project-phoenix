# Einrichtungs-Assistent: Zustand bei Settings, Fortschritt als Projektion

Status: proposed. Gilt für den Einrichtungs-Assistenten neuer Schulen aus
[#2832](https://github.com/moto-nrw/project-phoenix/issues/2832). Führt den
Projection-Owner `school-setup-view` ein, den
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) nur mit einer
Architekturentscheidung zulässt.

## Kontext

Eine neue OGS soll nach der Anlage durch den Operator ohne moto startklar
werden. Ein Assistent führt Schul-Admins durch sieben Schritte:

1. So arbeitet ihr: Anwesenheitsart, Betreuungsplan, feste Gruppen oder offene
   Betreuung, Eltern-App
2. Team einladen
3. Räume anlegen (nur bei Anwesenheit pro Raum)
4. Gruppen anlegen (nur bei festen Gruppen)
5. Kinder anlegen
6. Eltern einladen (nur wenn die Schule die Eltern-App nutzt)
7. Abschluss

Jeder Schritt lässt sich überspringen. Schritte 2 bis 6 gelten als erledigt,
sobald es mindestens einen passenden Datensatz gibt. Der Fortschritt gilt für
die ganze Schule, das Ausblenden des Assistenten nur für die Person.

Daraus entstehen zwei Arten von Daten:

- **Zustand, den der Assistent schreibt:** Antworten aus Schritt 1, die keine
  Einstellung sind (Eltern-App), übersprungene Schritte, Abschluss der Schule,
  Ausblenden pro Person.
- **Fortschritt, den der Assistent liest:** ob es Räume, Einladungen, Gruppen,
  Kinder und Eltern-Einladungen gibt. Diese Tabellen gehören fünf Ownern.

## Entscheidung

1. **Der Zustand gehört `settings-platform`.** Zwei neue Tabellen:
   `config.school_setups` (eine Zeile pro Schule: Eltern-App-Antwort,
   übersprungene Schritte, Abschlusszeitpunkt) und
   `config.school_setup_dismissals` (eine Zeile pro Schule und Konto). Beides
   ist Konfiguration der Schule wie `config.home_block_policies` und
   `config.home_layouts`, die `settings-platform` schon besitzt. Ein eigener
   Domain-Owner lohnt für zwei kleine Tabellen ohne eigene Fachregeln nicht.
2. **Der Fortschritt ist die Projektion `school-setup-view`** (Kind
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
3. **`settings-platform` setzt den Assistenten zusammen.** Die Routen unter
   `/api/school-setup` (Paket `modules/settings/inbound/setup`, Rolle `http`)
   verlangen `config:update`:

   | Route | Zweck |
   |---|---|
   | `GET /` | Zustand für die aufrufende Person |
   | `PUT /basics` | Anwesenheitsart und Eltern-App bestätigen |
   | `PUT /steps/{step}` | Schritt überspringen oder zurücknehmen |
   | `POST /complete` | Einrichtung für die Schule abschließen |
   | `PUT /dismissal` | Assistent für die Person aus- oder einblenden |

   Der Service `SchoolSetupService` (`services/config`) liest den eigenen
   Zustand und fragt die Projektion über den Consumer-Port
   `SchoolSetupProgress`. Er liefert je Schritt „gilt“, „erledigt“ und
   „übersprungen“. Welche Schritte gelten, folgt aus den Einstellungen
   `operations.presence_mode` und `operations.group_mode` und aus der
   Eltern-App-Antwort. Gruppenmodus und Betreuungsplan schreibt der Client
   über die vorhandene Settings-API. Das Paket
   `modules/settings/setup/compose` (Rolle `compose`) verdrahtet Speicher,
   Projektion, Service und Routen. Die Root-Komposition hängt die Routen über
   eine Liste von Modul-Routen ein, nicht über ein neues Feld in `api.API`.
4. **Die Anwesenheitsart darf der Admin genau während der Einrichtung
   setzen.** `operations.presence_mode` bleibt `AccessOperatorOnly`. Ein
   eigener Befehl von `settings-platform` schreibt den Wert für Schul-Admins
   nur, solange `config.school_setups.completed_at` leer ist. Er nutzt die
   Operator-Orchestrierung (`OperatorSettingsService`) innerhalb der
   Tenant-Transaktion der Anfrage: dieselbe Sperre `CheckPresenceModeSwitch`,
   derselbe Side-Effect-Hook, dieselbe Broadcast-Meldung, und Anwesenheitsart
   und Antworten werden gemeinsam gespeichert oder gar nicht. Nach dem
   Abschluss ändert nur noch moto die Anwesenheitsart. NFC und
   Web-Anwesenheit bleiben ausschließlich beim Operator.
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

Neu: Owner `school-setup-view` (Kind `projection`), Paket
`modules/schoolsetupview` (Rolle `postgres`), Read-Projection
`school-setup-progress` mit den fünf Tabellen oben und `"tenant_safe": true`.
Die Data-Objects `config.school_setups` und `config.school_setup_dismissals`
mit Write-Owner `settings-platform`. Owner-Id und Projection-Id unterscheiden
sich wie bei `operator-dashboard-view` / `operator-dashboard`.

Bei `settings-platform` kommen die Pakete `modules/settings/inbound/setup`
(`http`) und `modules/settings/setup/compose` (`compose`, externe Tests
`workflow-integration-test`) hinzu. Die Regeln `settings-platform.compose.*`,
`settings-platform.integration-test.*`,
`school-setup-view.integration-test.postgres` und
`root-composition.to.settings-platform-compose` hängen nur an diesen neuen
Paketen und Rollen. Architektur-Altlasten und Composition-Surface bleiben
unverändert (684 → 684, 641 → 641).

## Konsequenzen

- Die Projektion wiederholt Spaltennamen fremder Owner als SQL-Literale.
  Benennt ein Owner eine der fünf Tabellen oder Spalten um, muss er die
  Projektion im selben Change anpassen. Die Integrationstests der Projektion
  schlagen dann fehl.
- Die Eltern-App-Antwort ist bewusst keine Einstellung. Wird die Eltern-App
  später eine echte Einstellung (z. B. abhängig vom Tarif), ersetzt diese die
  Antwort, und die Spalte entfällt.
- Der Assistent erscheint nur Konten mit `config:update`. Das Ausblenden ist
  keine Berechtigungsgrenze: Jede Seite, auf die ein Schritt führt, prüft ihre
  Rechte selbst.
