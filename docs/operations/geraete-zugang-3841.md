# Gerätezugänge umstellen: Slice 3841-a

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
