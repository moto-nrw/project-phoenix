---
status: accepted
---

# Ferienbeitrag entsteht in der Elternabrechnung und wird bei der Bestätigung festgehalten

Entscheidung vom 25.10.2026 zu [#4664](https://github.com/moto-nrw/project-phoenix/issues/4664),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 29, 61, 63 und 69. Sie ist der erste Bau-Schnitt des Owners
Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
und baut auf
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md),
[ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md)
und [ADR 0114](0114-ferienanmeldungen-pruefen-mit-eigenem-protokoll-in-der-traeger-transaktion.md)
auf. Sie führt keinen Owner ein, der nicht schon in ADR 0088 angenommen ist,
und hält fest, was dieser erste Code in den Registern ändert (ADR 0057
Punkt 5, ADR 0070).

## Entscheidung

1. **Owner.** `parent-billing` (`modules/parentbilling`) ist der neunzehnte
   Domain-Owner, wie ADR 0088 beschlossen hat. Er steht in `policy.json`
   (Policy-Epoche 95), in der Liste der Domain-Owner
   (`backend/internal/architecture/policy.go`: genau die achtzehn Domains oder
   die achtzehn mit `parent-billing`, damit auch die Basis eines Pull Requests
   gültig bleibt), in ADR 0066 §1,
   in `backend/architecture/README.md` und im Glossar. Seine Tabellen liegen
   im Schema `parent_billing`, tragen `organization_id` und die Träger-Policy
   aus ADR 0105.
2. **Beitragsregeln.** `parent_billing.contribution_rules` ist die Tabelle
   „Beitragsregel“ aus ADR 0088 §4: feste Regelarten mit Parametern und
   Gültigkeit, je Träger und Leistung. v1 kennt die Leistung `holiday` und
   zwei Regelarten:
   - **Geschwister** (`sibling`): ab dem zweiten oder dritten Kind eine
     Ermäßigung als Anteil in Basispunkten oder als fester Betrag in Cent.
   - **Befreiung bei Leistungsbezug** (`benefits_exemption`): kein Beitrag,
     mit dem Vermerk, den die Regel nennt.
   Zwei Regeln derselben Art und Leistung gelten nie am selben Tag (Exclusion
   Constraint). Der Grundpreis ist keine eigene Zeile: Er ist der Preis des
   Zeitmodells jeder gebuchten Woche (ADR 0105 Punkt 6). Im Code steht kein
   Preis und keine Ermäßigung. Die Regeln pflegt die Rolle mit dem Recht
   „Abrechnung“ unter `/traeger/contribution-rules`; sie sind keine
   Einstellungen nach ADR 0086, weil sie Gültigkeit und Geltungsbereich
   haben.
