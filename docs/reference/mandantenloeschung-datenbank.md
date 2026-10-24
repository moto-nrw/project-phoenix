# Mandantenlöschung: Datenbankvertrag (3837-b)

Stand: 23.10.2026. Refs #3837, Teil von #3846.
Grundlage: [ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md).

## Stand und Zuständigkeit

**Im Code umgesetzt:** Migration `1.15.481`, Owner `organization-tenancy`,
benannte Ausnahme `organization-tenancy.tenant-data-lifecycle`. Sie führt
keine neue HTTP-Route und keine allgemeine Cross-Owner-Berechtigung ein.
Die bisherigen Schreib-Owner bleiben unverändert. Die neue Nachweistabelle
`platform.school_erasures` und die begrenzte Abrechnungs-View
`platform.live_billing_key_date_counts` gehören zu Organisation & Tenancy.

**CLI und Dateiphase im Code umgesetzt (3837-c):**
[Runbook](../operations/tenant-erasure.md), Migration `1.15.490`.
Der technische Abschluss umfasst DB und lokale Dateien, nicht die manuellen
Nebensystemschritte oder den rechtlichen Gesamtabschluss.
3837-d ergänzt eine außerhalb von DB-/Upload-Sicherungen liegende
Löschungsliste und eine sperrende Restore-Prüfung. Der CLI bestätigt erst nach
dauerhaftem Listeneintrag Erfolg; `completed` in der DB alleine beweist diesen
externen Schritt nicht. [Betriebsverfahren](../operations/tenant-erasure.md).
Keine neue Migration oder DB-Rollenfreigabe in 3837-d. Produktiver Rollout,
Rollenvergabe und Wirksamkeit sind offen. Dieser Slice wurde nur an lokalen,
synthetischen Testdaten geprüft. Kein Zugriff auf Produktion oder Staging.

## Berechtigung und Vorbedingungen

Die neue Rolle `phoenix_school_erasure` ist `NOLOGIN NOINHERIT NOBYPASSRLS`.
Sie erhält ausschließlich Schema-Nutzung und die unten genannten
Funktionen, keine Tabellen- oder Sequenzrechte. Migration `1.15.490` ergänzt
die drei CLI-Funktionen für Probelauf, Trägerschulliste und Dateiphase. Kein HTTP-/Schul-/Operator-
Web-Login erhält Mitgliedschaft. Der spätere CLI-Zugang muss diese Rolle
gezielt übernehmen können; dessen Einrichtung ist ein Mensch-Schritt in
3837-c, nicht Teil dieser Migration.

Alle ausführenden Löschfunktionen sind `SECURITY DEFINER` mit festem
`search_path = pg_catalog`; SQL nennt Schemas ausdrücklich. `PUBLIC`,
`phoenix_auth`, `phoenix_tenant` und `phoenix_admin` dürfen sie nicht ausführen.
Der Aufrufer liefert keine SQL-Fragmente, Tabellen- oder Spaltennamen.
Die statischen Phasen `school_erasure_preflight`, `school_erasure_delete`
und `school_erasure_assert_empty` sind interne `SECURITY INVOKER`-Helfer.
Keine Laufzeitrolle, auch nicht die CLI-Rolle, darf sie direkt ausführen.
Nur der freigegebene Definer-Einstieg orchestriert sie innerhalb desselben
Rollback-Blocks. Das trennt Vorprüfung, Löschen und Restprüfung, ohne die
Auftragsbindung oder Transaktion aufzuteilen.
Migration 1.15.92 hatte DML-/Sequenzrechte auf `auth.mfa_overrides` an
`PUBLIC` vergeben. 1.15.481 ersetzt diese durch dieselben expliziten Rechte
für `phoenix_auth`, `phoenix_tenant`, `phoenix_admin` und `phoenix_demo`.
So erbt die neue CLI-Rolle keine allgemeinen Tabellenrechte. Die bestehenden
MFA-Aufrufer und deren RLS bleiben unverändert; die Einschränkung bleibt auch
bei einem Down erhalten.

