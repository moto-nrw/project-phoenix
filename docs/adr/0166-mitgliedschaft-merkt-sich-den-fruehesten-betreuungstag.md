---
status: accepted
---

# Mitgliedschaft merkt sich den frühesten Betreuungstag eines neuen Kindes

Entscheidung vom 07.11.2026 für
[#6041](https://github.com/moto-nrw/project-phoenix/issues/6041) (Review-Befunde
zwei und drei in [#6065](https://github.com/moto-nrw/project-phoenix/pull/6065)).
Sie fügt School Membership eine Spalte hinzu, reicht sie über die vorhandene
Schülerprojektion an Care Plan weiter, gibt Care Plan über den vorhandenen
Port zu Student Presence die datierte Anwesenheit und genehmigt die geänderte
Bindung `roots/owners.CareStudents.FindCareStudents`. Kein neuer Owner, kein
neues Datenobjekt, keine Policy-Regel, kein neuer Port.

## Kontext

Care Plan hält ein vorgemerktes Kind (`pending`) vor seinem Betreuungsbeginn
aus der Teilnahme heraus (`AwaitsCareStartOn`,
[ADR 0120, Nachtrag #6041](0120-feriengast-ist-ein-befristetes-kind-der-ausrichtenden-ogs.md#vergangene-tage-neues-und-zurückkehrendes-kind-6041-review)).
Eine Wiederaufnahme oder Wiederanmeldung speichert ein bekanntes Kind wie ein
neues. Damit die echte frühere Betreuung nicht aus der Fehlzeitenliste fällt,
gilt die Regel ohne weiteres Wissen nur ab heute. Ein im Dezember für Februar
bestätigtes Kind stünde dadurch in der Fehlzeitenliste des laufenden
Halbjahres, und die Antwort für einen vergangenen Tag hinge vom Lesetag ab.

Kein vorhandenes Datum trennt beide Fälle verlässlich: Den Aktivierungsplan
stempelt auch die Wiederanmeldung, `created_at` deckt die Tage zwischen
Anlage und heute nicht ab, fehlende Anwesenheitsdaten beweisen keine
fehlende Betreuung.

Ein erster Ansatz (ein Merkmal „wartet auf die erste Betreuung“, von einem
Tabellen-Trigger gepflegt) hatte zwei Lücken: Die Aktivierung am Starttag
löschte das Merkmal, danach stand ein Februar-Kind wieder mit „nicht
erfasst“-Tagen im Januar und im laufenden Halbjahr. Und der Rückhalt an
vergangenen Tagen blendete auch Tage aus, an denen das Kind nachweislich da
war.

## Entscheidung

- `users.student_school_memberships.first_care_from` (School Membership,
  Migration 1.15.812): der früheste Tag, an dem das Kind an dieser Schule
  betreut worden sein kann; `NULL` heißt unbekannt.
- `EnrollStudent` setzt ihn auf den Beginn, wenn das Kind `pending` mit
  Beginn eingefügt wird und an der Schule keine andere (auch gelöschte)
  Mitgliedschaft hat, also bei einer neuen Genehmigung mit späterem Start.
  Sofort aktivierte Kinder und Kinder ohne Beginn bleiben unbekannt.
- Jeder Schreibvorgang, der Status oder Beginn bewegt (Statuswechsel,
  Wiederanmeldung und Speichern, Wiederaufnahme, Rückholung eines Abgangs),
  führt ihn in derselben Anweisung nach, am Tag der Anwendung (Uhr von School
  Membership, in Tests einfrierbar), nicht am Datum der Datenbank:
  - Unbekannt bleibt unbekannt; ein zurückkehrendes Kind behält seine
    vergangenen Tage.
  - Ein erreichter Tag kann Betreuung gesehen haben: Er rückt nur noch früher.
    Die Aktivierung am oder nach dem Beginn und eine Wiederaufnahme behalten
    ihn.
  - Ein Tag in der Zukunft hat keine Betreuung gesehen: Er folgt dem Beginn,
    bei einer Aktivierung vor dem Beginn oder ohne Beginn wird er heute.
- Der Nachlauf setzt ihn nur für seit dem Einfügen nachweislich unveränderte
  Zeilen (`pending` mit Beginn, `updated_at = created_at`, keine weitere
  Mitgliedschaft), ohne `updated_at` zu ändern. Alle anderen bleiben
  unbekannt. Down entfernt die Spalte; Care Plan fällt auf die Heute-Grenze
  zurück.
- Die Schülerprojektion (`studentdirectoryview`, People Directory) liest die
  Spalte in ihrem bestehenden Statement mit; `StudentState.FirstCareFrom`
  geht nicht über die API hinaus. `CareStudents.FindCareStudents` überträgt
  sie auf `careplan.CareStudent`.
- Care Plan: Ein Tag vor `first_care_from` ist keine Teilnahme, in jedem
  Status, also auch nach der Aktivierung. Ohne diesen Tag gilt weiter: ein
  `pending`-Kind vor seinem Beginn ab heute nicht.
- Belegte Betreuung bleibt: Die rückblickende Teilnahme
  (`HistoricalParticipatingStudentIDsByDate`, Fehlzeitenliste) zählt ein
  zurückgehaltenes Kind an einem Tag mit, an dem Student Presence eine
  Anwesenheit oder einen Raumbesuch des Kindes gespeichert hat
  (`PresenceDaysBetween` über den vorhandenen Port `CareExitPresence`), nicht
  über das Ende der Betreuung hinaus. Die laufenden Listen hatten das schon
  („außer tatsächlich anwesend“).
- Das zusätzliche Statement fällt nur an, wenn im Zeitraum ein Kind
  zurückgehalten ist. Kein Query-Budget ändert sich.

## Genehmigte Diagnose

Die Bindung `roots/owners.CareStudents.FindCareStudents` mappt ein Feld mehr.
Ihre Ziele `peopledirectory/ports.StudentRepository.FindByIDs` (query) und
`FindByIDsForUpdate` (E2 nach ADR 0067, Lock-Reihenfolge und Transaktion
unverändert) behalten Effekt und Einstufung; nur der Digest ändert sich.
Beide Diagnosen nennen dieses ADR als `approval_adr`.

`studentpresence.Owner.PresenceDaysBetween` ist ein neuer Lesezugriff
(`query`) auf `active.attendance` und `active.visits` im Tenant des Aufrufers.
Seine zwei Bindungen über `Service.observe` sind `unresolved` wie bei allen
anderen Lesezugriffen von Student Presence (dieselbe Beobachtungsfunktion,
ADR 0098) und nennen dieses ADR als `approval_adr`. Die Root-Übersetzung
`roots/owners.careExitPresence.PresenceDaysBetween` faltet die datierten
Zeilen in die Tageskarte, die Care Plan liest; das Pass-through-Register
führt sie mit diesem ADR als Entscheidung. Sobald dieses ADR in der
Basis liegt, genehmigt es keine weitere Diagnose.

## Folgen

- Die Lücke zwischen früherem Ende und Wiederaufnahme eines zurückkehrenden
  Kindes regelt [ADR 0177](0177-mitgliedschaft-merkt-sich-die-betreuungsluecke-eines-zurueckkehrenden-kindes.md)
  ([#6073](https://github.com/moto-nrw/project-phoenix/issues/6073)).
- Wird der Beginn eines neuen Kindes nach Erreichen verschoben, bleibt der
  frühere Tag; die Tage dazwischen zählen wie vor diesem ADR.
- Die Morgenmeldung der Klasse allein belegt keine Betreuung: Sie meldet den
  Unterricht, nicht die OGS.
