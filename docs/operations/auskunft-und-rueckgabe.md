# Interne Auskunft und Datenrückgabe

Stand: 23.10.2026, Slice 3838-a, Refs #3838, Teil von #3846.
Grundlage: [ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md),
[ADR 0097](../adr/0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md)
und [Datenkatalog](../reference/datenkatalog.md).

## Stand und Grenzen

**Im Code umgesetzt:** `tenant export` erstellt für genau eine Schule einen
lokalen, maschinenlesbaren Export aus dem gemeinsamen Katalog. Er liefert
JSON Lines, Originaldateien, eine Datenmodellbeschreibung, Ausschlussgründe,
Zeilenzahlen und SHA-256-Prüfsummen. Die Datenbank wird in einer einzigen
`REPEATABLE READ READ ONLY`-Transaktion gelesen. Migration 1.15.481 stellt
die Rolle `phoenix_tenant_export` bereit: `NOLOGIN`, keine Schreibrechte,
keine Mitgliedschaft für `phoenix_auth`. Der administrative Leser kann RLS
umgehen; jede Datenauswahl bleibt deshalb ausdrücklich schulbezogen.

**Kein vollständiger Art.-15-Auskunftsprozess:** Die personenbezogene Auswahl
für Kind, Erziehungsberechtigte oder Beschäftigte, Drittpersonenprüfung und
redigierte Zusammenstellung folgen in Slice 3838-b. Ein Gesamtexport darf
nicht als Antwort an eine einzelne betroffene Person weitergegeben werden.

**Weiter offen:** geprüfte Projektionen nicht geheimer Werte aus den bewusst
ausgeschlossenen Settings-/Zustellcontainern, erforderlicher Kontext aus
internen Protokollen sowie der gesonderte Data-Act-Schnittstellen- und
Wechselabgleich. Das Verfahren belegt die technische Rückgabe des aktuell
freigegebenen Katalogumfangs, nicht die Erfüllung aller AKs von #3838.

**Produktiv nachzuweisen:** benannte Zuständigkeit, Migration und wirksame
Berechtigungen, vollständiger DB-/Upload-Bestand, Schreibpause, Probelauf,
verschlüsselte Übergabe, Empfang und Entfernung der lokalen Kopien. Lokale
Tests sind kein produktiver Nachweis. Loki, Sentry, PostHog, journald und
Sicherungen sind nicht Bestandteil dieses PostgreSQL-/Upload-Exports.

## 1. Auftrag und Fristen

1. Dokumentierte Weisung des Kunden, Zweck, berechtigten Auftraggeber,
   Schul-ID und Empfänger prüfen. Bei einem Träger jede Schule einzeln
   autorisieren. Keine Auswahl allein anhand einer E-Mail-Adresse oder
   eines Plattform-Tokens. Die HTTP-Middleware wird nicht verwendet.
2. Vor Beginn eine verantwortliche Person und Vertretung im geschützten
   Auftrag benennen. Diese organisatorische Zuordnung ist noch offen und
   darf nicht durch einen CLI-Erfolg ersetzt werden.
3. Eingang binnen **2 Arbeitstagen** bestätigen, Zusammenstellung binnen
   **10 Arbeitstagen** bereitstellen. Gesetzliche oder kundenseitige kürzere
   Fristen haben Vorrang. Verzögerungen früh melden und eskalieren, damit
   der Kunde insbesondere seine regelmäßig einmonatige Antwortfrist bei
   Betroffenenanfragen einhalten kann. Keine automatische Fristverlängerung.
4. Ausschlüsse und gemeinsame Kontodaten mit dem Auftrag abgleichen. Eine
   Schulmitgliedschaft gestattet keinen ungefilterten Export anderer
   Schulmitgliedschaften. Personenbezogene Einzelanfragen an 3838-b geben.

## 2. Geschützten lokalen Arbeitsstand vorbereiten

1. Freigegebenen Commit und Migration 1.15.481 oder neuer verwenden. Das
   Verfahren benötigt eine privilegierte Wartungsverbindung, die in die
   reine Leserolle wechseln darf. Keine HTTP-Zugangsdaten nachrüsten und
   die Leserolle nicht an Anwendungsrollen vergeben.
