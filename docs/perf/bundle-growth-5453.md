# Bundle-Zuwachs seit dem 28.10.2026 (#5453)

## Gesicherter Zwischenstand

Die 17 historischen Überschreitungen aus #5453 sind kein eigenes Delta von
#5445. Dessen kalter Linux-Zwei-Baum-Vergleich in
[Run 41256428322](https://github.com/moto-nrw/project-phoenix/actions/runs/41256428322)
misst auf den 16 App-Routen jeweils +18 B und auf der Hilfe 0 B.

#5408 entfernt den gemeinsamen Laufzeitimport der vollständigen Icon-Registry.
[Run 41254911890](https://github.com/moto-nrw/project-phoenix/actions/runs/41254911890)
misst danach beispielsweise 643841 B auf `/[tenant]`, 763236 B auf
`/parents/login` und 731012 B auf `/help/[[...topic]]`.
Damit ist der historische App-Zuwachs bereits mehr als ausgeglichen; die Hilfe
muss getrennt untersucht werden. Die Budgetdateien bleiben zunächst unverändert.

## Messverfahren

Maßgeblich sind kalte Linux-Messungen mit dem Parser aus
`frontend/scripts/perf/bundle-routes.mjs`. Pro Route zählen komprimierte
Client-JS-Bytes einschließlich ihrer gemeinsamen Chunks. Werte verschiedener
Routen dürfen nicht zu einer vermeintlichen Gesamtgröße summiert werden.

Historische CI-Artefakte werden anhand ihrer Quellrevision geprüft. Weitere
Builds laufen einzeln über `~/bin/agent-heavy -- …`. macOS-Werte sind nur lokale
Diagnose, keine Grundlage für Linux-Budgets. Ein gerechtfertigtes neues Budget
verwendet `Math.round(referenceBytes * 1.02)`; geerbter Überlauf bleibt Warnung,
die Delta-Toleranz unverändert.
