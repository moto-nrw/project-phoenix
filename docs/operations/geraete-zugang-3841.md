# Gerätezugänge umstellen: Slices 3841-a und 3841-b

## Stand und Zuständigkeit

**Im Code umgesetzt:** Migration 1.15.474 ersetzt `iot.devices.api_key`
atomar durch eindeutige SHA-256-Fingerprints und eine maskierte Kennung.
Anlage und Rotation liefern den Klartext einmalig. Listen, Details und
Schultransfers liefern ihn nicht. Neue Operator-Keys werden ausschließlich
mit dem bestehenden kryptografischen Generator erzeugt (32 Zufallsbytes).
IoT nutzt unverändert `Authorization: Bearer` und die bisherigen `iot.*`-Codes.

**Produktiv nachzuweisen, hier nicht ausgeführt:** Auslieferung beider PRs,
Umstellung aller Kiosk-Startadressen, Rotation sämtlicher Bestands-Keys,
Bereinigung alter Zugangsspuren und Funktionstest je Gerät. Das ist
Mensch-Slice 3841-g. Bis zur Rotation können alte, manuell vergebene Keys
weiterhin schwach sein. Hashing erhöht deren ursprüngliche Entropie nicht.

**Offen:** Ende des Query-Übergangs in Folge-Slice 3841-k. Bis 3841-g für alle
Geräte nachgewiesen ist, bleibt PyrePortal mit alten Startadressen kompatibel.
Diese Anleitung ersetzt weder den PIN- noch den MFA-Slice.

## Mensch-Schritte für 3841-g

1. Geräteinventar nur mit Geräte-ID, Schule, Plattform und Status anlegen.
   Keine Keys, PINs, vollständigen Startadressen oder Screenshots mit Keys
   in Tickets, Chat, Nachweisdateien oder Shell-History ablegen.
2. Backend und Operator-UI gemeinsam ausliefern. Migration und Backend dürfen
   nicht mit einem alten Backend parallel laufen: Die Klartextspalte entfällt.
   Bestehende Bearer-Keys bleiben unverändert gültig. Backendfehler durch
   Vorwärtskorrektur beheben; ein automatisches Down wird bewusst abgelehnt.
3. Gekoppeltes PyrePortal ausliefern. An jedem Gerät zunächst die bestehende
   Startadresse von Query auf Fragment umstellen: `#key=<Geräteschlüssel>`.
   Nur lokal am Gerät oder im zugriffsbeschränkten Geräte-Verwaltungskanal
   arbeiten. Keine Startadresse in gemeinsam sichtbaren Protokollen ablegen.
   Weitere Parameter bleiben vor dem Fragment. Neu laden, Anmeldung und
   Neustart prüfen. Der importierte Key wird aus der Adresszeile entfernt.
4. Erst danach jeden Bestands-Key über „API-Key ändern“ neu erzeugen und
   unmittelbar am zugehörigen Gerät als Fragment importieren. Der alte Key
   ist sofort ungültig. Pro Gerät ein Wartungsfenster vorsehen. Den neuen Key
   nur aus dem einmaligen Dialog übernehmen. Bei Verlust erneut rotieren,
   nicht aus Backups oder Logs zurückholen. Danach Neustart, Anmeldung und
   mindestens einen vorgesehenen IoT-Ablauf prüfen.
5. Dauerhaft gespeicherte Startadressen auf verbleibende Query-Keys prüfen.
   Bei Browserprofilen ohne lokalen Speicher ist ein erneuter Fragment-Import
   erforderlich. Geschützter Gerätespeicher und der gespeicherte Startlink
   sind selbst Zugangsgeheimnisse. Keine gemeinsam genutzten Browserprofile.
6. Access-Logs des Kiosk-Hosts, dessen vorgeschalteter Dienste sowie lokale
   Browser-Historien und Verwaltungsprotokolle auf alte Query-URLs prüfen.
   Bereinigung durch die zuständige Person nach geltender Aufbewahrung und
   etwaiger Beweissicherung durchführen; keine ungeprüften pauschalen Deletes.
   Protokoll-Konfiguration so ändern, dass Zugangsdaten nicht erfasst werden.
   Rotation macht dort verbliebene alte Keys unbrauchbar.
