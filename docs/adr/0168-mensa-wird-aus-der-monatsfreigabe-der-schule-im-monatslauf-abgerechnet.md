---
status: proposed
---

# Mensa wird aus der Monatsfreigabe der Schule im Monatslauf abgerechnet

**Status: Vorgeschlagen.**

Entscheidung zu [#4686](https://github.com/moto-nrw/project-phoenix/issues/4686),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 64, 65 und 78. Sie setzt die Mensa-Monatsfreigabe aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
§ 6 um und baut die Mensa in den Monatslauf aus
[ADR 0158](0158-monatsbeitrag-entsteht-aus-der-wirksamen-vertragsfassung-und-den-tarifen-des-traegers.md)
ein. Sie führt keinen Owner und keinen zweiten Lauf ein.

## Entscheidung

1. **Abrechnungsart je Schule.** Der Träger legt für die Mensa jeder Schule ab
   dem 1. eines Monats fest, ob er eine **Pauschale** je Kind und Monat oder
   einen **Preis je Essen** (Spitzabrechnung) berechnet
   (`parent_billing.care_meal_tariffs`). Kein Preis steht im Code. Ein Preis,
   mit dem eine Rechnung einen Mensa-Monat der Schule abgerechnet hat, ändert
   sich für diese Monate nie (409 `parent_billing.care_meal_tariff_month_billed`).
   Eine stornierte Rechnung rechnet nicht mehr ab: Nach einem Storno darf der
   Träger einen falschen Preis korrigieren, die Neuausstellung nutzt ihn.
2. **Zählung.** Ein Essen zählt an einem Kalendertag des Monats in
   Europe/Berlin, der laut Schulkalender kein Wochenende, Schließtag,
   Feiertag oder Ferientag ist, wenn das Kind nach allen Abmeldungen der
   Eltern zum Mittagessen angemeldet ist (Anmeldung aus Meal Plan, #3873) und
   an dem Tag eine Anwesenheit von Student Presence vorliegt. „Anwesenheit mal
   Tagespreis, minus Abmeldungen“: Ein abgemeldeter Tag zählt nicht, auch
   wenn das Kind da war. Ein angemeldeter Tag ohne Anwesenheit ist eine
   Fehlzeit und zählt nicht.
3. **Mensa-Monatsfreigabe.** Nach dem letzten Tag des Monats prüft die
   OGS-Leitung (Rechte `meals:manage` und `meals:orders`) im Schulportal die
   Kinder, ihre angemeldeten Tage, Essen und Fehlzeiten und gibt den Monat
   frei. Die Freigabe rechnet neu und speichert nur, wenn Zahl der Kinder und
   Essen dem gezeigten Stand entsprechen (409 `meals.billing_month_changed`).
   Sie ist ein Objekt von Meal Plan (`schedule.meal_billing_months`,
   `…_children`); eine Freigabe ändert sich nie. Ändert eine nachgetragene
   Anwesenheit den Monat danach, zeigt die Seite den neuen Stand neben dem
   freigegebenen, und die OGS-Leitung gibt ihn **erneut frei**: mit Grund,
   als neue Version neben den früheren (wer und wann stehen in der Zeile,
   die alte Zahl in der Vorversion). Ein unveränderter Monat lässt sich nicht
   erneut freigeben (409 `meals.billing_month_released`), ohne Grund nicht
   (400 `meals.billing_month_reason_required`). Der Träger liest nur die
   neueste Version.
4. **Ablage beim Träger, Grenze nach ADR 0105.** Die Freigabe trägt Schule
   und Träger statt `tenant_id`: Sie ist Abrechnungsbeleg des Trägers und
   bleibt wie die Rechnungen über die Schule hinaus (ADR 0088 § 9). Nur die
   Schule liest und schreibt sie, in ihrer Schul-Transaktion; die Policy
   prüft, dass der genannte Träger der Träger der Schule ist. Der Träger liest
   sie in seiner eigenen Träger-Transaktion ausschließlich über
   `schedule.traeger_meal_billing_children`: nur Schule, Monat, Kind und Zahl
   der Essen, keine angemeldeten Tage, keine Fehlzeit, keine Klasse. Es gibt
   keine gemeinsame Transaktion über Schule und Träger.
5. **Im Monatslauf.** Ein Lauf des Monats M rechnet jeden freigegebenen
   Mensa-Monat bis M ab, den noch keine Rechnung abrechnet (Horizont zwölf
   Monate), eine Position je Monat auf der Rechnung des Vertrags des Kindes
   an dieser Schule in M. Verspätete Freigabe und nachträgliche Abrechnung
   mehrerer Monate (Hardt, MK-001: Januar bis März auf der Rechnung vom
   01.04.) sind dieselbe Regel. Ein Vertrag ohne gebuchtes Modul bekommt eine
   Rechnung nur mit Mensa. Ein Kind ohne Vertrag an der Schule und ein
   Vertrag, dessen Rechnung ein früherer Lauf des Monats schon ausgestellt
   hat, stehen unter „Nicht abgerechnet“; im zweiten Fall rechnet der Lauf
   des Folgemonats die Mensa ab. Ein beendeter Monat ohne Freigabe steht als
   „Freigabe fehlt“. Ein Mensa-Monat eines Kindes wird genau einmal
   abgerechnet (`uq_care_meal_positions_child_month`).
6. **Keine Ermäßigung auf die Mensa.** Die Regelkette aus
   [ADR 0164](0164-ermaessigungen-des-monatsbeitrags-sind-beitragsregeln-des-traegers-mit-nachweis-je-vertrag.md)
   (Einkommensstufe, Geschwister, Alleinerziehende, Höchstbetrag) wirkt auf
   gebuchte Module. Die Mensa ist kein Modul: Sie kommt nach der Kette auf die
   Rechnung und wird von keiner Regel gemindert, auch nicht vom Höchstbetrag.
   MK-001 zeigt keine Ermäßigung. Braucht ein Träger Ermäßigung auf Essen
   (BuT), ist das eine eigene Regelart.
7. **Unveränderlich.** Eine ausgestellte Mensa-Position hält Monat, Art,
   Menge, Preis und Betrag und ändert sich nie (Trigger wie ADR 0119 Punkt 7).
   Jede Position merkt sich die Version der Freigabe, die sie abrechnet
   (`release_version`). Gibt die Schule einen schon abgerechneten Monat
   erneut mit anderem Betrag frei, nennt jeder Lauf ihn unter „Nach der
   Ausstellung geändert“ mit Rechnung, altem und neuem Betrag; das Büro
   storniert die Rechnung, und die Neuausstellung des Monats rechnet die
   neueste Freigabe ab (ADR 0119). Eine Position folgt dabei der Storno-Kette
   ihres Kindes und Monats, auch wenn die Mensa inzwischen auf einer eigenen
   Rechnung steht.
   In der Prüfphase des Laufs (#5916) gehören die Mensa-Positionen, die Monate
   ohne Freigabe, die nicht mehr abrechenbaren Monate und die nicht
   abgerechneten Kinder und die nach der Ausstellung geänderten Monate zum
   Fingerabdruck des geprüften Stands. Eine
   ausgeschlossene Rechnung stellt auch ihre Mensa nicht aus; ein späterer
   Lauf rechnet sie ab.
8. **Protokoll.** Jedes Lesen eines Laufs nennt in `care_run_events` die
   Verträge und die Schülerdatensätze der Mensa ohne Vertrag.
9. **Demo-Löschung.** `platform.erase_closed_demo_visitors` (1.15.811) löscht
   die Freigaben stillgelegter Demo-Schulen mit ihren Kindern und ihren
   Löschstand der Anwesenheit; die Mensa-Positionen gehen mit den Rechnungen.
10. **Bündelung und Zahlung (#4689, ADR 0167).** `TermsSet.InvoiceService(ServiceMeal)`
   entscheidet, wo die Mensa steht: ohne eigene Bedingungen auf der Rechnung
   der Betreuung, sonst auf einer eigenen Mensa-Rechnung desselben Vertrags
   und Laufs (`care_invoices.service = 'meal'`, eine erste Rechnung je
   Vertrag, Monat und Leistung). Zahlungsweg und Fälligkeit löst
   `ResolvePayment` mit den Bedingungen der Mensa auf, den Empfänger
   `ResolveRecipient`; die Freigabe kopiert beides auf die Rechnung.
   Abgerechnet-Erkennung und Neuausstellung nach Storno gelten je Leistung.
11. **Nicht mehr abrechenbar nach dem Löschstand.** Gelöschte Anwesenheit
   sähe aus wie Fehlzeit. Ein Monat gilt deshalb als nicht mehr abrechenbar,
   wenn er vor dem ersten vollständigen Tag beginnt: dem späteren von „heute
   minus Aufbewahrungsfrist“ und dem Tag, bis zu dem die nächtliche Löschung
   schon gelöscht hat. Student Presence schreibt diesen Löschstand
   (`active.attendance_retention_marks`, wächst nur, je Schule ohne
   `tenant_id` und ohne Fremdschlüssel wie die Freigaben) und beantwortet
   `AttendanceCompleteFrom`. Eine später verlängerte Frist macht gelöschte
   Tage nicht wieder abrechenbar. Schule und Träger zeigen solche Monate als
   „Nicht mehr abrechenbar“ ohne Knopf; der Träger liest den Löschstand je
   Schule in einer eigenen Schul-Transaktion (ADR 0105).
12. **Kostenträger-Anteile wirken je Anteil auf Module, Mensa oder beides.**
   ADR 0169 legt den Anteil auf den Beitrag nach der Regelkette fest, nennt
   die Mensa aber nicht; Punkt 2 hält sie aus der Kette. Bei Bildung und
   Teilhabe ist die Übernahme des Mittagessens der Normalfall, ein Jugendamt
   übernimmt meist die Betreuung. Jeder Anteil trägt deshalb einen Geltungs-
   bereich (`scope`: `care`, `meal`, `both`, Migration `1.15.817`); ein
   Anteil ohne Angabe und jeder bestehende gilt für die Betreuung, wie vor
   der Mensa. Die Kette (ADR 0169 Punkt 3) wendet nur die Anteile an, die die
   Betreuung abdecken. Danach, wenn die Mensa auf den Rechnungen steht und
   gegebenenfalls auf eine eigene Rechnung gewandert ist (ADR 0167), nimmt
   `applyMealCostShares` den Anteil der Mensa-Positionen in derselben
   Reihenfolge der Arten: Prozent vom vollen Mensa-Betrag, je Teil auf den
   Cent gerundet; ein fester Betrag nur, was er im Monat beim Vertrag noch
   nicht genommen hat (erst die Module, der Rest die Mensa, auch über zwei
   Rechnungen); jeder höchstens den Rest. Die Mensa-Positionen bleiben
   ungemindert (Punkt 2); nur `total_cents` ist der Rest der Familie. Eine
   Mensa-Rechnung trägt nur die Anteile der Mensa. Die ausgestellte Rechnung
   kopiert den Geltungsbereich mit dem Anteil (ADR 0169 Punkt 9), Brief und
   Liste nennen ihn („100 % für die Mensa“), und die Liste des Kostenträgers
   zeigt als Beitrag die Grundlage seines Anteils. Die Eingabe schlägt für
   Bildung und Teilhabe „Mensa“ vor, für die anderen Arten „Betreuung“.
   Handgerechnet: `TestCareCostSharesCoverTheModulesTheMensaOrBoth`.

## Verworfene Optionen

- **Freigabe als Schultabelle mit `tenant_id`:** Der Träger sähe sie nur
  über eine Admin-Transaktion oder eine Projektion, und die Schullöschung
  nähme den Abrechnungsbeleg vor dem Träger mit.
- **Mensa als Modul mit Preis je Wochentag:** Die Spitzabrechnung hängt an
  Anwesenheit und Abmeldung, nicht am gebuchten Wochentag.
- **Eigener Mensa-Lauf und Nummernkreis:** Hardt will eine Rechnung für
  Betreuung und Essen; ein zweiter Lauf hätte Sperre, Vorschau und Freigabe
  doppelt.
- **Live-Lesen der Anwesenheit im Träger-Lauf:** Die Zahlen änderten sich
  nach der Prüfung der OGS-Leitung, und der Träger sähe Fehlzeiten.

## Register

- `policy.json`: fünf `data_objects` (zwei Meal Plan, zwei Elternabrechnung,
  ein Student Presence: `active.attendance_retention_marks`).
- `command-effects`: Kalender, Anwesenheit und Wochenlisten der Freigabe,
  die Träger-Lesung der Freigaben.
- Query-Budget `api.traeger.care_meal_tariffs` (4, exact).
- Budgets (gemessen auf dem Stand dieses PRs): `modules/mealplan/compose`
  74 → 105 Zeilen (Freigabe und Träger-Lesung, +31),
  `modules/parentbilling/compose` 67 → 74 Zeilen (Mensa im Monatslauf, +7,
  auf den 67 aus ADR 0167), `moduleComposeBudgetTotal` 13087 → 13125 (+38).
  Pass-through `modules/studentpresence` 122 → 123 und
  `modulePassthroughOwnerBudgetTotal` 1404 → 1405: `AttendanceCompleteFrom`
  prüft die Frist und nimmt das spätere von Stichtag und Löschstand; die
  Prüfung zählt es als einen Feldaufruf.
- Command-Effects: die Leseaufrufe von `AttendanceCompleteFrom` aus Meal Plan
  und der Elternabrechnung (`query`), die eigene Schul-Transaktion je Schule
  beim Träger (`platform`/`scope`) und der Beobachtungs-Hook von Student
  Presence (`unresolved`, Freigabe 0168).
- Aufbewahrung und Datenlebenszyklus: je fünf Einträge (der Löschstand ohne
  Personendaten, `tenantReference` über `school_id`). Die Schullöschung
  (`platform.school_erasure_*`) löscht den Löschstand mit seiner Schule.
- Migrationen `1.15.811` und `1.15.817` (Geltungsbereich der Kostenträger-Anteile).
