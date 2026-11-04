<!-- Kurz und prüfbar: meist 150–300 Wörter, bei komplexen Änderungen mehr.
     Dauerhafte Design-Erklärungen in verlinkte Repo-Dokumente, Prüflogs per Link.
     Die Projektregeln und erforderlichen Checks gelten weiterhin. -->

## TL;DR: Was ändert sich für Nutzer?
<!-- 1–3 einfache Sätze: Wer merkt was, wo im Produkt? Ohne Datei-/Klassennamen.
     Ohne sichtbare Wirkung: „Keine Änderung für Nutzer“ plus kurzer Grund. -->

## Warum und Umsetzung
<!-- Problem, gewählter Ansatz und nicht offensichtliche Entscheidungen in wenigen
     Sätzen oder Stichpunkten. Relevante Issues hier verlinken, z. B. Fixes #123.
     Den Diff erklären, nicht nacherzählen. -->

## Prüfung
<!-- Tatsächlich ausgeführte Checks mit Ergebnis; bei Fehlerbehebungen den
     Vorher-/Nachher-Nachweis nennen. Fehlende oder blockierte Prüfungen offen
     ausweisen. CI verlinken statt Logs oder dieselben Ergebnisse zu wiederholen. -->

<!-- Nur zutreffende Abschnitte aus diesem Block ergänzen, sonst weglassen.

## Backend-Architektur
Bei Backend-Änderungen: Owner/Capability und betroffene Codepfade mit passenden
Prinzip-ADRs und Policy-Einträgen begründen. Relevante Ports, Cross-Owner-Zugriffe,
Transaktionen und Composition erklären. Policy-/Baseline-/Inventar-Änderungen
und Vorher-/Nachher-Zahlen berührter Shrink-only-Flächen nennen; Abweichungen
mit ADR belegen, offene Architekturfragen benennen. Architekturchecks unter
„Prüfung“ dokumentieren. Details dürfen verlinkt sein; grüne Checks allein
ersetzen keine Begründung. Maßgeblich: backend/CLAUDE.md, „Target architecture“.

## Risiken und Betrieb
Konkrete Restrisiken, Sicherheitsfolgen, Breaking Changes, Migrationen oder
Deploy-/Rollback-Schritte, die Reviewer kennen müssen.

## Screenshots
Pflicht bei jeder Änderung, die Nutzer sehen können. Aktuelle Screenshots
einbetten oder die Screenshot-Serie im PR verlinken. Ändern spätere Fixes deren
Inhalt, betroffene Aufnahmen und Vorher-/Nachher-Paare erneuern; PR-Belege und
Links aktuell halten. Details: docs/agents/operations.md, „PR screenshots and QA evidence“.

## Missverständnis-Check
Bei user-sichtbaren Änderungen gemäß .claude/rules/verstaendlichkeit.md:
geprüfte Blöcke und Ergebnis am laufenden Produkt; Hilfe-Änderung verlinken,
wenn .claude/rules/help-guide-sync.md zutrifft.
-->