Vor Auftragserstellung prüft das interne Verfahren die Weisung, Berechtigung,
Schule und Träger, Zeitpunkt, Rückgabe sowie anwendbare Abruf- und gesetzliche
Aufbewahrungspflichten. Die unten beschriebene gesperrte Abrechnungs-
aufbewahrung ist die ausdrücklich entschiedene Ausnahme. Weitere notwendige Aufbewahrungsausnahmen müssen vor
Auftragserteilung geklärt werden. Keine pauschale 30-Tage-Wartefrist.

Eine vorherige Soft-Delete-Bedienhandlung ist **nicht** nötig. Stattdessen
ist der explizit angelegte Auftrag mit aktiver Operator-ID und einer nicht
leeren, auf 120 Zeichen begrenzten Auftragsreferenz die technische Bestätigung.
Die Referenz ist eine interne Vorgangsnummer, kein Ort für Namen,
E-Mail-Adressen, Inhalte oder Geheimnisse. Ihre fachliche Freigabe und
Personendatenfreiheit prüft das spätere CLI-/Betriebsverfahren; die Datenbank
prüft nur Existenz, Länge und gebundene IDs.

## Aufrufvertrag für 3837-c

1. `platform.begin_school_erasure(school_id, operator_id, order_reference)`
   liefert die Nachweis-ID. Sie speichert die Träger-ID aus der Schule und
   legt `pending` an. Gleiche Schule, Operator-ID und Referenz liefern dieselbe
   ID, auch nach bereits erfolgter DB-Löschung. Abweichende Aufträge werden
   abgelehnt. Auftrag **separat committen**, bevor die Löschung beginnt.
2. Dateiinventar und nötige Zuordnungen vor dem DB-Schritt sichern. Die
   DB-Funktion entfernt auch Datei-Metadaten und Aufräumwarteschlangen.
   Unabhängig davon alle tenant-bezogenen Upload-Namespaces einschließlich
   der Alt-Avatarablage `avatars/<tenantId>/` behandeln. Globale Kontoavatare
   nicht mit der Schule löschen. Details und dauerhafte Wiederholbarkeit
   der Dateiphase sind Aufgabe von 3837-c.
3. `platform.erase_school(school_id, erasure_id)` liefert JSON mit `status`,
   `row_counts` und gegebenenfalls `error_code`. Nur `db_done` bestätigt den
   DB-Schritt. Bei einem Fehler rollt der innere Transaktionsblock sämtliche
   Löschungen und Trigger-Schreibvorgänge zurück. Außerhalb dieses Blocks
   speichert er `failed`, Fehlerzeit und ausschließlich den SQLSTATE.
   **Auch `failed` committen**, danach muss das CLI mit Fehler enden.
   Ein technisch erfolgreicher SQL-Aufruf ist kein erfolgreicher Löschauftrag.
4. `platform.read_school_erasure(erasure_id)` liefert den datensparsamen
   Nachweis. Die DB-Funktion setzt weder `files_done_at` noch `completed_at`.
   Migration `1.15.490` ergänzt die Rollenfunktion
   `platform.school_erasure_file_phase` für Inventar, Fehler, `files_done`
   und `completed`; Aufruf und Vorbedingungen stehen im
   [CLI-Runbook](../operations/tenant-erasure.md). Ein erneuter
   DB-Aufruf nach `db_done_at` ist unabhängig vom späteren Phasenstatus ein
   No-op mit dem bestehenden Ergebnis. Ein fehlgeschlagener DB-Aufruf kann
   mit derselben Auftrags-ID erneut ausgeführt werden.
5. `platform.delete_expired_school_erasures()` löscht abgeschlossene
   Nachweise drei Jahre nach `completed_at`. Offene und fehlgeschlagene
   Aufträge sind noch nötiger Arbeitsbestand und werden nicht als erledigte
   Nachweise verworfen. Der regelmäßige Aufruf gehört in das interne
   Verfahren von 3837-c; dieser Slice richtet keinen Scheduler ein.

