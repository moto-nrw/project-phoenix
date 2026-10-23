# Mandantenlöschung: Datenbankvertrag (3837-b)

Stand: 23.10.2026. Refs #3837, Teil von #3846.
Grundlage: [ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md).

## Stand und Zuständigkeit

**Im Code umgesetzt:** Migration `1.15.481`, Owner `organization-tenancy`,
benannte Ausnahme `organization-tenancy.tenant-data-lifecycle`. Sie führt
keine neue HTTP-Route und keine allgemeine Cross-Owner-Berechtigung ein.
Die bisherigen Schreib-Owner bleiben unverändert. Nur das neue Datenobjekt
`platform.school_erasures` gehört zusätzlich zu Organisation & Tenancy.

**Nicht als vollständiges Verfahren freigegeben:** 3837-c liefert CLI,
Dateiinventar, Dateilöschung, Nebensystemschritte, Gesamtabschluss und Runbook.
3837-d behandelt die Wiederholung nach Restore. Produktiver Rollout,
Rollenvergabe und Wirksamkeit sind offen. Dieser Slice wurde nur an lokalen,
synthetischen Testdaten geprüft. Kein Zugriff auf Produktion oder Staging.

## Berechtigung und Vorbedingungen

Die neue Rolle `phoenix_school_erasure` ist `NOLOGIN NOINHERIT NOBYPASSRLS`.
Sie erhält ausschließlich Schema-Nutzung und die vier unten genannten
Funktionen, keine Tabellen- oder Sequenzrechte. Kein HTTP-/Schul-/Operator-
Web-Login erhält Mitgliedschaft. Der spätere CLI-Zugang muss diese Rolle
gezielt übernehmen können; dessen Einrichtung ist ein Mensch-Schritt in
3837-c, nicht Teil dieser Migration.

Alle ausführenden Funktionen sind `SECURITY DEFINER` mit festem
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
Aufbewahrungspflichten. Besteht eine notwendige Ausnahme, darf dieser
unbedingte DB-Löschauftrag noch nicht ausgeführt werden. Der Slice führt
keine neue Aufbewahrungsausnahme oder pauschale 30-Tage-Wartefrist ein.

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
   Die Statuswerte `files_done` und `completed` sind für 3837-c vorbereitet,
   aber noch nicht über eine CLI-Rollenfunktion beschreibbar. Ein erneuter
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

## Umfang und Schutz vor Teilverlust

Die Migration installiert einen eingefrorenen, generierten Plan und statische
SQL-Blöcke aus dem
[gemeinsamen Katalog](datenkatalog.md), keinen zweiten fachlichen Katalog.
Er umfasst `tenant`, `tenantChild`, die Schulwurzel und die losen
`platform.billing_key_date_counts.school_id`-Referenzen. Diese Zählungen
werden für die beauftragte Schule ausdrücklich mit gelöscht; es wird keine
unbegründete gesetzliche Aufbewahrung behauptet. Ihre globale Konfiguration
bleibt erhalten. `account`, `platform`, `demo` ohne Mandantenscope und die
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
