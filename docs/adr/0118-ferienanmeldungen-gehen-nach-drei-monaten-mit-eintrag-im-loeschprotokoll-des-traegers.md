---
status: accepted
---

# Ferienanmeldungen gehen nach drei Monaten mit Eintrag im Löschprotokoll des Trägers

Entscheidung vom 26.10.2026 zu [#4667](https://github.com/moto-nrw/project-phoenix/issues/4667),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Story 121. Sie wendet
[ADR 0073](0073-inhalte-der-schule-folgen-der-frist-ihrer-datenart.md) auf die
Ferien-Daten des Trägers an und baut auf
[ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md),
[ADR 0114](0114-ferienanmeldungen-pruefen-mit-eigenem-protokoll-in-der-traeger-transaktion.md)
und [ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
auf. Sie führt keinen neuen Owner ein.

## Kontext

Seit #4661 löscht der stündliche Lauf die Ferienanmeldungen eines Zeitraums
92 Tage nach seiner letzten Woche, seit #4663 auch das Protokoll der Prüfung.
Die Löschung stand aber in keinem Nachweis: Das Löschprotokoll der Schule
(`audit.data_deletions`) verlangt eine Schule, und eine Anmeldung gehört dem
Träger. Ob die Löschung an den unveränderlichen Rechnungen aus #4668 und
#4669 scheitert oder sie verändert, war nicht geprüft.

## Entscheidung

1. **Was geht:** 92 Tage nach dem letzten Tag der letzten Woche eines
   Zeitraums gehen seine Anmeldungen mit Eltern, Kindern samt
   Gesundheitsangaben, Feriengästen, Notfallkontakten, Abholberechtigten und
   Buchungen, das Protokoll der Prüfung, der festgehaltene Ferienbeitrag
   (ADR 0115) und die Korrekturen eines Abrechnungslaufs. Startzeitpunkt ist
   wie in ADR 0073 Punkt 2 das Ende des Inhalts. Die Frist ist fest; eine
   Einstellung je Träger folgt mit #4720.
2. **Was bleibt:** Ausgestellte Rechnungen, ihre Positionen, die
   Bestätigungsschreiben, die Angaben der Schreiben und die Spur des Laufs
   bleiben unverändert (§ 147 AO, ADR 0119, ADR 0121). Sie tragen ihre eigene
   Kopie von Empfänger und Kindern und haben keinen Fremdschlüssel auf die
   Anmeldung. Die Löschung ändert keine ihrer Zeilen, sodass die Trigger gegen
   Änderungen nicht greifen.
3. **Nichts Eigenes zu löschen:** Die Vorbefüllung (ADR 0113) und die
   Teilnehmerliste der ausrichtenden OGS (ADR 0116) speichern keine Kopie.
   Ihr Zugriff endet mit der Anmeldung. Ihre Einträge im Zugriffsprotokoll der
   Schule nennen keine Inhalte und folgen dessen Frist.
4. **Löschprotokoll des Trägers:** `enrollment.holiday_registration_deletions`
   (Schreib-Owner Enrollment) hält je gelöschtem Zeitraum Träger,
   Zeitraum-ID, Ende der letzten Woche und die Zahl der Anmeldungen, Kinder und
   Protokolleinträge, ohne Namen. Dieselbe Anweisung löscht und schreibt den
   Eintrag; alle Teile lesen denselben Stand, die Zahlen stimmen also mit dem
   Gelöschten überein. Ein Zeitraum ohne verbliebene Daten schreibt keinen
   Eintrag.
5. **Frist des Protokolls:** drei Jahre ab Löschung wie das Löschprotokoll
   der Schule; derselbe stündliche Lauf löscht ältere Einträge. Der Eintrag
   nennt den Zeitraum ohne Fremdschlüssel, damit er bleibt, wenn das
   Träger-Büro den Zeitraum später entfernt. Er geht mit dem Träger.
6. **Zugriff:** Nur die Admin-Transaktion des stündlichen Laufs schreibt und
   liest das Protokoll. Die Anfrage-Rolle hat kein Recht darauf; die
   Träger-Policy aus ADR 0105 begrenzt einen späteren Leser im Träger-Portal
   auf den eigenen Träger.

## Verworfene Optionen

- **`audit.data_deletions` um den Träger erweitern:** Die Tabelle gehört der
  Audit Platform, gilt je Schule unter RLS und verlangt ein Kind oder eine
  Person. Ein Träger-Bereich darin bräuchte eine zweite Policy, einen Port
  über die Owner-Grenze und eine Ausnahme von ihrer Personenpflicht, nur für
  eine Zeile je Zeitraum.
- **Eintrag im Protokoll der Prüfung:** Es geht mit dem Zeitraum und wird von
  demselben Lauf gelöscht; ein Nachweis darin verschwände mit dem, was er
  nachweist.

## Folgen

- Das Träger-Portal zeigt das Protokoll noch nicht. Eine Ansicht braucht ein
  Leserecht der Anfrage-Rolle und eine eigene Entscheidung.
- `NewHolidayRegistrationRetention` nimmt eine Uhr wie
  `NewHolidayHostParticipants`; der Test prüft mit einer Test-Uhr den 92. und
  den 93. Tag.
- Register: `policy.json` `data_objects` mit `write_owner` `enrollment`,
  Datenlebenszyklus mit `scope: organization`, Retention-Wächter,
  Seed-Ausnahme (der Seed plant laufende Ferien und hat nichts zu löschen),
  [Löschfristen](../reference/loeschfristen.md). Keine neue Regel, keine neue
  Stelle der Admin- oder Träger-Transaktion.
