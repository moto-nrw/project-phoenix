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

## Nachtrag 2026-11-11: Frist je Anmeldung, Gesundheitsangaben nach 92 Tagen, PDF (#6691)

Entscheidung vom 11.11.2026 zu
[#6691](https://github.com/moto-nrw/project-phoenix/issues/6691), aus dem
Termin mit der Gemeinde Bissendorf. Das Träger-Büro konnte eine Anmeldung
nicht ablegen und hatte nach drei Monaten keinen Nachweis mehr, was die
Eltern erklärt haben. Der Nachtrag ersetzt in Punkt 1 „Die Frist ist fest“
und lässt die Punkte 2 bis 6 bestehen.

1. **Die Frist gehört zur Anmeldung.** `enrollment.holiday_registrations.retention_days`
   hält die Tage nach der letzten Woche fest, die der Einwilligungstext der
   Anmeldung nennt. Der Lauf löscht eine Anmeldung, wenn diese Tage vorbei
   sind. Maßgeblich ist damit die Zusage an die Eltern, nicht der heutige Wert
   des Trägers: Eine spätere Änderung verlängert oder verkürzt keine
   abgegebene Anmeldung.
2. **Standard ein Jahr, einstellbar.** Die Träger-Einstellung
   `holiday_registration.retention_days` (ADR 0086, Ebene Träger) hat den
   Standard 365 und die Grenzen 92 und 1095. Sie wird nur bei Angebot und
   Abgabe der öffentlichen Anmeldung gelesen, in der Träger-Transaktion. Der
   stündliche Lauf liest keine Einstellung; er braucht nur die Spalte. Die
   Einstellung ist `shared` und nicht `operator_managed`: Der Träger ist
   Verantwortlicher und sagt den Eltern die Frist selbst zu.
3. **Fassung 4 des Einwilligungstextes.** Sie nennt die Tage statt „drei
   Monate“ und sagt, dass Gesundheitsangaben schon nach 92 Tagen gehen. Der
   Wortlaut einer Anmeldung ist Fassung plus ihre Tage; eine andere Frist ist
   ein anderer Wortlaut. Das Formular schickt die Tage zurück, die es gezeigt
   hat. Weichen sie vom Wert des Trägers ab, meldet die Abgabe „Formular
   geändert“, statt eine Zustimmung zu einem nicht gezeigten Text zu speichern.
4. **Bestand.** Anmeldungen unter den Fassungen 2 und 3 behalten 92 Tage;
   die Migration trägt den Wert ein. Für sie ist der PDF-Download der Weg zu
   einer längeren Ablage.
5. **Gesundheitsangaben folgen der längeren Frist nicht.** 92 Tage nach der
   letzten Woche leert derselbe Lauf die versiegelten Werte
   (`health_data` der Anmeldung und ihrer Kinder), auch wenn die Anmeldung
   bleibt. Sie dienen der Betreuung in den Ferienwochen (Art. 9 Abs. 2 lit. a
   DSGVO, Einwilligung für diesen Zweck); für Rückfragen, Korrekturen und
   Nachberechnungen im folgenden Jahr braucht sie niemand. Das Löschprotokoll
   zählt sie in `health_records`.
6. **Protokoll der Prüfung.** Einträge einer Anmeldung gehen mit ihr.
   Einträge zu Liste und Export nennen keine Anmeldung; sie gehen, wenn keine
   Anmeldung des Zeitraums mehr da ist, frühestens nach 92 Tagen. Ein Zeitraum
   mit Anmeldungen beider Fristen schreibt zwei Einträge ins Löschprotokoll.
7. **PDF je Anmeldung und ZIP je Zeitraum.**
   `GET /traeger/holiday-periods/{id}/registrations/{registrationId}/pdf` und
   `GET /traeger/holiday-periods/{id}/registrations/pdf` verlangen das Recht
   „Anmeldung verwalten“ und bleiben beim Träger der Sitzung. Jeder Abruf
   steht im Protokoll der Prüfung: eine Anmeldung als `read`, ein Zeitraum als
   `export`, beide mit `format: pdf`. Gesundheitsangaben enthält eine Datei
   nur mit `health=true`; das verlangt zusätzlich „Gesundheitsangaben sehen“
   und schreibt je Anmeldung einen Eintrag `health`, wie das Lesen am
   Bildschirm.
8. **Das ZIP wird gesendet, nicht gesammelt.** Die Träger-Transaktion hält
   ihre Antwort bis zum Commit zurück. Die ZIP-Route läuft deshalb außerhalb:
   Sie liest und protokolliert in der Träger-Transaktion, und nach deren
   Commit schreibt sie Datei für Datei in die Antwort. Die Zahl der Abfragen
   hängt nicht von der Zahl der Anmeldungen ab. Bricht der Abruf ab, steht er
   trotzdem im Protokoll.

### Verworfen

- **Frist je Zeitraum oder nur als Einstellung:** Eine Änderung der
  Einstellung würde Anmeldungen länger halten, als ihr Text zusagt, oder
  kürzer.
- **Gesundheitsangaben ein Jahr halten:** kein Zweck nach den Ferien; die
  längere Frist dient der Abrechnung und dem Nachweis der Erklärung.
- **ZIP als Hintergrundauftrag:** bräuchte einen Ablageort für
  personenbezogene Dateien mit eigener Frist und eine Benachrichtigung. Der
  Stream braucht beides nicht.

### Folgen

- Die PDF-Kopien liegen beim Träger. Für ihre Aufbewahrung und Löschung ist
  er verantwortlich; moto löscht sie nicht. Die Hilfe sagt das.
- Der Test `TestHolidayRegistrationsGoAfterTheirRetentionAndInvoicesStay`
  prüft mit einer Test-Uhr den 92., 93., 365. und 366. Tag.
- Register: Rückgabe-Register (zwei Spalten), Datenlebenszyklus,
  Retention-Wächter, [Löschfristen](../reference/loeschfristen.md).