7. Alte Datenbankkopien, WAL und Sicherungen können noch Klartext enthalten.
   Der Spaltenabbau ist keine sichere Löschung alter Datenträgerseiten.
   Sicherungen zugriffsgeschützt nach dem bestehenden Sicherungszyklus
   auslaufen lassen. Nach Wiederherstellung vor Freigabe Migration erneut
   anwenden und bereits widerrufene Keys erneut widerrufen bzw. rotieren.
8. Im Inventar Datum, geprüfte Versionen, Neustart/Anmeldung erfolgreich,
   Rotation abgeschlossen, Log-Bereinigung und zuständige Person bestätigen.
   Ein Teilfehler bleibt ausdrücklich offen. 3841-k erst freigeben, wenn alle
   Geräte keine Query-Keys mehr benötigen. Keine Keys als Nachweis speichern.

## Lokale Prüfung

Die Migration wird in einer isolierten Test-DB am nachgebildeten Bestand
geprüft: Hash eines bekannten öffentlichen SHA-256-Testvektors, maskierter
Kurz-Key, null-Key eines zweiten Mandanten und entfernte Klartextspalte.
Die Fleet-Vertragstests prüfen Bearer-Lookup, einmalige Ausgabe, spätere
Reads/Listen ohne Klartext und fremde Mandanten. Die Operator-Tests prüfen
Rotation, abgelehnte manuelle Keys und Hash-Erhalt beim Schultransfer.
PyrePortal-Tests prüfen Fragment-/Query-Import, Vorrang des neuen Fragments,
URL-Bereinigung, lokalen Speicher und Neustart.

Diese lokalen Prüfungen sind kein Nachweis des produktiven Zustands.

## Vorschlag AVV/TOM

Anhang 1 Ziff. 2, Geräteschlüssel-Teil konkretisieren:

> Geräteschlüssel werden individuell aus kryptografischen Zufallswerten
> erzeugt und sind einzeln widerrufbar. Im laufenden Datenbestand werden
> ausschließlich SHA-256-Fingerprints gespeichert. Der Schlüssel wird nur bei
> Anlage oder Rotation einmalig ausgegeben. Terminals übertragen ihn an die
> API im Authorization-Header, nicht in URL-Pfad oder Query. Die einmalige
> lokale Einrichtung nutzt ein anschließend entferntes URL-Fragment.

Erst nach Nachweis von 3841-g als produktiv bestätigt kennzeichnen.
PIN/MFA-Aussagen sind davon unabhängig offen. Während des Übergangs ist die
Query-Kompatibilität ausdrücklich zu benennen. Die Aussage „nicht in URLs“
meint keine Übertragung an Webserver; lokal wird zur Einrichtung ein Fragment
verwendet.

## PIN-Auflösung ohne Env-Fallback: Slice 3841-b

**Im Code umgesetzt:** PIN-geschützte IoT-Aufrufe lesen ausschließlich
`security.ogs_device_pin` der Geräteschule. Ein Lesefehler oder explizit leerer
Wert lehnt mit dem bestehenden Envelope `401 / iot.device_pin_invalid` ab.
Ein fehlender Resolver bricht den Aufbau ab. PIN-freie Geräteaufrufe bleiben
PIN-frei. PyrePortal behandelt diesen Code bereits; Header und Wire-Vertrag
bleiben kompatibel, ein Übergangsfenster ist dafür nicht erforderlich.

**Unverändert und offen:** Die schulweite vierstellige PIN, Registry-Default
und Speicherung bleiben nach Nutzerentscheidung vom 23.10.2026 unverändert.
Dieser Slice erfüllt nur den Teil „Schul-Settings ohne Env-Fallback“ von AK1.
Er erfüllt weder individuelle Einrichtung ohne Standard-PIN noch Hashing.
Anmeldeversuchsschutz (3841-c), MFA (3841-f) und produktive Nachweise sind
nicht Gegenstand dieses Slices.

