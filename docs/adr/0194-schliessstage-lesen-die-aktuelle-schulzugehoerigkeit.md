---
status: accepted
---

# Schließtage lesen die aktuelle Schulzugehörigkeit

Entscheidung vom 2026-11-10 für #4702 und PR #6418.

Ein Schulwechsel zum anderen Träger ändert `platform.schools.organization_id`.
Gespeicherte `school_ids` eines Träger-Schließtags allein berechtigen die Schule
anschließend nicht mehr zum Lesen. Die frühere Zuordnung darf weder Kalender
noch Soll oder Ferienbuchungen der Schule beeinflussen.

## Entscheidung

Die Leseprojektion `school-closing-days-view` verbindet School Calendar mit
Organisation & Tenancy. Sie besitzt keine Daten und schreibt nichts. Ihre zwei
Lesungen liefern Schul-Schließtage einschließlich aktuell zugehöriger
Träger-Schließtage sowie die Bereiche des Betreuungskalenders. Jede Anweisung
filtert auf die Schule der bestehenden Schul-Transaktion und prüft beim
Träger-Schließtag zusätzlich die aktuelle, nicht gelöschte Schulzugehörigkeit.
Das gilt auch unterhalb von RLS. Die Schul-Lesepolicy prüft dieselbe Beziehung.

School Calendar bindet die Projektion über seinen Port `ClosingDayReads`.
Die Root verdrahtet ihn; Verträge enthalten keine SQL- oder ORM-Typen.
Die Projektion liest ausschließlich `platform.schools`,
`schedule.closing_days`, `schedule.traeger_closing_days` und
`schedule.calendar_periods`. Die Schreib-Owner bleiben unverändert.

Die Zugehörigkeitsprüfung bleibt in derselben SQL-Anweisung. Ein separater
Port zur Schulzugehörigkeit würde pro Kalenderlesung eine weitere Abfrage
benötigen und die bestehenden Query-Budgets überschreiten. Die gespeicherten
Schließtage bleiben beim bisherigen Träger; ein Schulwechsel löscht sie nicht.

Die Ferienwochen prüfen vor dem Wechsel in eine Schul-Transaktion alle
angegebenen Schulen über Organisation & Tenancy. Eine Ablehnung enthält keine
Kalenderdaten einer fremden Schule.

## Nachweis

Die Regressionstests prüfen Erstellen und Ändern einer Ferienwoche mit fremder
Schule sowie den Schulwechsel mit RLS und unterhalb von RLS. Die bestehenden
Query-Budget-Tests für Statistik und Träger-Auswertungen bleiben unverändert.
Die Abgrenzung folgt ADR 0066, Punkt 6, und ADR 0068.
