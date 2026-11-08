---
status: accepted
---

# Rollover anlegen nimmt die Wiederholungssperre vor der neuen Phase

Entscheidung vom 08.11.2026 zu
[#6168](https://github.com/moto-nrw/project-phoenix/issues/6168). Sie ergänzt
die [Sperrreihenfolge](0162-zukuenftige-wiederanmeldung-uebernimmt-den-wochenplan-zum-start.md#sperrreihenfolge)
aus ADR 0162.

## Kontext

`Rollovers.runCreate` legte die Nachfolgephase an und nahm die
Wiederholungssperre erst danach, in der Katalogkopie von Care Plan. Der
Fremdschlüssel der neuen Phase auf die Quellphase hält `KEY SHARE` auf deren
Zeile. `Phases.DeletePhase` hält die Wiederholungssperre und wartet darauf, die
Quellphase zu löschen. Beide warteten aufeinander; PostgreSQL brach mit
`40P01` ab.

## Entscheidung

`createRolloverPhase` nimmt `Decisions.LockApprovalGates`, also Klassensperre
und Wiederholungssperre, nach der Eignungsprüfung und vor dem Einfügen der
neuen Phase. Unter der Sperre liest es die Quellphase erneut. Danach folgen die
neue Phase und die Katalogkopie. Wartet der Rollover auf ein Löschen, meldet er
danach `ErrRolloverSourceNotFound`.

Die Eignungsprüfung einer klassenbeschränkten Phase nimmt die
Klassenabfrage-Sperre (`LockClassCollectionPair`, exklusiv). Eine
Phasenänderung nimmt sie vor der Wiederholungssperre. Der Rollover nimmt sie
deshalb ebenfalls zuerst; Wiederholungssperre und danach Klassenabfrage-Sperre
schlossen gegen eine Phasenänderung einen neuen Zyklus.

Zwei Zwei-Verbindungs-Tests belegen das:
`TestRolloverService_CreatePhaseFromSource_WaitsForPhaseDelete` zeigt vorher
`40P01` gegen „Phase löschen“, danach keinen Deadlock.
`TestRolloverService_CreatePhaseFromSource_TakesCollectionLockBeforeGates`
zeigt `40P01`, wenn die Sperren vor der Eignungsprüfung genommen werden.

## Freigegebene Effekte

Der Rollover nimmt die Sperre jetzt über den Port
`application.RolloverDecider`, nicht mehr nur über die Katalogkopie. Diese
Entscheidung gibt genau diese Diagnosen frei:

| Bindung | Ziel | Effekt |
| --- | --- | --- |
| `*application.Rollovers.CreatePhaseFromSource` | `timetable.RecurrenceWriteLock.LockRecurrenceWrites` | lock, debt |
| `http.rolloverService.CreatePhaseFromSource` | `timetable.RecurrenceWriteLock.LockRecurrenceWrites` | lock, debt |

Es ist dieselbe Sperre, die die Katalogkopie in derselben Transaktion schon
nimmt. Die Abstimmung mit Care Plan bleibt die offene Folgearbeit aus ADR 0098.
Nach dem Einchecken dieser Entscheidung darf sie keine weiteren Diagnosen
freigeben.

## Nicht übernommen

Laut #6168 hält `ApplyDueWeeklyPlans` die Sperre über alle fälligen Gruppen
einer Schule in einer Transaktion. Das ist nicht gemessen, und diese
Entscheidung ändert es nicht.
