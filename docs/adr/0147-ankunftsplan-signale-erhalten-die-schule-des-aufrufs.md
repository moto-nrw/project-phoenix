---
status: accepted
---

# Ankunftsplan-Signale erhalten die Schule des Aufrufs

Für [#5492](https://github.com/moto-nrw/project-phoenix/issues/5492) wird der
bisher parameterlose Ankunftsplan-Vertrag schulgebunden. ADR 0145 ließ ihn
absichtlich lokal; eine globale Verteilung über alle APIs wäre falsch.

## Entscheidung

Care Plans HTTP-Schreibwege übernehmen die School-ID aus dem Tenant-Kontext
und reichen sie an ihren bestehenden After-Commit-Hook weiter. Der
Genehmigungsweg reicht die School-ID aus dem Aufrufkontext über seinen
eigenen `ArrivalScheduleChanged(tenantID)`-Port an die Root-Bindung weiter.
Deliverys `QueueArrivalScheduleChanged` liest die School-ID vor dem Einreihen
aus dem Aufrufkontext und verwendet nur `BroadcastToTenant`. Ohne Schule
wird kein Signal gesendet. Die Root-Bindung der Morgenmeldung implementiert
denselben schulgebundenen Port, bildet aber weiterhin auf `student_updated`
ab; daraus leitet der Hub `class_day_changed` für das Schulportal ab.

Die Browser behalten `arrival_schedule_changed` als schulweiten,
kinddatenfreien Refetch-Auslöser. Typ und Cache-Invalidierung bleiben erhalten. Die lokalen Quellen bleiben
unverändert; das Relay lässt sie wie bisher weg. Der Browser-Handler
invalidiert anhand des Ereignistyps und benötigt die Quelle nicht. Der PostgreSQL-Kanal bleibt
verlustbehaftet, ohne neue Infrastruktur, DSN oder Outbox. Rollback verwirft
die Hooks, Verlust kostet weiterhin nur die sofortige Aktualisierung.

## Exakte Notice-Migration

Die neue Schuladressierung ändert 13 eingefrorene Ausdrücke: acht Hooks in
`arrival_schedule_handlers.go`, zwei in `class_arrival_exception_handlers.go`,
einen in `pickup_adjustment_handlers.go`, Care Requests
`broadcastCareScheduleChanges` und Deliverys `QueueArrivalScheduleChanged`.
Ihre restlichen Wirkungen bleiben unverändert: Guardian-Wakes und
Cache-Invalidierungen, keine dauerhaften Schreibaufträge.

`backend/test/arrival_school_notice_migration.go` benennt jeden vollständigen
Alt-/Neu-Schlüssel einschließlich Fingerprint. Der Vergleich normalisiert
nur diese exakten Alt-Einträge des gefetchten Basisregisters. Jeder wird
einmal verbraucht, der Verlustgrund stammt unverändert aus der Basis.
Zusätzliche Einträge, beide Fassungen zugleich, veränderte Gründe,
unbekannte Alt-Bodies und spätere Änderungen der neuen Bodies bleiben
verboten. `sideEffectRegisterSubset` selbst bleibt unverändert und erlaubt
weiterhin nur Schrumpfung oder unveränderte Relokationen.

Dies ist die ausdrücklich begrenzte Vertragsmigration gemäß ADR 0067
Punkt 6 und ADR 0070 Punkt 4, keine Relokation und keine Registererhöhung.
Die Zahl der Notices bleibt gleich. Positive und negative Fixtures prüfen
alle 13 Paare. Sobald die Basis diese Migration enthält, ist die
Normalisierung wirkungslos; sie kann nach dem Merge entfernt werden.
Typed-Effects- und Pass-through-Inventare müssen die neue schulgebundene
Bindung abbilden. Policy-Regeln, Owner, Datenobjekte und Transaktionen
ändern sich nicht.
