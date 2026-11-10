---
status: proposed
---

# Ferienlauf gibt nur den geprüften Inhalt frei

**Status: Vorgeschlagen.** Vorschlag vom 10.11.2026 zu
[#6506](https://github.com/moto-nrw/project-phoenix/issues/6506) (EC-2, EC-9,
EC-10, EC-11). Grundlage ist die
[Recherche mit Optionen A bis C](https://github.com/moto-nrw/project-phoenix/issues/6506#issuecomment-6774486298)
und Yannicks Entscheidung vom 10.11.2026 für A: die Freigabe an einen
Fingerabdruck des geprüften Inhalts binden, wie bei der Monatsabrechnung.
Ändert Punkt 6 und die verworfene Option „Entwürfe beim Start speichern“ von
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
für den Ferienlauf; die Monatsabrechnung hat diesen Vertrag seit dem
Nachtrag vom 08.11.2026 (#5916).

## Kontext

Die Freigabe eines Ferienlaufs verglich nur Zahl und Summe der Rechnungen mit
dem, was die Abrechnung geprüft hatte. Prüft A Familie X mit 10 € und Y mit
20 €, korrigiert B danach auf 20 € und 10 € und gibt A dann frei, stellt moto
Bs Verteilung aus, die A nie gesehen hat. Dasselbe gilt für eine geänderte
Debitorennummer, Fälligkeit oder Zahlungsart bei gleichem Betrag. Ausgestellte
Rechnungen ändern sich nicht mehr; der Fehler lässt sich nur mit Storno und
Neuausstellung oder Gutschrift beheben.

Derselbe Befund betrifft die Tagesgrenze je Kind (#4701): Eine Buchung der
ganzen Woche nimmt die Betreuungstage der aktuellen Wochengrenzen. Neue
Grenzen einer gebuchten Woche änderten die Tage ihrer Kinder, ohne die Grenze
zu prüfen.

## Entscheidung

1. **Fingerabdruck der Vorschau.** Ein berechneter Ferienlauf zeigt
   `fingerprint`: einen 128-Bit-FNV-Hash über das kanonische JSON von Lauf,
   jeder auszustellenden Rechnung in Nummernreihenfolge (Empfänger mit
   Anschrift, Familie, Debitorennummer, Zahlungsweg mit Fälligkeit und Mandat,
   Kinder, Positionen, Korrekturen, übernommene Korrekturen, ersetzte
   Rechnung), den ausgelassenen Anmeldungen, den Fälligkeiten des Laufs sowie
   der Dokumentart mit dem Rechtsbehelf eines Bescheids. Die Reihenfolge der
   Rechnungen folgt den Anmeldungen; eine andere Lesereihenfolge ändert
   nichts. Der Hash erkennt Änderungen, er ist kein Geheimnis. Er ersetzt
   keine Berechtigungsprüfung.
2. **Freigabe.** Die Freigabe sendet `fingerprint` zurück. Unter der Sperre
   des Laufs und der Abrechnungssperre des Zeitraums rechnet sie die Vorschau
   neu und vergleicht Zahl, Summe und Fingerabdruck, bevor sie eine Nummer
   reserviert oder eine Rechnung schreibt. Ein fehlender oder anderer
   Fingerabdruck lehnt mit 409 `parent_billing.billing_run_changed` ab.
3. **Tagesgrenze bei Änderung des Zeitraums.** Verschiebt eine Änderung des
   Zeitraums die Tage einer gespeicherten Woche, liest sie unter der Sperre
   des Zeitraums die Buchungen jedes Kindes, dessen Anmeldung nicht storniert
   ist. Ein Kind darf seine Tage behalten oder senken, auch über einer später
   gesenkten Grenze; mehr Tage als die geschriebene Grenze lehnt die Änderung
   mit 409 `enrollment.holiday_week_day_limit` am Ende der Woche ab und
   ändert nichts. Eine Stornierung ist endgültig; ihre Buchungen zählen nicht.
   Anmeldung, Umbuchung und Änderung des Zeitraums halten dieselbe Sperre,
   die Prüfung zählt also die zuletzt bestätigten Buchungen.

## Verworfene Optionen

- **B: Vorschau versioniert speichern.** Braucht neue Persistenz,
  Löschfristen und trotzdem einen Abgleich geänderter Grundlagen. Kein
  eigener Bedarf an gespeicherten Prüffassungen.
- **C: Nur Zahl und Summe bestätigen.** Lässt ungeprüfte Einzelbeträge zu;
  das Ziel aus #4668 ist die Prüfung jeder Familie.
- **Neue JCS-Abhängigkeit (RFC 8785).** Feste Feldreihenfolge der Go-Structs
  und eine feste Reihenfolge der Rechnungen reichen für einen serverseitigen
  Vergleich.

## Folgen

- Jede Änderung zwischen Prüfung und Freigabe verlangt eine neue Prüfung,
  auch ein Wechsel des Tages, an dem die Fälligkeit aus der Zahlungsfrist
  folgt.
- Clients ohne Fingerabdruck können nicht mehr freigeben. Frontend, Seeder
  und Tests senden ihn im selben Änderungssatz.
- **Register.** Keine Migration und kein `data_object`; Fehlercode
  `enrollment.holiday_week_day_limit`; Feld `fingerprint` in Antwort und
  Anfrage von `POST /traeger/billing-runs/{id}/release`.
- **Belege.** `TestHolidayReleaseRefusesAmountsMovedBetweenFamilies`,
  `TestHolidayReleaseRefusesAChangedRecipientOrPaymentOfTheSameSum`,
  `TestHolidayFingerprintIgnoresTheOrderTheRegistrationsAreRead`,
  `TestHolidayWeekChangeCannotRaiseABookedChildOverTheDayLimit`,
  `TestHolidayWeekChangeKeepsBookingsAboveALoweredLimit` und
  `TestHolidayWeekChangeWaitsForAConcurrentRegistration`.
