---
status: accepted
---

# Mitgliedschaft merkt sich, ob ein Kind noch nie betreut wurde

Entscheidung vom 07.11.2026 für
[#6041](https://github.com/moto-nrw/project-phoenix/issues/6041) (zweiter
Review-Befund in [#6065](https://github.com/moto-nrw/project-phoenix/pull/6065)).
Sie fügt School Membership eine Spalte hinzu, reicht sie über die vorhandene
Schülerprojektion an Care Plan weiter und genehmigt die geänderte Bindung
`roots/owners.CareStudents.FindCareStudents`. Kein neuer Owner, kein neues
Datenobjekt, keine Policy-Regel, kein neuer Port.

## Kontext

Care Plan hält ein vorgemerktes Kind (`pending`) vor seinem Betreuungsbeginn
aus der Teilnahme heraus (`AwaitsCareStartOn`,
[ADR 0120, Nachtrag #6041](0120-feriengast-ist-ein-befristetes-kind-der-ausrichtenden-ogs.md#vergangene-tage-neues-und-zurückkehrendes-kind-6041-review)).
Eine Wiederaufnahme oder Wiederanmeldung speichert ein bekanntes Kind wie ein
neues. Damit die echte frühere Betreuung nicht aus der Fehlzeitenliste fällt,
galt die Regel nur ab heute. Ein im Dezember für Februar bestätigtes Kind
stand dadurch in der Fehlzeitenliste des laufenden Halbjahres, und die
Antwort für einen vergangenen Tag hing vom Lesetag ab.

Kein vorhandenes Datum trennt beide Fälle verlässlich: Den Aktivierungsplan
stempelt auch die Wiederanmeldung, `created_at` deckt die Tage zwischen
Anlage und heute nicht ab, fehlende Anwesenheitsdaten beweisen keine
fehlende Betreuung.

## Entscheidung

- `users.student_school_memberships.awaits_first_care` (School Membership,
  Migration 1.15.812). Der Tabellen-Trigger
  `trg_student_school_memberships_first_care` setzt die Spalte beim Einfügen
  einer `pending`-Mitgliedschaft mit Beginn, wenn das Kind an der Schule keine
  andere (auch gelöschte) Mitgliedschaft hat. Spätere Schreibvorgänge können
  sie nur behalten (Zeile bleibt `pending` mit Beginn, und der Beginn rückt
  früher oder liegt noch in der Zukunft) oder für immer löschen. Keine
  Anwendung schreibt sie.
- Der Nachlauf markiert nur seit dem Einfügen nachweislich unveränderte
  Zeilen (`pending` mit Beginn, `updated_at = created_at`, keine weitere
  Mitgliedschaft). Alle anderen bleiben unbekannt und behalten die
  Heute-Grenze.
- Die Schülerprojektion (`studentdirectoryview`, People Directory) liest die
  Spalte in ihrem bestehenden Statement mit; `StudentState.AwaitsFirstCare`
  geht nicht über die API hinaus. `CareStudents.FindCareStudents` überträgt
  sie auf `careplan.CareStudent`. Care Plan hält ein solches Kind auch an
  vergangenen Tagen vor dem Beginn zurück.
- Kein zusätzliches Statement: Kein Query-Budget ändert sich.

## Genehmigte Diagnose

Die Bindung `roots/owners.CareStudents.FindCareStudents` mappt ein Feld mehr.
Ihre Ziele `peopledirectory/ports.StudentRepository.FindByIDs` (query) und
`FindByIDsForUpdate` (E2 nach ADR 0067, Lock-Reihenfolge und Transaktion
unverändert) behalten Effekt und Einstufung; nur der Digest ändert sich.
Beide Diagnosen nennen dieses ADR als `approval_adr`. Sobald dieses ADR in der
Basis liegt, genehmigt es keine weitere Diagnose.

## Folgen

- Die Lücke zwischen früherem Ende und Wiederaufnahme sowie Tage vor dem
  Beginn nach der Aktivierung bleiben offen
  ([#6073](https://github.com/moto-nrw/project-phoenix/issues/6073)).
- Wird der Beginn eines neuen Kindes nach Erreichen verschoben, fällt es auf
  die Heute-Grenze zurück; das zeigt vergangene Tage wie vor diesem ADR.
