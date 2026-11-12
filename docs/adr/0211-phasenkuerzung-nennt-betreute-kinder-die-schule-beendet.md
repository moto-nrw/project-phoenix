---
status: proposed
---

# Phasenkürzung nennt betreute Kinder, die Schule beendet die Betreuung

**Status: Vorgeschlagen.** Vorschlag vom 11.11.2026 zu
[#6598](https://github.com/moto-ogs/project-phoenix/issues/6598). Ergänzt
[ADR 0202](0202-phasenaenderungen-verschieben-offene-plaene-und-schuetzen-wirksame-buchungen.md),
[ADR 0205](0205-phasenaenderungen-fuehren-die-buchungsgrenzen-der-phase-mit.md)
und
[ADR 0208](0208-schreibwege-lesen-phasengrenzen-unter-der-wiederholungssperre.md).

## Kontext

Die Bestätigung kopiert das Phasenende als Betreuungsende (`enrolled_until`)
an das Kind. ADR 0202 verschiebt dieses Ende nur bei Kindern, die noch nicht
betreut werden; ADR 0205 kürzt die Buchungen mit der Phase. Wird das
Phasenende gekürzt, laufen bei bereits betreuten Kindern Mitgliedschaft und
Betreuungsende weiter, während die Buchungen am neuen Phasenende enden.

Entscheidung des Nutzers im Issue (10.11.2026): Option B. Die Phasenkürzung
zeigt die betroffenen bereits betreuten Kinder mit altem und neuem Ende. Die
Schule beendet die Betreuung bewusst über den vorhandenen Prozess
„Betreuung beenden“. Kein stiller Statuswechsel.

## Entscheidung

1. **Nur Hinweis.** Die Phasenänderung ändert Status und Betreuungsende
   bereits betreuter Kinder nicht. Sie meldet in ihrer Antwort
   (`care_beyond_end` von `PUT /enrollment/phases/{id}`) die Kinder, deren
   Betreuung jetzt über das neue Phasenende hinausläuft, mit Betreuungsende
   und neuem Phasenende.
2. **Wer genannt wird.** Ein Kind, das in dieser Phase bestätigt wurde und
   noch wartet oder betreut wird, dessen Betreuungsende nach dem neuen und
   nicht nach dem alten Phasenende liegt, noch nicht vorbei ist und von
   keiner Bestätigung in einer anderen Phase erreicht wird. Ein später von
   Hand gesetztes Ende, eine frühere Kürzung und Kinder, deren Betreuung
   schon endete, bleiben außen vor. Wartende Kinder folgen nach ADR 0202 der
   Phase und erscheinen deshalb nicht.
3. **Wann gelesen wird.** Am Ende der Phasenänderung, in ihrer Transaktion,
   nach den Nachführungen und hinter der Wiederholungssperre, die die
   Änderung mit `enrollment.PhaseBoundsReader` genommen hat (ADR 0208). Das
   Betreuungsende nimmt dieselbe Sperre; der Hinweis entspricht damit jedem
   bestätigten Betreuungsende. Eine Betreuungsende-Vorschau von vor der
   Kürzung ist danach veraltet und wird abgelehnt.
4. **Weg zum Abschluss.** Die Verwaltung zeigt die Kinder nach dem Speichern
   in einem Dialog und öffnet daraus „Betreuung beenden“ für alle genannten
   Kinder, mit dem neuen Phasenende als letztem Betreuungstag, frühestens
   heute.
5. **Keine Aufgaben.** Nichts wird gespeichert. Wiederholtes Speichern
   erzeugt keine zweite Aufgabe; ein Kind, dessen Betreuung beendet ist,
   erscheint nicht mehr.

## Architektur

Enrollment besitzt Phase und Bestätigungen und entscheidet, wer genannt wird.
Status, Betreuungsende und Namen liest es über den eigenen Port
`enrollment.PhaseCareStudents`, den die Root an People Directory bindet
(`NewEnrollmentPhaseCareStudents`). Zwei Lesezugriffe für alle Kinder der
Phase, kein Schreibzugriff, keine neue Tabelle, keine Migration.

## Folgen

- Schließt die Verwaltung den Dialog, ohne zu handeln, gibt es keinen
  bleibenden Hinweis. Eine weitere Kürzung nennt nur Kinder, die sie selbst
  abschneidet.
- Liegt das neue Phasenende in der Vergangenheit, kann die Betreuung
  frühestens heute enden; der Dialog sagt das.
- Abgelehnt: automatisch kürzen (Option A) und Abweichung bestehen lassen
  (Option C), siehe #6598.
