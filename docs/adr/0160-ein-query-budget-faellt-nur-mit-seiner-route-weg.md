---
status: accepted
---

# Ein Query-Budget fällt nur mit seiner Route weg

Entscheidung vom 07.11.2026 zu
[#5972](https://github.com/moto-nrw/project-phoenix/issues/5972), Teil von
[#4611](https://github.com/moto-nrw/project-phoenix/issues/4611). Sie ist die
eigene ADR, die [ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)
Punkt 4 und [ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md)
Abschnitt 1 für das Stilllegen einer Messung verlangen.

Freigegeben von Yannick Wenger am 07.11.2026 (Option A in #5972).

## Kontext

- Jedes Szenario in `backend/test/query-budgets/` ist ein geprüfter
  SQL-Vertrag. `CompareQueryBudgets`
  (`backend/internal/architecture/query_budgets.go`) liest das Register am
  unveränderlichen Git-Base-Commit und meldet ein fehlendes Szenario als
  `disappeared`. Bisher wurde nie ein Budget entfernt.
- Seit #5942 ruft niemand mehr `GET /traeger/schools/today` auf. Ohne Weg zum
  Stilllegen bliebe der tote Endpoint nur wegen seines Budgets bestehen.
- #5964 schlug ein Archiv `retired-query-budgets` vor. Ein solches Archiv ist
  an nichts gebunden: Jedes aktive Budget ließe sich dorthin verschieben,
  während seine Route weiterläuft. Das Register wäre dann nicht mehr
  shrink-only.

## Entscheidung

1. **Ein Budget fällt nur mit seiner Route weg.** Ein Szenario des
   Base-Registers darf fehlen, wenn dieselbe Änderung in
   `backend/architecture/query-budget-retirements.json` einen Eintrag
   `{scenario, route, issue}` hinzufügt und die genannte Route aus dem
   Router-Golden `backend/api/testdata/routes/` verschwindet.
2. **Der Checker prüft das selbst, nicht das Review.** Ein neuer Eintrag wird
   abgelehnt, wenn
   - sein Szenario im Base-Register fehlt (Leerlauf),
   - sein Szenario im Kandidaten noch registriert ist (Leerlauf),
   - seine Route im Router-Golden der Basis fehlt (erfundene Route),
   - seine Route im Router-Golden des Kandidaten noch steht.
3. **Das Router-Golden ist der Beleg.** `TestFullProductionRouterGolden`
   vergleicht es mit dem vollständigen Produktions-Router. Eine Route lässt
   sich deshalb nicht aus dem Golden streichen, solange der Router sie noch
   anbietet.
4. **Ein Eintrag gilt nur in der Änderung, die ihn hinzufügt.** Ein Eintrag,
   den die Basis schon enthält, ist Geschichte und erlaubt nichts. Ein
   Szenario, das später neu entsteht und wieder wegfallen soll, braucht einen
   neuen Eintrag mit einer Route, die erneut verschwindet.
5. **Strenge Form.** Der Eintrag ist ein JSON-Array ohne unbekannte Felder.
   Jeder Eintrag nennt Szenario, Route als `METHOD /pfad` genau wie im Golden
   und ein Issue. Ein Szenario darf nur einmal vorkommen. Fehlerhafte Einträge
   oder Golden-Zeilen schlagen fehl.
6. **Was bleibt.** Ein Budget einer bestehenden Route sinkt weiter nur durch
   ein niedrigeres `max`. Höher werden, `exact` verlieren und Umbenennen
   bleiben verboten. Weitere Stilllegungen brauchen keine neue ADR, nur diesen
   Weg.

Erste Anwendung: `api.traeger.schools_today` fällt mit
`GET /traeger/schools/today` weg (#5972).

## Folgen

- Toter Code mit Budget lässt sich entfernen, ohne das Register zu schwächen.
- Budgets ohne eigene Route (Repository-, Workflow- oder Service-Szenarien)
  lassen sich auf diesem Weg nicht stilllegen. Dafür bräuchte es eine eigene
  Entscheidung.
- Die Tests in `backend/internal/architecture/query_budgets_test.go`
  (`TestCompareQueryBudgetsRetiresABudgetOnlyWithItsRoute`) decken den
  erlaubten Fall und jede Ablehnung ab. Ohne die Routenprüfung wird der Fall
  „Route bleibt“ rot.