3. **Regelkette.** Die Reihenfolge ist fest (ADR 0088 §4): Grundpreis,
   Geschwister, Befreiung. Es gelten die Regeln, die am ersten gebuchten Tag
   der Anmeldung gültig sind. Geschwister sind die Kinder derselben Anmeldung
   mit mindestens einer Buchung; das Kind mit dem höchsten Grundpreis ist das
   erste, bei Gleichstand gilt die Reihenfolge im Formular. Jede Regel rundet
   kaufmännisch auf ganze Cent und schreibt eine eigene Position; eine
   Ermäßigung ist höchstens so hoch wie der Grundpreis. Die Befreiung setzt
   den Rest jedes Kindes auf null und schreibt ihren Vermerk auch dann, wenn
   nichts übrig ist. Geschwister über Anmeldungen und Schulen hinweg folgen
   mit der Familie (#4685).
4. **Leistungsbezug.** Eltern geben in der Anmeldung an, dass sie Leistungen
   beziehen (`enrollment.holiday_registrations.benefits_declared`, Owner
   Enrollment). Die Zusammenfassung zeigt dann den Beitrag mit Befreiung. Ob
   die Befreiung gilt, entscheidet das Träger-Büro bei der Bestätigung; ohne
   gültige Befreiungsregel lehnt die Bestätigung sie mit 409
   `parent_billing.benefits_exemption_unavailable` ab.
5. **Festhalten.** Die Bestätigung einer Ferienanmeldung zieht von
   `/traeger/holiday-periods/{id}/registrations/{id}/confirm` nach
   `/traeger/holiday-contributions/{Zeitraum}/registrations/{Anmeldung}/confirm`
   um, mit demselben Recht „Anmeldung verwalten“. Die Elternabrechnung ruft
   dort in derselben Träger-Transaktion den unveränderten Befehl
   `ChangeHolidayRegistration` von Enrollment (Teilnahme nach dem Nachtrag zu
   [ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)):
   Enrollment sperrt Zeitraum und Anmeldung, bestätigt und schreibt sein
   Protokoll; die Elternabrechnung rechnet aus den zurückgegebenen Buchungen
   und schreibt je Kind `parent_billing.holiday_contributions` mit seinen
   Positionen in `parent_billing.holiday_contribution_lines`. Scheitert danach
   etwas, markiert die Route die Transaktion zum Zurückrollen. Jede Position
   hält, was sie zeigt (Woche, Zeitmodell, Rang, Anteil, Vermerk); spätere
   Preise, Zeitmodelle und Regeln ändern einen festgehaltenen Beitrag nicht
   (User Story 63). Die Anfrage-Rolle darf ihn lesen und anlegen, nie ändern
   oder löschen.
6. **Abhängigkeit.** Die Elternabrechnung erreicht Enrollment nur über ihren
   eigenen Port `HolidayRegistrations`, den die Root an die öffentliche
   Ferienanmeldung und die Prüfung der Anmeldungen bindet
   (`api/serve_mounts_parent_billing.go`). Kein Import geht von
   `parent-billing` zu `enrollment` und keiner zurück. Der Graph bleibt
   azyklisch; Enrollment hängt nicht von der Elternabrechnung ab. Bis der
   Betreuungsvertrag (#4678, #4680) die Vertragsversion als einzige Quelle
   für Beiträge einführt, ist der festgehaltene Ferienbeitrag diese Quelle für
   den Ferienlauf (#4668). Der Vertrag übernimmt ihn dann in seine Version.
7. **Öffentliche Zusammenfassung.** `POST /traeger/holiday-quotes/{Schlüssel}`
   rechnet ohne Sitzung für die Buchungen des Formulars. Der Schlüssel wird
   wie bei der Ferienanmeldung in einer Admin-Transaktion aufgelöst; danach
   liest die Route Preise und Regeln in der Träger-Transaktion des
   gefundenen Trägers und speichert nichts. Die Anfrage trägt keine
   personenbezogenen Daten. Das sind die beiden neuen Stellen in
   `transaction_boundaries.json`; `backend/test/holiday_contribution_transaction.go`
   lässt genau sie am Schrumpf-Vergleich vorbei.
8. **Zugriffsprotokoll.** Der Beitrag einer Anmeldung ist personenbezogen
   (Kind, Leistungsbezug). Jede Anzeige liest die Anmeldung über die Prüfung
   von Enrollment, die den Träger prüft und den Zugriff in
   `enrollment.holiday_registration_audit` schreibt (ADR 0114 Punkt 6); die
   Bestätigung steht dort als `confirm`. Die Regeln enthalten keine
   personenbezogenen Daten.
9. **Aufbewahrung.** Ein festgehaltener Beitrag geht mit dem Kind seiner
   Anmeldung (Fremdschlüssel mit `ON DELETE CASCADE`), also 92 Tage nach der
   letzten Woche oder mit dem Träger. Die Pflicht nach § 147 AO beginnt erst
   mit dem Beitragsdokument des Laufs (#4668); die Fristen legt #4720 fest.
10. **Register.**
    - `policy.json`: Owner `parent-billing` (domain), fünf Pakete, drei
      `data_objects`, vier Regeln (HTTP auf den eigenen Vertrag, `api/common`
      und `tenant`; Postgres auf `tenant`), Epoche 95.
    - `command-effects.json`: vier geprüfte Bindungen auf Enrollment
      (Bestätigung und Lesen als Teilnahme, Link und Zeitraum als Abfrage).
    - `passthrough.json`: acht Methoden mit genauer Evidenz.
    - Budgets: `modules/parentbilling/compose` mit 20 Zeilen
      (`moduleComposeBudgetTotal` 12620 → 12640) und
      `modules/parentbilling` mit 5 Durchreichungen
      (`modulePassthroughOwnerBudgetTotal` 1406 → 1411).
    - Query-Budget `api.traeger.contribution_rules` (2, exact).
    - Datenlebenszyklus: drei Einträge mit `scope: organization`.

## Verworfene Optionen

- **Preis in der Anmeldung von Enrollment festhalten:** ADR 0105 und
  ADR 0114 weisen das Festhalten der Elternabrechnung zu; Enrollment würde
  Regeln der Abrechnung kennen.
- **Enrollment ruft die Elternabrechnung bei der Bestätigung:** Ein unterer
  Owner hinge von der Elternabrechnung ab (ADR 0088 §10).
- **Neuer Workflow für die Bestätigung:** ein neuer Owner ohne angenommene
  Entscheidung. Die Teilnahme an einem geschützten Befehl reicht, weil kein
  gemeinsames Sperr-Protokoll und kein Abgleich nach dem Commit nötig ist.
- **Regeln als Träger-Einstellungen (ADR 0107):** Einstellungen haben keine
  Gültigkeit und kein Recht „Abrechnung“; ADR 0088 legt die Tabelle
  `contribution_rules` fest.
- **Beitrag im Browser rechnen:** Zwei Umsetzungen der Kette würden
  auseinanderlaufen, und die Zusammenfassung zeigte nicht, was die
  Bestätigung festhält.

## Folgen

- Eine Umbuchung nach der Bestätigung ändert den festgehaltenen Beitrag
  nicht. Die Änderung mit neuer Version und Nachberechnung folgt mit dem
  Vertrag (#4681) und dem Lauf (#4668).
- Weitere Regelarten (Einkommensstufe, Ermäßigung, Höchstbetrag,
  Kostenträger) und weitere Leistungen kommen als Code und als neuer Wert
  der Prüfungen in `contribution_rules` dazu (#4684, #4685, #4687).
- Die Zusammenfassung zeigt den Beitrag nach den Regeln von heute; was gilt,
  hält erst die Bestätigung fest.

## Nachtrag 2026-11-03: Kostenübernahme durch einen Kostenträger (#5627)

Für Familien mit Leistungsbezug, deren Beitrag eine Stelle erstattet (zum
Beispiel das Jugendamt nach § 90 Abs. 4 SGB VIII), reicht die Befreiung
nicht: Sie erlässt den Betrag, niemand schuldet ihn. Der schmale Schnitt vor
#4687:

1. **Kostenträger des Trägers.** `parent_billing.cost_bearers` (Owner
   Elternabrechnung, Träger-Policy aus ADR 0105) hält Name, Anschrift,
   Aktenzeichen und ob die Stelle noch zur Wahl steht. Die Abrechnung pflegt
   sie mit den Regeln (`ContributionRules`), weil ADR 0088 §4 den
   Kostenträger als Zahler der Kette führt. Das Träger-Büro sieht beim
   Bestätigen nur die aktiven, über die Einstellungen der Bestätigung.
2. **Neue Zeilenart in der festen Kette.** Nach Grundpreis und Geschwister
   setzt `cost_takeover` den Rest jedes Kindes auf 0 €: Der Kostenträger
   übernimmt, was die Familie nach der Geschwisterermäßigung zahlen müsste.
   Befreiung und Kostenübernahme schließen sich aus; die Kostenübernahme gilt
   für alle Kinder der Anmeldung.
3. **Zahler als Kopie an der Zeile.** Die Zeile und die Position der
   Rechnung halten ID und Namen des Kostenträgers fest. Eine Umbenennung
   ändert keinen festgehaltenen Beitrag und keine ausgestellte Rechnung
   (ADR 0119). Die ID trägt keinen Fremdschlüssel: Die Löschung eines Trägers
   entfernt Kostenträger und Rechnungen in einer Kaskade, die ein
   Fremdschlüssel abweisen würde. Einen Kostenträger, den eine Zeile oder
   Position nennt, löscht die Elternabrechnung nicht; er lässt sich nur
   ausschalten.
4. **Brief, Übersicht und Liste.** Die Rechnung zeigt 0 €; der Brief nennt
   den Kostenträger statt des Zahlungstexts. Die Übersicht für die Kämmerei
   (ADR 0122) nennt unter der Gesamtsumme je Kostenträger, was er übernimmt;
   die Gesamtsumme bleibt, was die Familien zahlen. Je freigegebenem Lauf und
   Kostenträger gibt es eine Liste für die Erstattung, deren Download im
   Protokoll des Laufs steht.
5. **Nachweis.** Das Träger-Büro lädt einen Nachweis, den die Familie auf
   Papier bringt, zur Anmeldung hoch, mit den Grenzen des Nachweises der
   Eltern (#5238). Er ist keine Bedingung der Kostenübernahme.

Ein eigenes Beitragsdokument an den Kostenträger, Anteile, Ablaufdaten und
die Kostenübernahme je Kind bleiben #4687.

## Nachtrag 2026-11-03: Geschwister aus getrennten Anmeldungen (#5628)

Eltern melden Geschwister oft in zwei Formularen an, und ein Formular nimmt
höchstens drei Kinder auf. Punkt 3 kannte Geschwister nur in derselben
Anmeldung. Für Ferienanmeldungen eines Zeitraums gilt jetzt:

1. **Geschwistergruppe.** Das Träger-Büro führt Anmeldungen desselben
   Zeitraums per Klick als Geschwister zusammen, nie automatisch. Gleiche
   E-Mail oder gleicher Nachname des Elternteils sind nur ein Hinweis in der
   Liste. Die Gruppe gehört der Elternabrechnung:
   `parent_billing.holiday_sibling_groups` und
   `parent_billing.holiday_sibling_group_members` (Träger-Policy aus
   ADR 0105). Eine Anmeldung gehört höchstens zu einer Gruppe. Das Mitglied
   geht mit seiner Anmeldung (ADR 0118); eine Gruppe mit nur noch einer
   Anmeldung geht mit (Trigger). Eine stornierte Anmeldung lässt sich nicht
   zusammenführen, wohl aber trennen.
2. **Rang über die Gruppe.** Die Regelkette rankt die Kinder aller nicht
   stornierten Anmeldungen der Gruppe wie eine Anmeldung: höchster Beitrag
   zuerst, bei Gleichstand die früher abgesendete Anmeldung, dann die
   Reihenfolge im Formular. Es gelten die Regeln am ersten gebuchten Tag der
   Gruppe. Jede Anmeldung hält nur die Zeilen ihrer eigenen Kinder fest. Eine
   bestätigte Anmeldung rankt mit den Grundpreisen und der Befreiung, die sie
   festgehalten hat; ein späterer Preis ändert sie nicht. Die Befreiung
   bleibt Sache jeder Anmeldung, ebenso die Kostenübernahme (#5627): Sie
   folgt nach der Geschwisterermäßigung und übernimmt nur den Rest der
   Kinder ihrer Anmeldung; die Ermäßigung der anderen Anmeldungen bleibt.
   Eine bestätigte Anmeldung rankt mit dem Kostenträger, den sie festgehalten
   hat. Die Elternabrechnung liest die weiteren
   Anmeldungen über ihren Port `HolidayRegistrations`; jeder Lesezugriff
   steht im Protokoll der jeweiligen Anmeldung (ADR 0114 Punkt 6).
3. **Fassungen statt Änderung (zu Punkt 5).** Zusammenführen, Trennen,
   das Stornieren (Punkt 8) und das Bestätigen einer Anmeldung der Gruppe
   rechnen den festgehaltenen Beitrag jeder bestätigten Anmeldung der
   Gruppe neu. Ändert er sich, kommt er als neue Fassung dazu
   (`holiday_contributions.version`); die alte bleibt. Gültig ist die
   höchste Fassung. Die Anfrage-Rolle fügt weiter nur ein und liest. Eine
   neue Fassung behält die Zeit der Bestätigung; ihr Konto ist das, das
   zusammengeführt, getrennt oder storniert hat.
4. **Grenze: ausgestellte Rechnung.** Zusammenführen und Trennen gehen nur,
   solange keine Anmeldung der Gruppe eine ausgestellte Rechnung hat
   (ADR 0119); sonst 409 `parent_billing.sibling_group_invoiced`. Die
   Einzelansicht jeder Anmeldung der Gruppe erfährt diese Sperre. Sie laufen
   unter derselben Sperre der Rechnungen des Zeitraums wie die Freigabe
   eines Laufs und halten danach den Zeitraum in Enrollment
   (`HoldHolidayPeriod`), bevor sie die Gruppe lesen: Eine Stornierung oder
   Umbuchung läuft vorher ganz oder danach. Ein offener Lauf zeigt die neuen
   Beträge in seiner Vorschau, und die Freigabe prüft ohnehin die geprüfte
   Vorschau. Das Bestätigen und das Ändern der Debitorennummer nehmen
   dieselben Sperren.
5. **Ein Brief je Anmeldung.** Rechnung und Brief bleiben je Anmeldung. Die
   Ermäßigung steht auf der Rechnung des Kindes, das sie bekommt. Ein Brief
   nennt nie Kinder oder Eltern einer anderen Anmeldung; zwei Anmeldungen
   können von getrennt lebenden Eltern kommen.
6. **Debitorennummer.** Eine Gruppe zahlt mit einer Nummer (ADR 0121
   Nachtrag #5621): Beim Zusammenführen bekommen die bestätigten Anmeldungen
   die Nummer der ersten nicht stornierten Anmeldung der Gruppe, die eine
   hat. Eine Bestätigung ohne Nummer übernimmt die Nummer der Gruppe; eine
   neue, geänderte oder entfernte Nummer gilt für alle bestätigten
   Anmeldungen der Gruppe ohne ausgestellte Rechnung.
7. **Protokoll und Recht.** Die Routen liegen unter
   `/traeger/holiday-contributions/{periodId}/sibling-groups` hinter dem
   Recht „Anmeldung verwalten“. Zusammenführen und Trennen stehen mit den
   anderen Anmeldungen im Protokoll jeder betroffenen Anmeldung
   (`sibling_merge`, `sibling_split`), geschrieben über den Port in
   derselben Träger-Transaktion (ADR 0067, wie die Bestätigung).
8. **Stornierung (#5662).** Die Elternabrechnung storniert eine Anmeldung
   über Enrollment, wie sie bestätigt (Route
   `/traeger/holiday-contributions/{periodId}/registrations/{registrationId}/cancel`,
   Recht „Anmeldung verwalten“); die Route unter
   `/traeger/holiday-periods` entfällt. Sie nimmt dieselben Sperren wie ein
   Zusammenführen und liest die Gruppe erst danach. In derselben
   Transaktion nimmt sie die stornierte Anmeldung aus der Gruppe und rechnet
   jede übrige bestätigte Anmeldung der Gruppe neu (neue Fassung, Punkt 3).
   Eine ausgestellte Rechnung hält die Stornierung nicht auf: Die Anmeldung,
   die sie abrechnet, behält ihren Beitrag. Ihre Einzelansicht meldet
   `correction_needed`, solange die Gruppe sie heute anders rangiert, als
   die Rechnung abrechnet; korrigiert wird mit Storno oder Gutschrift
   (#4690). Jede betroffene Anmeldung bekommt den Eintrag `sibling_cancel`
   mit den anderen Anmeldungen.

Nicht in diesem Nachtrag: das Alter als Rang, Stufenregeln, Geschwister über
Zeiträume, Schulen und Verträge (#4685), eine Rechnung je Familie (#4689).

## Nachtrag 2026-11-04: Rang und Kostenübernahme je Träger einstellbar (#5664)

Die Kita-Satzung der Gemeinde Bissendorf nennt das ältere Kind als erstes, und
ob der Landkreis den vollen Preis erstattet, ist offen. Beide Regeln der Kette
werden deshalb zu Einstellungen des Trägers. Die Standardwerte sind die feste
Kette von vorher.

1. **Einstellungen statt Beitragsregeln.** `parent_billing.sibling_rank`
   (`highest_contribution`, `oldest_child`) und
   `parent_billing.cost_takeover_order` (`after_discount`, `before_discount`)
   sind Träger-Einstellungen (ADR 0086, ADR 0107) unter Einstellungen →
   Ferienbetreuung, mit dem Recht „Einstellungen“. Sie sind Wahlen in der
   Kette, keine Regelart mit Gültigkeit (Punkt 2). Ein unbekannter Wert oder
   ein Lesefehler lehnt die Rechnung ab; es gibt keinen Rückfall auf den
   Standard.
2. **Ältestes Kind zuerst.** Das Geburtsdatum liest die Elternabrechnung aus
   der Anmeldung über den Port `HolidayRegistrations`; es steht im Protokoll
   wie jeder Lesezugriff (ADR 0114 Punkt 6). Sie speichert es nicht. Bei
   gleichem Datum gilt die frühere Anmeldung, dann die Reihenfolge im Formular.
3. **Öffentliche Zusammenfassung (zu Punkt 7).** Das Formular schickt je Kind
   nur den Platz nach Alter (`age_rank`), nicht das Geburtsdatum. Die Anfrage
   trägt weiter keine personenbezogenen Daten, und der Browser rechnet keinen
   Beitrag.
4. **Vor dem Rabatt (zu Punkt 3).** Der Kostenträger übernimmt den vollen Preis
   der Kinder seiner Anmeldung. Sie behalten ihren Rang und zählen weiter als
   Geschwister, bekommen aber keine Ermäßigung; die Ermäßigung der übrigen
   Kinder ändert sich nicht. Die Befreiung folgt weiter nach der Ermäßigung.
5. **Festhalten (zu Punkt 5).** Jeder festgehaltene Beitrag hält beide Werte
   (`holiday_contributions.sibling_rank`, `.cost_takeover_order`, Migration
   1.15.749). Eine Gruppe mit festgehaltenem Beitrag rechnet mit dessen Werten,
   auch beim Bestätigen einer weiteren Anmeldung, beim Zusammenführen und beim
   Trennen: Eine spätere Umstellung ändert keinen festgehaltenen Beitrag.
   Vorschau und Bestätigung nehmen dieselben Werte. Nur wenn Anmeldungen mit
   verschiedenen Werten zusammenkommen, gelten für die Gruppe die aktuellen;
   jede Anmeldung, deren Werte oder Beträge sich dadurch ändern, bekommt eine
   neue Fassung.

Offen bleibt die Entscheidung der Gemeinde vom 11.11.2026. Sie ändert nur, was
Bissendorf einstellt, nicht diesen Code.
