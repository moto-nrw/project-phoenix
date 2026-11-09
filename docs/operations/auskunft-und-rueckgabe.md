# Interne Auskunft und Datenrückgabe

Stand: 09.11.2026, Refs #3838, Teil von #3846.
Grundlage: [ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md),
[ADR 0097](../adr/0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md),
[ADR 0184](../adr/0184-mandantenrueckgabe-oeffnet-feldverschluesselte-bankdaten.md),
[ADR 0185](../adr/0185-mandantenrueckgabe-gibt-gesundheit-einstellungen-und-traegerpaket-zurueck.md)
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

**Geöffnet und projiziert:** IBAN und Kontoinhaber der SEPA-Daten
(Bankring) und alle Gesundheitsangaben (Gesundheitsring) stehen geöffnet im
Paket, nie als Chiffrat. Einstellungen gehen ohne Geheimnisse mit:
`password`-Definitionen und unbekannte Schlüssel als `null`. Zustellprotokolle
(Sperrliste, Zustellereignisse, Push-Inhalte) gehen mit. Ein Wert, der nicht
öffnet, oder ein Container ohne Besitzer bricht den Export ab.

**Weiter offen:** Der Mail-Payload bleibt ausgeschlossen (Sicherheitsmails
tragen dort Link oder Code); der Inhalt einer Mail ist der verknüpfte
Datensatz. Die rechtliche Bewertung der Data-Act-Schnittstellenpflicht
(Abschnitt 7) und die menschlichen Nachweise unten. Die Beschäftigtenauskunft
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
   ([ADR 0185](../adr/0185-mandantenrueckgabe-gibt-gesundheit-einstellungen-und-traegerpaket-zurueck.md)).
   Der Export öffnet damit IBAN und Kontoinhaber der SEPA-Mandate und
   -Lastschriften sowie die Gesundheitsangaben; ohne passende Ringe bricht er
   ab, statt Chiffrat oder eine Lücke zu übergeben, auch für Schulen ohne
   solche Daten. Die Werte nur per SOPS in die Umgebung des Exportprozesses
   geben, nie als Datei, Shell-Literal oder CLI-Argument.
6. Privates, lokal verschlüsseltes Arbeitsmedium mit genug freiem Platz
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

Für das Trägerpaket statt `--school "$SCHOOL_ID"` den Parameter
`--organization "$ORGANIZATION_ID"` setzen. Es enthält die Trägertabellen
(Verträge, Elternabrechnung, SEPA, Ferienanmeldungen, Trägereinstellungen), die
Stammzeilen der Schulen des Trägers und die Konten der Träger-Geschäftsstelle
samt Avataren. Was eine Schule hält, steht im Manifest als `schoolPackage`
ohne Zeilen und kommt mit dem Schulpaket. Ein Schulpaket trägt weiter den
Trägerbestand; bei einer Trägerrückgabe ist das Trägerpaket desselben Auftrags
für diese Tabellen maßgeblich. Wechselt nur eine Schule, enthält ihr Schulpaket
die Trägerdaten aller Schulen des Trägers: Umfang mit dem Träger abstimmen.

