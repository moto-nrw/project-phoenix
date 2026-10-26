---
status: accepted
---

# Übersicht je Lauf für die Kämmerei wird aus den ausgestellten Rechnungen gerendert

Entscheidung vom 26.10.2026 zu [#4672](https://github.com/moto-nrw/project-phoenix/issues/4672),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 79 und 80. Sie baut auf dem Abrechnungslauf aus
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
auf, führt keinen Owner ein und hält fest, was die Übersicht in den Registern
ändert (ADR 0057 Punkt 5, ADR 0070).

## Entscheidung

1. **Nur freigegebene Läufe.** Die Übersicht (Abrechnungsanordnung) zeigt die
   ausgestellten Rechnungen eines Laufs in `released`. Ein Lauf in der
   Vorschau hat noch keine Rechnung, die die Kämmerei einziehen könnte; die
   Route antwortet 409 `parent_billing.billing_run_not_released`.
2. **Inhalt.** Je Familie jede Position mit Rechnungsnummer, Empfänger, Kind
   und Betrag, danach die Summe der Rechnung; am Ende die Summe des Laufs.
   Über der Tabelle stehen Ferienzeitraum, Leistungszeitraum,
   Rechnungsdatum, Zahl und Nummern der Rechnungen und die Gesamtsumme. Die
   Mail-Adresse des Empfängers fehlt: Die Kämmerei braucht, wer was zahlt,
   nicht, wie die Familie erreichbar ist. Das ist das allgemeine Layout; das
   Muster eines Trägers (Bissendorf) kommt dazu, sobald es vorliegt.
3. **Formate.** PDF, Excel und CSV über das gemeinsame Listenmuster aus
   ADR 0114: Die Elternabrechnung füllt die Tabelle, Document Rendering
   rendert sie mit `lists.Renderer`. Beträge stehen als deutscher Text
   („1.234,56“) in der Spalte „Betrag in Euro“; CSV hat Semikolon, BOM und
   entschärfte Formeln.
4. **Abhängigkeit.** `parent-billing/http` darf Document Rendering nicht
   importieren, und eine neue Regel zwischen zwei bestehenden Punkten lehnt
   die Policy ab. Die Elternabrechnung erklärt deshalb ihren Port
   `TableRenderer` mit eigenen Werten (`ExportTable`, `ExportFile`); die Root
   bindet ihn in `api/serve_mounts_parent_billing.go`
   (`parentBillingTableRenderer`) an `lists.NewRenderer()` und entfernt dabei
   die Stilmarken des Renderers aus jedem Wert. Das Layout bleibt im Owner,
   die Root übersetzt nur. Kein Import geht zwischen den Owners.
5. **Recht und Grenze.** `GET /traeger/billing-runs/{id}/export?format=pdf|xlsx|csv`
   liegt hinter dem Träger-Recht „Abrechnung“ und läuft in der
   Träger-Transaktion der Sitzung (ADR 0105). Ein Lauf eines anderen Trägers
   ist 404.
6. **Zugriffsprotokoll.** Jeder Download steht mit Konto und Format in
   `parent_billing.run_events` (Aktion `export`, ADR 0119 Punkt 8). Scheitert
   der Eintrag, scheitert die Anfrage und rollt zurück. Die Migration
   `1.15.672` erweitert dafür die erlaubten Aktionen.
7. **Register.**
   - `policy.json`: unverändert; keine Regel, kein `data_object`.
   - `command-effects.json`: eine geprüfte Bindung (`TableRenderer.RenderTable`
     → `lists.Renderer.Render`, query).
   - Budgets: unverändert.
   - Route-Golden: eine lesende Route.

## Verworfene Optionen

- **Regel `parent-billing/http` → `document-rendering/contract`:** Wie bei
  Enrollment wäre das die kürzeste Kante, aber die Policy lässt keine neue
  Regel zwischen bestehenden Punkten zu.
- **Übersicht auch für die Vorschau:** Eine Abrechnungsanordnung ohne
  Rechnungsnummern würde wie eine fertige aussehen. Die Vorschau prüft die
  Abrechnung auf der Lauf-Seite.
- **Beträge als Zahl in Excel:** Das Listenmuster schreibt Text; ein eigener
  Arbeitsmappen-Pfad nur für diese Liste lohnt erst mit dem Muster eines
  Trägers.

## Folgen

- Das Bissendorfer Muster ergänzt Spalten oder ein eigenes Layout in
  `billingRunOverview`, ohne die Route zu ändern.
- Die Finanz-Exporte (SEPA, DATEV, Sollstellung) sind eigene Formate des
  Owners und nicht diese Übersicht.