### Settings-Störungen erkennen

Beide Fälle schreiben `ERROR` mit `tenant_id`, ohne PIN oder Geräteschlüssel:

- Lesefehler: `device authentication rejected: tenant PIN lookup failed`,
  zusätzlich `error` mit der Fehlerursache.
- Leerer Wert: `device authentication rejected: tenant PIN is empty`,
  ohne Fehlerursache, weil die Auflösung erfolgreich war.

Die Ursache kommt aus dem Settings-Lesepfad: Tenant-Transaktion,
Repository-Lesezugriff oder Snapshot-Aufbau. Dieser Pfad formatiert keine
PIN-Werte in Fehlermeldungen; die Resolver-Schnittstelle verlangt dies auch
von ihren Implementierungen. Der gesendete Header und aufgelöste Wert werden
nicht geloggt. Der Client erhält unverändert `401 / iot.device_pin_invalid`.

Die bestehende Loki-Regel `server-error-spike` in
[server-errors.yml](../../monitoring/grafana/provisioning/alerting/server-errors.yml)
zählt diese ERROR-Zeilen im Production-Server-Log mit. Sie alarmiert bei
**mehr als zehn Fehlerzeilen in fünf Minuten**, nicht bei jedem Einzelereignis.
Siehe [Server-Fehler-Runbook](../../monitoring/runbooks/server-errors.md).
Aktive Provisionierung, Log-Anlieferung und Alarmzustellung sind produktiv
nachzuweisen; eine einzelne betroffene Schule unterhalb der Schwelle löst
nicht zwingend einen Alarm aus.

`phoenix_settings_lookups_total{outcome="error"}` kann ergänzend auf
Settings-Fehler hinweisen. Die Metrik erfasst aber nicht jeden Fehler vor
`ResolveMany`, etwa beim Start der Tenant-Transaktion oder im vorgelagerten
Cache-Pfad. Fehlende Metrikfehler widerlegen deshalb keinen Ausfall. Für die
Geräteauthentifizierung die obigen Log-Meldungen prüfen und den Settings-/DB-
Zugriff reparieren, keinen Env-Fallback einschalten.

### Mensch-Schritte vor und nach dem Deploy

1. **Vor dem Deploy**, getrennt für Staging und Produktion, die nur lesende
   Datei [SQL-Zählabfrage](sql/geraete-pin-vorpruefung-3841.sql) über den
   freigegebenen Datenbankzugang ausführen. Eine berechtigte administrative
   Rolle verwenden, die alle Schulen und Settings sieht. Nicht als
   `phoenix_tenant` ausführen: RLS könnte einen unvollständigen Nullbefund
   liefern. Die Abfrage setzt deshalb `row_security = off`: Ohne Berechtigung
   zum RLS-Bypass muss sie mit einem Fehler statt mit gefilterten Zahlen enden.
   `schools_total` mit der bekannten Anzahl nicht gelöschter Schulen abgleichen;
   eine Abweichung ist kein Freigabebefund. Bei SQL- oder Zugriffsfehlern
   abbrechen, nicht als null Treffer dokumentieren. Die Abfrage gibt nur
   Anzahlen aus, keine PINs oder Schul-IDs.
2. Fehlende Overrides verwenden bereits heute den Registry-Default, nicht
   den Env-Fallback. Trotzdem jeden Treffer vor der Freigabe im geschützten
   Schul-Inventar prüfen und die beabsichtigte PIN über die Schul-Einstellung
   setzen. Leere, JSON-null- oder ungültige Overrides vor dem Deploy
   korrigieren. Leere Strings und JSON-null zählen sowohl als leer als auch
   als ungültig; die Spalten sind daher nicht zu addieren. Keine PIN per SQL,
   Shell-Argument, Ticket oder Log übertragen.
   Die PIN-Einstellung ist `admin_only`; die berechtigte Schul-Administration
   setzt sie in der App. Danach die Zählabfrage wiederholen. Nur aggregierte
   Anzahlen, Datum und verantwortliche Person als Nachweis ablegen.
