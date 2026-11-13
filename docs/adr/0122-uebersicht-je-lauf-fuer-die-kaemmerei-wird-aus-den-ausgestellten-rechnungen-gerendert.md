---
status: accepted
---

# Übersicht je Lauf für die Kämmerei wird aus den ausgestellten Rechnungen gerendert

Entscheidung vom 26.10.2026 zu [#4672](https://github.com/moto-ogs/project-phoenix/issues/4672),
Etappe 1 der Spec [#4611](https://github.com/moto-ogs/project-phoenix/issues/4611),
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
   entschärfte Formeln. Ein negativer Betrag („-32,50“) ist keine Formel:
   Der CSV-Renderer lässt eine reine Zahl ohne Apostroph, damit Rabatte und
   Korrekturen als Zahl in der Tabellenkalkulation ankommen.
4. **Abhängigkeit.** Die Anwendung erklärt ihren Port `TableRenderer` mit
   eigenen Werten (`ExportTable`, `ExportColumn`) und antwortet mit der
   öffentlichen Datei `parentbilling.ExportFile`. Der Adapter
   `internal/adapters/rendering`, der auch die Bestätigungsschreiben setzt
   (ADR 0121), bindet ihn an `lists.NewRenderer()` und entfernt dabei die
   Stilmarken des Renderers aus jedem Wert; `compose.NewBillingRuns` baut
   ihn ein. Das Layout bleibt im Owner, der Adapter übersetzt nur. Nur er
   importiert Document Rendering, über die Adapter-Regeln aus ADR 0121.
5. **Recht und Grenze.** `GET /traeger/billing-runs/{id}/export?format=pdf|xlsx|csv`
   liegt hinter dem Träger-Recht „Abrechnung“ und läuft in der
   Träger-Transaktion der Sitzung (ADR 0105). Ein Lauf eines anderen Trägers
   ist 404.
6. **Zugriffsprotokoll.** Jeder Download steht mit Konto und Format in
   `parent_billing.run_events` (Aktion `export`, ADR 0119 Punkt 8). Scheitert
   der Eintrag, scheitert die Anfrage und rollt zurück. Die Migration
   `1.15.672` erweitert dafür die erlaubten Aktionen.
7. **Register.**
   - `policy.json`: keine neue Regel, kein `data_object`; die drei
     Adapter-Regeln aus ADR 0121 nennen in ihrer Beschreibung jetzt auch die
     Übersicht, das Adapterpaket heißt `rendering`.
   - `command-effects.json`: die Bindung von `lists.Renderer.Render` über
     den Adapter (query).
   - Budgets: unverändert.
   - Route-Golden: eine lesende Route.

## Verworfene Optionen

- **Regel `parent-billing/http` → `document-rendering/contract`:** Wie bei
  Enrollment wäre das die kürzeste Kante, aber die Policy lässt keine neue
  Regel zwischen bestehenden Punkten zu.
- **Bindung in der Root:** Ein öffentlicher Port, den
  `api/serve_mounts_parent_billing.go` an `lists.NewRenderer()` bindet, war
  die erste Fassung. Seit ADR 0121 hat die Elternabrechnung einen eigenen
  Adapter zu Document Rendering; zwei Wege zum selben Renderer wären eine
  Doppelung.
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

## Nachtrag 2026-11-03: Anschriftspalten (#5625)

Punkt 2 bekommt nach „Empfänger“ die Spalten „Straße und Hausnummer“, „PLZ“
und „Ort“ aus der Kopie der Rechnung (ADR 0121 Nachtrag vom 03.11.2026), in
PDF, Excel und CSV. Die Spalten erscheinen nur, wenn mindestens eine
Rechnung des Laufs eine Anschrift hat; sonst bleibt die Übersicht bei den
Spalten von bisher. Die Zeile mit der Summe einer Rechnung nennt die
Anschrift wie ihre Positionen. Die Mail-Adresse fehlt weiter.

## Nachtrag 2026-11-03: Spalte Debitorennummer (#5621)

Punkt 2 bekommt direkt nach „Empfänger“ die Spalte „Debitorennummer“ aus der
Kopie der Rechnung (ADR 0121 Nachtrag „Debitorennummer beim Bestätigen“), in
PDF, Excel und CSV, auf den Zeilen der Positionen und der Summe je Rechnung.
Wie die Anschriftspalten erscheint sie nur, wenn mindestens eine Rechnung
des Laufs eine Nummer hat; eine Rechnung ohne Nummer lässt die Zelle leer.
Hat der Lauf beides, steht die Debitorennummer vor der Anschrift.

## Nachtrag 2026-11-03: Storno und Gutschrift (#4690)

Hat ein Lauf einen Storno oder eine Gutschrift oder ersetzt eine Rechnung
eine stornierte (ADR 0119 Nachtrag vom 03.11.2026), heißt die erste Spalte
„Nummer“, und nach ihr stehen „Art“ (Rechnung, Storno, Gutschrift) und
„Bezug“ („zu FE-…“, „ersetzt FE-…“). Storno und Gutschrift sind eigene
Zeilen mit Minusbetrag und eigener Summenzeile; die Gesamtsumme des Laufs
ist die Summe aller Dokumente. Über der Tabelle zählen die Rechnungen
weiter für sich, Storno und Gutschrift haben je eine Zeile mit ihren
Nummern. Ohne solche Dokumente bleibt die Übersicht wie bisher.
