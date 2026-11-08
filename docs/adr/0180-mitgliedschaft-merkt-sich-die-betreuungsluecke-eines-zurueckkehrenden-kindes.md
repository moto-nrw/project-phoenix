---
status: accepted
---

# Mitgliedschaft merkt sich die Betreuungslücke eines zurückkehrenden Kindes

Entscheidung vom 08.11.2026 für
[#6073](https://github.com/moto-nrw/project-phoenix/issues/6073). Sie ergänzt
[ADR 0166](0166-mitgliedschaft-merkt-sich-den-fruehesten-betreuungstag.md)
um zwei Spalten von School Membership, reicht sie über die vorhandene
Schülerprojektion an Care Plan weiter und genehmigt die geänderte Bindung
`roots/owners.CareStudents.FindCareStudents`. Kein neuer Owner, kein neues
Datenobjekt, keine Policy-Regel, kein neuer Port.

## Kontext

Plant die Schule für ein Kind mit beendeter Betreuung einen späteren
Neustart (Wiederaufnahme, Wiederanmeldung eines bekannten Kindes), ersetzt
School Membership das Intervall: neuer Beginn, das Ende geleert oder neu
gesetzt. Das frühere Ende stand danach nirgends dauerhaft. Care Plan hält ein
vorgemerktes Kind ohne weiteres Wissen nur ab heute zurück
(`AwaitsCareStartOn`). Beispiel: Ende 31.10., Wiederaufnahme am 07.11. für
Februar. In der Fehlzeitenliste erschienen der 01.11. bis 06.11. wieder, als
„nicht erfasst“ oder mit Fehlzeiten, die nach dem Ende eingetragen wurden.
Jeder weitere Tag bis Februar wäre nach seinem Ablauf ebenso zurückgekehrt.

## Entscheidung

- `users.student_school_memberships.care_paused_after` und
  `care_resumed_from` (School Membership, Migration 1.15.825): der letzte
  Betreuungstag vor dem Neustart und der früheste Tag, an dem das Kind wieder
  betreut worden sein kann. Die Tage echt dazwischen sind keine
  Betreuungstage. Beide `NULL` heißt: keine Lücke bekannt.
- Ein Neustart ist ein Schreibvorgang von `ResumeStudentCare` oder
  `RenewStudentEnrollment`, bei dem das Ende der Zeile vor dem Tag der
  Anwendung liegt und das neue Intervall bis heute oder später reicht (kein
  Ende oder ein Ende ab heute). Er setzt `care_paused_after` auf das bisherige
  Ende. `care_resumed_from` wird der neue Beginn, wenn er nach dem bisherigen
  Ende liegt (bei einer sofortigen Aktivierung `LEAST(Beginn, heute)`), sonst
  heute: Eine Genehmigung, die ein beendetes Kind mit seinem früheren Beginn
  verlängert (Enrollment, `renewExistingStudent`), nimmt die Betreuung mit
  diesem Schreibvorgang wieder auf. Ein Beginn direkt nach dem Ende
  hinterlässt keinen Tag in der Lücke. Ein Ende, das noch nicht erreicht ist,
  oder ein Ende, das auf einen anderen vergangenen Tag korrigiert wird, ist
  kein Neustart.
- Jeder andere Schreibvorgang, der Status oder Beginn bewegt, behält
  `care_paused_after` und führt `care_resumed_from` nach den Regeln von
  `first_care_from` (ADR 0166) nach: Ein erreichter Tag rückt nur noch früher,
  ein Tag in der Zukunft folgt dem Beginn, eine Aktivierung vor dem Beginn
  setzt ihn auf heute. Speichern ohne Neustart ändert die Lücke nicht.
- Kein Nachlauf: Das frühere Ende vergangener Neustarts ist nirgends
  verlässlich gespeichert (die Care-Exit-Zeile ist gelöscht,
  `audit.student_field_edits` hat eine Aufbewahrungsfrist). Bestehende
  Zeilen behalten das bisherige Verhalten. Down entfernt beide Spalten.
- Die Schülerprojektion (`studentdirectoryview`, People Directory) liest beide
  Spalten in ihrem bestehenden Statement mit; sie gehen nicht über die API
  hinaus. `CareStudents.FindCareStudents` überträgt sie auf
  `careplan.CareStudent`.
- Care Plan: `AwaitsCareStartOn` gilt auch für einen Tag in der Lücke, in
  jedem Status. Damit gelten alle Leser, die schon auf einen Betreuungsbeginn
  warten: laufende Listen, Fehlzeitenliste und Export, administrative Liste.
  Belegte Betreuung bleibt: Ein Tag in der Lücke mit gespeicherter Anwesenheit
  oder einem Raumbesuch zählt über `PresenceDaysBetween` wie bei einem neuen
  Kind (ADR 0166).
- Ein Datum je Kind reicht: Gespeichert wird nur die letzte Lücke. Ein
  weiterer Neustart ersetzt sie; die Tage einer früheren Lücke zählen dann
  wieder wie vor diesem ADR. Zwei Neustarts innerhalb eines Berichtszeitraums
  sind selten; eine Historie aller Lücken bräuchte ein eigenes Datenobjekt.
- Kein Query-Budget ändert sich: Die Spalten kommen in den bestehenden
  Statements mit.

## Genehmigte Diagnose

Die Bindung `roots/owners.CareStudents.FindCareStudents` mappt zwei Felder
mehr. Ihre Ziele `peopledirectory/ports.StudentRepository.FindByIDs` (query)
und `FindByIDsForUpdate` (E2 nach ADR 0067, Lock-Reihenfolge und Transaktion
unverändert) behalten Effekt und Einstufung; nur der Digest ändert sich. Die
E2-Diagnose nennt dieses ADR als `approval_adr`. Sobald dieses ADR in der
Basis liegt, genehmigt es keine weitere Diagnose.

## Folgen

- Wird eine Lücke nach einer weiteren Wiederaufnahme ersetzt, zählen die
  Tage der früheren Lücke wieder als Betreuungstage.
- Neustarts vor diesem ADR bleiben ohne Lücke.
- Verlängert die Schule das vergangene Ende eines Kindes, das in Wahrheit
  durchgehend betreut wurde (Korrektur statt Neustart), zählen die Tage
  zwischen altem Ende und Korrektur nur mit gespeicherter Anwesenheit. Die
  Schule korrigiert das, indem sie den Beginn vor das alte Ende legt; dann
  ist die Lücke leer.
