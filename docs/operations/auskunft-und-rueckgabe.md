# Interne Auskunft und Datenrückgabe

Stand: 10.11.2026, Refs #3838, #6412, Teil von #3846.
Grundlage: [ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md),
[ADR 0097](../adr/0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md),
[ADR 0192](../adr/0192-mandantenrueckgabe-oeffnet-feldverschluesselte-bankdaten.md),
[ADR 0193](../adr/0193-mandantenrueckgabe-gibt-gesundheit-einstellungen-und-traegerpaket-zurueck.md),
[ADR 0197](../adr/0197-mandantenrueckgabe-oeffnet-bankdaten-von-eltern-und-personal-und-den-mandatstext.md)
und [Datenkatalog](../reference/datenkatalog.md). Öffentliche Beschreibung für
Kunden: [Datenexport und Anbieterwechsel](https://moto-ogs.de/rechtliches/datenexport)
und [Infrastruktur und technische Anbieter](https://moto-ogs.de/rechtliches/infrastruktur)
(Website-Repo, `src/app/(de)/rechtliches/`). Wer Format, Ablauf oder Grenzen
hier ändert, gleicht diese Seiten im selben Zug an.

## Stand und Grenzen

**Im Code umgesetzt:** `tenant export` erstellt für genau eine Schule
(`--school`) oder genau einen Träger (`--organization`) einen
lokalen, maschinenlesbaren Export aus dem gemeinsamen Katalog. Er liefert
JSON Lines, Originaldateien, eine Datenmodellbeschreibung, Ausschlussgründe,
Zeilenzahlen und SHA-256-Prüfsummen. Die Datenbank wird in einer einzigen
`REPEATABLE READ READ ONLY`-Transaktion gelesen. Migration 1.15.489 stellt
die Rolle `phoenix_tenant_export` bereit: `NOLOGIN`, keine Schreibrechte,
keine Mitgliedschaft für `phoenix_auth`. Der administrative Leser kann RLS
umgehen; jede Datenauswahl bleibt deshalb ausdrücklich schulbezogen.

**Personenbezogenes internes Prüfpaket im Code:** `tenant export --person …`
leitet aus denselben freigegebenen Schulzeilen ein Paket für Kind,
Erziehungsberechtigte oder Beschäftigte ab. Es enthält Zeilen-/Dateimarkierungen
in `review.json`; `manifest.json` setzt `reviewRequired: true`. Das ist keine
automatisch vollständige oder freigegebene Art.-15-Antwort. Die zuständige
Person muss die Ergänzungs- und Drittpersonenprüfung in Abschnitt 6 abschließen.
Weder Gesamtexport noch internes Prüfpaket unverändert an Betroffene geben.

**Register (Art. 26 Data Act, [ADR 0201](../adr/0201-mandantenrueckgabe-hat-ein-register-und-einen-abrufweg.md)):**
`GET /public/data-return/register` der API liefert ohne Anmeldung die
Schemabeschreibung des Pakets: jede Tabelle mit Feldern, Typen, Nullbarkeit,
Beziehungen und den Paketen, die sie tragen, die ausgeschlossenen Tabellen,
Spalten und Dateiarten mit Grund, Formatversion, Standards und eine
unverbindliche Zeitschätzung. Keine Kundendaten. Die Datei
`backend/modules/organizationtenancy/return_register.json` ist im Binary
eingebettet und entsteht mit demselben Code wie `model.json`;
`TestReturnRegisterMatchesMigratedSchema` erzwingt, dass sie zum Schema passt.
`formatVersion` und `catalogSHA256` im Manifest eines Pakets nennen den
Registerstand, zu dem es gehört. Kunden und Zielanbieter auf diese Adresse
verweisen, nicht auf eine Kopie.

**Abruf über HTTPS (Art. 30 Abs. 2 Data Act, ADR 0201):** Ein geprüftes Paket
gibt `tenant return release` für den Abruf frei (Abschnitt 8). Kunde und ein
vom Kunden benannter neuer Anbieter laden es mit einem kurzlebigen Token über
`GET /public/data-return/package/archive` selbst herunter; die Schnittstelle
beschreibt `/public/data-return/openapi.json`. Wir erstellen und prüfen das
Paket weiter selbst; es gibt keinen Selbstbedienungs-Export.

**Geöffnet und projiziert:** IBAN und Kontoinhaber der SEPA-Daten, der
bestätigte Text eines Online-Mandats, IBAN und Kontoinhaber der Eltern sowie
IBAN, Steuer-ID und Sozialversicherungsnummer des Personals (alle Bankring)
und alle Gesundheitsangaben (Gesundheitsring) stehen geöffnet im Paket, nie
als Chiffrat. Einstellungen gehen ohne Geheimnisse mit:
`password`-Definitionen und unbekannte Schlüssel als `null`. Zustellprotokolle
(Sperrliste, Zustellereignisse, Push-Inhalte) gehen mit. Ein Wert, der nicht
öffnet, oder ein Container ohne Besitzer bricht den Export ab.

**Weiter offen:** Der Mail-Payload bleibt ausgeschlossen (Sicherheitsmails
tragen dort Link oder Code); der Inhalt einer Mail ist der verknüpfte
Datensatz. Die anwaltliche Bestätigung, dass Register und Abrufweg Art. 26 und
Art. 30 Abs. 2 Data Act genügen (Abschnitt 7), und die menschlichen Nachweise
unten. Die Beschäftigtenauskunft
nimmt weiter mehr Kontextzeilen auf als nötig; Pflicht-Redigierung und
Zweitprüfung fangen das ab.

**Produktiv nachzuweisen:** benannte Zuständigkeit, Migration und wirksame
Berechtigungen, vollständiger DB-/Upload-Bestand, Schreibpause, Probelauf,
verschlüsselte Übergabe, Empfang und Entfernung der lokalen Kopien. Lokale
Tests sind kein produktiver Nachweis. Loki, Sentry, PostHog, journald und
Sicherungen sind nicht Bestandteil dieses PostgreSQL-/Upload-Exports.

## 1. Auftrag und Fristen

1. Dokumentierte Weisung des Kunden, Zweck, berechtigten Auftraggeber,
   Schul-ID und Empfänger prüfen. Bei einem Träger jede Schule einzeln
   autorisieren; die Rückgabe eines Trägers besteht aus dem Trägerpaket und je
   Schule einem Schulpaket (Abschnitt 3). Keine Auswahl allein anhand einer E-Mail-Adresse oder
   eines Plattform-Tokens. Die HTTP-Middleware wird nicht verwendet.
2. Vor Beginn eine verantwortliche Person und Vertretung im geschützten
   Auftrag benennen. Diese organisatorische Zuordnung ist noch offen und
   darf nicht durch einen CLI-Erfolg ersetzt werden.
3. Eingang binnen **2 Arbeitstagen** bestätigen, Zusammenstellung binnen
   **10 Arbeitstagen** der Schule bereitstellen. Diese internen Zielwerte
   sind **vorläufig; die organisatorische Bestätigung ist offen (3838-d)**.
   Die Schule bleibt für ihre Antwortfrist verantwortlich. Gesetzliche oder kundenseitige kürzere
   Fristen haben Vorrang. Verzögerungen früh melden und eskalieren, damit
   der Kunde insbesondere seine regelmäßig einmonatige Antwortfrist bei
   Betroffenenanfragen einhalten kann. Keine automatische Fristverlängerung.
4. Ausschlüsse und gemeinsame Kontodaten mit dem Auftrag abgleichen. Eine
   Schulmitgliedschaft gestattet keinen ungefilterten Export anderer
   Schulmitgliedschaften. Personenbezogene Einzelanfragen nach Abschnitt 6 bearbeiten.

## 2. Geschützten lokalen Arbeitsstand vorbereiten

1. Freigegebenen Commit und Migration 1.15.489 oder neuer verwenden. Das
   Verfahren benötigt eine privilegierte Wartungsverbindung, die in die
   reine Leserolle wechseln darf. Keine HTTP-Zugangsdaten nachrüsten und
   die Leserolle nicht an Anwendungsrollen vergeben.
2. Einen berechtigten Menschen DB-Dump und vollständigen Upload-Stand
   derselben Installation bereitstellen lassen. Zuerst sämtliche Upload-,
   Änderungs- und Löschvorgänge einschließlich Cleanup-Jobs anhalten, dann
   den DB-Dump erstellen und anschließend den vollständigen Upload-Root
   kopieren. Die Schreibpause bis zum Ende beider Kopien halten und die
   gemeinsame Herkunft im geschützten Auftrag dokumentieren. Ein beliebiges
   älteres Upload-Backup passt nicht zu einer aktuellen DB; PostgreSQL-
   Snapshot-Isolation allein friert das Dateisystem nicht ein.
3. Diesen zusammengehörigen Stand auf einer getrennten, geschützten lokalen
   Installation nach dem [Restore-Verfahren](daily-backup.md#wiederherstellung-app-bleibt-gesperrt)
   wiederherstellen: frische DB und frisches Upload-Volume vorbereiten,
   Rollen/ACLs und DB-Dump einspielen, danach das zugehörige Upload-Archiv
   wiederherstellen. Bei einem Daily-Bundle erledigt
   `scripts/restore-daily-backup.sh` diese Schritte samt Prüfsummenprüfung.
   Die dort beschriebene Löschprüfung nach Restore und den DB-Dateiabgleich
   abschließen, erforderliche Migrationen bis mindestens 1.15.489 anwenden
   und Rollenrechte prüfen. App und Jobs während Vorbereitung und Export
   gestoppt lassen. Ein online erzeugtes Daily-Bundle ist nicht automatisch
   dateikonsistent; seine gemeinsame Herkunft und Dateireferenzen müssen
   geprüft sein. Die folgenden Beispiele sind ausschließlich für
   **localhost** gedacht. Agents greifen niemals auf Produktion/Staging zu
   und verwenden kein SSH.
4. Vollständigen Upload-Root angeben, nicht ein einzelnes Schulverzeichnis.
   Die Pfade sind `kind/school/name`, `kind/school_name` und für freigegebene
   Kontoreferenzen `avatars/global/name`. Vorhandene verwaiste Dateien in
   Schul-Namespaces werden ebenfalls kopiert. Ein fehlender Upload-Root,
   ein fehlendes referenziertes Original oder ein Symlink ist ein Fehler.
   Bereits abgeschlossene Dateilöschungen erfordern kein Original;
   Cleanup-Intents können auch nie geschriebene Dateien beschreiben.
5. Bankring der Herkunftsinstallation bereitstellen: `BANK_DATA_KEYS` und,
   solange deren Rekey nicht abgeschlossen ist, `BANK_DATA_LEGACY_KEYS`
   ([ADR 0183](../adr/0183-bankdaten-haben-einen-eigenen-schluesselring-und-einen-wiederholbaren-rekey.md)).
   und den Gesundheitsring `HEALTH_DATA_KEYS` derselben Installation
   ([ADR 0193](../adr/0193-mandantenrueckgabe-gibt-gesundheit-einstellungen-und-traegerpaket-zurueck.md)).
   Der Export öffnet damit IBAN und Kontoinhaber der SEPA-Mandate und
   -Lastschriften, den Mandatstext, die Bankdaten der Eltern, Bank- und
   Steuerdaten des Personals sowie die Gesundheitsangaben; ohne passende Ringe bricht er
   ab, statt Chiffrat oder eine Lücke zu übergeben, auch für Schulen ohne
   solche Daten. Die Werte nur per SOPS in die Umgebung des Exportprozesses
   geben, nie als Datei, Shell-Literal oder CLI-Argument.
6. Privates, lokal verschlüsseltes Arbeitsmedium mit genug freiem Platz
   und einem vorhandenen privaten Elternverzeichnis verwenden. Ziel und
   Uploads dürfen nicht ineinander liegen. Dateinamen und Inhalte nicht
   in Tickets, Shell-Traces, Bildschirmaufnahmen oder Logs ausgeben.

## 3. Ausführen und prüfen

Aus dem Repository-Root im vorbereiteten lokalen Dev-Loop, Variablen vorab
auf die autorisierte Schul-ID, die eigene Operator-ID und lokale
Verzeichnisse setzen:

```bash
umask 077
scripts/dev-native.sh backend go run . tenant export \
  --school "$SCHOOL_ID" --operator "$OPERATOR_ID" \
  --uploads "$UPLOADS_SNAPSHOT" --out "$EXPORT_DIR"
```

Jeder Lauf verlangt `--operator` und steht im Operator-Protokoll, beim
Schulpaket mit Ressource `school` statt `organization` (#6326, siehe
**Operator-Protokoll** unten).

Für das Trägerpaket statt `--school "$SCHOOL_ID"` den Parameter
`--organization "$ORGANIZATION_ID"` setzen. Es enthält die Trägertabellen
(Verträge, Elternabrechnung, SEPA, Ferienanmeldungen, Trägereinstellungen), die
Stammzeilen der Schulen des Trägers und die Konten der Träger-Geschäftsstelle
samt Avataren. Was eine Schule hält, steht im Manifest als `schoolPackage`
ohne Zeilen und kommt mit dem Schulpaket. Ein Schulpaket trägt weiter den
Trägerbestand; bei einer Trägerrückgabe ist das Trägerpaket desselben Auftrags
für diese Tabellen maßgeblich. Wechselt nur eine Schule, enthält ihr Schulpaket
die Trägerdaten aller Schulen des Trägers: Umfang mit dem Träger abstimmen.

Das Trägerpaket legt zusätzlich die gespeicherten Dokumente und die
PAIN.008-Dateien als Dateien bei (ADR 0187):

```bash
umask 077
scripts/dev-native.sh backend go run . tenant export \
  --organization "$ORGANIZATION_ID" --operator "$OPERATOR_ID" \
  --uploads "$UPLOADS_SNAPSHOT" --out "$EXPORT_DIR"
```

- **Offline:** Nur mit gestoppten Schreibern ausführen (API, Worker, Uploads,
  Importe und geplante Läufe aus, wie beim Löschverfahren mit
  `--writers-stopped`). Die PAIN.008-Dateien liest die Elternabrechnung in
  einer eigenen Transaktion **nach** dem Katalog-Snapshot; nur ohne Schreiber
  passen Tabellen-Dateien und Dateien zusammen. Der deaktivierte Träger allein
  reicht nicht.
- **Operator-Protokoll (Schul- und Trägerpaket):** `--operator` muss ein
  bestehender, **aktiver** Operator sein (wie beim Löschverfahren); sonst
  liest und schreibt der Lauf nichts. Jeder Lauf steht zweimal in
  `platform.operator_audit_log` (Aktion `export`, Ressource `school` mit der
  Schul-ID bzw. `organization` mit der Träger-ID), ohne Personendaten und
  ohne lokalen Pfad: vor dem ersten Lesen der Versuch (`outcome: started`,
  `operator_id`, `school_id` bzw. `organization_id`, `started_at`), danach
  das Ergebnis. `completed` nennt die Prüfsumme aus `COMPLETE`
  (`manifest_sha256`), `exported_at`, die Anzahl der Tabellen und Dateien,
  beim Trägerpaket zusätzlich die Dokumente je Spalte und die Herkunft jeder
  PAIN.008-Datei, beim internen Prüfpaket `person_review: true` ohne die
  Person; erst danach entsteht `COMPLETE`. Schlägt dieser Eintrag fehl,
  entsteht kein `COMPLETE`.
  `failed` nennt die Fehlerklasse (`snapshot`, `filesystem`,
  `document_files`, `sepa_debit_files`, `manifest`, `audit`,
  `complete_marker`). Das Trägerpaket löscht sein unvollständiges
  Verzeichnis und meldet das in `partial_files_removed`; steht dort `false`,
  das Verzeichnis von Hand löschen. Ein fehlgeschlagenes Schulpaket lässt
  seine Teilstände liegen (kein `partial_files_removed`): sperren und nach
  Abschnitt 4 entfernen.
- **Dokumente:** `DOKUMENTE/<Tabelle>/<Zeilen-ID>-<Spalte>.<Endung>` je
  gespeichertem Dokument der Trägertabellen: Schreiben und Bestätigungen,
  Bescheide, alle Vertragsfassungen, Papierverträge, Unterschriftsbilder,
  hochgeladene Nachweise, Dokumente und Formulare des Trägers, Logo. Welche
  Spalten Dokumente sind, folgt aus dem Katalog: jede exportierte
  Binärspalte einer Trägertabelle; eine neue kommt ohne Codeänderung mit.
  Die Zeilen-ID ist der Primärschlüssel, die Endung folgt aus dem
  gespeicherten Medientyp, sonst aus dem Inhalt (`.bin`, wenn unbekannt).
  `DOKUMENTE/index.jsonl` nennt je Datei `table`, `row_id`, `row_key`,
  `column`, `media_type`, `sha256` und `bytes`; `LIESMICH.txt` erklärt den
  Ordner. `manifest.json` zählt sie unter `documentFiles` je Spalte und prüft
  jede Datei mit Prüfsumme. Die Tabellen-Dateien behalten das Dokument als
  Hex-Wert unverändert. Lässt sich ein Dokument nicht schreiben (etwa ohne
  Primärschlüssel), scheitert der Lauf mit `document_files`.
- **PAIN.008-Dateien:** `SEPA-LASTSCHRIFTDATEIEN/<Nachrichten-ID>.xml` je
  eingereichte Lastschriftnachricht, mit `LIESMICH.txt`; `manifest.json`
  nennt sie unter `sepaDebitFiles` mit Herkunft und prüft jede mit Prüfsumme.
  IBAN und Kontoinhaber je Mandat und Lastschrift stehen nicht dort, sondern
  geöffnet in den Tabellen-Dateien (siehe Exportformat).
  - **Original:** Seit Migration `1.15.880` speichert moto beim Download die
    Datei bytegenau, versiegelt mit dem Bank-Schlüssel, mit SHA-256 und dem
    Gläubigerkonto darin (`parent_billing.sepa_debit_files`); ein späterer
    Download derselben Nachricht ersetzt sie bis zur Bestätigung der
    Einreichung. Ein nicht eingereichter Download geht nach 30 Tagen; eine
    eingereichte Datei bleibt, solange eine ihrer Lastschriften oder einer
    ihrer Ratenbescheide bleibt. Das Paket enthält diese Datei unverändert
    und prüft die Prüfsumme, auch wenn keine ihrer Lastschriften mehr da ist.
    Ein späterer Wechsel des Standardkontos ändert sie nicht.
  - **Rekonstruiert:** Fehlt das Original, erzeugt der Export die Datei mit
    demselben Generator neu aus den gespeicherten Lastschriften
    (Nachrichten-ID, eingereichte Beträge, Sequenz, Einzugstag, Konto zur
    Freigabe, Gläubiger-ID). Die Erstellzeit ist dann die Bestätigung der
    Einreichung. Das Gläubigerkonto ist das bei der Einreichung gespeicherte,
    für Nachrichten von vor `1.15.880` das heutige Standardkonto des Trägers;
    ohne Standardkonto scheitert der Export.
- Den Wert aus `COMPLETE` beim Löschauftrag im Operator-Portal als Prüfsumme
  eintragen; so belegt der Auftrag, welche Rückgabe übergeben wurde.

`EXPORT_DIR` darf noch nicht existieren. Die CLI erzeugt Verzeichnisse mit
0700 und Dateien mit 0600. Sie schreibt keine Datenzeilen oder Originalnamen
ins Terminal. Jeden Exit-Code ungleich 0 als fehlgeschlagen behandeln.
Teilstände haben keinen `COMPLETE`-Marker und werden nicht übergeben.
Fehler beheben und in ein **neues** Verzeichnis exportieren. Vorhandene
Exports werden nicht überschrieben oder mit Wiederholungen vermischt.

Exportformat, Version 1:

- `schema.tabelle.jsonl`: eine JSON-Zeile je freigegebenem Datensatz,
  einschließlich vorhandener Soft-Delete-Daten; keine Roh-Geheimnisspalten.
  Feldverschlüsselte Werte, die der Katalog unter `openSealedColumns` führt,
  stehen geöffnet in eigenen Spalten (`iban`, `account_holder` bei SEPA-
  Mandaten und -Lastschriften, `text` beim bestätigten Mandatstext,
  `iban_opened`, `account_holder_opened` bei Bankdaten der Eltern,
  `iban_opened`, `tax_id_opened`, `social_security_number_opened` bei Bank-
  und Steuerdaten des Personals, `health_info` bei Betreuungsprofilen,
  `health` als JSON-Text bei Ferienanmeldungen, -kindern und -gästen), ihr
  Chiffrat nicht. Bei Eltern und Personal tragen `iban`, `account_holder`,
  `tax_id` und `social_security_number` nur noch Klartext von Zeilen, die der
  Backfill (#6332) nicht versiegelt hat; maßgeblich ist der geöffnete Wert,
  sonst dieser Klartext. Container unter `projectedColumns` stehen projiziert in eigenen
  Spalten (`exported_value`, `exported_old_value`/`exported_new_value` bei
  Einstellungen, `exported_settings` bei Schulen), ihr Rohwert nicht.
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
- `manifest.json`: Paketart (`package: organization` beim Trägerpaket),
  Format-/Katalogversion, SHA-256 des eingebetteten
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

### Referenziertes Original fehlt endgültig

Der Export bricht bei einem fehlenden referenzierten Original bewusst ab und
schreibt kein `COMPLETE`. Ein vorhandener Namespace allein genügt nicht.
Es gibt keinen Modus zum stillen Überspringen.

1. Wenn möglich, das Original aus einer zugehörigen Sicherung in den
   geschützten lokalen Arbeitsstand wiederherstellen. Herkunft und Zuordnung
   prüfen; anschließend in ein neues Zielverzeichnis exportieren.
2. Ist keine Wiederherstellung möglich, nach dokumentierter Entscheidung den
   betroffenen Verweis in Vorlage oder Snapshot gezielt bereinigen. Keine
   automatische oder eigenmächtige Änderung historischer Nachweise. Entscheidung,
   Umfang und verbleibende Lücke im geschützten Auftrag festhalten und die
   fachliche Vollständigkeit neu bewerten. Danach den Export neu ausführen.

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
4. Statt der Übergabe nach 1. und 2. kann das Paket über die
   HTTPS-Schnittstelle abgerufen werden (Abschnitt 8); bei einem Wechsel ist das
   der vorgesehene Weg, auch für den neuen Anbieter.
5. Ein Wechselauftrag folgt zusätzlich dem Wechselprozess mit seinen Fristen
   in Abschnitt 7. Bei einem Wechsel bleibt das verschlüsselte Paket bis zum
   Ende des Abrufzeitraums erhalten; erst danach die lokalen Kopien entfernen.

## 5. Datensparsamer Durchführungsnachweis

Im zugriffsbeschränkten Auftrag festhalten: Auftragsreferenz, geprüfte
Berechtigung, interne Schulreferenz, verantwortliche Person/Vertretung,
Commit, Verfahrens-/Katalogversion, Zeitpunkte, Prüfschritte mit Anzahlen,
Manifest-Prüfsumme, Fehler/Wiederholungen, Verweis auf die Einträge im
Operator-Protokoll (Aktion `export`, Ressource `school` bzw. `organization`), begründete Ausschlüsse und offene
Ergänzungen, bestätigten Übergabekanal/Empfang sowie Entfernung lokaler Kopien.
Keine Namen aus dem Export, Inhalte, Originaldateinamen, Passwörter, Tokens
oder Schlüssel aufnehmen. Nachweisfristen und Zugriff organisatorisch
festlegen; nicht pauschal unbegrenzt aufbewahren.

Der Nachweis trennt **Code-Lauf erfolgreich**, **Rückgabe fachlich geprüft**
und **produktive Durchführung belegt**. Ein offener Teil darf keinen
Gesamterfolg erhalten.

## 6. Personenauskunft auf Weisung der Schule

### Eingang, Identität und Auswahl

1. Direkt bei moto eingehende Betroffenenanfragen unverzüglich an die Schule
   weiterleiten. moto handelt nur auf dokumentierte Weisung der Schule,
   nicht auf die bloße Behauptung einer Person oder deren Schulmitgliedschaft.
   Die Schule prüft Identität, Vertretung, Sorgerecht und Umfang der Berechtigung.
   Der anfragende Elternteil erhält nicht automatisch Daten anderer
   Erziehungsberechtigter oder aller Geschwister. Auftrag und Antwortempfänger
   getrennt prüfen; keine Ausweiskopien in technische Protokolle übernehmen.
2. Zuständige Person, Vertretung und zweite prüfende Person im geschützten
   Auftrag benennen. Eingang/Prüfung binnen 2, Bereitstellung an die Schule
   binnen 10 Arbeitstagen gemäß Abschnitt 1 planen. Unklare Berechtigung oder
   drohende Fristüberschreitung sofort an die Schule eskalieren; keine
   eigenmächtige Verlängerung. Der Monat läuft nicht erst ab CLI-Ausführung.
3. Die Schule bestätigt Schul-ID und Personen-ID: `kind` verwendet
   `users.student_profiles.id`, `guardian` verwendet `users.guardian_profiles.id`,
   `staff` verwendet `users.staff_school_memberships.id`, nicht Konto-/Personen-ID.
   Bei mehreren Schulen jede einzeln beauftragen. Vollständig gelöschte
   Identitäten können nicht automatisch zugeordnet werden; historische IDs,
   Umbenennungen und frühere Rollen ausdrücklich im Auftrag prüfen.
4. Den lokalen, geschützten Stand wie in Abschnitt 2 vorbereiten. Keine
   zusätzlichen Anwendungsrechte oder Kundenschnittstelle einrichten.

```bash
umask 077
scripts/dev-native.sh backend go run . tenant export \
  --school "$SCHOOL_ID" --operator "$OPERATOR_ID" \
  --person "${PERSON_SELECTOR:?Personenauswahl erforderlich}" \
  --uploads "$UPLOADS_SNAPSHOT" --out "$PERSON_REVIEW_DIR"
```

`PERSON_SELECTOR` ist beispielsweise `kind:<ID>`, ohne spitze Klammern.
Das Ziel muss neu sein. Fehlende/mehrdeutige Identität der beauftragten Schule,
ungültige Auswahl, fehlende benötigte Originale und Teilfehler brechen ohne
`COMPLETE` ab. Keine Umgehung durch die ID derselben Person an einer anderen
Schule. Fehlversuche können im privaten Arbeitsverzeichnis noch Schulzeilen
und `.person`-Zwischendateien enthalten: sperren, nicht übergeben und entfernen.

### Auswahl und Markierungsregeln

Die Ableitung läuft im bestehenden Leseadapter nach dem Tabellenexport und
vor der Dateikopie. Sie verwendet denselben Katalog, die exportierten
Fremdschlüssel und ausdrücklich bekannte historische Audit-IDs. Es gibt
keinen zweiten SQL-Lesepfad, kein neues Recht und keine Migration.
Nur Schlüssel/Beziehungen bleiben im Arbeitsspeicher, Inhalte werden gestreamt.

1. Eigene Profil-/Schulmitgliedschaft, eigene Person und eigenes Konto sind
   die Startpunkte. Davon abhängige Schulzeilen sind personenbezogene Treffer.
   Bei zusammengesetzten FKs ist `tenant_id` keine Personenkante. Die
   vorhandene Exportauswahl hat die Schulgrenze bereits angewandt.
2. Benötigte Elternzeilen liefern Kontext, begründen aber **keinen** weiteren
   Personenexport. Gemeinsame Erziehungsberechtigte führen deshalb nicht zu
   Geschwisterakten. Ein Beschäftigtenkonto an zwei Schulen führt nicht zum
   Bestand der anderen Schule. Andere Identitätsketten werden nicht verfolgt.
3. Gesprächsinhalte, Versionen/Optionen und Anhänge einer relevanten
   Elternmitteilung sowie Änderungsnachrichten eines relevanten Anmeldeantrags
   sind Kontext. Die begrenzten Regeln stehen in `personContentChild`.
   Ein gemeinsamer Anmeldeantrag lädt nicht automatisch alle Kindzeilen nach;
   sein eigener Snapshot kann dennoch Geschwisterdaten enthalten.
4. Historische Auditbezüge ohne FK (`student_id`, `matched_student_id`,
   `guardian_profile_id`, `staff_id`, `target_staff_id`, `account_id`, `actor_account_id`,
   `request_id`) sowie typisierte Finanz-/Dokumentzugriffe und Personalvorschau
   werden zugeordnet. Nicht eindeutige Akteurskennungen und Sammelzugriffe
   bleiben Gegenstand der manuellen Vollständigkeitsprüfung.
5. Nur Originale aus den ausgewählten Zeilen und ihrem Kontext werden kopiert,
   darunter Foto-, Dokument- und freigegebene Anmelde-PDF-Referenzen.
   Nicht zugeordnete Schulobjekte werden gezählt, nicht still ausgeliefert.
   Fehlende referenzierte Originale bleiben Fehler. Geheimnisausschlüsse aus
   dem Katalog gelten unverändert, auch für geteilte Konten und MFA-Daten.

`review.json` markiert **jede** ausgegebene Zeile (`table`, einsbasierte `row`
im gefilterten JSONL) mit `third-party-review`. Zusätzlich gelten:

| Regel | Verbindliche Prüfung |
|---|---|
| `context-not-subject` | Gehört der Kontext zur Anfrage? Andere Erziehungsberechtigte, Geschwister, Beschäftigte und deren Kennungen entfernen oder begründet freigeben. |
| `free-text-snapshots-and-embedded-persons` | Freitexte, Nachrichten, frühere/neue Werte, JSON und Formulare auf eingebettete Drittpersonen und Geheimnisse prüfen. Keine bloße Suche nach Spaltennamen. |
| `audit-actors-and-historical-values` | Akteure und historische Werte prüfen; den erforderlichen Auskunftskontext nicht pauschal als „intern“ zurückhalten. |
| `special-category-health-data` | Geöffnete Gesundheitsangaben (Art. 9 DSGVO): nur die der betroffenen Person und nur an den von der Schule bestätigten Empfänger. Zeilen von Geschwistern oder anderen Gästen entfernen. |
| `especially-sensitive-tax-and-social-security-ids` | Geöffnete Steuer-ID und Sozialversicherungsnummer: lebenslange Kennungen mit hohem Missbrauchsrisiko. Nur die der betroffenen Beschäftigten, nur an den von der Schule bestätigten Empfänger, verschlüsselt übergeben; Zeilen anderer Beschäftigter entfernen. |
| `original-content-and-group-photos` | Jede markierte Originaldatei ansehen. Gruppenfotos und Mehrpersonen-Dokumente redigieren oder begründet zurückhalten; das Original bleibt nur im internen Paket. |

Die Markierungen sind bewusst konservativ, keine automatische juristische
Entscheidung. Ein leerer Objektzähler oder unveränderte Prüfsummen beweisen
keine Vollständigkeit. `COMPLETE` bestätigt nur den technischen Lauf.

### Ergänzen, freigeben und sicher übergeben

1. Die erste prüfende Person arbeitet alle `requiredChecks` und Markierungen
   ab. Gegen den [Datenkatalog](../reference/datenkatalog.md) prüfen:
   Stammdaten, Sorge-/Kontaktbeziehungen, Anwesenheit und Raumhistorie,
   Gesundheit, Betreuung/Planung, Personal/Finanzen, Nachrichten/Elternbriefe,
   Erklärungen, Anmeldungen, Änderungsanfragen, Dateien und Protokolle.
   Für indirekte Namens-/Freitextbezüge und verwaiste Dateien nötigenfalls
   einen getrennten vollständigen Schul-Export mit dem vorhandenen Befehl
   erzeugen und ausschließlich im geschützten Arbeitsbereich prüfen.
2. Die Nichttreffer, frühere Identitäten, nicht verknüpfte Anmeldesnapshots,
   Sammelzugriffe und `unassignedObjects` prüfen. Zusätzlich die Ausschlüsse
   aus Manifest/Katalog auf erforderlichen nicht geheimen Kontext prüfen,
   insbesondere Settings, Zustellcontainer und Operator-Protokolle.
   **Keine** Rohcontainer, Kennwörter, Hashes, Tokens, Schlüssel oder MFA-
   Geheimnisse ergänzen. Fehlt eine sichere Projektion, den offenen Teil
   dokumentieren und zur fachlichen/technischen Ergänzung eskalieren, nicht
   als erledigt markieren. Externe Systeme gesondert durch berechtigte Menschen
   abgleichen; Agents greifen nicht auf Produktiv-/Staging-Systeme zu.
3. Die Schule ergänzt die Art.-15-Angaben zu Zwecken, Kategorien, Empfängern,
   Herkunft, Speicherfristen und Rechten sowie gegebenenfalls weiteren
   einschlägigen Angaben. Sie entscheidet über Rechte Dritter und begründet
   Beschränkungen. Art. 15 nicht mit Art. 20 gleichsetzen. Offene Pflichtdaten
   verhindern die Freigabe als vollständige Auskunft.
4. Eine separate, redigierte Übergabekopie erstellen. Zweite prüfende Person
   prüft Umfang, Drittpersonen, Ausschlüsse und Ergänzungen; die Schule gibt
   Empfänger und Inhalt frei. Ursprüngliche Manifest-Prüfsumme als internen
   Quellenbeleg erhalten, Prüfsummen der Übergabekopie neu bilden. Nicht das
   interne Prüfpaket als fertige Auskunft tarnen oder dessen `COMPLETE` als
   Freigabe nutzen. Danach verschlüsselt nach Abschnitt 4 an die Schule oder
   deren ausdrücklich benannten Empfänger übergeben.
5. Nach Abschnitt 5 nur Prüfschritte, Mengen, Entscheidungen und Prüfsummen
   protokollieren, keine Auskunftsinhalte. Empfang bestätigen und alle lokalen
   Arbeits-/Gesamtexport-/Zwischenkopien einschließlich Fehlversuchen löschen.
   Benannte Rollen, Fristenbestätigung und ein praktischer Durchlauf sind
   **produktive Mensch-Nachweise**, nicht durch lokale Tests erledigt.

## 7. Anbieterwechsel: Rückgabe und anschließende Löschung

1. Kundenwahl, autorisierte Schulen, Zielanbieter, Umfang, Zeitpunkt und
   zulässige Aufbewahrung dokumentieren. Keine Löschung allein aus einem
   Auskunftsauftrag ableiten. Auskunftspaket und vollständige Rückgabe sind
   verschiedene Aufträge.
2. Den vollständigen Schulbestand nach Abschnitten 2 bis 5 zurückgeben,
   Modell, Originale und fachliche Ergänzungen prüfen, sicheren Empfang und
   Nutzbarkeit bestätigen lassen. Ein Prüfpaket für eine Person ersetzt die
   Rückgabe nicht.
3. Die getrennte Löschweisung nach [tenant-erasure.md](tenant-erasure.md)
   ausführen: DB-/Dateiphase, Nebensysteme, beschränkte gesetzliche
   Aufbewahrung, Sicherungszyklus und Wiederholung nach Restore nachweisen.
   Die CLI-Nachweise allein bedeuten nicht „vollständig gelöscht“.

### Wechselprozess und Fristen (AGB Ziff. 10, AVV Ziff. 8.2)

Technisch steht das Verfahren dieses Runbooks bereit: vollständiger
Schul- und Trägerbestand in offenem, dokumentiertem Format (JSON Lines,
`model.json`, Originaldateien, Prüfsummen), ohne Wechselentgelt. Das Register
unter `/public/data-return/register` beschreibt das Paket schon vor dem
Wechsel; bei der Wechselanfrage dem Kunden und dem Zielanbieter nennen.
Die Fristen laufen ab dem Eingang der Wechselanfrage:

| Schritt | Frist nach AGB | Wer |
|---|---|---|
| Wechselanfrage bestätigen, Umfang (Schulen, Träger, Ziel) festhalten | unverzüglich; Ankündigungsfrist höchstens 2 Monate (10.1) | zuständige Person |
| Ist der Übergang in 30 Tagen nicht machbar: begründen, Ersatzzeitraum ≤ 7 Monate nennen | 14 Arbeitstage nach der Anfrage (10.3) | zuständige Person |
| Pakete erstellen, prüfen, verschlüsselt übergeben; Dienst läuft weiter | Übergangszeitraum ≤ 30 Kalendertage nach der Ankündigungsfrist, einmal verlängerbar (10.3) | zuständige Person, zweite Person prüft |
| Pakete nach Abschnitt 8 freigeben (`--keep-until` = Ende des Abrufzeitraums), Tokens bei Bedarf neu ausstellen | ab Freigabe bis 30 Kalendertage nach dem Übergangszeitraum (10.3) | zuständige Person |
| Löschung nach [tenant-erasure.md](tenant-erasure.md), Nachweis auf Anfrage | danach (10.4, AVV 8.2) | zuständige Person |

Bekannte Wechselrisiken, die dem Kunden mitzuteilen sind: Daten ändern sich
während des Übergangs weiter, also vor der letzten Übergabe eine Schreibpause
vereinbaren und neu exportieren; Kennwörter, Passkeys, MFA-Faktoren, Tokens und
`password`-Einstellungen (Geräte-PIN, SFTP-Passwort) kommen nicht mit und
müssen im Zielsystem neu gesetzt werden; der Import braucht eine Feldzuordnung.

**Offen (Recht):** Die Gründer haben am 10.11.2026 Option B gewählt
(ADR 0201): Register nach Art. 26 und HTTPS-Abruf des geprüften Pakets mit
kurzlebiger Berechtigung nach Art. 30 Abs. 2. Ob das genügt, bestätigt die
anwaltliche Prüfung. Ein jederzeitiger Selbstbedienungs-Export ist nicht
zugesagt.

## 8. Paket zum Abruf freigeben (Art. 30 Abs. 2 Data Act)

Grundlage: [ADR 0201](../adr/0201-mandantenrueckgabe-hat-ein-register-und-einen-abrufweg.md).
Das Paket enthält vollständige Bankdaten und Gesundheitsangaben. Wer ein Token
hat, kann es laden.

1. Paket nach Abschnitt 3 erstellen und prüfen. Freigegeben wird nur ein Paket
   mit passendem `COMPLETE`, unveränderten Dateien laut Manifest, ohne
   zusätzliche Datei oder Link, ohne Personen-Prüfpaket und mit genau der
   angegebenen Schule bzw. dem angegebenen Träger. Die CLI prüft das und bricht
   sonst ab.
2. Das geprüfte Paketverzeichnis auf den Server der Installation bringen
   (verschlüsselter Kanal, geschützte temporäre Ablage) und mit derselben
   Binary wie `server` freigeben. Der Einmal-Container nutzt die Volumes von
   `server`, also auch `phoenix-<env>-data-return`. Das Paketverzeichnis muss für UID 1001 lesbar sein:

   ```sh
   docker compose run --rm --no-deps -v "$PACKAGE_DIR":/package:ro server \
     /app/main tenant return release --package /package \
     --school "$SCHOOL_ID" --operator "$OPERATOR_ID" \
     --keep-until "$LETZTER_ABRUFTAG" \
     --recipient customer --recipient-name "$KUNDE" \
     --valid-for 168h --max-downloads 5
   ```

   Für einen Träger `--organization` statt `--school`. Lokal gilt
   `scripts/dev-native.sh backend go run . tenant return …`.
   `--keep-until` ist der letzte Tag des Abrufzeitraums (YYYY-MM-DD). Die
   Ausgabe nennt `grant_id`, `archive`, `archive_sha256`, `token` und
   `archive_key`. **Beides erscheint nur einmal.** Das Token über einen
   bestätigten zweiten Kanal an die Empfängerorganisation geben, nie in das
   Ticket oder eine unverschlüsselte Mail. Den `archive_key` im geschützten
   Auftrag (Passwortmanager) bis zum Ende des Abrufzeitraums aufbewahren.
   Danach die Paketkopie auf dem Server entfernen.
3. Token für den benannten neuen Anbieter oder ein neues Token nach Ablauf:

   ```sh
   printf '%s\n' "$ARCHIVE_KEY" | docker compose run --rm --no-deps -T server \
     /app/main tenant return grant --archive "$ARCHIVE" --operator "$OPERATOR_ID" \
     --recipient target_provider --recipient-name "$ZIELANBIETER"
   ```

   Den Schlüssel nur über stdin geben, nie als Argument. Ein Token gilt
   höchstens 30 Tage und nie über `--keep-until` hinaus.
4. Abruf durch den Empfänger, beschrieben in `/public/data-return/openapi.json`:

   ```sh
   curl -fsS -H "Authorization: Bearer $TOKEN" \
     "$API_URL/public/data-return/package"
   curl -fsS -H "Authorization: Bearer $TOKEN" -o paket.zip \
     "$API_URL/public/data-return/package/archive"
   ```

   `$API_URL` ist die API-Adresse der Installation; Agents verwenden nur
   localhost. Nach dem Download die SHA-256 der ZIP mit `archiveSHA256`
   vergleichen, dann `COMPLETE` und `manifest.json`. Jeder Download zählt; ein
   abgebrochener Download liefert weniger Bytes als `Content-Length`.
5. Überblick ohne Tokens: `tenant return list --operator "$OPERATOR_ID"`.
   Abrufe stehen in `platform.data_return_downloads`, Freigaben, Tokens,
   Widerrufe und Bereinigungen im Operator-Protokoll (Aktion `data_return_*`).
6. Ein Token sofort beenden: `tenant return revoke --grant "$GRANT_ID"`; alle
   Tokens eines Pakets: `--archive "$ARCHIVE"`, jeweils mit `--operator`.
7. Nach dem Abrufzeitraum, vor jeder Löschung nach
   [tenant-erasure.md](tenant-erasure.md) und regelmäßig:
   `tenant return purge --operator "$OPERATOR_ID"`. Das löscht Archive nach
   `--keep-until` oder mit nur widerrufenen Tokens und leert deren Schlüssel;
   die Nachweiszeilen gehen drei Jahre nach dem Abrufzeitraum.

Die Ablage `phoenix-<env>-data-return` ist nicht Teil der täglichen
Sicherung. Fehlt ein Archiv nach einem Restore, antwortet der Abruf mit 503;
dann neu exportieren und freigeben.