Verbindungsabbruch, Prozessverlust oder Rollback durch den Aufrufer können
auch die Fehlerstatus-Schreiboperation verhindern. Dann bleibt der zuvor
committete Auftrag offen; ohne `db_done_at`/`completed_at` gibt es keinen
Erfolgsnachweis. Vor Fortsetzung Status erneut lesen. Keine Roh-DB-Fehler mit
möglichen Datensatzinhalten in Logs oder Ausgaben übernehmen.

## CLI-Ergänzung (3837-c, Migration 1.15.490)

- `platform.school_erasure_preview(school_id, operator_id)` liefert nur
  Anzahlen je Planzieltabelle, prüft aktive Operator-ID und Schulwurzel und
  schreibt keinen Auftrag. Statische Selektoren entsprechen dem DB-Plan.
- `platform.school_erasure_schools(organization_id, operator_id, reference)`
  liefert Schul-IDs des Trägers sowie bereits gelöschte Schulen desselben
  Vorgangs für die Wiederaufnahme. Kein Löschen der Trägerwurzel.
- `platform.school_erasure_file_phase(erasure_id, phase, counts)` speichert
  vor dem DB-Schritt neun Dateiart-Anzahlen und `files_inventory_at`.
  Nach `db_done_at` sind `files_done`, danach `completed` erlaubt; ein
  Dateifehler setzt `failed` mit dem generischen Code `F0001`. Der CLI muss
  alle Namespaces entfernen und erneut auf Leere prüfen, bevor er diese
  Zustände meldet. Die DB kann das Dateisystem nicht selbst prüfen.

Die Funktionen sind `SECURITY DEFINER` mit festem `search_path`, ohne
allgemeine Tabellenrechte für die CLI-Rolle. Nur die Rollenfunktion darf
Inventar und Abschluss schreiben. Der technische Abschluss ersetzt keine
menschliche Prüfung der Nebensysteme oder Sicherungskopien. Bestehende reine
DB-Aufträge ohne Inventar können nicht automatisch abgeschlossen werden.

## Umfang und Schutz vor Teilverlust

