---
status: accepted
---

# Abrechnungslauf stellt unveränderliche Rechnungen aus einem Nummernkreis aus

Entscheidung vom 26.10.2026 zu [#4668](https://github.com/moto-nrw/project-phoenix/issues/4668),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 70, 71, 75 und 78. Sie ist der zweite Bau-Schnitt des Owners
Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
und baut auf
[ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md)
auf. Sie führt keinen Owner ein und hält fest, was der Lauf in den Registern
ändert (ADR 0057 Punkt 5, ADR 0070).

## Entscheidung

1. **Lauf.** `parent_billing.runs` ist der Abrechnungslauf aus ADR 0088 §4
   für einen Ferienzeitraum. Er hat die Zustände `calculated` und
   `released`; `abgeschlossen` folgt mit Zahlung und Export. Ein Zeitraum hat
   höchstens einen Lauf, der noch nicht freigegeben ist (eindeutiger Index).
   Der Lauf hält Name und Tage des Zeitraums als Kopie und hat keinen
   Fremdschlüssel auf Enrollment.
2. **Vorschau.** Ein Lauf in `calculated` speichert keine Entwürfe. Seine
   Vorschau entsteht bei jedem Lesen: je bestätigter Anmeldung des Zeitraums,
   die noch keine Rechnung hat, ein Entwurf mit den Positionen des Beitrags,
   den die Bestätigung festgehalten hat (ADR 0115 Punkt 6), und danach den
   Korrekturen des Laufs. Anmeldungen ohne festgehaltenen Beitrag zählt die
   Vorschau als `unbilled` und lässt sie aus. Bis zur Freigabe zeigt der Lauf
   so immer den Stand von heute, auch nach weiteren Bestätigungen.
3. **Im Voraus.** Ferien werden im Voraus abgerechnet: Der Lauf rechnet die
   gebuchten Wochen nach dem festgehaltenen Beitrag ab, nicht nach der
   Anwesenheit, und darf vor der ersten Woche laufen. Die nachträgliche
   Abrechnung (Mensa) kommt mit dem Workflow `billingrun` (ADR 0088 §6).
4. **Korrektur.** Eine Korrektur setzt den Betrag eines Kindes in einem
   Lauf in `calculated` auf einen neuen Wert, nie unter null, mit Pflicht-
   begründung. `parent_billing.run_corrections` speichert die Differenz,
   den Betrag davor und danach, die Begründung und das Konto; Korrekturen
   werden nur angefügt. Jede Korrektur ist eine eigene Position der Rechnung
   (ADR 0088 §4), die Liste der Korrekturen ist ihre sichtbare Historie.
5. **Nummernkreis.** `parent_billing.number_ranges` hält je Träger und
   Leistung ein Muster aus Text, laufender Nummer `{N}` bis `{NNNNNNNN}`
   (mit Nullen aufgefüllt) und optional dem Jahr `{JJJJ}` oder `{JJ}`. Ein
   Muster mit Jahr zählt jedes Jahr ab 1, eines ohne zählt weiter.
   `parent_billing.number_range_counters` hält die letzte Nummer je Träger,
   Leistung und Jahr. Ein Rechnungssteller (ADR 0088 §4) fehlt noch; bis er
   kommt, ist der Träger der Rechnungssteller.
6. **Freigabe.** Die Freigabe sperrt den Lauf (`FOR UPDATE`), rechnet die
   Vorschau in derselben Träger-Transaktion neu und lehnt ab, wenn Zahl oder
   Summe der Rechnungen nicht mehr dem entsprechen, was die Abrechnung geprüft
   hat (409 `parent_billing.billing_run_changed`). Dann erhöht ein
   `INSERT … ON CONFLICT DO UPDATE` den Zähler um die Zahl der Rechnungen und
   hält die Zeile bis zum Ende der Transaktion. Jede Rechnung bekommt in
   derselben Transaktion ihre Nummer in der Reihenfolge der Anmeldungen.
   Scheitert danach etwas, rollt die Route zurück und gibt die Nummern frei.
   So sind die Nummern lückenlos, auch bei parallelen Freigaben (Test mit
   parallelen Freigaben). Eine Anmeldung hat je Leistung höchstens eine
   Rechnung.
7. **Unveränderlich.** `parent_billing.documents` und
   `parent_billing.document_positions` halten eine eigene Kopie von
   Empfänger (die anmeldende Person mit Name und Mail), Leistungstagen,
   Kindern und jeder Position mit Brutto, Netto und Steuer (Steuer 0 bis zum
   Tarif). Ein Trigger lehnt jede Änderung einer Rechnung, ihrer Positionen
   und eines freigegebenen Laufs ab, für jede Rolle. Die Anfrage-Rolle darf
   Rechnungen, Korrekturen und die Spur nur lesen und anlegen. Storno und
   Gutschrift (User Story 72) sind eigene Dokumente und kommen später.
8. **Zugriffsprotokoll.** Die Vorschau liest die Anmeldungen über die
   Prüfung von Enrollment (`ListHolidayRegistrations` mit Status
   `confirmed`), die den Träger prüft und das Lesen in
   `enrollment.holiday_registration_audit` schreibt (ADR 0114 Punkt 6).
   Jedes Lesen eines Laufs und jede Änderung schreibt die Elternabrechnung in
   `parent_billing.run_events` mit Konto (`create`, `read`, `correct`,
   `release`). Die Liste der Läufe und die Nummernkreise enthalten keine
   personenbezogenen Daten.
9. **Abhängigkeit.** Die Elternabrechnung erreicht Enrollment nur über ihren
   Port `HolidayBillingRegistrations`, den die Root bindet
   (`api/serve_mounts_parent_billing.go`): Zeiträume über `HolidayPeriods` und
   bestätigte Anmeldungen über `HolidayRegistrationReview`. Kein Import geht
   zwischen den Owners; der Graph bleibt azyklisch.
10. **Recht.** Alle Routen unter `/traeger/billing-runs` und
    `/traeger/number-ranges` verlangen das Träger-Recht „Abrechnung“ und
    laufen in der Träger-Transaktion der Sitzung (ADR 0105). Damit die
    Abrechnung ohne das Recht „Anmeldung verwalten“ einen Lauf starten kann,
    liest `GET /traeger/billing-runs/periods` die veröffentlichten
    Ferienzeiträume (Name und Tage, keine Personen).
11. **Aufbewahrung.** Eine Rechnung bleibt, wenn ihre Anmeldung nach 92 Tagen
    geht, und geht mit dem Träger nach seinem Export (ADR 0088 §9, #4657).
    Die Frist je Rechnung legt #4720 fest. Korrekturen gehen mit dem Kind
    ihrer Anmeldung; die Rechnung behält ihre Kopie.
12. **Register.**
    - `policy.json`: sieben `data_objects` mit `write_owner`
      `parent-billing`; keine neue Regel, kein neues Paket.
    - `command-effects.json`: drei geprüfte Bindungen auf Enrollment (Liste
      der bestätigten Anmeldungen als Teilnahme, ein Zeitraum und die Liste
      der Zeiträume als Abfrage).
    - `passthrough.json`: `BillingRuns.ListBillingRuns` und die beiden
      Root-Übersetzungen der Zeiträume mit genauer Evidenz.
    - Budgets: `modules/parentbilling/compose` 20 → 27 Zeilen
      (`moduleComposeBudgetTotal` 12640 → 12647) und `modules/parentbilling`
      5 → 6 Durchreichungen (`modulePassthroughOwnerBudgetTotal`
      1411 → 1412).
    - Query-Budgets `api.traeger.billing_runs` (2, exact) und
      `api.traeger.billing_run_periods` (4, exact).
    - Datenlebenszyklus: sieben Einträge mit `scope: organization`.

## Verworfene Optionen

- **Entwürfe beim Start speichern und neu berechnen:** Weitere
  Bestätigungen bräuchten einen eigenen Abgleich, und Korrekturen hingen an
  Zeilen, die beim Neuberechnen verschwinden. Die Vorschau aus den
  festgehaltenen Beiträgen ist ohne Abgleich immer aktuell; die Prüfung von
  Zahl und Summe bei der Freigabe schützt vor einer unbemerkten Änderung.
- **Sequenz der Datenbank als Nummernkreis:** Eine Sequenz verliert Nummern
  beim Zurückrollen und kennt kein Jahr; die Nummern hätten Lücken.
- **Nummer beim Start des Laufs:** ADR 0088 §4 verlangt, dass Entwürfe keine
  Nummer haben.
- **Nummernkreis als Träger-Einstellung (ADR 0107):** Der Zähler ändert sich
  mit jeder Freigabe in derselben Transaktion; Einstellungen haben kein
  Recht „Abrechnung“.
- **Zugriffe in `audit.data_access_log`:** Das Protokoll gehört zu einer
  Schule; eine Ferienrechnung hat keine Schule. Die Spur des Laufs folgt
  dem Muster von ADR 0114.

## Folgen

- Das Bestätigungsschreiben als Rechnung im Layout des Trägers mit PDF
  (#4669) und die Übersicht je Lauf für die Kämmerei (#4672) bauen auf den
  ausgestellten Rechnungen auf.
- Storno, Gutschrift und Neuausstellung (User Story 72) lockern die
  Eindeutigkeit „eine Rechnung je Anmeldung und Leistung“.
- Ändert die Abrechnung ein Muster so, dass es eine vergebene Nummer noch
  einmal erzeugt, lehnt die Freigabe mit 409
  `parent_billing.invoice_number_taken` ab und vergibt keine Nummer.

## Nachtrag 2026-11-03: Storno, Gutschrift und Neuausstellung (#4690)

Vorgezogen in Etappe 1 für Bissendorf (User Story 72). Dieser Nachtrag löst
Punkt 7 („Storno und Gutschrift kommen später“) und die Folge zur
Eindeutigkeit ein:

- **Eigene Dokumente.** `parent_billing.documents.kind` ist `invoice`,
  `cancellation` (Storno) oder `credit` (Gutschrift); bestehende Zeilen sind
  Rechnungen. Storno und Gutschrift nennen ihre Rechnung in
  `reference_document_id`, tragen eine Pflichtbegründung (`reason`, bis 500
  Zeichen) und gehören zum Lauf ihrer Rechnung. Der Storno verneint jede
  Position der Rechnung, die Gutschrift ist eine Position `credit` für ein
  Kind, höchstens so hoch wie der Rest des Kindes nach früheren
  Gutschriften. CHECKs halten die Vorzeichen; der Trigger gegen Änderungen
  bleibt für jede Art.
- **Ein Nummernkreis.** Storno und Gutschrift nehmen die nächste Nummer aus
  demselben Muster und Zähler wie die Rechnungen (Punkt 5). Begründung aus
  dem Modell: Das Muster gilt je Träger und Leistung
  (`uq_number_ranges_service`), der Zähler je Träger, Leistung und Jahr, und
  `uq_documents_sequence` hält eine Folge je Leistung und Jahr über alle
  Dokumente. Ein eigenes Präfix bräuchte ein zweites Muster und einen
  zweiten Zähler je Leistung (neuer Schlüssel in beiden Tabellen und eine
  zweite Pflege unter `Abrechnung`) und teilte die lückenlose Folge. So
  bleibt eine Folge ohne Lücke über alle Belege; die Art zeigen Titel des
  Schreibens und die Spalten „Art“ und „Bezug“ der Übersicht.
- **Regeln.** Eine Rechnung wird höchstens einmal storniert
  (`uq_documents_cancellation`). Eine Rechnung mit Gutschrift wird nicht
  storniert (409 `parent_billing.invoice_credited`); der Rest wird
  gutgeschrieben. Eine stornierte Rechnung bekommt keinen weiteren Storno
  und keine Gutschrift (409 `parent_billing.invoice_cancelled`). Storno und
  Gutschrift sperren den Lauf und die Rechnungen des Zeitraums (dieselbe
  Sperre wie Freigabe und Debitorennummer, #5621).
- **Neuausstellung.** Eine stornierte Rechnung rechnet ihre Anmeldung nicht
  mehr ab. Der nächste Lauf des Zeitraums nimmt sie in die Vorschau und
  nennt die ersetzte Rechnung; die Freigabe stellt sie mit neuer Nummer aus
  und schreibt den Bezug (`reference_document_id` auf einer Rechnung,
  höchstens einmal: `uq_documents_reissue`). Die Eindeutigkeit „eine
  Rechnung je Anmeldung und Leistung“ gilt weiter für die erste Rechnung
  (`uq_documents_first_invoice`). Die neue Rechnung kopiert Empfänger,
  Anschrift und Debitorennummer zur Freigabe neu; die Debitorennummer lässt
  sich nach dem Storno wieder ändern. Korrekturen des alten Laufs gehen
  nicht mit; die Vorschau zeigt die festgehaltenen Beträge.
- **Schreiben, Mail, Postfach.** Hat der Lauf seine Schreiben, bekommt
  Storno oder Gutschrift sein Schreiben sofort, sonst mit den Schreiben des
  Laufs. Das Schreiben nennt Nummer, Rechnung mit Datum, Grund und jede
  Position mit Minus; die Gutschrift nennt, was die Rechnung danach noch
  verlangt. Keines fordert eine Zahlung; wer schon mehr bezahlt hat, meldet
  sich beim Träger. Die Mail hat eigenen Betreff und Text
  (`billing-correction-letter.html`), das Postfach der Eltern die Arten
  `cancellation_letter` und `credit_letter`.
- **Historie.** Jede ausgestellte Rechnung nennt in `history` ihren Storno,
  ihre Gutschriften und die Rechnung, die sie ersetzt, auch aus einem
  anderen Lauf. Die Spur des Laufs schreibt `cancel` und `credit` mit
  Konto, Dokument und Rechnung.
- **Recht und Grenze.** `POST /traeger/billing-runs/{id}/documents/{documentID}/cancellation`
  und `…/credits` liegen hinter dem Recht „Abrechnung“ in der
  Träger-Transaktion; ein Lauf eines anderen Trägers ist 404. Beide gehören
  zu `BillingLetterService`, weil sie mit ihrem Schreiben ausgestellt
  werden. Kein neuer Owner, keine neue Regel, kein neues `data_object`.
- **Migration** `1.15.743` (nach `1.15.739`; 740 bis 742 sind von offenen
  Pull Requests belegt).
