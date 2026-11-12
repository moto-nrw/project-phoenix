---
status: accepted
---

# Export Transfer übergibt Finanz-Exporte an einen Empfänger des Trägers

**Status: Akzeptiert.** Freigegeben von Yannick Wenger am 12.11.2026.

Entscheidung vom 12.11.2026 zu
[#6824](https://github.com/moto-ogs/project-phoenix/issues/6824), Teil von
[#4611](https://github.com/moto-ogs/project-phoenix/issues/4611). Entscheidung
Yannick vom 12.11.2026 in
[#4693](https://github.com/moto-ogs/project-phoenix/issues/4693#issuecomment-6818459345):
A ist das Ziel, C bleibt, B wird nie umgesetzt. Ergänzt
[ADR 0012](0012-export-transfer-is-the-tenth-platform.md) und
[ADR 0074](0074-delivery-and-export-transfer-speak-their-own-contract.md) um
einen Träger-Scope; setzt auf
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
(Owner Elternabrechnung),
[ADR 0183](0183-bankdaten-haben-einen-eigenen-schluesselring-und-einen-wiederholbaren-rekey.md)
(Bankring) und
[ADR 0187](0187-traegerpaket-enthaelt-dokumente-pain008-originale-und-ein-operator-protokoll.md)
(PAIN.008-Original) auf.

## Kontext

- Export Transfer kennt genau ein Gegenüber je Schule. Es liest sieben
  Schuleinstellungen (`sftp.*`) in der Schultransaktion und schreibt sein
  Journal `audit.export_transfers` mit `tenant_id` auf `platform.schools`.
- Sollstellung, DATEV-Buchungsstapel und PAIN.008 der Elternabrechnung
  entstehen in der Träger-Transaktion (`app.current_org_id`, ADR 0086 §5).
  Jede Datei bekommt die nächste Dateinummer des Trägers und einen Eintrag
  im Exportprotokoll `parent_billing.export_files` (#4693, E-b, E-d).
- Heute lädt der Träger die Datei herunter und lädt sie beim Empfänger
  selbst hoch (C). Der Empfang ist nirgends vermerkt.
- Passwort-Einstellungen gibt es nur je Schule und unverschlüsselt in
  `config.setting_values` (`registry.go`: `password` nicht auf Träger-Ebene).

## Optionen

| Option | Wirkung | Bewertung |
|---|---|---|
| **A** Export Transfer bekommt einen Träger-Scope | eigener Empfänger, eigene Zugangsdaten, eigenes Journal und eigene Transaktion je Träger | **gewählt** |
| **B** Übergabe über das Gegenüber einer gewählten Schule | Finanzdaten des Trägers mit den Zugangsdaten einer Schule, Zuständigkeit unklar, falscher Empfänger möglich | verworfen, auch nicht als stiller Ersatz |
| **C** manueller Download und Upload | sofort nutzbar, Empfang nicht vermerkt | bleibt als Weg neben A erhalten |

## Entscheidung

### 1. Owner und Abhängigkeitsrichtung

- **Export Transfer** (`export-transfer`, Plattform) besitzt den Empfänger
  des Trägers `config.traeger_export_recipients`, das Journal
  `audit.traeger_export_transfers`, die Auflösung des Ziels, die
  Adressprüfung und den SFTP-Transport. Es wählt keinen Inhalt und kennt
  keine Rechnung.
- **Elternabrechnung** (`parent-billing`) erzeugt die Datei, vergibt die
  Dateinummer, schreibt das Exportprotokoll und vermerkt dort die Übergabe
  in `parent_billing.export_file_transfers`.
- Richtung: Domäne → Plattform. Die Elternabrechnung deklariert den Port
  `parentbilling.FinanceExportTransfer`; die Root bindet ihn an die
  öffentliche Fähigkeit `exporttransfer.TraegerTransfers`. Kein Paket der
  Elternabrechnung importiert Export Transfer, Export Transfer importiert
  nichts aus der Elternabrechnung. Es entsteht keine Kante im Owner-Graph
  und keine neue Policy-Regel über Owner-Grenzen.
- Die Routen des Empfängers (`/traeger/export-recipient`) gehören Export
  Transfer selbst: ein neues Paket `modules/exporttransfer/http` mit der
  Rolle `http`, das nur den eigenen Vertrag und `api/common` nutzt. Die
  Übergabe-Routen (`/traeger/finance-handovers/runs/{kind}/{id}/{format}`)
  gehören der Elternabrechnung, weil sie deren Datei erzeugen. Der Port
  `FinanceExportTransfer` der Elternabrechnung hat nur `Handover`; die Root
  bindet ihn an `TraegerTransfers.Transfer`.

### 2. Scope von Empfänger, Zugangsdaten und Journal

- Genau ein Empfänger je Träger (`organization_id` ist Primärschlüssel).
  Mehrere Empfänger (Kämmerei und Steuerbüro getrennt) sind ein eigener
  Beschluss; die Tabelle lässt ihn ohne Umbau des Journals zu.
- Empfänger und Journal tragen `organization_id` auf `platform.organizations`
  (`ON DELETE CASCADE`) und keine `tenant_id`. RLS mit `FORCE` prüft
  `app.current_org_id`; in einer Schultransaktion sind beide Tabellen leer.
  Jede Abfrage nennt den Träger zusätzlich im eigenen Filter, auch unterhalb
  von RLS.
- Der Schul-Scope bleibt unverändert: dieselben sieben Einstellungen,
  dieselbe Tabelle `audit.export_transfers`, dieselbe Fähigkeit
  `exporttransfer.Capability`. Der Träger-Scope liest keine
  Schuleinstellung, der Schul-Scope keinen Träger-Empfänger.

### 3. Zugangsdaten

- Das Passwort wird nur versiegelt gespeichert
  (`password_sealed BYTEA`), mit Security Runtimes Bank-Cipher
  (`BANK_DATA_KEYS`, ADR 0183) über den consumer-owned Port
  `BankDataCipher` von Export Transfer. Kontext:
  `config.traeger_export_recipients.password|<organization_id>`. Ein
  Chiffrat eines anderen Trägers oder einer anderen Spalte öffnet nicht.
- Begründung für den Bankring: Die Zugangsdaten entscheiden, wohin
  PAIN.008-Dateien mit IBAN und Mandaten gehen. Sie gehören in denselben
  Schutzbereich wie diese Bankdaten. Ein eigener Ring bräuchte vor dem
  Merge neue SOPS-Werte in allen Umgebungen; der Bankring ist dort schon
  gesetzt.
- Rotation: `bank-data-rekey --organization ID` verarbeitet zusätzlich die
  Quelle `export_recipients` mit denselben Garantien wie ADR 0183 Punkt 4
  bis 6 (Sperre vor dem Öffnen, Filter auf den Träger, nur die versiegelte
  Spalte, idempotent). Der Lauf je Träger aus ADR 0183 erfasst den
  Empfänger also ohne neuen Schritt.
- Nie ausgegeben: Keine Antwort, kein Journal, kein Log und keine
  Fehlermeldung enthält das Passwort, auch nicht maskiert. Die Antwort sagt
  nur `password_set`. Speichern ohne Passwort behält das gespeicherte.
  Lässt sich ein Chiffrat nicht öffnen (Schlüssel entfernt), gilt das
  Passwort als fehlend und der Empfänger als nicht bereit.
- Der Text des Gegenübers erreicht keine Fehlermeldung: Ein Gegenüber
  kann mit dem Passwort antworten, das es erhalten hat. Der SFTP-Adapter
  behält von einer Antwort nur den SFTP-Statuscode; der Fehler, der ins Log
  geht, nennt Schritt und Code. Das gilt auch für den Schul-Scope, der
  denselben Adapter nutzt.
- Das Journal hält weder Benutzername noch Passwort noch Fingerabdruck,
  nur Adresse, Port und Ordner.

### 4. Transport

SFTP, wie Export Transfer es heute für Schulen kann: derselbe Client, dieselbe
Adressprüfung (nur öffentliche Adressen), derselbe gepinnte
SHA256-Fingerabdruck ohne Trust-on-first-use, dasselbe atomare Ersetzen unter
dem endgültigen Namen. Adresse, Port, Benutzername, Passwort, Ordner und
Fingerabdruck sind Werte des Empfängers, die der Träger einstellt; keiner
ist fest eingebaut. Was beim Kunden noch fehlt (Kämmerei Bissendorf, SJR),
trägt der Träger dort ein.

### 5. Ablauf einer Übergabe

Eine Übergabe ist ein Export wie ein Download, nur mit dem Empfänger statt
des Browsers als Ziel. `TraegerTransfers.Transfer` öffnet seine eigenen
Träger-Transaktionen, getrennt von der Anfrage:

1. **Vorhaben:** Empfänger auflösen. Ist er nicht bereit, endet der Aufruf
   mit `not_configured`: keine Datei, keine Nummer, kein Journal.
   Sonst ruft Export Transfer `PrepareFile` der Elternabrechnung in dieser
   Transaktion auf: Sie erzeugt die Datei über denselben Weg wie der
   Download (Dateinummer, Protokolleintrag, Zugriffsvermerke) und schreibt
   die Übergabe mit Status `pending` und dem Ziel. Danach schreibt Export
   Transfer das Vorhaben ins Journal. Alles committet zusammen.
2. **Zaun:** eigener Commit des einmaligen Dispatch-Zauns wie im Schul-Scope
   (`audit.claim_traeger_export_transfer`, zwei Minuten).
3. **Senden:** erst nach beiden Commits.
4. **Abschluss:** In einer Transaktion setzt Export Transfer das Journal auf
   Erfolg oder Grund (`audit.complete_traeger_export_transfer`) und ruft
   `Settle` der Elternabrechnung auf, die die Übergabe auf `transferred`
   oder `failed` mit Grund setzt. Journal und Exportprotokoll stimmen so
   immer überein.

### 6. Wiederholung und Fehlerfälle

- **Kein automatisches Wiederholen.** Jeder Versuch ist eine Handlung einer
  Person, wie im Schul-Scope.
- **Idempotent:** Eine Übergabe ist ein neuer Export mit neuer Nummer
  (E-d). Liegt für denselben Lauf und dasselbe Format schon eine Übergabe
  mit gleichem Inhalt vor, die angekommen, unterwegs oder ungeklärt ist,
  verweigert die Elternabrechnung eine neue mit
  `parent_billing.export_already_transferred`, bevor eine Nummer vergeben
  ist. Nur eine ausdrückliche Wiederholung (`repeat`) sendet sie erneut.
  Zwei gleichzeitige Anfragen serialisiert die Zählersperre der Dateinummer;
  die zweite sieht die erste und wird verweigert.
- **Fehlgeschlagen** (Adresse, Fingerabdruck, Anmeldung, Verbindung, Upload,
  Größe, intern): Journal und Protokoll nennen den stabilen Grund. Eine neue
  Übergabe ist ohne `repeat` möglich, weil die Datei nicht angekommen ist.
  Beim DATEV-Buchungsstapel gibt der Fehlschlag die Buchungen dieser Datei
  wieder frei: Ein Trigger auf `parent_billing.export_file_transfers` löscht
  beim Wechsel von `pending` auf `failed` ihre Zeilen in
  `parent_billing.datev_booked_documents`. Die nächste Übergabe enthält sie
  wieder, statt mit „keine neuen Buchungen“ zu enden; das Steuerbüro bucht
  nichts doppelt, weil die fehlgeschlagene Datei nie ankam. Hielt eine
  spätere DATEV-Datei, deren Übergabe nicht fehlschlug, dieselbe Buchung
  (etwa ein Download „Alle Buchungen erneut“), wandert die Zeile zu dieser
  Datei: Die Buchung ging dort hinaus. Dafür hält jede DATEV-Datei ihre
  Buchungen genau (`export_files.datev_booking_keys`: Rechnung, Storno oder
  Beleg mit ID); eine Rechnung und ihr Storno sind verschiedene Buchungen.
  Die Freigabe nimmt zuerst die Sperre des Dateinummern-Zählers des
  Trägers, die jeder Export bis zum Commit hält: Sie wartet auf einen
  laufenden Export und sieht seine Buchungen, und zwei Fehlschläge laufen
  nacheinander.
- **Gleicher Inhalt einer Sollstellung:** Schreibt die Feldzuordnung die
  Dateinummer in jede Zeile, vergleicht die Wiederholungsprüfung die
  Zeilen ohne Nummer (`booking_sha256`, wie bei DATEV); ältere Dateien ohne
  diesen Wert vergleicht sie über den Inhalt. So fragt auch eine
  nummerierte Sollstellung vor der zweiten Übergabe nach.
  „Alle Buchungen erneut“ gibt es auch als Übergabe, mit Wirtschaftsjahr,
  für eine Datei, die der Empfänger verloren hat.
- **Ungeklärt:** Antwortet das Gegenüber nicht auf das Umbenennen (die
  Verbindung bricht ab, die Zeit läuft ab), kann die Datei angekommen
  sein. Ein SFTP-Status ist eine Antwort und ein Fehlschlag, auch
  „keine Berechtigung“ und „nicht gefunden“, die `pkg/sftp` als
  `os.ErrPermission`/`os.ErrNotExist` weitergibt; alles andere
  bleibt offen: kein Abschluss, keine Freigabe von Buchungen. Ebenso,
  wenn der Abschluss nach dem Senden scheitert. Dann bleibt die
  Übergabe `pending`. Nach Ablauf des Zauns zeigt das Protokoll „Ergebnis
  unklar“; die Wiederherstellung im Worker setzt das Journal wie im
  Schul-Scope auf `unknown`, ohne erneut zu senden. Eine neue Übergabe
  braucht `repeat`.
- **Nicht eingerichtet:** `parent_billing.export_recipient_not_ready` (409),
  ohne Nummer und ohne Protokolleintrag.

### 7. Rechte

- Empfänger lesen, speichern, entfernen: Träger-Recht „Finanz-Export“
  (`traeger_finance:export`), wie DATEV-Werte und Feldzuordnung.
- Sollstellung und DATEV-Buchungsstapel übergeben: „Finanz-Export“, wie ihr
  Download. PAIN.008 übergeben: „Abrechnung“
  (`traeger_billing:manage`), wie ihr Download. Wer den Empfänger nicht
  lesen darf, sieht die Übergabe der Lastschriftdatei trotzdem; ein
  fehlender Empfänger kommt dann als `export_recipient_not_ready` zurück.
- Die Organisation kommt immer aus der Sitzung, nie aus der Anfrage.

### 8. Protokoll, Aufbewahrung und Löschung

- Das Finanz-Exportprotokoll bleibt beim Owner Elternabrechnung.
  `export_file_transfers` hängt mit `ON DELETE CASCADE` an seiner
  Exportdatei und lebt so lange wie diese.
- Das Träger-Journal hält das Konto (`ON DELETE SET NULL`), keinen Namen.
  Es bleibt wie das Exportprotokoll, das es belegt, und geht mit dem
  Träger.
- Der Empfänger nennt das ändernde Konto (`ON DELETE SET NULL`) und geht
  mit dem Träger.

### 9. Architektur und Register

- Kein neuer Owner, kein Workflow und kein neuer Job: Die vorhandene
  Wiederherstellung (`NewDispatchRecovery`) erledigt beide Journale in
  derselben Admin-Transaktion. Eine Budget-ADR nach Muster 0209/0215 ist
  deshalb nicht nötig; die berührten Register stehen unten.
- Export Transfer bekommt einen Port `BankDataCipher` (Marker
  `BankDataRing()`) und damit keine neue Importregel; die Root bindet
  `env.BankData`.
- Messwerte der Register (Compose-LOC, Pass-through, Fassadenbreite) stehen
  im Abschnitt „Register“ und gelten nur für diese Stellen.

## Invarianten und ihre Tests

1. Träger gegen Träger: Empfänger, Journal und Übergaben eines Trägers sind
   für einen anderen weder lesbar noch schreibbar, auch unterhalb von RLS.
2. Träger gegen Schule: Der Träger-Scope liest keine Schuleinstellung und
   schreibt nicht in `audit.export_transfers`; eine Schultransaktion sieht
   keinen Empfänger und kein Träger-Journal.
3. Der Schul-Scope verhält sich unverändert.
4. Das Passwort steht nur versiegelt in der Datenbank, öffnet nur im
   eigenen Kontext und erscheint in keiner Antwort.
5. Keine Verbindung vor dem Commit von Datei, Protokoll und Vorhaben.
6. Eine Übergabe gleichen Inhalts wird ohne `repeat` nicht erneut gesendet.
7. Journal und Protokoll werden in derselben Transaktion abgeschlossen.
8. Eine fehlgeschlagene DATEV-Übergabe verbraucht keine Buchungen.
9. Der Zaun nimmt ein Vorhaben einmal an und schließt es einmal ab, nur mit
   seinem Token, nur im eigenen Träger und nur vor Ablauf.
10. Ein Gegenüber, das das Passwort zurückgibt, bringt es nicht ins Log.
11. Ein unbeantwortetes Umbenennen gilt als ungeklärt, nicht als
    fehlgeschlagen.
12. Eine Buchung, die eine spätere, nicht fehlgeschlagene DATEV-Datei hielt,
    bleibt nach dem Fehlschlag gebucht, auf dieser Datei.
13. Eine nummerierte Sollstellung gleichen Inhalts geht ohne `repeat` nicht
    erneut hinaus.
14. Die Freigabe wartet auf einen laufenden Export und auf eine andere
    Freigabe desselben Trägers.

Jede Invariante hat einen Test, der ohne den Schutz rot wird; die Belege
stehen im PR.

## Register

Gemessen auf dem Stand dieser Entscheidung; jede spätere Abweichung bekommt
Wert alt → neu und Grund.

| Register | alt → neu | Grund |
|---|---|---|
| `moduleComposeBudgets["modules/exporttransfer/compose"]` | 149 → 182 | Träger-Scope (`NewTraegerTransfers`) und Rekey des Empfänger-Passworts |
| `moduleComposeBudgets["modules/parentbilling/compose"]` | 124 → 129 | `NewExportHandovers` |
| `moduleComposeBudgetTotal` | 13378 → 13416 | Summe der beiden Zeilen |
| `demoErasureExceptions` | 189 → 191 | Empfänger und Journal entstehen nur im Träger-Büro des Demo-Trägers, nie bei einem Besucher (`traegerBueroOnly`) |
| Transaktionsgrenzen (`test/transaction_boundaries.json`) | +3 | Bindung des Träger-Scopes, die eine Grenze `TraegerService.within`, die Vorhaben-API `TraegerJournal.Begin`; geprüft in `test/traeger_export_transfer_transaction.go` |
| Command-Effects `parent-billing` | +1 | `FinanceExportTransfer.Handover` → `TraegerTransfers.Transfer`, `platform`/`durable` nach ADR 0098 |
| Policy | +3 Datenobjekte, +1 Paket, +2 Regeln | Datenobjekte `config.traeger_export_recipients`, `audit.traeger_export_transfers` (Export Transfer), `parent_billing.export_file_transfers` (Elternabrechnung); Paket `modules/exporttransfer/http` mit `http → contract` und `http → api/common` |
| Pass-through-Register | unverändert | `TraegerService.Recipient` liest Empfänger und Journal, `DeleteRecipient` löscht und liest neu; der Root-Adapter übersetzt nur `Handover` mit Fehlerklassen |
| `belegErasureFunctions` (`database/migrations/beleg_deletion_guard_test.go`) | 6 → 7 | `parent_billing.release_failed_handover_bookings` löscht nur die DATEV-Markierungen einer Datei, deren Übergabe fehlschlug (Abschnitt 6); Rechnungen, Stornos, Belege und Protokolleintrag bleiben |
| Abfragebudget | neu `api.traeger.export_recipient` = 3 | Träger-Bindung, Empfänger, Journal; flach in der Zahl der Übergaben |

## Folgen

- CIP- und SIMBA-Adapter (#4698) nutzen denselben Empfänger und denselben
  Ablauf.
- Der manuelle Download bleibt unverändert.
