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
   (`backend/internal/architecture/policy.go`, ab Epoche 95), in ADR 0066 §1,
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
