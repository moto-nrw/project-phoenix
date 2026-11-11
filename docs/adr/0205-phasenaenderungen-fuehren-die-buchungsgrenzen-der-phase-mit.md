---
status: proposed
---

# Phasenänderungen führen die Buchungsgrenzen der Phase mit

**Status: Vorgeschlagen.** Vorschlag vom 11.11.2026 zu
[#6597](https://github.com/moto-nrw/project-phoenix/issues/6597). Grundlage
sind die Akzeptanzkriterien des Issues und die in
[#6549](https://github.com/moto-nrw/project-phoenix/pull/6549#issuecomment-6780433342)
zurückgestellte Quorum-Variante. Ergänzt
[ADR 0202](0202-phasenaenderungen-verschieben-offene-plaene-und-schuetzen-wirksame-buchungen.md)
um die Buchungen.

## Kontext

Eine Anmeldung kopiert Beginn und Ende der Phase in ihre Buchungen
(`enrollment.care_offering_bookings`, `valid_from` inklusiv, `valid_until`
exklusiv am Tag nach dem Ende). Seit ADR 0202 folgen Aktivierung,
Klassenwechsel und Wochenplan einer Phasenänderung, die Buchungen nicht. Zog
die Schule den Beginn vor, wurde ein vorgemerktes Kind am neuen Tag aktiv,
stand aber erst ab dem alten Buchungsbeginn auf den angebotsbasierten Listen.
Eine Verlängerung ließ die Buchungen und damit die Listen am alten Ende
auslaufen.

## Entscheidung

1. **Eigener Befehl in Care Plan.** Care Plan besitzt die Buchungen und
   verschiebt sie mit `PhaseBookingBounds.FollowPhaseBookingBounds`.
   Enrollment ruft den Befehl in der Transaktion der Phasenänderung vor dem
   Listenabgleich und vor den offenen Plänen aus ADR 0202 auf. Enrollment
   schreibt keine Buchung selbst.
2. **Nur die Grenzen der Phase.** Eine Grenze folgt nur, wenn sie noch der
   alten Phasengrenze entspricht **und** die äußere Grenze der Buchungen
   derselben Anmeldung für dasselbe Angebot ist: der Beginn der frühesten,
   das Ende der spätesten Buchung. Die Grenze eines Wechsels ab Datum, ein
   Betreuungsende und eine Handänderung bleiben. Eine gleiche Datumsangabe
   allein macht eine innere Grenze nicht zur Phasengrenze. Das gilt für alle
   Anmeldungen der Phase, auch für noch nicht bestätigte: Sonst bekäme ein
   Kind, das nach der Änderung bestätigt wird, die alten Grenzen.
3. **Vergangenheit bleibt.** Ein Beginn folgt nur, solange er nach heute
   liegt (wie die Aktivierung in ADR 0202). Ein Ende folgt nur, solange die
   Buchung heute noch gilt. Keine Grenze rückt vor heute: Ein Beginn in der
   Vergangenheit wird heute, ein Ende in der Vergangenheit endet heute.
4. **Keine leere Buchung.** Würde eine Buchung dadurch enden, bevor sie
   beginnt, lehnt der Befehl vor dem Schreiben ab
   (`enrollment.phase_booking_bounds_conflict`, 409, mit dem Namen des
   Kindes in `details.child`). Die Phasenänderung rollt ganz zurück.

## Sperren

Die Phasenänderung hält die Wiederholungssperre und die Phasenzeile (ADR
0162, ADR 0202). Der Befehl sperrt die Buchungen jeder Anmeldung der Phase in
aufsteigender ID (dieselbe Sperre wie alle Buchungsschreiber), liest sie
unter diesen Sperren neu und entscheidet erst dann. Taucht beim erneuten
Lesen eine Anmeldung auf, deren erste Buchungen inzwischen eingetragen
wurden, sperrt er auch sie und liest noch einmal. Er nimmt keine weitere
Sperre. Bestätigung, Buchungsanpassung und Betreuungsende nehmen die
Wiederholungssperre vor den Buchungen und warten damit auf die ganze
Phasenänderung. Ein Schreiber, der nur die Buchungssperre hält, etwa beim
Ersetzen der Buchungen eines noch nicht entschiedenen Kindes, lässt die
Phasenänderung warten; sie verschiebt danach seine Buchungen. Der
Speicher schreibt eine Grenze nur, solange die Zeile noch die entschiedenen
alten Grenzen hat.

## Architektur

Der Befehl ist ein geschützter Blattbefehl im Sinn des
[Nachtrags zu ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#addendum-2026-10-23-command-participation-and-coordination-4538):
Er läuft in der Transaktion der Phasenänderung, prüft und schreibt unter
eigenen Sperren und braucht kein erneutes Lesen fremder Daten durch
Enrollment. Er ist eine eigene kleine Capability neben der Facade von Care
Plan; die Root bindet sie über `NewEnrollmentPhaseBookings` an den Port
`enrollment.PhaseBookings`. Die Facade wächst nicht.

Diese Entscheidung gibt den neuen Bindungs-Effekt (ein Blattbefehl), den
Root-Adapter im Pass-through-Register und genau diese Compose-Zeilen frei:
`modules/enrollment/compose` +2 (das Feld `Bookings` und seine Übergabe).

## Folgen

- Nach einer Phasenänderung beginnen und enden Aktivierung, Buchungen und
  künftige angebotsbasierte Listen am selben Tag, solange keine eigene Grenze
  des Kindes greift. Das Betreuungsende eines Kindes begrenzt die Listen
  weiterhin unabhängig von der Buchung.
- Eine Phasenänderung, die einen Wechsel ab Datum überholen würde, ist
  abgelehnt. Die Leitung passt zuerst die Angebote des Kindes an.
- Buchungen mit Grenzen, die schon vor dieser Änderung von der Phase
  abwichen, korrigiert sie nicht rückwirkend.
- Ohne gespeicherte Herkunft ist eine Handgrenze, die genau auf die alte
  Phasengrenze fällt und die äußere Grenze ist, nicht von ihr zu
  unterscheiden; sie folgt mit.
- Hebt die Schule ein Betreuungsende nach einer Phasenänderung wieder auf,
  folgt die Wiederherstellung seit
  [ADR 0207](0207-schreibwege-lesen-phasengrenzen-unter-der-wiederholungssperre.md)
  der aktuellen Phase.
  Betreuungsenden, die vor dem Nachtrag geplant wurden, tragen keine
  Phasengrenzen im Protokoll und stellen die damaligen Grenzen her.
- Ist die Phase schon zu Ende, verlängert eine spätere Änderung des Endes
  die beendeten Buchungen nicht.
- Eine Einreichung oder ein Änderungsantrag, der die Phase vor der Änderung
  gelesen hat, liest sie seit
  [ADR 0207](0207-schreibwege-lesen-phasengrenzen-unter-der-wiederholungssperre.md)
  unter der Sperre neu.
- Abgelehnt: Die Grenzen beim Lesen aus der Phase ableiten. Das ändert das
  Zeitmodell aller Leser der Buchungen und unterscheidet keine Handänderung
  von einer Phasengrenze.
