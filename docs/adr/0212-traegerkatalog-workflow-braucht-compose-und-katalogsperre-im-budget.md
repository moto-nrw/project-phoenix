---
status: proposed
---

# Trägerkatalog-Workflow braucht Compose und Katalogsperre im Budget

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
   `backend/test/module_legacy_budget_ratchet_test.go`. Gemessen: 34 LOC.
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

- `moduleComposeBudgets["workflows/traegercatalog/compose"] = 34`,
  `moduleComposeBudgetTotal` + 34.
- `crossOwnerTriggerWrites` bekommt die zwei Schreibstellen von
  `release_traeger_copies`.
- Die Command-Effects der Kopie-Befehle von Care Plan und Enrollment auf
  `timetable.RecurrenceWriteLock.LockRecurrenceWrites` stehen als `debt`/`lock`
  mit `approval_adr` 0212, ihre unaufgelösten Felder als `unresolved` mit
  `approval_adr` 0212. Die Sperrreihenfolge bleibt die der Schule
  (ADR 0195 Abschnitt 5).

Weitere Erhöhungen, auch für spätere Teilnehmer des Workflows, brauchen
einen eigenen Beleg.

## Verworfene Alternativen

- **Ohne Compose-Paket, Root bindet die Transaktionen selbst:** widerspricht
  ADR 0068; die Root hielte den Transaktionsläufer des Workflows.
- **Kopie schreibt am Katalog vorbei direkt in `enrollment.care_offerings`:**
  umginge die Prüfungen der Schule, die ADR 0195 Abschnitt 3 verlangt.
