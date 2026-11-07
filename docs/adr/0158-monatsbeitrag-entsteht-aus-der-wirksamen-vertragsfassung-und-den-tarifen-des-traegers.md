---
status: proposed
---

# Monatsbeitrag entsteht aus der wirksamen Vertragsfassung und den Tarifen des Trägers

Vorschlag vom 07.11.2026 zu [#4684](https://github.com/moto-nrw/project-phoenix/issues/4684),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 59 und 63. Er ist ein weiterer Bau-Schnitt des Owners
Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
und überträgt den Lauf aus
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
auf den Monat. Er liest die Vertragsfassungen aus
[ADR 0153](0153-betreuungsvertrag-entsteht-im-traeger-buero-aus-der-angenommenen-anmeldung.md)
und #4681. Er führt keinen Owner ein und hält fest, was er in den Registern
ändert (ADR 0057 Punkt 5, ADR 0070).

## Entscheidung

1. **Preisform E3 A (vorläufig).** Ein Tarif ist der Monatspreis eines
   gebuchten Wochentags eines Moduls (`care_offering`) an einer Schule, gültig
   ab dem 1. eines Monats bis zum nächsten Preis desselben Moduls und
   Wochentags (`parent_billing.care_tariffs`). Der Monatsbeitrag eines Kindes
   ist die Summe der Preise der Wochentage, die seine Vertragsfassung bucht.
   Kein Preis steht im Code; Preise je Schule bilden die Standorte des SJR ab.
   Kommt Utes Preisliste in einer anderen Form (Staffel, Tagespreis), ändert
   sich nur die Berechnung, nicht der Lauf.
2. **Positionen je Modul.** Eine Rechnung hat je gebuchtem Modul eine
   Position mit den gebuchten Wochentagen und ihrem Preis, nie eine Zeile je
   Kalendertag. Früh- und Nachmittagsbetreuung sind zwei Positionen derselben
   Rechnung.
3. **Monatswechsel.** Ob ein Monat mit Vertragsänderung, Eintritt oder
   Austritt anteilig oder voll zählt, ist beim Kunden offen. Die
   Träger-Einstellung `parent_billing.care_month_change` entscheidet:
   `version_on_first` (Standard) rechnet den vollen Monat nach der Fassung am
   1., bei Eintritt im Monat nach der ersten Fassung; `pro_rata` rechnet jede
   Fassung für ihre Kalendertage, kaufmännisch gerundet je Wochentag. Die
   Freigabe hält die angewendete Regel an Lauf und Rechnung fest.
4. **Eine Fassung je Tag.** Der Lauf liest die Fassungen über den Port
   `CareContracts`, den die Root an `carecontract.ContractBilling` bindet.
   Liefert er zwei Fassungen eines Vertrags für einen Tag, rechnet der Lauf
   den Monat nicht ab (409 `parent_billing.care_versions_overlap`).
5. **Lauf.** `parent_billing.care_runs` folgt ADR 0119 Punkte 1, 2 und 6:
   `calculated` ohne gespeicherte Entwürfe, Vorschau bei jedem Lesen,
   höchstens ein Lauf in der Vorschau je Monat. Die Freigabe sperrt die
   Monatsabrechnung des Trägers (Advisory Lock) und den Lauf (`FOR UPDATE`),
   rechnet neu, prüft Zahl und Summe und vergibt lückenlose Nummern aus dem
   eigenen Nummernkreis der Leistung `care`. Ein Vertrag hat je Monat
   höchstens eine Rechnung (`uq_care_invoices_contract_month`); ein späterer
   Lauf desselben Monats rechnet nur, was frühere ausgelassen haben. Der Lauf
   darf im Leistungsmonat laufen und rechnet nach Vertrag, nicht nach
   Anwesenheit.
6. **Unveränderlich.** Rechnung und Positionen halten eine eigene Kopie von
   Kind, Schule, Empfänger und jedem Preis; derselbe Trigger wie ADR 0119
   Punkt 7 lehnt jede Änderung ab. Ein Preis beginnt nie in einem Monat, den
   eine Rechnung seiner Schule schon abrechnet (409
   `parent_billing.care_tariff_month_billed`); Preispflege und Freigabe
   halten dieselbe Sperre.
7. **Empfänger.** Die Rechnung geht an die erste Vertragspartei mit
   E-Mail-Adresse, sonst an die erste. Abweichende Zahler und Bündelung
   kommen mit #4689.
8. **Zugriffsprotokoll und Recht.** Routen unter `/traeger/care-tariffs` und
   `/traeger/care-billing-runs` verlangen das Träger-Recht „Abrechnung“ und
   laufen in der Träger-Transaktion. Jedes Lesen eines Laufs schreibt
   `parent_billing.care_run_events` mit Konto und den gezeigten Verträgen.
9. **Abgrenzung.** Prüfphase mit Standortwahl, Rechnungsdatum, Ausschluss
   und Prüfexport (#5916), Senden und Druck der Schreiben, Empfänger und
   Zahlungsweg (#4689) sowie Ermäßigungen und Geschwister (#4685) bauen auf
   diesen Rechnungen auf und sind nicht Teil dieser Entscheidung.

## Verworfene Optionen

- **Monatsrechnungen in `parent_billing.documents`:** Die Tabelle und ihre
  Lesepfade (Schreiben, Postfach, Storno, Familien) hängen an Ferienanmeldung
  und -zeitraum. Eigene Tabellen halten die Ferienpfade unverändert; die
  Zusammenführung für Versand und Storno entscheidet #5916 oder #4689.
- **Tagespreis mal Betreuungstage (E3 C):** schwankt je Monat und braucht die
  Anwesenheit; der SJR rechnet nach Vertrag.
- **Monatswechsel fest im Code:** Die Kundenentscheidung fehlt; eine
  Einstellung ändert die Regel ohne Codeänderung.

## Register

- `policy.json`: fünf `data_objects` mit `write_owner` `parent-billing`; keine
  neue Regel, kein neues Paket.
- `command-effects`: drei geprüfte Abfragen (Fassungen, Empfänger,
  Monatsregel).
- `passthrough`: `CareBilling.ListCareRuns` und die Root-Übersetzungen
  `parentBillingSettings.CareMonthRule` und
  `parentBillingCareContracts.Recipients`.
- Budgets: `modules/parentbilling/compose` 50 → 55 Zeilen
  (`moduleComposeBudgetTotal` 12946 → 12951) und `modules/parentbilling`
  6 → 7 Durchreichungen (`modulePassthroughOwnerBudgetTotal` 1409 → 1410).
- Query-Budgets `api.traeger.care_billing_runs` (2, exact) und
  `api.traeger.care_tariffs` (4, exact).
- Datenlebenszyklus und Aufbewahrung: je fünf Einträge.
- Migration `1.15.788`.
