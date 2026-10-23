---
status: accepted
---

# Team-Chat gehört zu moto Basis und ist standardmäßig an

Entscheidung vom 23.10.2026 zu
[#3262](https://github.com/moto-nrw/project-phoenix/issues/3262), aufbauend auf
[#3254](https://github.com/moto-nrw/project-phoenix/issues/3254).

## Ausgangslage

Der Team-Chat für Mitarbeitende (`operations.staff_messaging_enabled`, #2598)
war Opt-in: Jede Schule musste ihn in den Einstellungen selbst einschalten.
Im Call am 16.09.2026 berichtete Swantje Kröger (Grundschule Wissingen), dass
sie ihn nur zufällig beim Durchklicken der Einstellungen gefunden hat. Seitdem
nutzt ihr Team ihn täglich zwischen den Räumen. Ihre Frage war, ob der Chat
standardmäßig an sein sollte.

Noch am selben Tag stellte
[PR #3266](https://github.com/moto-nrw/project-phoenix/pull/3266) (#3254) den
Registry-Standard auf `true`. Mit Release-PR #3307 kam das am 17.09.2026 in
Produktion. Gespeicherte Werte `false` blieben bewusst stehen, ohne Migration.

## Entscheidung

**1. Team-Chat ist Teil von moto Basis.** Der
[Modulkatalog](https://github.com/moto-nrw/collaterals/blob/main/src/app/modulkatalog/data.ts)
führt den Baustein „Team-Chat“ im Modul Verwaltung von moto Basis, und Basis
ist „immer dabei“. Der Chat ist damit kein Zusatzmodul, das eine Schule bucht
oder einschaltet. Er gehört auch nicht unter die Modulschalter aus #4812.

**2. Opt-out statt Opt-in.** Der Standard bleibt `true` (#3266). Eine Schule,
die keinen Team-Chat will, schaltet ihn in den Einstellungen aus. Der Schalter
bleibt bei der Schule (`config:update`, `AccessShared`). Eine Leitung erfährt
vom Chat, weil er in der Navigation steht, nicht über einen Hinweis.

**3. Abschaltungen aus der Opt-in-Zeit fallen weg.** Migration 1.15.482 löscht
jeden gespeicherten Wert `false`, der zuletzt vor dem 17.09.2026 03:33:54 UTC
geschrieben wurde. Zu dieser Zeit erreichte der neue Standard die Produktion.
Damals war „aus“ der Standard, also war ein gespeichertes Aus keine
Entscheidung gegen den Chat. Diese Schulen bekommen den Standard wie jede
Schule ohne Wert. Jedes Zurücksetzen steht im Einstellungs-Audit
(`action = 'reset'`, `changed_by` leer).

Ein Wert `false`, der seit diesem Zeitpunkt gespeichert oder neu geschrieben
wurde, bleibt. Diese Schule hat den Chat bewusst ausgeschaltet, als er
standardmäßig an war.

## Folgen

- Das Rückgängigmachen der Migration stellt keine Werte wieder her. Das Audit
  nennt jede betroffene Schule, und das moto-Team kann einen Wert gezielt neu
  setzen.
- Auf Staging gilt derselbe Stichtag. Staging lief schon ab 16.09.2026 mit dem
  neuen Standard. Ein dort in diesem einen Tag gesetztes Aus würde mit
  zurückgesetzt.
- Gruppenchats (#4776) erweitern denselben Baustein und folgen demselben
  Schalter.
