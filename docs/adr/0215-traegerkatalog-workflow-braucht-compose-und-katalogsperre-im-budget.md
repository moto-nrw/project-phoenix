---
status: accepted
---

# Trägerkatalog-Workflow braucht Compose und Katalogsperre im Budget

**Status: Akzeptiert.** Freigegeben von Yannick Wenger am 11.11.2026.

Vorgeschlagen am 11.11.2026 zu
[#4704](https://github.com/moto-nrw/project-phoenix/issues/4704). Beleg nach
[ADR 0195](0195-traegerphase-und-traegerangebot-werden-je-schule-kopiert-und-erst-nach-freigabe-abgeglichen.md),
Abschnitt „Policy und Register“: ADR 0195 legt den Workflow-Owner
`traeger-catalog-rollout` mit dem Paket `workflows/traegercatalog/compose`
fest und lässt die Kopie eines Trägerangebots über die Befehle des
Angebotskatalogs der Schule schreiben. Diese ADR hält die zwei Register fest,
die das berührt, und gilt nur für die unten gemessenen Stellen, wie
[ADR 0209](0209-holiday-allocation-workflow-braucht-compose-und-sperrbefehl-im-budget.md)
für den Ferienplatz-Workflow.

## Kontext

1. `workflows/traegercatalog/compose` baut den Workflow und bindet die
   Träger-Transaktion und die Tenant-Transaktion je Schule (ADR 0068). Jedes
   Compose-Paket hat einen Eintrag im Compose-LOC-Register von
   `backend/test/module_legacy_budget_ratchet_test.go`. Gemessen nach der Absicherung überlappender Läufe: 38 LOC.
2. `TraegerOfferingCopies.ApplyTraegerOffering` und
   `WithdrawTraegerOffering` (Care Plan) schreiben die Kopie über
   `CreateOffering`, `UpdateOffering` und `DeleteOffering` des Katalogs. Der
   Katalog nimmt dabei die Wiederholungssperre des Stundenplans und liest
   Uhr und Sperre über Felder seiner Abhängigkeiten, genau wie bei einer
   Änderung der Schule. Mit der Bindung in der Root werden diese Wege für die
   Kopie-Befehle sichtbar: je eine Sperre (`LockRecurrenceWrites`) und je
   drei unaufgelöste Felder (`LockTemplateRecurrence`, `Today`, `api.now`).
   Die Kopie-Befehle selbst nehmen keine weitere Sperre.

3. Ein Trigger auf `platform.schools` löst beim Trägerwechsel die Kopien
   (ADR 0195 Abschnitt 8). Er schreibt `enrollment.phases` (Enrollment) und
   `enrollment.care_offerings` (Care Plan) aus einer Tabelle von
   Organisation & Tenancy; `crossOwnerTriggerWrites` in
   `backend/test/trigger_ownership_test.go` nennt jede solche Schreibstelle.
4. Die Kopie einer Trägerphase (`TraegerPhaseCopies`, Enrollment) nimmt die
   Wiederholungssperre vor der Zeilensperre ihrer Kopie und schreibt über
   die Phasenverwaltung der Schule; dieselben Wege wie in Punkt 2.

## Entscheidung

- `moduleComposeBudgets["workflows/traegercatalog/compose"]`: 0 → 38. Freigegeben
  waren 34; die weiteren 4 LOC binden `WithinExclusive`, die
  transaktionsfreie Sperre, mit der ein Übertragen je Trägerphase nur einmal
  zugleich läuft (Befund aus dem Review: zwei gleichzeitige Freigaben
  derselben Phase).
- `moduleComposeBudgets["modules/enrollment/compose"]`: 613 → 615 (+2), aus
  diesem Diff: der Konstruktor `NewTraegerPhases` (+4) und
  `NewOfferingSelectionReferences` (+4, Schutz vor dem Löschen eines noch
  gewählten Angebots beim Zurückziehen, Befund aus dem Review), abzüglich
  gekürzter Kommentare und einer zusammengelegten Datei (−6).
- `modules/careplan/compose` bleibt bei 1675: die Konstruktoren der
  Trägerangebote passen in gekürzte Kommentare.
- `moduleComposeBudgetTotal`: 13320 → 13360 (+40).
- `crossOwnerTriggerWrites` bekommt die zwei Schreibstellen von
  `release_traeger_copies`.
- Die Command-Effects der Kopie-Befehle einschließlich `ApplyTraegerOfferings`
  von Care Plan und Enrollment auf
  `timetable.RecurrenceWriteLock.LockRecurrenceWrites` stehen als `debt`/`lock`
  mit `approval_adr` 0215, ihre unaufgelösten Felder als `unresolved` mit
  `approval_adr` 0215. Die Sperrreihenfolge bleibt die der Schule
  (ADR 0195 Abschnitt 5).

Die Erweiterung der Katalogfähigkeit ändert auch die berechneten
Bindungsdigests bestehender Schüler- und Wiederholungssperren. Die exakten
Einträge werden mit neu geprüftem Grund eingefroren; Ziel, Sperrreihenfolge
und Uhrbindung bleiben gleich. Die Stapel-Ports zur
Roster- und Gehzeitpflege nutzen dieselben Transaktions-, Schüler- und
Uhrbindungen wie die vorhandenen Einzelbefehle; die unaufgelösten
Gehzeit-Rückrufe bleiben ausdrücklich als solche vermerkt.

Weitere Erhöhungen, auch für spätere Teilnehmer des Workflows, brauchen
einen eigenen Beleg.

## Verworfene Alternativen

- **Ohne Compose-Paket, Root bindet die Transaktionen selbst:** widerspricht
  ADR 0068; die Root hielte den Transaktionsläufer des Workflows.
- **Kopie schreibt am Katalog vorbei direkt in `enrollment.care_offerings`:**
  umginge die Prüfungen der Schule, die ADR 0195 Abschnitt 3 verlangt.
