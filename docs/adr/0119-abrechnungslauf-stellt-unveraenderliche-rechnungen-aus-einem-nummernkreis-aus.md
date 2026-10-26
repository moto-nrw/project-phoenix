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
   (`api/serve_mounts_parent_billing.go`): Zeitraum über `HolidayPeriods` und
   bestätigte Anmeldungen über `HolidayRegistrationReview`. Kein Import geht
   zwischen den Owners; der Graph bleibt azyklisch.
10. **Recht.** Alle Routen unter `/traeger/billing-runs` und
    `/traeger/number-ranges` verlangen das Träger-Recht „Abrechnung“ und
    laufen in der Träger-Transaktion der Sitzung (ADR 0105).
11. **Aufbewahrung.** Eine Rechnung bleibt, wenn ihre Anmeldung nach 92 Tagen
    geht, und geht mit dem Träger nach seinem Export (ADR 0088 §9, #4657).
    Die Frist je Rechnung legt #4720 fest. Korrekturen gehen mit dem Kind
    ihrer Anmeldung; die Rechnung behält ihre Kopie.
12. **Register.**
    - `policy.json`: sieben `data_objects` mit `write_owner`
      `parent-billing`; keine neue Regel, kein neues Paket.
    - `command-effects.json`: zwei geprüfte Bindungen auf Enrollment (Liste
      der bestätigten Anmeldungen als Teilnahme, Zeitraum als Abfrage).
    - `passthrough.json`: `BillingRuns.ListBillingRuns` mit genauer
      Evidenz.
    - Budgets: `modules/parentbilling/compose` 20 → 27 Zeilen
      (`moduleComposeBudgetTotal` 12640 → 12647) und `modules/parentbilling`
      5 → 6 Durchreichungen (`modulePassthroughOwnerBudgetTotal`
      1411 → 1412).
    - Query-Budget `api.traeger.billing_runs` (2, exact).
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