2. DB und vollständiges Upload-Verzeichnis derselben Installation prüfen.
   Einen geschützten lokalen Arbeitsstand verwenden. Die folgenden
   Beispiele sind ausschließlich für **localhost** gedacht. Agents greifen
   niemals auf Produktion/Staging zu und verwenden kein SSH.
3. Upload-, Änderungs- und Löschvorgänge einschließlich Cleanup-Jobs vor
   dem DB-Snapshot anhalten und bis zum Abschluss der Dateikopie angehalten
   lassen, oder DB und Uploads aus demselben eingefrorenen Stand lesen.
   Ein beliebiges älteres Upload-Backup passt nicht zu einer aktuellen DB.
   PostgreSQL-Snapshot-Isolation allein friert das Dateisystem nicht ein.
4. Vollständigen Upload-Root angeben, nicht ein einzelnes Schulverzeichnis.
   Die Pfade sind `kind/school/name`, `kind/school_name` und für freigegebene
   Kontoreferenzen `avatars/global/name`. Vorhandene verwaiste Dateien in
   Schul-Namespaces werden ebenfalls kopiert. Ein fehlender Upload-Root,
   ein fehlendes referenziertes Original oder ein Symlink ist ein Fehler.
   Bereits abgeschlossene Dateilöschungen erfordern kein Original;
   Cleanup-Intents können auch nie geschriebene Dateien beschreiben.
5. Privates, lokal verschlüsseltes Arbeitsmedium mit genug freiem Platz
   und einem vorhandenen privaten Elternverzeichnis verwenden. Ziel und
   Uploads dürfen nicht ineinander liegen. Dateinamen und Inhalte nicht
   in Tickets, Shell-Traces, Bildschirmaufnahmen oder Logs ausgeben.

## 3. Ausführen und prüfen

Aus dem Repository-Root im vorbereiteten lokalen Dev-Loop, Variablen vorab
auf die autorisierte Schul-ID und lokale Verzeichnisse setzen:

```bash
umask 077
scripts/dev-native.sh backend go run . tenant export \
  --school "$SCHOOL_ID" --uploads "$UPLOADS_SNAPSHOT" --out "$EXPORT_DIR"
```

`EXPORT_DIR` darf noch nicht existieren. Die CLI erzeugt Verzeichnisse mit
0700 und Dateien mit 0600. Sie schreibt keine Datenzeilen oder Originalnamen
ins Terminal. Jeden Exit-Code ungleich 0 als fehlgeschlagen behandeln.
Teilstände haben keinen `COMPLETE`-Marker und werden nicht übergeben.
Fehler beheben und in ein **neues** Verzeichnis exportieren. Vorhandene
Exports werden nicht überschrieben oder mit Wiederholungen vermischt.

Exportformat, Version 1:

- `schema.tabelle.jsonl`: eine JSON-Zeile je freigegebenem Datensatz,
  einschließlich vorhandener Soft-Delete-Daten; keine Roh-Geheimnisspalten.
  PostgreSQL-JSON stellt Zahlen numerisch dar. Empfänger müssen `bigint`
  ohne Rundung lesen; Zeit-/Datumsbedeutungen ergeben sich aus `model.json`.
- `model.json`: exportierte Spalten mit `information_schema`-Typen,
  UDT und NULL-Zulässigkeit sowie tatsächliche FK-Beziehungen. Ziele können
  bewusst nicht exportierte Tabellen oder globale Rollen-/Vokabulardaten
  referenzieren. Erforderlichen fachlichen Kontext gesondert prüfen.
- `objects/…`: unveränderte Originalbytes unter neutralen laufenden Namen.
  `objects.jsonl` ordnet diese Dateien den ursprünglichen Speicherschlüsseln
  zu. Diese Zuordnung gehört in den geschützten Datenexport, nicht ins
  Durchführungsprotokoll.
