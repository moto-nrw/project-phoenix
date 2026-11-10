---
status: proposed
---

# Phasenänderungen verschieben offene Pläne und schützen wirksame Buchungen

**Status: Vorgeschlagen.** Vorschlag vom 10.11.2026 zu
[#6505](https://github.com/moto-nrw/project-phoenix/issues/6505) (EC-4, EC-5,
EC-6). Grundlage ist die
[Recherche mit Optionen A bis C](https://github.com/moto-nrw/project-phoenix/issues/6505#issuecomment-6774472458)
und Yannicks Entscheidung vom 10.11.2026 für B: offene Pläne atomar
mitverschieben und den Buchungsabschluss schützen, analog zur Wochenplanlogik
aus [ADR 0162](0162-zukuenftige-wiederanmeldung-uebernimmt-den-wochenplan-zum-start.md).

## Kontext

Eine Bestätigung kopiert den Start der Phase in drei Pläne: den Wochenplan
(`weekly_plan_on`), den Klassenwechsel eines laufend betreuten Kindes
(`class_switch_on`) und die Aktivierung eines vorgemerkten Kindes
(`activate_on` und `enrolled_from` der Schulzugehörigkeit). Eine spätere
Phasenänderung verschob nur den Wochenplan. Das vorgemerkte Kind wurde am
alten Tag aktiv, die neue Klasse galt vor oder nach dem neuen Wochenplan. Das
Löschen einer laufenden Phase entfernte die letzte Buchung eines betreuten
Kindes ohne Betreuungsende und ohne Abschlussaufgabe.

## Entscheidung

1. **Klassenwechsel.** Der geplante Klassenwechsel folgt dem Start der Phase
   in derselben Anweisung wie der Wochenplan. Ein ausgeführter Wechsel
   entsteht nicht neu; eine inzwischen geänderte Klasse schützt der Lauf wie
   bisher.
2. **Vorgemerkte Kinder.** Die Aktivierung und der Zeitraum eines noch
   vorgemerkten Kindes folgen Start und Ende der Phase.
   - Eine Aktivierung, deren Tag gekommen ist, gilt als ausgeführt. Nur ein
     Start nach heute bewegt sich. Der stündliche Aktivierungslauf liest nur
     Starts bis heute; er kann deshalb nie einen verschobenen Start
     aktivieren, auch wenn er zwischen Lesen und Schreiben auf die Änderung
     trifft.
   - School Membership entscheidet mit einem eigenen Befehl
     (`PendingWindows.MovePendingWindow`) unter der Zeilensperre des Kindes:
     Nur ein vorgemerktes Kind bewegt sich, und nur eine Grenze, die noch der
     alten Phasengrenze entspricht. Eine Grenze aus einer anderen Anmeldung
     oder einer Handänderung bleibt. Ein Zeitraum, der vor seinem Beginn
     enden würde, wird vor dem Schreiben abgelehnt; die Phasenänderung rollt
     dann ganz zurück.
   - Bei Anmeldungen in mehreren Phasen schlägt Enrollment den frühesten
     Start und das späteste Ende der Anmeldungen innerhalb des alten
     Zeitraums vor. Eine Anmeldung, die das Schuljahr schon ersetzt hat,
     zählt nicht.
   - Nur wenn das Kind noch vorgemerkt war, verschiebt Enrollment danach den
     eigenen Aktivierungsplan.
3. **Löschen.** Eine Phase, deren Angebote noch Buchungen bestätigter
   Kinder haben, die heute oder später gelten, wird nicht gelöscht
   (`enrollment.phase_has_effective_bookings`, 409). Die Prüfung läuft unter
   der Wiederholungssperre vor jedem Schreiben. Eine beendete Phase, beendete
   Buchungen und noch nicht bestätigte Anmeldungen blockieren nicht. Zum
   Ausblenden bleibt „Deaktivieren“; ein Betreuungsende läuft über den
   bestehenden Abschluss.

## Sperren

Die Phasenänderung hält wie bisher die Wiederholungssperre, dann die
Phasenzeile und die offenen Pläne. Der Befehl der Schulzugehörigkeit nimmt
nur die Zeilensperre des Kindes und **keine** Klassensperre: Ein Weg darf nach
der Wiederholungssperre nicht mehr auf die Klassensperre warten (Reihenfolge
aus ADR 0162). Ein Jahrgangswechsel hält die Klassensperre exklusiv und nimmt
danach die Wiederholungssperre; ein Warten in der anderen Richtung schloss
einen Zyklus. Kinder werden in aufsteigender ID gesperrt.

Der Befehl hebt die Kontingentzahl nicht: Gezählt werden vorgemerkte Kinder
mit einem Start nach heute; die Verschiebung behält das oder beendet es.

## Architektur

Der Befehl ist ein geschützter Blattbefehl im Sinn des
[Nachtrags zu ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#addendum-2026-10-23-command-participation-and-coordination-4538):
Er läuft in der Transaktion der Phasenänderung, prüft und schreibt selbst und
braucht keine gemeinsame Sperrreihenfolge und kein erneutes Lesen fremder
Daten durch Enrollment. Er ist eine eigene kleine Capability neben der
Facade der Schulzugehörigkeit; die Root setzt sie über
`ComposePendingWindows` ein. Enrollment liest seine eigenen Anmeldungen über
die bestehenden Abfragen; die Owner-Facade wächst nicht.

Diese Entscheidung gibt die neuen Bindungs-Effekte (ein Blattbefehl, die
Buchungsabfrage der Löschung), die zwei Root-Adapter im Pass-through-Register
und genau diese Compose-Zeilen frei: `modules/enrollment/compose` +2 (das
Feld `Students` und seine Übergabe), `modules/schoolmembership/compose` +5
(der Konstruktor `NewPendingWindows`).

## Folgen

- Anzeige, Aktivierungsplan und tatsächlicher Start eines vorgemerkten
  Kindes stimmen nach jeder Verschiebung überein. Rückt der Start in die
  Vergangenheit, aktiviert der nächste stündliche Lauf das Kind.
- Das bisher kopierte Betreuungsende eines schon betreuten Kindes ändert eine
  Phasenänderung weiterhin nicht; das ist ein Betreuungsende und kein offener
  Plan.
- Bestehende widersprüchliche Pläne aus der Zeit vor dieser Änderung
  korrigiert sie nicht rückwirkend.
- Abgelehnt: A (Änderungen nach Bestätigungen sperren) verhindert nötige
  Korrekturen; C (Zeiträume je Anmeldung einfrieren) ändert das Zeitmodell
  aller Leser.
