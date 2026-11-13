---
status: accepted
---

# Ein Query-Budget fällt nur mit seiner Route weg

Entscheidung vom 07.11.2026 zu
[#5972](https://github.com/moto-ogs/project-phoenix/issues/5972), Teil von
[#4611](https://github.com/moto-ogs/project-phoenix/issues/4611). Sie ist die
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

1. **HTTP-Budgets nennen die gemessene Route ausdrücklich.** `route` ist
   `METHOD /muster`, genau wie im Router-Golden `backend/api/testdata/routes/`.
   Die Budget-Prüfung erhält das nach dem Routing vollständige chi-Muster
   des gemessenen Requests, einschließlich verschachtelter Mount-Präfixe und
   Parameter. Mehrere gemessene Requests erhalten eine sortierte, deduplizierte
   Liste im selben Feld; Szenario und gesamter SQL-Messumfang bleiben gleich.
   Jede einzelne Route muss im Golden stehen. Innenliegende `/*/`-Artefakte
   von `chi.Walk` werden für den Vergleich normalisiert; echte abschließende
   Wildcards und Blatt-Slashes bleiben erhalten. Beschreibung, Szenarioname
   und konkrete URL sind keine Quelle.
   Diese Laufzeitpflicht gilt unabhängig vom Szenariopräfix. Jede Assertion
   erhält einen vollständigen Counter-Snapshot statt eines nackten SQL-Slices;
   gemessene HTTP-Requests ohne aufgelöstes Muster sind ebenfalls Fehler.
   Eine normale Prüfung vergleicht gespeicherte und gemessene Route und
   überschreibt nichts. Die initiale Erfassung ist ein ausdrücklicher Lauf
   nach der Anleitung in `backend/test/query-budgets/README.md` (#6057,
   Option 1). Szenario, `max`, `exact` und SQL-Messumfang bleiben unverändert.
2. **Ein Budget fällt nur mit seiner eigenen Route weg.** Ein Szenario des
   Base-Registers darf fehlen, wenn dieselbe Änderung in
   `backend/architecture/query-budget-retirements.json` einen Eintrag
   `{scenario, route, issue}` hinzufügt. Dessen Route muss dem expliziten
   `route`-Feld des Budgets am unveränderlichen Git-Base-Commit entsprechen
   und aus dem Router-Golden des Kandidaten verschwunden sein.
3. **Der Checker prüft das selbst, nicht das Review.** Er lehnt einen neuen
   Stilllegungseintrag ab, wenn sein Szenario an der Basis fehlt, im Kandidaten
   weiterlebt, eine fremde oder erfundene Route nennt oder seine eigene Route
   noch im Kandidaten-Golden steht. Ungebundene historische Base-Budgets lassen
   sich nicht mit einer im selben Diff erfundenen Zuordnung stilllegen.
4. **Eine umbenannte Route verwaist ihr Budget.** Der Checker verlangt für
   jedes API-Budget eine nicht leere Route im Kandidaten-Golden. Verschwindet
   etwa `GET /api/students/`, bleibt das Budget rot, bis es die neue Route
   ausdrücklich nennt und der Test diese Route misst oder es nach Punkt 2
   stillgelegt ist. Eine noch existierende, aber für den gemessenen Request
   falsche Route scheitert zusätzlich am Laufzeitvergleich.
5. **Nur Nicht-HTTP-Budgets sind ausdrücklich ausgenommen.** Reine Use-Case-
   Budgets bleiben erlaubt. Routelose API-Szenarien brauchen je Szenario eine
   Begründung in `backend/architecture/query-budget-non-http.json`. Nach der
   initialen Einführung darf dieses Register nur schrumpfen: Der Checker
   vergleicht Szenario und Begründung mit dem unveränderlichen Base-Stand,
   nicht nur die Anzahl. Ein gleich großer Austausch, eine neue Ausnahme,
   eine umgeschriebene Begründung und das Löschen des Registers sind rot.
   Eine HTTP-Messung mit solcher Ausnahme ist ebenfalls rot. Nicht-HTTP-
   Budgets können nicht mit einem Routen-Stilllegungsnachweis entfallen.
6. **Die Goldens schlagen fehl, statt still zu bestehen.** Der Checker liest
   das Router-Golden der Basis und des Kandidaten bei jedem Lauf. Rot ist:
   eine leere Golden-Datei, eine Zeile, die keine Route ist, eine Golden-Datei
   der Basis, die im Kandidaten fehlt, ein fehlendes Golden-Verzeichnis und
   ein Lauf, der keine einzige Route gelesen hat.
7. **Das Router-Golden ist der Beleg.** `TestFullProductionRouterGolden`
   vergleicht es mit dem vollständigen Produktions-Router. Eine Route lässt
   sich deshalb nicht aus dem Golden streichen, solange der Router sie noch
   anbietet.
8. **Ein Eintrag gilt nur in der Änderung, die ihn hinzufügt.** Ein Eintrag,
   den die Basis schon enthält, ist Geschichte und erlaubt nichts. Ein
   Szenario, das später neu entsteht und wieder wegfallen soll, braucht einen
   neuen Eintrag mit einer Route, die erneut verschwindet.
9. **Strenge Form.** Der Eintrag ist ein JSON-Array ohne unbekannte Felder.
   Jeder Eintrag nennt Szenario, Route als `METHOD /pfad` genau wie im Golden
   und ein Issue. Ein Szenario darf nur einmal vorkommen. Fehlerhafte Einträge
   schlagen fehl.
10. **Was bleibt.** Ein Budget einer bestehenden Route sinkt weiter nur durch
   ein niedrigeres `max`. Höher werden, `exact` verlieren und Umbenennen des
   Szenarios bleiben verboten. Weitere Stilllegungen brauchen keine neue ADR,
   nur diesen Weg.

Erste Anwendung: `api.traeger.schools_today` fiel mit
`GET /traeger/schools/today` weg (#5972). Damals band die Beschreibung das
Budget; seit #6057 ersetzt das explizite Feld diese Ableitung.

## Folgen

- Toter Code mit Budget lässt sich entfernen, ohne das Register zu schwächen.
- Alle API-Budgets sind entweder an ihre tatsächlich gemessene Route gebunden
  oder ausdrücklich als Nicht-HTTP-Szenario begründet. Schwer migrierbare
  HTTP-Tests rechtfertigen keine Ausnahme.
- Die initiale Erfassung und spätere normale Testläufe sind getrennt. Auch das
  Backfill-Skript verweigert das Überschreiben einer bestehenden anderen Route.
- Die günstigen Fixtures prüfen Rename, Mount-Präfixe, Beschreibungen ohne
  Route, Nicht-HTTP-Ausnahmen und die Schutzfälle der Stilllegungsprüfung.