- `manifest.json`: Format-/Katalogversion, SHA-256 des eingebetteten
  Katalogs, Snapshot-Zeitpunkt in UTC, Tabellen mit Zeilenzahlen und
  Ausschlüssen sowie Pfad, Größe und SHA-256 jedes Daten-/Modellartefakts.
  Keine Datensätze, Personennamen oder ursprünglichen Dateinamen.
- `COMPLETE`: SHA-256 von `manifest.json`, erst nach allen Schritten und
  erfolgreichem Abschluss der DB-Transaktion geschrieben.

Alle Prüfsummen vor Übergabe prüfen, ohne Dateninhalte auszugeben:

```bash
python3 - "$EXPORT_DIR" <<'PY'
import hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
def digest(path):
    with path.open('rb') as file:
        return hashlib.file_digest(file, 'sha256').hexdigest()
assert (root / 'COMPLETE').read_text().strip() == digest(root / 'manifest.json')
manifest = json.loads((root / 'manifest.json').read_text())
for artifact in manifest['artifacts']:
    path = root / artifact['path']
    assert path.is_file() and path.stat().st_size == artifact['bytes']
    assert digest(path) == artifact['sha256']
print('Prüfsummen vollständig geprüft')
PY
```

Zusätzlich Ausschlüsse, Zeilenzahlen, Originaldateien und Modellbezüge
gegen den Auftrag prüfen. Ein unveränderter Hash beweist weder fachliche
Vollständigkeit noch die Berechtigung des Empfängers. Offene Pflichtbestandteile
verhindern die Meldung „Rückgabe vollständig“, auch wenn der CLI-Lauf gelingt.

## 4. Verschlüsselt übergeben und aufräumen

1. Empfänger und dessen Verschlüsselungsschlüssel über einen bestätigten
   zweiten Kanal prüfen. Beispielsweise den Export lokal archivieren und
   mit `age` an den Empfängerschlüssel verschlüsseln. Alternativ ein geeignet
   verschlüsseltes Archiv mit getrennt übermitteltem starkem Passwort nutzen.
   Keine unverschlüsselte Übermittlung, keine Zugangsdaten im Auftragstext.
2. Nur das verschlüsselte Ergebnis über den vereinbarten sicheren Kanal
   übergeben. Empfang und Prüfsummenprüfung bestätigen lassen. Exportzugang
   zeitlich begrenzen und anschließend entfernen.
3. Lokale Export-, Archiv- und Zwischenkopien einschließlich fehlgeschlagener
   Versuche nach bestätigtem Empfang und Freigabe entfernen. Bei SSDs und
   Sicherungen genügt Überschreiben einzelner Dateien nicht als sicherer
   Löschbeleg. Geschütztes temporäres Medium und dessen Lebenszyklus nutzen;
   keine dauerhaften unkontrollierten Kopien erstellen.
4. Ein Wechselauftrag folgt zusätzlich dem **Wechselprozess nach AGB 10.3**
   und dem noch offenen Data-Act-Abgleich. Dieser CLI-Lauf legt weder
   Abruf-/Wechselfristen noch offene Schnittstellen abschließend fest.

## 5. Datensparsamer Durchführungsnachweis

Im zugriffsbeschränkten Auftrag festhalten: Auftragsreferenz, geprüfte
Berechtigung, interne Schulreferenz, verantwortliche Person/Vertretung,
Commit, Verfahrens-/Katalogversion, Zeitpunkte, Prüfschritte mit Anzahlen,
Manifest-Prüfsumme, Fehler/Wiederholungen, begründete Ausschlüsse und offene
Ergänzungen, bestätigten Übergabekanal/Empfang sowie Entfernung lokaler Kopien.
Keine Namen aus dem Export, Inhalte, Originaldateinamen, Passwörter, Tokens
oder Schlüssel aufnehmen. Nachweisfristen und Zugriff organisatorisch
festlegen; nicht pauschal unbegrenzt aufbewahren.

Der Nachweis trennt **Code-Lauf erfolgreich**, **Rückgabe fachlich geprüft**
und **produktive Durchführung belegt**. Ein offener Teil darf keinen
Gesamterfolg erhalten.