3. Backend ausliefern und am Gerät je Schule Anmeldung, absichtlich falsche
   PIN und einen vorgesehenen Ablauf prüfen. Bei Settings-Störungen bleibt
   die Anmeldung abgelehnt. Settings-/DB-Zugriff reparieren und erneut prüfen;
   keinen Env-Fallback wieder einschalten. Ein erfolgreicher SQL-Zähltest
   belegt nicht die Lesbarkeit über die Serving-Rolle oder Gerätefunktion.
4. Nach erfolgreicher Auslieferung `OGS_DEVICE_PIN` durch einen Menschen mit
   SOPS aus `environments/production.sops.env` und `staging.sops.env` entfernen.
   Demo behält den Schlüssel als Seed-Eingabe. Im selben Folge-PR die bisher
   identische SOPS-Key-Menge in `scripts/env-check.sh` gezielt auf den
   Demo-only-Schlüssel umstellen und die nun entfallende Root-Seed-Vorgabe
   prüfen; `env-sync-check` muss weiter grün bleiben. Diese Dateien wurden in
   diesem Slice weder entschlüsselt noch verändert. Kein SSH-Schritt nötig.
5. Nach Wiederherstellung einer Datenbank oder Konfiguration vor Freigabe
   dieselbe Zählung und Geräteprüfung wiederholen. Ein alter Env-Wert kann
   in diesem Code keine PIN ersetzen. Alte Versionen mit Fallback nicht als
   Fehlerbehebung ausrollen. Fehlschläge und ungeprüfte Schulen bleiben offen.

`OGS_DEVICE_PIN` erreicht keinen Serving-Container mehr, auch nicht den
Demo-Server. Nur Demo-Sidecar, Seeder und CI behalten ihn als Eingabe, die
explizit in die Schul-Einstellung geschrieben wird. `.env.example` bleibt
zugleich Seed-Vorlage und Fixture der Demo-Compose-/SOPS-Key-Prüfung; ihr
Eintrag ist keine Laufzeitvorgabe für Geräteauthentifizierung.

**Lokal geprüft:** Die SQL-Zählung lief auf einem isolierten synthetischen
PostgreSQL-Schema mit fehlendem Override, leerem String, JSON-null, gültiger
PIN, falschem JSON-Typ und gelöschter Schule. Erwartete Anzahlen:
`schools_total = 6`, ein fehlender, zwei leere und drei ungültige Overrides;
die gelöschte Schule zählt nicht. Eine eingeschränkte Rolle mit filternder
RLS-Policy scheiterte durch `row_security = off`, statt einen Nullbefund zu
liefern. Das ist kein Nachweis produktiver Werte oder produktiver Leserechte.

**Produktiv nachzuweisen:** Zählung, Korrekturen, Auslieferung, Geräteprüfung
und anschließende SOPS-Bereinigung. Hier wurden keine Produktiv- oder
Staging-Zugriffe durchgeführt.

### Vorschlag AVV/TOM für den PIN-Teil

> Terminals prüfen zusätzlich zum Geräteschlüssel eine schulweite
> vierstellige PIN aus der Schul-Einstellung. Lesefehler und leere Werte
> führen zur Ablehnung; eine installationsweite Ersatz-PIN existiert nicht.
> Der Registry-Standard und die bisherige Speicherung sind unverändert.
> Individuelle Einrichtung ohne Standard-PIN und gehashte PIN-Speicherung
> sind nicht umgesetzt und bleiben offen.

Die bisherige Zusage in Anhang 1 Ziff. 2 „individuelle PIN ohne universelle
Standard-PIN“ und „PINs werden gehasht gespeichert“ darf durch diesen Slice
nicht als erfüllt gelten. Erst nach menschlichem Nachweis den produktiven
Stand ergänzen. Legal-Dateien bleiben unverändert.
