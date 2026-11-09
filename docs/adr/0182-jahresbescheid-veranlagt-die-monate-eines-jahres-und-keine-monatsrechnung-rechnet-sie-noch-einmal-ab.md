---
status: proposed
---

# Jahresbescheid veranlagt die Monate eines Jahres, und keine Monatsrechnung rechnet sie noch einmal ab

**Status: Vorgeschlagen.**

Vorschlag vom 08.11.2026 zu [#6219](https://github.com/moto-nrw/project-phoenix/issues/6219),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
Rest aus [#4688](https://github.com/moto-nrw/project-phoenix/issues/4688), Vorbild
Gemeinde Hardt (MK-007, nur Seite 1 belegt). Er baut auf
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
(mit den Nachträgen zu Storno und Prüfphase),
[ADR 0158](0158-monatsbeitrag-entsteht-aus-der-wirksamen-vertragsfassung-und-den-tarifen-des-traegers.md),
[ADR 0164](0164-ermaessigungen-des-monatsbeitrags-sind-beitragsregeln-des-traegers-mit-nachweis-je-vertrag.md),
[ADR 0165](0165-monatsrechnungen-nehmen-schreiben-versand-und-storno-der-ferienrechnungen.md),
[ADR 0167](0167-zahlungsweg-faelligkeit-und-empfaenger-entscheidet-eine-regel-und-die-rechnung-haelt-sie-fest.md),
[ADR 0169](0169-kostentraeger-anteile-teilen-den-monatsbeitrag-nach-der-regelkette.md) und
[ADR 0170](0170-dokumentart-je-leistung-und-gebuehrenbescheid-mit-rechtsbehelfsbelehrung-des-traegers.md)
auf. Er führt keinen Owner ein und hält fest, was er in den Registern ändert
(ADR 0057 Punkt 5, ADR 0070).

## Kontext

Hardt veranlagt Verlässliche Grundschule und Nachmittag in einem
Gebührenbescheid über einen Zeitraum eines Kalenderjahres (MK-007: 01.09. bis
31.12.2025). Je Kind und Modul steht eine Zeile mit Monatsbetrag (12,00 € und
20,00 €), Anzahl und anteiligem Jahresbetrag (48,00 € und 80,00 €), jeweils
„bisher“ und „neu“ mit „Zu-/Abgang“. Der Gesamtbetrag (128,00 €) ist in
Monatsraten zum Monatsersten fällig (vier zu 32,00 €), vergangene Monate sofort.
Der Monatslauf (ADR 0158, Prüfphase nach ADR 0119) stellt dagegen je Monat aus.
ADR 0170 hat den Jahresbescheid als eigenen Lauf über mehrere Monate offen
gelassen. Seite 2 von MK-007 fehlt; zwölf Raten, Rechtsgrundlage und
Forderungsstand nach Zahlungseingängen sind nicht belegt.

## Entscheidung

1. **Eigener Lauf, dieselbe Rechnung.** `parent_billing.care_notice_runs` ist
   ein Lauf über die Monate eines Jahres vom gewählten ersten Monat bis
   Dezember für eine Auswahl der Schulen des Trägers. Ein Jahr hat höchstens
   einen offenen Lauf (eindeutiger Index). Der Lauf rechnet jeden Monat mit
   derselben Monatsrechnung wie der Monatslauf (Vertragsfassung, Preisliste,
   Monatsregel, Regelkette mit Geschwistern und Kinderzahl, Nachweise,
   Kostenträger-Anteile; ADR 0158, 0164, 0169) und liest alles außer den
   abgerechneten Verträgen je Monat einmal für den Zeitraum. Eine zweite
   Rechen- oder Belegmechanik gibt es nicht. Die Mensa gehört nicht in den
   Bescheid: Sie wird nachträglich nach Essen abgerechnet (ADR 0168).
2. **Bescheid je Vertrag.** Der Bescheid hat je Berechnungsgrundlage (Modul
   mit Wochentagen bzw. Staffelstufe, Ermäßigung, Kostenträger-Anteil) und
   Folge von Monaten mit gleichem Betrag eine Zeile: Zeitraum, Monatsbetrag
   bisher und neu, Anzahl 1, anteiliger Jahresbetrag bisher und neu und
   Zu-/Abgang. Je Monat gibt es eine Rate, fällig am Monatsersten. Liegt der
   Monatserste vor dem Bescheiddatum, ist die Rate sofort fällig, also am
   Bescheiddatum. Zahlungsweg und Empfänger bestimmt die Regel aus ADR 0167
   (Abweichung der Familie, sonst Bedingungen des Monatsbeitrags).
3. **Keine Doppelabrechnung.** Ein Monat eines Bescheids, den niemand geändert
   hat, gilt für den Monatslauf als abgerechnet: `InvoicedContracts` liest ihn
   neben den Monatsrechnungen. Der Monatslauf stellt für diesen Vertrag und
   Monat keine Rechnung aus und zählt ihn unter „schon abgerechnet“. Das gilt
   nur für den Betreuungsbeitrag: Die Essen eines Bescheidmonats rechnet der
   Monatslauf weiter ab (ADR 0168). Umgekehrt
   veranlagt der Bescheid keinen Monat, den eine Monatsrechnung abrechnet.
   Beide Freigaben halten dieselbe Sperre der Monatsabrechnung des Trägers und
   rechnen in ihr neu, deshalb gewinnt immer nur eine. Die Raten werden weiter
   monatlich fällig und eingezogen, ohne dass Monatsrechnungen entstehen. Der
   SEPA-Einzug (#4695) liest die Raten (`care_notice_months.due_on`,
   `amount_cents`) wie die Fälligkeit einer Rechnung. Der Monat hält für den
   Monatslauf und die Regelkette eine Kopie dessen, was er abrechnet, mit den
   von der Regelkette verteilten Cent.
4. **Fingerabdruck.** Die Freigabe eines Bescheidlaufs prüft Zahl, Summe und
   den Fingerabdruck des geprüften Stands wie der Monatslauf: Schulen,
   Zeitraum, Bescheiddatum, Monatsregel, jeder Bescheid mit Empfänger,
   Zahlungsweg, Zeilen, Raten und den Monaten mit verteilten Cent, der
   geänderte Bescheid, die unveränderten und die per Monatsrechnung
   abgerechneten Verträge, Buchungen ohne Preis, doppelte Verträge und die
   Belehrung. Der Fingerabdruck des Monatslaufs nennt zusätzlich die Bescheide,
   die einen Monat seiner Verträge veranlagen. Wird zwischen Prüfung und
   Freigabe ein Bescheid ausgestellt, lehnt die Freigabe des Monatslaufs ab
   (409 `parent_billing.billing_run_changed`).
5. **Änderungsbescheid.** Ändert sich ein Vertrag, rechnet ein neuer Lauf des
   Jahres die Monate des geltenden Bescheids und die seines Zeitraums neu. Weicht
   etwas ab, entsteht ein Bescheid der Art `change`, der den geltenden Bescheid
   nennt (`amends_notice_id`) und je Zeile und Rate „bisher“, „neu“ und
   „Zu-/Abgang“ zeigt. Von da an gilt er für seine Monate. Ein Bescheid wird
   höchstens einmal geändert (eindeutiger Index), ein Vertrag hat je Jahr
   höchstens einen ersten Bescheid. Eine Senkung bis 0,00 € ist die teilweise
   Aufhebung; die Monate bleiben veranlagt, also rechnet sie kein Monatslauf
   ab. Sofort fällig ist im Änderungsbescheid nur eine Erhöhung eines
   vergangenen Monats. Unveränderte Verträge zählt der Lauf und stellt nichts
   aus.
   Kann die Monatsrechnung einen Monat nicht rechnen (Buchung ohne Preis,
   doppelter Vertrag), behält der Änderungsbescheid dort die bisherige
   Festsetzung; ein fehlender Entwurf gilt nur als Abgang, wenn die Rechnung
   gelang. Ohne bisherigen Bescheid bleibt der Monat beim Monatslauf. Ein
   Lauf ohne Bescheid lässt sich abschließen, damit er das Jahr nicht
   blockiert.
6. **Unveränderlich (ADR 0119 Punkt 7).** Ein Trigger lehnt jede Änderung von
   Bescheid und Monaten ab, für jede Rolle. Ein freigegebener Lauf ändert sich
   nicht mehr, nur das Konto, das ihn begann, darf verschwinden. Die
   Anfrage-Rolle darf Bescheide, Monate und Spur nur lesen und anlegen. Eine
   spätere Vertragsänderung ändert den Bescheid nicht; sie erzeugt den
   Änderungsbescheid.
7. **Nummer und Belehrung.** Der Bescheid nimmt die nächste Nummer aus Muster
   und Zähler des Monatsbeitrags (`care`). Keine Nummer wiederholt eine
   Ferien-, Monats-, Storno- oder Bescheidnummer des Trägers; die Prüfung gilt
   für alle Belege. Ohne Belehrung in der Vorlage „Bescheid“ wird kein
   Bescheid ausgestellt (409 `parent_billing.legal_remedy_missing`, ADR 0170
   Punkt 4). Der Bescheid hält die Belehrung bei der Ausstellung als Kopie.
   Seite 2 von MK-007 wird nicht nachgebaut.
8. **Seite 1 des Bescheids.** `GET …/{id}/notices/{contractID}/letter` zeigt den
   Bescheid mit Briefkopf, Kontakt und Logo des Trägers so, wie Hardt das
   Wichtigste auf Seite 1 setzt. Gezeigt werden Kopf mit Nummer, Datum und
   Auskunft, Kind, I. Berechnung, II. Abrechnung mit Raten und Zahlungsweg
   sowie die Belehrung. Jeder Abruf steht mit dem Vertrag in der Spur. Das
   Schreiben als PDF, die Mail und das Postfach der Eltern folgen wie beim
   Monatsbeitrag (ADR 0165), sobald Hardt sie verlangt.
9. **Zugriffsprotokoll.** Jedes Lesen eines Laufs, jeder Abruf eines Bescheids
   und jede Änderung schreibt `parent_billing.care_notice_run_events` mit
   Konto und den gezeigten Verträgen (`create`, `read`, `letter`, `release`).
10. **Recht.** Alle Routen unter `/traeger/care-notice-runs` verlangen das
    Träger-Recht „Abrechnung“ und laufen in der Träger-Transaktion der Sitzung.
    Ein Lauf eines anderen Trägers ist 404.
11. **Golden-Test.** `TestCareNoticeMatchesPageOneOfMK007` prüft die
    Berechnung gegen die Faktenliste von Seite 1 (Zeitraum, Zeilen, Beträge,
    Anzahl, bisher/neu/Zu-/Abgang, 128,00 €, vier Raten zu 32,00 €, September
    sofort fällig). Liegt das Original-PDF vor, wird die Erwartung dagegen
    getauscht.

12. **Kostenträger einmal je Zeitraum.** Was ein Bescheid oder eine
    Monatsrechnung eines Monats schon von einem Kostenträger nimmt, liest der
    Monatslauf gemeinsam (`InvoicedContracts`: Anteile der Bescheidraten aus
    ihrem Stand und Anteile der Monatsrechnungen). Ein fester Zuschuss wirkt
    so je Zeitraum nur einmal (ADR 0169); die Mensa trägt danach die Familie.
13. **Aufhebung gibt keine Monate frei.** Senkt ein Änderungsbescheid einen
    Monat auf 0,00 €, bleibt der Monat für Monatsrechnungen gesperrt, sonst
    droht doppelte Abrechnung. Die Prüfphase des Monatslaufs nennt solche
    Kinder (`fee_notice_held`) mit dem Hinweis, einen neuen Bescheid
    auszustellen, und verweist auf die Jahresbescheide.
14. **Lesefunktion für den Einzug.** `CareFeeNotices.EffectiveNoticeDues(ctx,
    organizationID, month)` liefert je Vertrag die wirksame Stufe eines Monats:
    Rate, Fälligkeit, sofort fällig, Bescheid-ID und -nummer, Zahlungsweg. Der
    SEPA-Einzug (#4697) löst die Änderungsstufen damit nicht selbst auf. Die
    Methode prüft den Monat und wählt die jüngste Stufe; sie zählt als
    Pass-through der Owner-Kante (ADR 0075), das Budget von parentbilling
    steigt um eins.

## Verworfene Optionen

- **Monatsrechnungen je Rate:** Ein Bescheid mit zwölf Monatsrechnungen wäre
  doppelt veranlagt und hätte zwölf Nummern; Hardt kennt eine Nummer
  (Buchungszeichen) je Veranlagung.
- **Eigene Rechnung für das Jahr:** Eine zweite Preis- und Regelrechnung liefe
  auseinander (Geschwister, Kinderzahl, Kostenträger) und widerspräche der
  Vorgabe, auf dem Monatslauf aufzubauen.
- **Änderung als Storno und Neuausstellung:** Der Storno verneint den ganzen
  Bescheid; Hardt zeigt aber „bisher / neu / Zu-/Abgang“ je Grundlage in einem
  Bescheid. Die Aufhebung eines ganzen Bescheids bleibt der Weg aus ADR 0170
  Punkt 6. Hier ist sie die Senkung auf 0,00 €.
- **Preise veranlagter Monate sperren:** Ändert eine Gebührensatzung den
  Preis, ist der Änderungsbescheid die richtige Folge. Die Sperre aus ADR 0158
  gilt weiter nur für Monatsrechnungen.
- **Forderungsstand nach Zahlungseingängen:** Zahlungen kennt moto noch nicht
  (#4695). Der Bescheid zeigt die Raten; ein Kontostand folgt mit dem Einzug.

## Folgen

- Der SEPA-Einzug (#4695) zieht Raten wie Rechnungen ein und nennt die
  Bescheidnummer im Verwendungszweck.
- Schreiben, Mail und Elternpostfach für Bescheide kommen nach dem Muster von
  ADR 0165 dazu.

## Register

- Migration `1.15.823` (vom Orchestrator vergeben): vier Tabellen mit RLS,
  Triggern gegen Änderungen und Rechten nur zum Lesen und Anlegen; die
  Demo-Löschung `platform.erase_closed_demo_visitors` löscht die Bescheide der
  Schule, mit und ohne Namen, und die Lesungen ihrer Läufe
  (`TestDemoSchoolQueueErasesTheFeeNoticesOfTheRetiredSchool`).
- `policy.json`: vier `data_objects` für `parent-billing`; keine neue Regel,
  kein neuer Port. Die Root bindet dieselben Ports wie den Monatslauf und den
  Briefkopf (`NewCareNoticeHTTP`).
- Datenlebenszyklus: vier Einträge mit `scope: organization`; `visitorTraceCounts`
  zählt Bescheid und Monat mit dem Namen der Besucherin.
- Routen im Route-Golden; die schreibenden in der Nutzungsanalyse als
  `notCaptured` (Träger, keine Schule). Query-Budget
  `api.traeger.care_notice_runs` (2, exact).
- Compose-Budget: `modules/parentbilling/compose` 74 → 81 Zeilen
  (`moduleComposeBudgetTotal` 13130 → 13137) für den Konstruktor
  `NewCareFeeNotices`.
- Fehlercodes `parent_billing.care_notice_run_not_found`,
  `parent_billing.care_notice_run_open`, `parent_billing.care_notice_run_invalid`,
  `parent_billing.care_notice_not_found`.
