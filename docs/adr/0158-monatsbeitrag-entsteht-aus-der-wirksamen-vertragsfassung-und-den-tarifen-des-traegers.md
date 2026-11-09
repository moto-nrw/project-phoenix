---
status: accepted
---

# Monatsbeitrag entsteht aus der wirksamen Vertragsfassung und den Tarifen des Trägers

**Status: Akzeptiert.** Freigegeben von Yannick Wenger am 07.11.2026.

Entscheidung vom 07.11.2026 zu [#4684](https://github.com/moto-nrw/project-phoenix/issues/4684),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 59 und 63. Sie ist ein weiterer Bau-Schnitt des Owners
Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
und überträgt den Lauf aus
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
auf den Monat. Sie liest die Vertragsfassungen aus
[ADR 0153](0153-betreuungsvertrag-entsteht-im-traeger-buero-aus-der-angenommenen-anmeldung.md)
und #4681. Sie führt keinen Owner ein und hält fest, was sie in den Registern
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
   Punkt 7 lehnt jede Änderung ab. Ein Preis, mit dem eine Rechnung einen
   Wochentag eines Moduls abgerechnet hat, ändert sich für diesen Monat nie
   (409 `parent_billing.care_tariff_month_billed`); ein fehlender Preis lässt
   sich auch für einen abgerechneten Monat ergänzen, damit der nächste Lauf
   des Monats das ausgelassene Kind abrechnet. Preispflege und Freigabe
   halten dieselbe Sperre.
7. **Schulen.** Die Preisliste nennt die Schulen des Trägers über den Port
   `CareSchools`, den die Root an Organisation & Tenancy bindet. Ein Preis für
   eine Schule eines anderen Trägers wird abgelehnt (400
   `parent_billing.care_tariff_invalid`).
8. **Empfänger.** Die Rechnung geht an die erste Vertragspartei mit
   E-Mail-Adresse, sonst an die erste. Abweichende Zahler und Bündelung
   kommen mit #4689.
9. **Zugriffsprotokoll und Recht.** Routen unter `/traeger/care-tariffs` und
   `/traeger/care-billing-runs` verlangen das Träger-Recht „Abrechnung“ und
   laufen in der Träger-Transaktion. Jedes Lesen eines Laufs schreibt
   `parent_billing.care_run_events` mit Konto und den gezeigten Verträgen.
10. **Betragsregeln.** Ein gebuchter Wochentag eines Moduls zählt je Kind
   und Monat höchstens einmal. Buchen zwei aktive Verträge desselben Kindes
   (Schülerakte; fehlt sie bei einem, Name und Geburtsdatum) dasselbe Modul
   an einem Tag, den der Lauf für beide abrechnet, rechnet er keinen von
   beiden ab und zeigt sie als „Doppelter Vertrag – bitte prüfen“. Ein
   Duplikat hält die Abrechnung des Trägers nicht auf: Die Freigabe stellt
   die Rechnungen der übrigen Kinder aus und nennt die Zahl der Kinder mit
   Duplikat; der freigegebene Lauf zeigt sie unter „Nicht abgerechnet –
   Doppelter Vertrag“. Ist das Duplikat behoben, rechnet ein späterer Lauf
   des Monats das Kind ab, wie ein Kind, dem ein Preis fehlte. Ein Modul, das über alle Fassungen des Monats
   gleich bleibt, ist eine Position zum vollen Monatspreis; geteilte
   Positionen ergeben zusammen den Betrag des Moduls auf den Cent (größter
   Rest). Die Demo-Löschung (ADR 0062) entfernt Monatsrechnungen und die
   Lesungen der Laufspur stillgelegter Demo-Schulen; die Nummernlücke nimmt
   sie wie ADR 0119 hin.
11. **Abgrenzung.** Prüfphase mit Standortwahl, Rechnungsdatum, Ausschluss
   und Prüfexport (#5916), Senden und Druck der Schreiben, Empfänger und
   Zahlungsweg (#4689) sowie Ermäßigungen und Geschwister (#4685) bauen auf
   diesen Rechnungen auf und sind nicht Teil dieser Entscheidung.

## Nachtrag 2026-11-07: Abrechnungseinheit Kind-Monat

Präzisiert in Punkt 3 den Eintritt unter `version_on_first` (je Kind, nicht
je Vertrag) und ersetzt in Punkt 10 die Regeln zum doppelten Vertrag. Die Duplikatlogik ist eine reine
Funktion (`childMonthHolds`); die Anzeige eines freigegebenen Laufs eine
zweite (`heldView`).

- **E1 Einheit.** Abgerechnet wird je Kind und Monat. Ein Kind ist ein
  Schülerdatensatz. Ohne Schülerdatensatz gilt das Paar aus normalisiertem
  Namen (klein, Leerzeichen zusammengefasst) und Geburtsdatum; ein Vertrag
  ohne Datensatz gehört zu dem Kind mit Datensatz, dessen Verträge dasselbe
  Paar tragen, wenn es genau eines gibt.
- **E2 Abrechnungstage.** Für jeden Vertrag des Kindes steht zuerst fest,
  welche Kalendertage des Monats er nach `care_month_change` abrechnet.
  Unter „voller Monat“ rechnet den ganzen Monat der Vertrag ab, der am 1.
  gilt, mit den Modulen seiner Fassung am 1. Gilt am 1. kein Vertrag des
  Kindes, rechnet der erste Vertrag, der im Monat beginnt, den ganzen Monat
  ab, mit den Modulen seiner ersten Fassung (Eintritt; bei gleichem
  Beginn der zuerst gelesene). Jeder weitere Vertrag, der später im Monat
  beginnt, rechnet in diesem Monat nichts ab. Unter „anteilig“ rechnet jeder
  Vertrag nur seine eigenen Tage ab.
- **E3 Duplikat.** Ein Duplikat liegt genau dann vor, wenn zwei Verträge
  desselben Kindes nach E2 am selben Kalendertag dasselbe Modul am selben
  Wochentag abrechnen. Laufzeit-Überlappung ohne doppelt abgerechneten Tag
  ist kein Duplikat.
- **E4 Ganzes Kind.** Hat ein Kind im Monat ein Duplikat, hält der Lauf das
  ganze Kind für diesen Monat zurück, mit allen seinen Verträgen. Die
  Freigabe stellt die Rechnungen der übrigen Kinder aus und speichert die
  zurückgehaltenen Kinder als Momentaufnahme (`parent_billing.care_run_holds`:
  Name, Verträge mit Schule, doppelt abgerechnete Module und Wochentage,
  Grund).
- **E5 Momentaufnahme.** Ein freigegebener Lauf zeigt ausschließlich die bei
  der Freigabe gespeicherten Daten: seine ausgestellten Rechnungen und die
  Momentaufnahme aus `care_run_holds`. Er liest keinen Vertrag neu; ein
  später widerrufener, ersetzter oder umbenannter Vertrag ändert die Anzeige
  nicht, und später entstandene Verträge erscheinen erst in einem neuen Lauf.
  Die Momentaufnahme hat RLS, ist unveränderlich und geht mit der
  Demo-Löschung (1.15.805). Ein
  späterer Lauf des Monats rechnet ein zurückgehaltenes Kind ab, sobald es
  kein Duplikat mehr hat.
- **E6 Tabellentest.** `TestCareMonthChildMonthTable` rechnet jede
  Kombination aus Einstellung (voll, anteilig) und Vertragsfolge (15./16.,
  1./1., Überlappung an einem Wochentag, verschiedene Module, drei Verträge
  mit einem Duplikat, Eintritt am 16., Eintritt am 10. mit zweitem Vertrag
  am 20., zwei Schulen) gegen einen von Hand gerechneten Betrag
  oder die Zurückhaltung; die Rechnung steht je Zeile als Kommentar. Je eine
  Mutation von E2 bis E5 macht ihn rot, ebenso das Entfernen der Ausnahme
  „kein Vertrag am 1.“ (Zeile „Eintritt am 16.“).

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
- `command-effects`: geprüfte Abfragen der Fassungen und Empfänger
  (Betreuungsvertrag), der Schulen des Trägers (Organisation & Tenancy) und
  der Monatsregel (Settings).
- `passthrough`: `CareBilling.ListCareRuns` und die Root-Übersetzungen
  `parentBillingSettings.CareMonthRule`,
  `parentBillingCareContracts.Recipients` und
  `parentBillingCareSchools.TraegerSchools`.
- Budgets: `modules/parentbilling/compose` 50 → 55 Zeilen
  (`moduleComposeBudgetTotal` 12946 → 12951) und `modules/parentbilling`
  6 → 7 Durchreichungen (`modulePassthroughOwnerBudgetTotal` 1409 → 1410).
- Query-Budgets `api.traeger.care_billing_runs` (2, exact) und
  `api.traeger.care_tariffs` (6, exact).
- Datenlebenszyklus und Aufbewahrung: je fünf Einträge.
- Migrationen `1.15.789` (Tabellen) und `1.15.805` (`platform.erase_closed_demo_visitors`, auf der Fassung von 1.15.797).

## Nachtrag 2026-11-08: Schreiben, Versand und Storno (#4684)

Punkt 11 nannte „Senden und Druck der Schreiben“ als offen. Die Monatsrechnungen
bekommen Schreiben, Versand, Postfach, Sammeldruck, Storno und Neuausstellung
über die Strecke der Ferienrechnungen in eigenen Tabellen:
[ADR 0165](0165-monatsrechnungen-nehmen-schreiben-versand-und-storno-der-ferienrechnungen.md).
Punkt 5 („ein Vertrag hat je Monat höchstens eine Rechnung“) gilt seitdem für
die erste Rechnung; eine stornierte Rechnung ersetzt höchstens eine neue.

## Nachtrag 2026-11-08: Schulen je Lauf (#5916)

Punkt 5 („höchstens ein Lauf in der Vorschau je Monat“) gilt seitdem je Schule:
Ein Lauf nennt seine Schulen, und eine Schule ist je Monat in höchstens einem
offenen Lauf. Die Freigabe stellt das Rechnungsdatum des Laufs aus. Details im
[Nachtrag zu ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md#nachtrag-2026-11-08-prüfphase-im-monatslauf-5916).

## Nachtrag 2026-11-08: Preisformen der Preisliste (#6196)

**Status: Vorgeschlagen.**

1. **Zwei Preisformen, eine Preisliste.** Ein Modul einer Schule hat ab einem
   Monatsersten entweder einen Preis **je Wochentag** (Summe der gebuchten
   Wochentage, Ispringen) oder eine **Staffel nach Tagen je Woche** (Preis
   für 1 bis 7 gebuchte Tage, Birkenfeld; nicht Tagespreis mal Tage). Die
   Form gilt je Modul und folgt dem jüngsten Preis des Moduls, der bis zum
   Monat begonnen hat; Preise der anderen Form zählen dann nicht. Ein
   Preiswechsel, auch der Wechsel der Form, wirkt ab seinem Gültigkeitsdatum.
2. **Staffelposition.** Eine Position nach Staffel kostet die Stufe ihrer
   Zahl gebuchter Wochentage. Anteilig (pro rata) gilt Betragsregel 2 mit der
   Stufe als Monatspreis; die Stufe wird auf die Wochentage zu gleichen
   Teilen verteilt (größter Rest), damit Teile auf den Cent aufgehen. Die
   ausgestellte Position hält Stufe und Stufenpreis fest
   (`days_per_week`, `tier_monthly_cents`). Fehlt die Stufe, bleibt der
   Vertrag unter „Ohne Preis“ mit der fehlenden Stufe, nie mit 0 €.
3. **Abgerechnete Preise** (ADR 0119 Punkt 7): Eine abgerechnete Stufe oder
   ein abgerechneter Wochentag behält im Monat Preis, Geschwisterpreis und
   Form; ein abgerechneter Monat wird nie beitragsfrei.
4. **Beitragsfreie Monate** sind eine Einstellung der Preisliste je Schule
   (`care_price_lists.free_months`, z. B. August in Ispringen). Der Lauf
   stellt dafür keine Rechnung aus und zeigt die Zahl der Verträge.
5. **Essen.** Ein Modul kann „Essen extra“ tragen (`meal_extra`); der Preis
   enthält das Essen dann nicht. Das Essen rechnet die Mensa ab (#6169),
   nie als stillschweigende 0 €. Der Essenspreis Birkenfeld wartet auf Ute.
6. Prüfphase (#6177): Stufe, Form und die Zahl beitragsfreier Verträge
   gehören zum Fingerabdruck; eine Änderung danach lässt die Freigabe
   ablehnen.

Register: Migration `1.15.819`; `policy/parent-billing.json` mit
`parent_billing.care_price_lists`; Datenlebenszyklus und Aufbewahrung je ein
Eintrag (keine Personendaten).