Das Trägerpaket verlangt zusätzlich `--operator "$OPERATOR_ID"` und legt die
PAIN.008-Dateien bei (ADR 0187):

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
- **Operator-Protokoll:** `--operator` muss ein bestehender, **aktiver**
  Operator sein (wie beim Löschverfahren); sonst liest und schreibt der Lauf
  nichts. Jeder Lauf steht zweimal in `platform.operator_audit_log` (Aktion
  `export`, Ressource `organization`), ohne Personendaten und ohne lokalen
  Pfad: vor dem ersten Lesen der Versuch (`outcome: started`), danach das
  Ergebnis. `completed` nennt die Prüfsumme aus `COMPLETE`, die Anzahlen und
  die Herkunft jeder PAIN.008-Datei; erst danach entsteht `COMPLETE`.
  `failed` nennt die Fehlerklasse (`snapshot`, `filesystem`,
  `sepa_debit_files`, `manifest`, `audit`, `complete_marker`) und
  `partial_files_removed`: Der Lauf löscht sein unvollständiges Verzeichnis.
  Steht dort `false`, das Verzeichnis von Hand löschen. Ein Schulpaket
  schreibt (noch) keinen Eintrag (#6326).
- **PAIN.008-Dateien:** `SEPA-LASTSCHRIFTDATEIEN/<Nachrichten-ID>.xml` je
  eingereichte Lastschriftnachricht, mit `LIESMICH.txt`; `manifest.json`
  nennt sie unter `sepaDebitFiles` mit Herkunft und prüft jede mit Prüfsumme.
  IBAN und Kontoinhaber je Mandat und Lastschrift stehen nicht dort, sondern
  geöffnet in den Tabellen-Dateien (siehe Exportformat).
  - **Original:** Seit Migration `1.15.833` speichert moto beim Download die
    Datei bytegenau, versiegelt mit dem Bank-Schlüssel, mit SHA-256 und dem
    Gläubigerkonto darin (`parent_billing.sepa_debit_files`); ein späterer
    Download derselben Nachricht ersetzt sie bis zur Bestätigung der
    Einreichung. Ein nicht eingereichter Download geht nach 30 Tagen; eine
    eingereichte Datei bleibt, solange eine ihrer Lastschriften bleibt. Das
    Paket enthält diese Datei unverändert und prüft die Prüfsumme. Ein
    späterer Wechsel des Standardkontos ändert sie nicht.
  - **Rekonstruiert:** Fehlt das Original, erzeugt der Export die Datei mit
    demselben Generator neu aus den gespeicherten Lastschriften
    (Nachrichten-ID, eingereichte Beträge, Sequenz, Einzugstag, Konto zur
    Freigabe, Gläubiger-ID). Die Erstellzeit ist dann die Bestätigung der
    Einreichung. Das Gläubigerkonto ist das bei der Einreichung gespeicherte,
    für Nachrichten von vor `1.15.833` das heutige Standardkonto des Trägers;
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
  Mandaten und -Lastschriften, `health_info` bei Betreuungsprofilen, `health`
  als JSON-Text bei Ferienanmeldungen, -kindern und -gästen), ihr Chiffrat
  nicht. Container unter `projectedColumns` stehen projiziert in eigenen
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
4. Ein Wechselauftrag folgt zusätzlich dem Wechselprozess mit seinen Fristen
   in Abschnitt 7. Bei einem Wechsel bleibt das verschlüsselte Paket bis zum
   Ende des Abrufzeitraums erhalten; erst danach die lokalen Kopien entfernen.

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
  --school "$SCHOOL_ID" --person "${PERSON_SELECTOR:?Personenauswahl erforderlich}" \
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
`model.json`, Originaldateien, Prüfsummen), ohne Wechselentgelt.
Die Fristen laufen ab dem Eingang der Wechselanfrage:

| Schritt | Frist nach AGB | Wer |
|---|---|---|
| Wechselanfrage bestätigen, Umfang (Schulen, Träger, Ziel) festhalten | unverzüglich; Ankündigungsfrist höchstens 2 Monate (10.1) | zuständige Person |
| Ist der Übergang in 30 Tagen nicht machbar: begründen, Ersatzzeitraum ≤ 7 Monate nennen | 14 Arbeitstage nach der Anfrage (10.3) | zuständige Person |
| Pakete erstellen, prüfen, verschlüsselt übergeben; Dienst läuft weiter | Übergangszeitraum ≤ 30 Kalendertage nach der Ankündigungsfrist, einmal verlängerbar (10.3) | zuständige Person, zweite Person prüft |
| Verschlüsselte Pakete zum Abruf bereithalten, erneute Übergabe auf Anfrage | 30 Kalendertage nach dem Übergangszeitraum (10.3) | zuständige Person |
| Löschung nach [tenant-erasure.md](tenant-erasure.md), Nachweis auf Anfrage | danach (10.4, AVV 8.2) | zuständige Person |

Bekannte Wechselrisiken, die dem Kunden mitzuteilen sind: Daten ändern sich
während des Übergangs weiter, also vor der letzten Übergabe eine Schreibpause
vereinbaren und neu exportieren; Kennwörter, Passkeys, MFA-Faktoren, Tokens und
`password`-Einstellungen (Geräte-PIN, SFTP-Passwort) kommen nicht mit und
müssen im Zielsystem neu gesetzt werden; der Import braucht eine Feldzuordnung.

**Offen (Recht, Yannick):** Ob ein von moto-Mitarbeitenden betreuter Export
in offenem, dokumentiertem Format die „offenen Schnittstellen“ nach Art. 30
Abs. 2 Data Act erfüllt oder ob Kunde oder Zielanbieter einen eigenen Abruf
(etwa eine API) erhalten müssen. Bis zur Entscheidung kein jederzeitiger
Selbstdownload zugesagt.
