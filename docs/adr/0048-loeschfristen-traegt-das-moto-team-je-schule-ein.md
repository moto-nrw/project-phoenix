# Löschfristen trägt das moto-Team je Schule ein, die Schule sieht sie nur

Status: accepted. Gilt für die Löschfristen aus
[#3839](https://github.com/moto-nrw/project-phoenix/issues/3839) und
[#2788](https://github.com/moto-nrw/project-phoenix/issues/2788), Teil des
Epics [#3846](https://github.com/moto-nrw/project-phoenix/issues/3846)
(Vertragsreife). Recherche:
[aufbewahrungsfristen-2026-10-12.md](../research/aufbewahrungsfristen-2026-10-12.md).

## Kontext

Der AVV sagt für jede Datenart eine Löschfrist zu. moto ist für die Schuldaten
Auftragsverarbeiter; die Fristen bestimmt rechtlich der Verantwortliche, also
die Schule bzw. ihr Träger. Für die OGS-Kerndaten (Anwesenheit, Krankmeldungen,
Nachrichten, Kinderstammdaten) gibt es keine gesetzliche Frist, nur die Pflicht,
nicht länger als nötig zu speichern. Wie lange eine Schule die Daten braucht,
klärt sich im Vertriebsgespräch und im Vertrag, nicht in der Software.

Bisher durften Schulen einige Fristen und sogar den ganzen Löschlauf in ihren
Einstellungen ändern. Damit konnte jede Schule die Zusage im AVV unbemerkt
aufheben.

Verworfene Alternativen:

- **Die Schule stellt ihre Fristen selbst ein.** Das ist rechtlich naheliegend,
  aber dann weicht der Code still vom unterschriebenen Vertrag ab, und der
  Löschlauf ließ sich ganz abschalten.
- **Ein Rollen-Umschalter „Schule oder freier Träger“** mit eigenen Fristen je
  Rolle. Die schulrechtlichen Fristen der VO-DV I erfassen die OGS nicht sicher.
  Ein falsch gesetzter Umschalter würde still falsch löschen.

## Entscheidung

1. **Das moto-Team trägt die Löschfristen fachlicher Daten je Schule ein.** Die
   Werte entsprechen dem, was mit der Schule bzw. ihrem Träger vereinbart ist.
   Ohne Vereinbarung gilt die Voreinstellung aus der Registry. Jede Frist hat
   einen festen Rahmen aus Unter- und Obergrenze.
2. **Die Schule sieht ihre Fristen, ändert sie aber nicht.** Sie braucht die
   Speicherdauer für ihre Datenschutzhinweise (Art. 13 Abs. 2 lit. a DSGVO).
   Dafür gibt es eine eigene Stufe: Operator ändert, Schule liest. Bisher von
   Schulen änderbare Fristen wechseln auf diese Stufe; gesetzte Werte bleiben.
3. **Technische Daten und Protokolle haben feste Fristen für alle Schulen.**
   Das betrifft Warteschlangen, Einladungen, MFA-Codes, Tokens und Audit-Logs.
   Diese Fristen stehen in der Fristenliste in `docs/`.
4. **Die Schule kann den Löschlauf nicht abschalten.** Nur das moto-Team kann
   ihn als Notbremse bei einem Vorfall anhalten.
5. **moto ist Ablage, kein gesetzliches Archiv.** Für Personaldokumente gelten
   deshalb nur Höchstfristen. Gesetzliche Mindestfristen (Lohnkonto,
   Buchungsbelege) erfüllt der Träger in seinem eigenen System.
6. **Keine Rolle des Verantwortlichen im System.** Die Rahmen sind so weit, dass
   auch längerer Bedarf hineinpasst. Bestätigt eine Datenschutzberatung, dass
   Schulen längere Mindestfristen brauchen, kommt die Rolle mit genau diesem
   Zweck.

## Folgen

- Eine Friständerung ist ein Vertragsvorgang. Das Einstellungsprotokoll weist
  nach, wer sie wann eingetragen hat.
- Verkürzt das moto-Team eine Frist, löscht der nächste Lauf alles, was älter
  ist. Das Operator-Dashboard verlangt dafür eine Bestätigung.
- Für Bestandsschulen ist der angekündigte Release der Stichtag; einen eigenen
  Einschalter je Schule gibt es nicht. Schulen mit längerem Bedarf melden sich
  vorher.
- Es gibt keine Vorwarnung vor automatischen Löschungen und keine Sperre
  einzelner Daten bei konkretem Anlass. Beides kommt nur bei konkretem Bedarf.
