---
status: accepted
---

# Anmeldungswerte reisen dekodiert als Formularwerte im Vertrag

Entscheidung vom 08.11.2026 für
[#6092](https://github.com/moto-ogs/project-phoenix/issues/6092). Sie ersetzt
für die Anmeldung (`enrollment.Request`, `enrollment.RequestChild`) und die
Eingaben der Anmeldestrecke die Festlegung E4 aus
[#3565](https://github.com/moto-ogs/project-phoenix/issues/3565), die Antworten,
Zustimmungen und Herkunftsdaten der Familie als rohes JSON
(`json.RawMessage`) im öffentlichen Vertrag zu führen. Kein neuer Owner, kein
neues Datenobjekt, keine Policy-Regel, keine Migration;
[ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
Punkt 1 gilt unverändert.

## Kontext

Eine Anmeldung lag in vier Formen vor: roh im Vertrag
(`modules/enrollment/request.go`), dekodiert in der Anwendung
(`internal/application/retained_request.go`, `decision_records.go`,
`report_records.go`) und noch einmal dekodiert in einer HTTP-Kopie
(`http/owner_values.go` mit Decodern in `owner_decisions.go`,
`owner_intake.go`, `owner_change_requests.go`). Jede Feldänderung musste in
allen Kopien nachgezogen werden, und die Kopien kodierten leere Werte
verschieden: Die Anwendung schrieb für leere `CustomData` das JSON-Literal
`null` in eine Spalte `JSONB NOT NULL DEFAULT '{}'`, die HTTP-Kopie gar keinen
Wert. Welche Form in der Datenbank landete, hing vom schreibenden Pfad ab.

E4 hatte rohes JSON gewählt, weil ADR 0067 Punkt 1 `map[string]any` im
öffentlichen Vertrag verbietet; die Architekturprüfung erzwingt das
(`contracts.filter-map`) auch für benannte Typen, Aliase und Signaturen
exportierter Funktionen und Methoden. Die Antworten der Familie folgen dem
Formularschema der Schule und haben keinen festen Go-Typ.

## Entscheidung

1. **`enrollment.FormValues` ist der eine dekodierte Wert** für ein
   JSON-Objekt, das eine Familie einreicht: die Antworten (`CustomData` von
   Anmeldung und Kind), die Zustimmungen (`ConsentFlags`) und die
   Herkunftsdaten (`SourceMetadata`), in der gespeicherten Anmeldung wie in
   den Eingaben der Anmeldestrecke. Der Typ hält seine Map unexportiert und
   bietet nur `Lookup`, `Set`, `Unset`, `All`, `Len`, `IsZero`, `Clone` und
   `CollectFormValues` an; keine exportierte Methode nimmt oder liefert
   `map[string]any`. ADR 0067 Punkt 1 bleibt damit wörtlich erfüllt, die
   Prüfung braucht keine Ausnahme. Die Namen weichen `Get`/`Delete` aus, die
   `contracts.generic-crud` als CRUD-Form ablehnt; `All` und
   `CollectFormValues` schreiben die Form von `iter.Seq2` aus, weil die Policy
   `iter` keine Abhängigkeitsklasse gibt.
2. **Ein Änderungswunsch unterscheidet „nicht geschickt“ von „leer“.**
   `EditPatch.ConsentFlags` und `EditPatch.CustomData` sind `*FormValues`:
   `nil` lässt den gespeicherten Wert stehen, jeder Wert, auch `{}`, ersetzt
   ihn, wie zuvor beim rohen JSON.
3. **Eine Kodierung, eine leere Form.** `FormValues` kodiert sich selbst:
   Leer ist immer `{}`, nie `null` und nie „kein Wert“. Beim Lesen werden
   `null` und ein fehlender Wert zu leeren Werten; alles, was kein Objekt
   ist, ist ein Fehler. Vorhandene `null`-Zeilen heilen beim nächsten
   Schreiben, eine Datenmigration ist nicht nötig.
4. **Der Speicher dekodiert genau einmal.** Der Postgres-Adapter des Owners
   übersetzt Zeile und Vertragswert; Anwendung und HTTP arbeiten nur noch mit
   dem Vertragswert. Die dekodierten Kopien der Anwendung und der HTTP-Rolle
   und ihre Codecs entfallen. Die Datumsfelder des Kinds reisen als
   `calendar.Date`, in der Zeile wie im Vertrag, so dass auch das
   Datumsparsen der Kopien entfällt. Wo die
   Formularlogik (Prüfen, Filtern, Zusammenführen) eine veränderbare Map
   braucht, kopiert sie die Werte ohne JSON in eine lokale Map und zurück.
5. **Die Einwilligungsnachweise bleiben Bytes.** `LegalBlocksSnapshot` reist
   typisiert als `[]LegalBlocksSnapshotEntry`; die darin eingefrorenen
   `ConsentFlags` eines Eintrags bleiben `json.RawMessage`, damit der Nachweis
   einer früheren Antwort nicht beim Umkodieren neu geschrieben wird.

## Folgen

- Eine Feldänderung an einer Anmeldung betrifft den Vertrag, die Zeile des
  Adapters und die lesenden Stellen, keine Kopien mehr.
- Die HTTP-Antworten behalten ihre Feldnamen. Leere Antworten erscheinen
  einheitlich als `{}`; wo ein Feld bisher bei leeren Werten entfiel
  (`omitempty`), entfällt es weiter (`omitzero`).
- Die Änderung bleibt im Enrollment-Owner: Außerhalb benutzen nur Tests die
  Anmeldetypen.
- Eine neu unterschriebene Anmeldung führt leere Antworten im signierten
  Dokument als `{}`. Gespeicherte Prüfsummen werden nicht neu berechnet.
- Die Einreichung schrieb leere Antworten eines Kinds bisher als `null`, so
  dass viele Basis-Snapshots offener Änderungswünsche `null` festhalten. Die
  Freigabe liest dort `null` für Antworten und Zustimmungen als `{}`, wie
  schon ein fehlendes `target_school_class`; sonst meldete jeder ältere
  Änderungswunsch einen Konflikt
  (`modules/enrollment/http/change_request_null_base_snapshot_test.go`).
  Antworten in Änderungswünschen erscheinen in den Antworten der Routen
  ebenfalls als `{}`.
- Zwei HTTP-Dienstmethoden (`CreateManualApprovedEnrollment`,
  `ListReviewQueue`) hätten nach dem Wegfall des Codecs nur noch an den Owner
  weitergereicht; die HTTP-Dienste befördern stattdessen die Fähigkeit des
  Owners wie schon die Formularvorschau, es entsteht kein neues Pass-through.
  `GetByStatusToken` bleibt eine Übersetzung der Route-Form und steht mit
  Verweis auf diese Entscheidung im Register.
- Ein Dekodierfehler einer gespeicherten Spalte nennt Anmeldung oder Kind und
  Spalte.
- Ein Regressionstest
  (`modules/enrollment/http/rollover_empty_form_values_test.go`) hält die
  leere Form am Speicherpfad fest.
