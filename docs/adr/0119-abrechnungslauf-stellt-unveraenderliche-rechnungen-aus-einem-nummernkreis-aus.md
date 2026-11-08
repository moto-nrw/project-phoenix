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
  sich nach dem Storno wieder ändern. Korrekturen des alten Laufs übernimmt
  die Vorschau als Vorschlag (Nachtrag 2026-11-04, #5663).
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
- **Migration** `1.15.743` (nach `1.15.740`; 741 und 742 sind von offenen
  Pull Requests belegt).

## Nachtrag 2026-11-04: Neue Rechnung übernimmt die Korrekturen (#5663)

Löst den Satz „Korrekturen des alten Laufs gehen nicht mit“ ab: Vergaß das
Büro die Korrektur im neuen Lauf, bekam die Familie wieder den unkorrigierten
Betrag.

- **Vorschlag ohne eigene Zeile.** Die Vorschau bleibt ohne gespeicherte
  Entwürfe (Punkt 2). Sie liest die Positionen `correction` der stornierten
  Rechnung und wendet sie nach den festgehaltenen Zeilen und vor den
  Korrekturen des Laufs an. Quelle ist die Position, nicht
  `run_corrections`: Die Korrektur geht mit dem Kind, die Rechnung behält ihre
  Kopie (Punkt 11). Ein Storno während eines offenen Laufs zeigt den Vorschlag
  damit sofort, und eine Kette (`C` ersetzt `B` ersetzt `A`) bringt alle mit.
- **Grenze null.** Ist der Beitrag jetzt niedriger als auf der stornierten
  Rechnung, senkt ein Vorschlag den Betrag nur bis 0 €, weil eine Rechnung nie
  negativ ist. `amount_cents` ist dann, was der Entwurf anwendet,
  `source_amount_cents`, was die stornierte Rechnung hatte; die Seite nennt
  beides.
- **Entfernen und ändern.** `parent_billing.run_correction_dismissals` hält je
  Lauf die Vorschläge, die er nicht mehr macht; es wird nur angefügt.
  `DELETE /traeger/billing-runs/{id}/carried-corrections/{positionID}` entfernt
  einen Vorschlag und lehnt ab, wenn Korrekturen des Laufs das Kind sonst unter
  null brächten. `PUT` auf denselben Pfad ersetzt ihn durch eine Korrektur des
  Laufs mit neuem Betrag und Grund. Beide sperren den Lauf, schreiben `correct`
  in die Spur und liegen hinter dem Recht „Abrechnung“; ein fremder Lauf ist
  404, eine Position ohne Vorschlag 404
  `parent_billing.carried_correction_not_found`.
- **Historie.** Die Freigabe schreibt jeden angewendeten Vorschlag nach
  `run_corrections` mit `carried_from_number`, damit die ausgestellte Rechnung
  ihre Korrekturen mit Herkunft nennt.
- **Gutschriften.** Sie bleiben eigene Dokumente: Die Vorschau nennt sie in
  `replaced_credits` als Hinweis und zieht sie nicht ab. Nach den Regeln dieses
  Nachtrags von 2026-11-03 hat eine stornierte Rechnung heute keine Gutschrift,
  weil eine Rechnung mit Gutschrift nicht storniert wird und eine stornierte
  keine Gutschrift bekommt. Der Hinweis greift erst, wenn sich diese Regel
  ändert; bis dahin deckt ihn ein Test mit direkt geschriebenem Zustand ab.
- **Register.** `policy.json`: ein `data_object`
  `parent_billing.run_correction_dismissals`, keine neue Regel. Datenlebenszyklus
  und Aufbewahrung je ein Eintrag; die Zeile geht mit ihrem Lauf oder der
  Position. Zwei schreibende Routen im Route-Golden und in der Nutzungsanalyse
  als `notCaptured` (Träger, keine Schule). Migration `1.15.748`.

## Nachtrag 2026-11-08: Storno vor dem Versand und Monatsrechnungen (#4684)

Präzisiert den Nachtrag vom 03.11.2026 („Hat der Lauf seine Schreiben, bekommt
Storno oder Gutschrift sein Schreiben sofort“) für die Monatsrechnungen nach
dem SJR-Abgleich E1 A (Entscheidung D3 in #4684); die Ferienrechnungen bleiben
unverändert.

- **Prüfen vor dem Ausstellen, danach nur Storno.** Ausstellen und Senden
  bleiben zwei Schritte. Was der SJR „nach der Erstellung ändern“ nennt,
  geschieht vor der Freigabe (Prüfphase, #5916); nach der Freigabe gilt Storno
  und Neuausstellung. Keine Rechnung wird nach der Ausstellung geändert.
- **Kein Schreiben an die Familie für einen Storno vor dem Versand.** Wurde das
  Schreiben einer Monatsrechnung nie gesendet, bekommt ihr Storno kein Schreiben,
  die Rechnung wird nicht mehr gesendet, und weder Postfach noch Druck nennen
  sie. Der Storno bleibt mit Nummer und Grund in Lauf und Spur. Wurde die
  Rechnung gesendet, bekommt der Storno sofort sein Schreiben.
- **Ein Nummernkreis.** Der Storno einer Monatsrechnung nimmt wie die
  Ferienbelege die nächste Nummer aus Muster und Zähler seiner Leistung (`care`);
  keine Nummer wiederholt eine Ferien- oder Monatsnummer des Trägers.
- **Neuausstellung.** Der nächste Lauf des Monats rechnet den Vertrag einer
  stornierten Rechnung neu ab und nennt sie (`replaces_invoice_id`). Details:
  [ADR 0165](0165-monatsrechnungen-nehmen-schreiben-versand-und-storno-der-ferienrechnungen.md).

## Nachtrag 2026-11-08: Prüfphase im Monatslauf (#5916)

Setzt die Entscheidung D3 / SJR-Abgleich E1 A für die Monatsrechnungen um:
Die Abrechnung prüft vor der Ausstellung, nicht danach. Kein Widerruf der
Unveränderlichkeit (Punkt 7); keine gespeicherten, frei editierbaren
Rechnungsentwürfe (Option B) und keine Änderung ausgestellter Rechnungen bis
zum Versand (Option C). Punkt 2 bleibt: Die Vorschau wird bei jedem Lesen
berechnet.

- **Monat und Schulen.** Ein Monatslauf nennt seinen Monat und eine Auswahl der
  Schulen des Trägers, vorbelegt mit allen Schulen ohne offenen Lauf des Monats
  (`parent_billing.care_run_schools`). Eine Schule ist je Monat in höchstens
  einem offenen Lauf; ein eindeutiger Index der offenen Schulen sichert das auch
  beim gleichzeitigen Anlegen. Läufe mit getrennten Schulen dürfen nebeneinander
  geprüft werden. Das ersetzt für den Monat „höchstens ein Lauf in der Vorschau“
  aus ADR 0158 Punkt 5. Die Vorschau rechnet den ganzen Monat des Trägers, damit
  Geschwister und Regelkette jedes Kind sehen, und zeigt dann nur die Schulen
  des Laufs.
- **Rechnungsdatum.** Das Rechnungsdatum ist ein Feld des Laufs (vom Ersten des
  Vormonats bis zum Letzten des Folgemonats, also auch im Leistungsmonat, z. B.
  Oktober mit 30.10.). Ohne Datum gilt der Tag der Freigabe. Die Freigabe setzt
  es auf jede Rechnung und auf den Lauf; das Jahr der Nummer folgt ihm.
- **Fälligkeiten.** Die Fälligkeit je Zahlungsweg (Überweisung, Lastschrift)
  mit Abweichung der Familie kommt mit #4689 als Regel; die Prüfphase nimmt
  dann ihre Eingabe auf. Bis dahin nennt erst das Schreiben das Zahlungsdatum
  (ADR 0165 Punkt 2).
- **Ausschluss.** Vor der Freigabe lässt sich die Rechnung eines Vertrags mit
  Grund ausschließen und wieder aufnehmen. Beides wird nur angefügt: als Eintrag
  `exclude` bzw. `include` der Laufspur mit Konto, Zeitpunkt, Vertrag und Grund.
  Eine ausgeschlossene Rechnung wird nicht ausgestellt, bleibt in der Vorschau
  sichtbar und zählt nicht zur Summe. Der Ausschluss gehört zu seinem Lauf: Der
  nächste Lauf desselben Monats rechnet das Kind wieder ab, wenn es abrechenbar
  ist. Es gibt keine dauerhafte Abrechnungssperre.
- **Prüfliste.** Die Vorschau eines offenen Laufs lässt sich als Excel oder CSV
  laden, eine Zeile je Position und je Ermäßigung, mit Rechnungsdatum, Schule,
  Kind, Empfänger, Modul, Wochentagen, Zeitraum, Betrag und Status
  (ausgestellt, ausgeschlossen mit Grund, ohne Preis, doppelter Vertrag).
  Ausgeschlossene Rechnungen sind gekennzeichnet und stehen außerhalb der
  Summe. Die Liste braucht keine ausgestellte Rechnung und vergibt keine
  Nummer; jeder Download steht mit den gezeigten Verträgen in der Laufspur.
  Debitorennummer, Zahlungsweg und Fälligkeit folgen mit #4689 und #4694; das
  Muster des SJR wird nachträglich abgeglichen (E4).
- **Ausstellung prüft erneut.** Die Freigabe sperrt Monatsabrechnung und Lauf,
  rechnet die Vorschau in derselben Transaktion neu und vergleicht Zahl, Summe
  und einen Fingerabdruck des geprüften Stands (Schulen, Rechnungsdatum,
  Monatsregel, Ausschlüsse, Empfänger, Positionen und Ermäßigungen jeder
  Rechnung). Weicht etwas ab, lehnt sie ab (409
  `parent_billing.billing_run_changed`). Ausschluss und Rechnungsdatum halten
  dieselbe Sperre des Laufs.
- **Storno vor dem Versand.** Präzisiert den Nachtrag vom 03.11.2026 für die
  Monatsrechnungen wie der Nachtrag oben: Der Storno einer nie gesendeten und nie
  zum Drucken geladenen Monatsrechnung erzeugt kein Schreiben, keine Mail und
  keinen Eintrag im Postfach; Storno und Spur bleiben nachvollziehbar. Eine
  gesendete Rechnung behält den bestehenden Korrekturweg. Belegt durch
  `TestCareInvoiceCancelledBeforeTheSendReachesNoFamilyAndIsIssuedAgain` und
  `TestCareInvoiceCancelledAfterTheSendGetsItsLetter`.
- **Register.** Migration `1.15.814`; ein `data_object`
  (`parent_billing.care_run_schools`) für `parent-billing`; Fehlercode
  `parent_billing.care_exclusion_conflict`; Routen
  `GET /traeger/care-billing-runs/schools`, `PUT …/{id}/invoice-date`,
  `POST …/{id}/exclusions`, `DELETE …/{id}/exclusions/{contractID}`,
  `GET …/{id}/review-export`.