Die Migration installiert einen eingefrorenen, generierten Plan und statische
SQL-Blöcke aus dem
[gemeinsamen Katalog](datenkatalog.md), keinen zweiten fachlichen Katalog.
Er umfasst 235 Löschziele: `tenant`, `tenantChild` und die Schulwurzel.
Migration `1.15.519` installiert Plan, Sperrliste, Löschung, Restprüfung und
Probelauf erneut mit den sieben schulbezogenen Mittagessen-Tabellen
(Bestellungen, Anmeldeänderungen, Caterer-Einladungen und Belieferungen, #3873).
Der schulübergreifende Caterer-Betrieb und seine Konten bleiben erhalten.
`platform.billing_key_date_counts.school_id` ist die explizite Ausnahme nach
[ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md#ergänzung-aufbewahrung-der-abrechnungszählungen-3837-b-runde-2).
Diese Rechnungsreferenzen werden behalten und nach der Schulentfernung gesperrt,
nicht gelöscht. Ihre globale Konfiguration bleibt erhalten. `account`, `platform`, `demo` ohne Mandantenscope und die
Trägerwurzel bleiben unangetastet. `platform.demo_school_states` ist dagegen
als `tenant` eingestuft und wird für die konkrete Schule entfernt.

Die Träger-ID im Nachweis berechtigt **nicht** zur Löschung anderer Schulen
oder des Trägers. Ein Trägerauftrag muss später alle berechtigten Schulen
separat behandeln und erst danach die Trägerwurzel prüfen. Gemeinsame Konten
werden nicht gelöscht: Die Schulzuordnungen verschwinden, danach greift der
bestehende Job `auth.erase_unused_accounts`, soweit keine andere Schule das
Konto benötigt. Globale Operator-Protokolle werden nicht pauschal gelöscht.

Der DB-Schritt sperrt alle Planzieltabellen und eingehenden FK-Quelltabellen
in stabiler Reihenfolge mit `SHARE ROW EXCLUSIVE`. Das ist ein bewusstes
administratives Wartungsfenster: Schreibzugriffe auch anderer Schulen können
warten; normale Lesezugriffe bleiben möglich. Der spätere CLI-Aufrufer muss
passende Lock-/Statement-Zeitlimits setzen und fehlgeschlagene Versuche
sichtbar behandeln. Keine App-Server werden für die Tests gestartet.

Fünf Tabellen hatten vorher keinen direkten Schul-FK: die beiden Audit-
Migrationsbackups, `active.presence_backfill_batches`,
`auth.mfa_email_challenges` und `auth.parent_declaration_permission_grants`.
1.15.481 ergänzt validierte Schul-FKs. So scheitert auch ein vorher wartender
Schreibvorgang, wenn er nach dem Lösch-Commit neue Mandantenzeilen anlegen
will. Die registrierte `Precondition` zählt verwaiste Referenzen bereits in
`migrate preflight`, bevor der Deploy die App stoppt. `Up` wiederholt die
Prüfung; die validierten FKs sichern auch konkurrierende Schreibvorgänge ab.
Vor dem Deploy sind die unten genannten Mensch-Schritte nötig. `TestEveryTenantTableHasASchoolErasureWriteFence`
verhindert neue ungeprüfte Nachlauf-Lücken.

Vor der ersten Löschung prüft die Funktion jede eingehende FK-Kante. Eine
Zeile außerhalb des Auftrags, die auf dessen Daten verweist, führt zu
`failed`, auch bei `CASCADE` oder `SET NULL`. So kann ein alter einfacher FK
keine fremde Schulzeile löschen oder ändern. Restrict-/NO-ACTION- und
CASCADE-Kinder kommen vor ihren Eltern; SET-NULL-/DEFAULT-Kanten können
Zyklen auflösen, nachdem ihre Zeilen die Scope-Prüfung bestanden haben.
Bekannte Kalender- und Essensstatus-Löschtrigger erhalten explizite
Nachlaufabhängigkeiten. Trigger werden nicht abgeschaltet.

Die Restprüfung kontrolliert alle planmäßigen Mandantenzeilen und die
Schulwurzel. Indirekte Kindtabellen werden zusätzlich über vorab gesicherte
Schlüssel geprüft, damit ein unterdrücktes DELETE nicht durch eine bereits
entfernte Elternzeile unsichtbar wird. Erst wenn alles leer ist, schreibt die
Funktion `db_done` und die Anzahlen je Tabelle. Die Zahlen zählen den Bestand
unmittelbar vor der jeweiligen Löschung, einschließlich erzeugter Nachläufe.
Ein Rollback verwirft sämtliche Anzahlen des Fehlversuchs.

## Nachweis und Prüfung

`platform.school_erasures` hat absichtlich keine FKs auf Schule, Träger oder
Operator. Der Nachweis enthält nur interne IDs, Vorgangsreferenz,
Katalog-/Verfahrensversion, Phasenzeitpunkte, Status, Anzahlen und SQLSTATE.
Ein Down mit vorhandenen Aufträgen wird verweigert. Gelöschte Fachdaten
lassen sich durch einen Migrationsrollback nicht wiederherstellen.

`TestSchoolErasure*` prüft lokal:

- zwei breite synthetische Schulen einschließlich Audit, Outbox, Settings,
  Kinder, Personal, Eltern-Kind-Zuordnung, Anmeldungen und indirekte Tabellen;
- vollständige Leere von A und unveränderten Zeilenbestand von B pro
  Planzieltabelle, gemeinsame und anschließend unzugeordnete Konten;
- injizierten Fehler mitten im Löschen, vollständigen Rollback, `failed`
  ohne Fehlerinhalt und erfolgreiche Wiederholung;
- fremde CASCADE-Referenzen, verweigerte Web-Rollen, falsche Auftragsbindung,
  fehlende Tabellen im Plan, unterdrückte Löschungen und Nachweisfrist.

Der Planwächter vergleicht die installierten Selektoren und statischen
DELETE-Blöcke mit dem aktuellen Katalog und prüft deren FK-/Trigger-Reihenfolge.
Eine SHA-256-Signatur aller eingehenden FK-Definitionen verhindert die
Ausführung bei ungeprüften Schemaänderungen. Neue Tabellen müssen auch dessen Schemawächter passieren. Neue
DELETE-Trigger verlangen eine Prüfung ihrer Seiteneffekte und Abhängigkeiten.
Ein Laufzeitwächter verweigert unbekannte `tenant_id`-Tabellen zusätzlich.
Das belegt Code und lokale Tests, nicht das produktive DB-Schema oder einen
abgeschlossenen Kundenauftrag.

## Gesperrte Abrechnungsreferenzen

**Im Code umgesetzt:** Die Zählungen überdauern die Schule unverändert. Die
bei Erfassung gespeicherten Schul-/Trägernamen gehören nach Migration
1.15.416 zur Rechnungsreferenz: spätere Umbenennungen oder Löschungen dürfen
diese historische Zuordnung nicht ändern. Deshalb werden sie nicht durch
neutrale aktuelle Namen ersetzt. Es sind Organisationsnamen und Summen,
keine Daten betroffener Personen; kein Export an Dritte.

`phoenix_admin` hat `BYPASSRLS`. Deshalb entzieht die Migration dieser Rolle
`SELECT` und `INSERT` auf der Basistabelle. Schreiben erfolgt ausschließlich
über `platform.capture_billing_key_date_counts(...)`, einen eng berechtigten
`SECURITY DEFINER`-Schreiber mit unverändertem Insert-once-Verhalten. So
erhält der Operator auch nicht die von `ON CONFLICT` benötigten Basistabellen-
Leserechte. Migration und passendes Backend müssen gemeinsam ausgerollt werden;
das alte Backend kann mit diesen eingeschränkten Rechten nicht arbeiten.
Der reguläre Abrechnungsleser und die Prüfung
bereits erfasster Perioden nutzen ausschließlich die `security_barrier`-View
`platform.live_billing_key_date_counts`, die nur Zeilen mit noch vorhandener
Schulwurzel zeigt. Das gilt auch für normale Operator-Lesewege. Die separate
Rolle `phoenix_billing_retention` ist `NOLOGIN NOINHERIT NOBYPASSRLS`, darf die
Basis lesen und die Fristbereinigung aufrufen, aber keine Daten frei ändern.
Keine Web-/Schul-/CLI-Löschrolle erhält Mitgliedschaft in dieser Rolle.
Ein verspätetes Billing-INSERT darf den Rechnungsnachweis noch ergänzen; ohne
Schulwurzel wird es im regulären Leser nicht sichtbar. Das bisherige
Billing-Race ist damit kein Löschproblem mehr.

`platform.delete_expired_billing_key_date_counts()` löscht nach zehn Jahren
ab Ende des Kalenderjahres von `key_date`, unabhängig vom Schulbestand.
Beispiel: Stichtage aus 2026 laufen am 31.12.2036 aus und werden ab
01.01.2037 gelöscht. Die Berechnung nutzt das Kalenderjahr in Europe/Berlin.
Die Frist ist die vorsorgliche Entscheidung des Orchestrators zu § 147 AO /
§ 257 HGB, keine abgeschlossene rechtliche Einordnung. Die Bestätigung bleibt
im Recht-Slice offen. 3837-c muss den regelmäßigen Aufruf mit der gesonderten
Abrechnungsberechtigung einbinden; dieser Slice liefert die getestete
Funktion, keinen Scheduler. Produktive Rollenvergabe, Sperrwirksamkeit und
Fristausführung sind menschlich nachzuweisen.

## Mensch-Schritte vor Deploy und erster Löschung

1. **Vor dem Deploy** die nur lesende
   [SQL-Vorprüfung](../operations/sql/mandantenloeschung-vorpruefung-3837.sql)
   mit der freigegebenen Migrationsrolle ausführen. Sie muss RLS umgehen
   können; `row_security = off` verhindert unbemerkt unvollständige Zählungen.
   Die Datei enthält `BEGIN READ ONLY`, zeigt nur die fünf Spaltenbezeichnungen
   und Anzahlen, keine IDs oder Zeileninhalte. Freigabe nur bei fünfmal
   `orphaned_rows = 0` und `all_school_references_valid = true`.
2. Bei Treffern den Deploy stoppen. Im freigegebenen internen Vorgang je
   Tabelle Zweck, Ursache und Anzahl prüfen, ohne Zeileninhalte in Logs,
   Tickets oder PRs zu kopieren. Nie pauschal einem anderen Mandanten zuordnen
   oder `NULL` setzen. Existiert die Schule tatsächlich noch, muss ihre
   korrekte Zuordnung anhand der autorisierten Unterlagen geklärt werden.
   Ist sie nachweislich endgültig entfernt und besteht kein Aufbewahrungs-
   oder Wiederherstellungszweck mehr, kann der berechtigte Betrieb ausschließlich
   die verwaisten Hilfszeilen dieser Tabelle löschen: in einer separaten
   Transaktion, mit dem identischen Prädikat `tenant_id IS NOT NULL AND NOT
   EXISTS (SELECT FROM platform.schools s WHERE s.id = r.tenant_id)`, vorher
   gezähltem Bestand und Kontrolle der betroffenen Anzahl. Bei Abweichung
   zurückrollen; erst nach Freigabe committen. Keine automatische Reparatur.
   `auth.mfa_email_challenges.tenant_id IS NULL` ist zulässig und kein Treffer.
3. Bei `audit.room_color_migration_backup` und
   `audit.wc_alias_migration_backup` zusätzlich prüfen, ob der historische
   Migrations-Restore-Zweck vollständig entfallen ist. Dann eine gesonderte
   geprüfte Migration zur Entfernung der ganzen Backup-Tabelle vorbereiten,
   samt Anpassung von Katalog und Erasure-Plan. Nicht vor diesem Deploy spontan
   die Tabelle droppen: 1.15.481 benötigt sie für FK und Löschplan. Bis zu einer
   solchen Migration nur belegte verwaiste Zeilen wie in Schritt 2 behandeln.
   Vorprüfung danach erneut ausführen; `migrate preflight` muss bestehen.
4. Vor der ersten Löschung für alle `SECURITY DEFINER`-Funktionen den
   tatsächlichen Produktions-Owner prüfen. Er muss die betroffenen RLS-
   Tabellen vollständig sehen und löschen können, etwa als freigegebener
   Superuser oder mit `BYPASSRLS`; die CLI-Rolle selbst darf dies nicht.
   Ownerprüfung und Rollenmitgliedschaften ohne Zugangsdaten dokumentieren.
   Nachweise im Produktivsystem sind noch offen.
5. Das Wartungsfenster für 235 Löschziele und eingehende FK-Quellen einplanen.
   `SHARE ROW EXCLUSIVE` kann Schreibzugriffe anderer Schulen blockieren.
   Lock-/Statement-Zeitlimits und die Behandlung eines abgebrochenen Versuchs
   bleiben Aufgabe von 3837-c. Der Write-Fence-Wächter prüft `tenant_id`, nicht
   beliebige `school_id`-Spalten; die gesperrten Billing-Referenzen und die
   absichtlich FK-freien Löschbelege dürfen die Schule überdauern.
