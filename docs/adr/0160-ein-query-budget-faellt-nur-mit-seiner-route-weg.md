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

1. **Jedes Budget hat seine eigene Route.** Die Route eines Budgets ist jede
   Route `METHOD /pfad`, die seine Beschreibung nennt und die das Router-Golden
   `backend/api/testdata/routes/` listet. Mandantenrouten stehen im Golden mit
   dem Präfix `/api`, in Beschreibungen oft ohne; beide Schreibweisen binden.
   Ein Budget, dessen Beschreibung keine Route des Goldens nennt, ist nicht
   gebunden.
2. **Ein Budget fällt nur mit seiner eigenen Route weg.** Ein Szenario des
   Base-Registers darf fehlen, wenn dieselbe Änderung in
   `backend/architecture/query-budget-retirements.json` einen Eintrag
   `{scenario, route, issue}` hinzufügt, die Route des Eintrags eine Route
   ist, an die die Beschreibung des Budgets **an der Basis** es bindet, und
   keine dieser Routen mehr im Router-Golden des Kandidaten steht.
3. **Der Checker prüft das selbst, nicht das Review.** Ein neuer Eintrag wird
   abgelehnt, wenn
   - sein Szenario im Base-Register fehlt (Leerlauf),
   - sein Szenario im Kandidaten noch registriert ist (Leerlauf),
   - seine Route nicht die eigene Route des Budgets ist (fremde oder
     erfundene Route; ein ungebundenes Budget lässt sich so nicht stilllegen),
   - eine eigene Route des Budgets im Router-Golden des Kandidaten noch steht.
4. **Eine umbenannte Route verwaist ihr Budget.** Verschwinden alle eigenen
   Routen eines bleibenden Budgets aus dem Golden, etwa weil die Route
   umbenannt wurde, ist das Budget verwaist und der Check rot. Er bleibt rot,
   bis die Beschreibung des Budgets die neue Route nennt (das Budget ist dann
   umbenannt) oder das Budget auf diesem Weg stillgelegt ist.
5. **Die Goldens schlagen fehl, statt still zu bestehen.** Der Checker liest
   das Router-Golden der Basis und des Kandidaten bei jedem Lauf. Rot ist:
   eine leere Golden-Datei, eine Zeile, die keine Route ist, eine Golden-Datei
   der Basis, die im Kandidaten fehlt, ein fehlendes Golden-Verzeichnis und
   ein Lauf, der keine einzige Route gelesen hat.
6. **Das Router-Golden ist der Beleg.** `TestFullProductionRouterGolden`
   vergleicht es mit dem vollständigen Produktions-Router. Eine Route lässt
   sich deshalb nicht aus dem Golden streichen, solange der Router sie noch
   anbietet.
7. **Ein Eintrag gilt nur in der Änderung, die ihn hinzufügt.** Ein Eintrag,
   den die Basis schon enthält, ist Geschichte und erlaubt nichts. Ein
   Szenario, das später neu entsteht und wieder wegfallen soll, braucht einen
   neuen Eintrag mit einer Route, die erneut verschwindet.
8. **Strenge Form.** Der Eintrag ist ein JSON-Array ohne unbekannte Felder.
   Jeder Eintrag nennt Szenario, Route als `METHOD /pfad` genau wie im Golden
   und ein Issue. Ein Szenario darf nur einmal vorkommen. Fehlerhafte Einträge
   schlagen fehl.
9. **Was bleibt.** Ein Budget einer bestehenden Route sinkt weiter nur durch
   ein niedrigeres `max`. Höher werden, `exact` verlieren und Umbenennen des
   Szenarios bleiben verboten. Weitere Stilllegungen brauchen keine neue ADR,
   nur diesen Weg.

Erste Anwendung: `api.traeger.schools_today` fällt mit
`GET /traeger/schools/today` weg (#5972). Seine Beschreibung bindet es an
genau diese Route.

## Folgen

- Toter Code mit Budget lässt sich entfernen, ohne das Register zu schwächen.
- Budgets ohne eigene Route lassen sich auf diesem Weg nicht stilllegen, und
  bei ihnen erkennt der Checker keine verwaiste Route. Am 07.11.2026 nennen
  105 von 632 Beschreibungen eine Route des Goldens. Repository-, Workflow-
  und Service-Szenarien haben keine Route; für die übrigen API-Szenarien
  bindet erst eine Beschreibung mit `METHOD /pfad`.
- Die Tests in `backend/internal/architecture/query_budgets_test.go`
  (`TestCompareQueryBudgetsRetiresABudgetOnlyWithItsRoute`) decken den
  erlaubten Fall und jede Ablehnung ab, einschließlich fremder Route, leerer
  oder fehlender Goldens, eines Laufs ohne Routen und einer umbenannten Route.
