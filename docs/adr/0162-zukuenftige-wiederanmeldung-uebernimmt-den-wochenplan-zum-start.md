---
status: proposed
---

# Zukünftige Wiederanmeldung übernimmt den Wochenplan zum Start

Vorschlag vom 07.11.2026 zu [#6043](https://github.com/moto-nrw/project-phoenix/issues/6043).

## Kontext

Eine Wiederanmeldung in einer Phase für bestehende Kinder erneuert das
vorhandene Kind (`attachApprovalToExistingStudent`). Bisher schrieb die
Bestätigung alle Formularziele sofort, auch wenn die Betreuung erst später
beginnt: Die Abholzeiten je Wochentag, die Betreuungstage und die
Heimwegregeln der neuen Anmeldung ersetzten die laufenden Werte am
Bestätigungstag. Diese Zeilen haben keinen Gültigkeitszeitraum
(`schedule.student_pickup_schedules`, `schedule.student_arrival_schedules`,
`users.student_profiles.allowed_departure_modes`). Bestätigt Barnstorf im
Dezember die Anmeldung für die Betreuung ab 03.02.2027, galten die neuen Zeiten
schon im Dezember.

Für die Klasse gibt es dasselbe Muster schon: Die Bestätigung plant den
Klassenwechsel auf `request_children.class_switch_on`, und der stündliche
Rollover-Lauf wendet ihn am Starttag an (#3917).

## Entscheidung

Der Wochenplan eines Kindes, das die Schule gerade betreut, folgt dem
Klassenwechsel. Wochenplan sind die Ziele `schedule.pickup`,
`schedule.arrival`, `student.departure`, `student.allowed_departure_modes`,
`student.bus_days`, `student.bus` und `student.pickup_status`, dazu die
Begleitnotiz.

- Beginnt die Betreuung nach dem Bestätigungstag, schreibt die Bestätigung
  alle übrigen Ziele sofort und den Wochenplan nicht. Sie speichert den Starttag
  in `enrollment.request_children.weekly_plan_on`.
- `ApplyDueWeeklyPlans` läuft im Rollover-Lauf nach den Klassenwechseln. Für
  jeden fälligen Plan ersetzt es den laufenden Wochenplan durch die bestätigte
  Anmeldung, mit denselben Schreibwegen wie eine sofortige Wiederanmeldung, und
  löscht den Plan. Fällige Pläne zusammenhängender Laufgemeinschaften laufen
  gemeinsam in einem Savepoint; die bestehende Stranding-Batch-Prüfung
  bewertet ihren gemeinsamen Endzustand. Auch Verbindungen über Kinder ohne
  fälligen Plan gehören zur Gruppe. Ein Fehler rollt die ganze Gruppe zurück:
  Die laufenden Pläne und Verknüpfungen bleiben, die Pläne bleiben fällig, und
  der Lauf meldet den Fehler auf Error-Ebene. Unabhängige Pläne laufen weiter.
- Der Start übernimmt die Anmeldung wie die Bestätigung einer schon begonnenen
  Wiederanmeldung: Ein Feld ohne Wert lässt seinen Wert stehen.
- Eine bestätigte Änderung vor dem Start ändert nur, was der Start übernimmt.
  Ersetzt sie die Formulardaten, merkt sich der Plan das
  (`weekly_plan_replaces`): Der Start übernimmt dann den ganzen Wochenplan, und
  ein entferntes Feld leert seinen Wert wie bei derselben Änderung sofort. Ist
  der Starttag schon da, der Lauf aber noch nicht, übernimmt die Änderung den
  Plan selbst, mit denselben Regeln wie der Lauf, und schreibt danach ihre
  übrigen Felder.
- Als Urheber der neuen Wochenzeilen speichert der Plan die Mitarbeiterin, die
  bestätigt hat, oder zuletzt geändert hat (`weekly_plan_author`). Der Start
  hängt damit nicht davon ab, dass dieses Konto Monate später noch besteht.
- Jede Übernahme protokolliert den bisherigen und den neuen Heimweg in
  derselben Transaktion in der bestehenden Heimweghistorie, bevor sie den
  fälligen Plan löscht. Der Lauf ist dabei der Urheber „System“; bei einer
  Korrektur am Starttag ist es das handelnde Konto. Sie protokolliert den
  Übergang nur einmal. Der gespeicherte Mitarbeiter bleibt der Urheber der
  Wochenzeilen.
- Nach jeder Übernahme erfahren offene Ansichten über `student_updated` von
  der Änderung.
- Ein Kind ohne laufende Betreuung, eine schon begonnene Betreuung und neue
  Kinder bekommen den Wochenplan wie bisher sofort.

Abgelehnt: Wochenzeilen mit Gültigkeitszeitraum in Care Plan. Das wäre eine
Migration der beiden Wochentabellen und eine Änderung aller ihrer Leser
(Anwesenheit, Kiosk, Listen, Elternportal, Auto-Entschuldigungen). Mit der
Übernahme zum Start bleiben die Tabellen und ihre Leser unverändert und lesen
an jedem Tag den gültigen Plan.

## Freigegebene Effekte

`ApplyDueWeeklyPlans` ist eine neue Bindung des Ports
`application.RolloverDecider`. Sie erreicht dieselben Ziele, die
`compose.Decisions.Decide` für eine sofortige Wiederanmeldung schon erreicht
(ADR 0098). Diese Entscheidung gibt genau diese Diagnosen für die Bindung
`*compose.Decisions.ApplyDueWeeklyPlans` frei:

| Ziel | Effekt |
| --- | --- |
| `careplan.PickupAutoExcusal.RecordWeeklyPickupChanges` | mutation, debt |
| `careplan.PickupAutoExcusal.ResyncFutureExceptions` | mutation, debt |
| `careplan/compose.LockWeeklyPickupStudents` | lock, debt |
| `unresolved:callable:.../api.now` | unresolved |
| `unresolved:interface{Field(byte) string; error}.Field` | unresolved |

Die Abstimmung mit Care Plan bleibt die offene Folgearbeit aus ADR 0098; sie
wächst nur um den Zeitpunkt, nicht um einen neuen Schreibweg. Nach dem
Einchecken dieser Entscheidung darf sie keine weiteren Diagnosen freigeben.

## Folgen

- Bis einschließlich des Vortags gilt der bisherige Wochenplan, ab dem Starttag
  der bestätigte. Der Lauf kommt stündlich; am Starttag gilt der neue Plan
  spätestens nach dem ersten Lauf des Tages.
- Ansichten, die vor dem Start einen Tag nach dem Start zeigen, zeigen bis zum
  Start den laufenden Plan.
