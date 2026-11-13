---
status: accepted
---

# Träger übergibt jedes Finanz-Exportformat an einen eigenen Empfänger

**Status: Akzeptiert.** Freigegeben von Yannick Wenger am 13.11.2026.

Entscheidung zu
[#6911](https://github.com/moto-ogs/project-phoenix/issues/6911), Teil von
[#4611](https://github.com/moto-ogs/project-phoenix/issues/4611). Yannick
Wenger hat am 13.11.2026 Option A gewählt
([Kommentar](https://github.com/moto-ogs/project-phoenix/issues/6911#issuecomment-6838383473)).
Ergänzt [ADR 0218](0218-export-transfer-uebergibt-finanz-exporte-an-einen-empfaenger-des-traegers.md)
und ersetzt dort nur Abschnitt 2, Satz „Genau ein Empfänger je Träger“. Alle
übrigen Entscheidungen und Invarianten von ADR 0218 gelten unverändert, jetzt
je Empfänger. Kein neuer Owner, kein neuer Workflow, kein neuer Job.

## Kontext

- Seit ADR 0218 hat ein Träger genau einen Empfänger
  (`config.traeger_export_recipients`, Primärschlüssel `organization_id`).
  Gehen Sollstellung und DATEV-Buchungsstapel an verschiedene Stellen
  (Kämmerei und Steuerbüro), bleibt ein Format beim manuellen Download.
- ADR 0218 hielt die Tabelle für mehrere Empfänger offen; das Journal braucht
  keinen Umbau, nur den Empfänger je Eintrag.

## Optionen

| Option | Wirkung | Bewertung |
|---|---|---|
| **A** mehrere Empfänger je Träger, jedem Exportformat genau einer | deckt getrennte Stellen ab | **gewählt** |
| **B** ein Empfänger, Zielpfad je Format | hilft nur bei einem gemeinsamen Server | verworfen |
| **C** ein Empfänger, anderes Format per Download | kein Aufwand, Übergabe bleibt manuell | bleibt als Weg für jedes Format ohne Zuordnung |

## Entscheidung

### 1. Empfänger und Zuordnung

- Ein Träger hat beliebig viele Empfänger. Jeder hat eine eigene `id` und
  einen Namen (1 bis 80 Zeichen, ohne Steuerzeichen), der unter den
  Empfängern des Trägers ohne Rücksicht auf Groß- und Kleinschreibung
  eindeutig ist (`uq_traeger_export_recipients_name`).
- `config.traeger_export_recipient_formats` (Export Transfer) ordnet jedem
  Format höchstens einen Empfänger zu: Primärschlüssel
  `(organization_id, export_format)`, zusammengesetzter Verweis
  `(organization_id, recipient_id)` auf einen Empfänger **desselben** Trägers.
  Formate sind `sollstellung`, `datev`, `pain008` (Prüfung in der Tabelle und
  `exporttransfer.TraegerFormats`). Ein Format ohne Zeile wird nur
  heruntergeladen (Option C).
- Ein neuer Empfänger bekommt kein Format von selbst. Der Träger ordnet zu;
  eine Zuordnung gilt ganz oder gar nicht.
- **Löschen:** Wer einen Empfänger entfernt, entfernt sein versiegeltes
  Passwort und seine Zuordnungen (`ON DELETE CASCADE`). Seine Formate gehen
  zurück auf den Download; andere Empfänger und ihre Formate bleiben. Das
  Journal behält seine Einträge mit dem Namen von damals und verliert nur den
  Verweis (`ON DELETE SET NULL (recipient_id)`).

### 2. Auflösung bei der Übergabe

`TraegerTransfers.Transfer` löst den Empfänger über das Format der Anfrage auf
(`RecipientForFormat`), in derselben Träger-Transaktion wie das Vorhaben
(ADR 0218 Abschnitt 5). Hat das Format keinen bereiten Empfänger, endet der
Aufruf wie bisher mit `not_configured`: keine Datei, keine Nummer, kein
Journal. Die Elternabrechnung erfährt Empfänger, Name, Adresse und Ordner
(`parentbilling.HandoverDestination`) und vermerkt sie mit der Übergabe.

### 3. Journal, Wiederholung und Fehlerzustand je Empfänger

- `audit.traeger_export_transfers` und `parent_billing.export_file_transfers`
  nennen `recipient_id` und `recipient_name` (Name zur Zeit der Übergabe).
  Die Übergabe im Exportprotokoll hält die ID ohne Verweis über die
  Owner-Grenze; ihr Trigger hält den Empfänger wie Datei und Ziel fest.
- **Wiederholung:** Die Prüfung auf gleichen Inhalt (ADR 0218 Abschnitt 6)
  fragt je Empfänger. Hat derselbe Empfänger die Datei schon (angekommen,
  unterwegs oder ungeklärt), braucht eine neue Übergabe `repeat`. Ein anderer
  Empfänger hatte sie nie; die Übergabe an ihn fragt nicht nach.
- **Fehlerzustand:** Jeder Empfänger zeigt seine neueste Übergabe
  (`last_handover`) bis zur nächsten. Ein Fehlschlag bei einem Empfänger
  ändert nichts am Zustand eines anderen.

### 4. Passwort

Das Passwort bleibt versiegelt mit dem Bankring im Kontext
`config.traeger_export_recipients.password|<organization_id>` (ADR 0218
Abschnitt 3). Der Empfänger ist nicht Teil des Kontexts, damit das vorhandene
Chiffrat ohne Neuversiegelung weiter öffnet. Nur der Store schreibt die
Spalte, und keine Route übernimmt ein Chiffrat von einem Empfänger auf einen
anderen; ein neuer Empfänger beginnt immer ohne Passwort. Die Regel „Passwort
folgt keinem neuen Ziel“ gilt je Empfänger. Der Rekey (ADR 0183) verarbeitet
alle Empfänger des Trägers in Stapeln nach `id`, mit Sperre vor dem Öffnen.

### 5. Übernahme der Daten

Die Migration 1.15.963 gibt dem vorhandenen Empfänger eine `id` und als Namen
seine Adresse (sonst „Empfänger der Exporte“), ordnet ihm alle drei Formate
zu, die er bisher bediente, und trägt ihn in alle vorhandenen Journal- und
Übergabezeilen seines Trägers ein. Werte und Chiffrat bleiben unverändert.
Der Rückweg verweigert sich, solange ein Träger mehr als einen Empfänger hat.

### 6. Rechte und Routen

- `/traeger/export-recipient` (Export Transfer): Liste mit Zuordnung und
  letzten Übergaben, Anlegen, `/{id}` Lesen, Speichern, Entfernen,
  `/formats` Zuordnen. Alles hinter „Finanz-Export“
  (`traeger_finance:export`), in der Träger-Transaktion der Sitzung; die
  Organisation kommt nur aus der Sitzung. Ein Empfänger eines anderen Trägers
  antwortet `parent_billing.export_recipient_not_found` (404).
- Die Übergabe-Routen und ihre Rechte aus ADR 0218 Abschnitt 7 bleiben.

## Invarianten und ihre Tests

1. Ein Format geht nur an den Empfänger, dem der Träger es zugeordnet hat;
   ohne bereiten Empfänger entsteht keine Datei.
2. Ein Format zeigt nie auf einen Empfänger eines anderen Trägers, auch ohne
   RLS und ohne den Filter des Stores (zusammengesetzter Verweis).
3. Jede Abfrage nennt den Träger im eigenen Filter, auch unterhalb von RLS
   (Liste, Lesen, Sperren, Format, Speichern, Löschen, Zuordnen, Rekey,
   Journal).
4. Entfernen gibt die Formate des Empfängers an den Download zurück und
   lässt die anderen unverändert; das Journal behält seine Einträge.
5. Die Wiederholungsprüfung gilt je Empfänger; die Übergabe hält ihren
   Empfänger fest.
6. Zwei Empfänger eines Trägers tragen nie denselben Namen.
7. Die Migration übernimmt Empfänger, Passwort, Formate, Journal und
   Übergaben ohne Verlust.

Jede Invariante hat einen Test, der ohne den Schutz rot wird; die Belege
stehen im PR.

## Register

| Register | alt → neu | Grund |
|---|---|---|
| `demoErasureExceptions` | 191 → 192 | `config.traeger_export_recipient_formats` entsteht nur im Träger-Büro des Demo-Trägers, nie bei einem Besucher (`traegerBueroOnly`) |
| Policy `export-transfer` | +1 Datenobjekt | `config.traeger_export_recipient_formats` |
| Abfragebudget | `api.traeger.export_recipient` = 3, unverändert | Träger-Bindung, Empfänger mit Formaten und letzter Übergabe, Journal; flach in Empfängern und Übergaben |
| Command-Effects `parent-billing` | Digest der Bindung `FinanceExportTransfer.Handover` | die Bindung reicht den Empfänger durch; Effekt bleibt `platform`/`durable` |

## Folgen

- CIP- und SIMBA-Adapter (#4698) bekommen je ein eigenes Format und damit
  einen eigenen Empfänger, sobald ihre Formate feststehen (neue Werte in der
  Formatprüfung).
- Der manuelle Download bleibt für jedes Format unverändert.
