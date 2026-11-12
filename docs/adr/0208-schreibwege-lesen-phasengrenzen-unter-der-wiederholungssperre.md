---
status: proposed
---

# Schreibwege lesen Phasengrenzen unter der Wiederholungssperre

**Status: Vorgeschlagen.** Vorschlag vom 11.11.2026 zu
[#6657](https://github.com/moto-ogs/project-phoenix/issues/6657). Ergänzt
[ADR 0202](0202-phasenaenderungen-verschieben-offene-plaene-und-schuetzen-wirksame-buchungen.md)
und
[ADR 0205](0205-phasenaenderungen-fuehren-die-buchungsgrenzen-der-phase-mit.md).

## Kontext

ADR 0202 und 0205 führen beim Ändern einer Phase Aktivierungen, Pläne und
Buchungen mit. Zwei Wege blieben offen (PR #6634, „Risiken und Betrieb“):
Einreichung, Bearbeitung und Änderungsantrag lasen die Phase ohne die Sperre
der Phasenänderung und schrieben danach Buchungen mit den alten Grenzen. Die
Aufhebung eines Betreuungsendes stellte die Buchungen mit den Grenzen aus dem
Protokoll wieder her, auch wenn die Phase sich inzwischen geändert hatte.
Nach #6505 und #6597 ist das die dritte Runde desselben Befunds. Statt
weiterer Nachführungen liest jeder Schreibweg die Phasengrenzen an genau
einer Stelle.

## Entscheidung

1. **Ein Leser.** `enrollment.PhaseBoundsReader` nimmt in der Transaktion
   des Aufrufers zuerst die Wiederholungssperre, die auch die
   Phasenänderung hält, und liest dann die Phase. Nur er baut eine
   `enrollment.LockedPhase` mit Phase; nur aus ihr kommen Buchungsgrenzen
   (`BookingWindow`, exklusives Ende `ServiceEndDate.AddDays(1)`). Eine
   vorher gelesene Phase dient höchstens der unverbindlichen Vorprüfung.
2. **Alle Schreiber.** Einreichung (`intake_submit`), Bearbeitung
   (`intake_replace`), Änderungsantrag (`change_request_prepare`, Erstellung
   und Freigabe), Bestätigung (`decide`), die Phasenänderung selbst und die
   Aufhebung eines Betreuungsendes (`RestoreCareOfferingBookings`) nutzen ihn.
   Ausgenommen ist die neue Phase des Halbjahreswechsels: Sie entsteht in
   derselben Transaktion und ist für niemanden sonst sichtbar.
3. **Sperrreihenfolge.** Unverändert Klassensperre → Wiederholungssperre →
   Zeilen (Anfrage, Kinder, Buchungen) → Kind (ADR 0162, 0202, 0205).
   Einreichung, Bearbeitung und Erstellung eines Änderungsantrags nehmen den
   Leser als Erstes in ihrer Transaktion, vor der Doppelungs-, Anfrage- und
   Kindersperre. Die händische Anmeldung nimmt die Freigabesperren (Klasse,
   Wiederholung) vor der Einreichung, weil die anschließende Bestätigung sie
   braucht. Freigabe und Bestätigung halten die Sperren schon; der Leser
   nimmt die Wiederholungssperre dort erneut, ohne zu warten. Andere
   Entscheidungen als die Bestätigung schreiben keine Grenze und nehmen die
   Sperre nicht nach ihren Zeilensperren.
4. **Wiederherstellung.** Das Betreuungsende schreibt zu jeder Buchung die
   Phase und deren Grenzen ins Protokoll (`phase` im Snapshot). Die Aufhebung
   liest die Phase mit dem Leser neu. Care Plan sperrt die Buchungen der
   Anmeldung, liest sie und entscheidet nach den Regeln von ADR 0205 (Punkt 2
   und 3), welche wiederhergestellte Grenze die Phasengrenze war und der
   aktuellen Phase folgt. Wechsel ab Datum, individuelle Enden und
   Vergangenheit bleiben. Eine gekappte Buchung stellt nur ihr Ende her; ihr
   Beginn ist live und folgte der Phasenänderung schon. Eine Buchung, die die
   aktuelle Phase nicht mehr abdeckt, kommt nicht zurück; sie bleibt, wie das
   Betreuungsende sie hinterließ.
5. **Absicherung.** `TestPhaseBoundsWritersUseTheLockedReader`
   (`backend/test`) verhindert, dass Enrollment Buchungsgrenzen aus einer
   anders gelesenen Phase baut, dass die Buchungsschreiber ohne
   `LockedPhase` arbeiten und dass der Betreuungsende-Workflow Phasen am
   Leser vorbei liest. Lesende Wege (Formular, Statusseite, Berichte) lesen
   weiter ohne Sperre.

## Architektur

Enrollment besitzt Phase und Leser, Care Plan die Buchungen. Der Leser ist
ein Wert im öffentlichen Enrollment-Paket; die Root bindet ihn an die Phasen
des Owners und an die Wiederholungssperre von Timetable
(`NewEnrollmentPhaseBounds`). Der Betreuungsende-Workflow liest damit die
Phase und übergibt Care Plan nur die Verschiebung (`PhaseBoundsMove`); Care
Plan entscheidet unter den Sperren seiner Buchungen. Keine neue Tabelle,
keine Migration.

Diese Entscheidung gibt die neuen und geänderten Sperr-Effekte der
Schreibwege auf `RecurrenceWriteLock.LockRecurrenceWrites` frei
(`approval_adr` 0208 in `command-effects/reviews/enrollment.json`).

## Folgen

- Einreichungen, Bearbeitungen und Erstellungen von Änderungsanträgen einer
  Schule laufen nacheinander durch die Wiederholungssperre. Sie warten auf
  eine laufende Phasenänderung oder Planänderung; der Durchsatz einer Schule
  reicht dafür aus.
- Eine Phasenänderung wartet auf einen laufenden Schreibweg und verschiebt
  danach dessen Buchungen.
- Betreuungsenden, die vor dieser Entscheidung geplant wurden, tragen keine
  Phase im Protokoll. Ihre Aufhebung stellt die damaligen Grenzen her.
- Abgelehnt: Grenzen beim Lesen aus der Phase ableiten (ADR 0205). Eine
  Zeilensperre auf der Phase statt der Wiederholungssperre: Die händische
  Anmeldung und die Freigabe nehmen die Wiederholungssperre ohnehin; eine
  zweite Sperre davor oder danach öffnet eine neue Reihenfolge.
