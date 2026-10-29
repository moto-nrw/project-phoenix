---
status: accepted
---

# Träger-Zugriffsprotokoll zeigt die Ferienanmeldungen über Enrollment

Entscheidung vom 29.10.2026 zu [#5326](https://github.com/moto-nrw/project-phoenix/issues/5326).
Sie ergänzt [ADR 0109](0109-traeger-zugriffsprotokoll-liest-die-audit-spur-ueber-den-owner.md)
um eine zweite Quelle und führt keinen neuen Owner ein.

## Kontext

Die Hilfe verspricht: Jedes Öffnen einer Ferienanmeldung, jedes Öffnen eines
Nachweises und jeder Export steht im Protokoll des Trägers. Die Ferienanmeldungen
gehören dem Träger und stehen in `enrollment.holiday_registration_audit`
([ADR 0114](0114-ferienanmeldungen-pruefen-mit-eigenem-protokoll-in-der-traeger-transaktion.md)
Punkt 6), nicht in der Audit-Spur. Das Zugriffsprotokoll las bisher nur die
Audit-Spur und zeigte sie deshalb nicht.

## Entscheidung

1. **Lesen über den Owner.** Enrollment bietet die öffentliche Fähigkeit
   `enrollment.HolidayAccessTrail` (`modules/enrollment/holiday_access_trail.go`).
   Ihre Abfragen liegen im Postgres-Paket von Enrollment und laufen wie
   ADR 0109 Punkt 1 in der Admin-Einheit der Projektion; ohne sie verweigert der
   Adapter das Lesen. Jede Abfrage filtert mit `organization_id = ?0` auf den
   Träger des Betrachters. Der Ratchet
   `TestHolidayAccessTrailQueriesFilterOnTheTraeger` prüft das.
2. **Keine neue Träger-Transaktion.** Die Projektion öffnet für das Lesen
   keine Träger-Transaktion ([ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)).
   Das Query-Budget `api.traeger.access_log` bleibt bei 10.
3. **Ohne Schule.** Die Einträge tragen keine Schule; das Portal zeigt
   „Träger (keine Schule)“. Ein Schulfilter blendet sie aus. Das Lesen des
   Protokolls selbst schreibt für sie keinen Eintrag in
   `audit.data_access_log`, weil keine Schule betroffen ist.
4. **Seiten.** Gibt es Ferieneinträge, liest die Projektion aus beiden
   Spuren die neuesten `offset + limit` Einträge und mischt sie in der
   Reihenfolge der Audit-Spur. Sonst bleibt die Seite der Audit-Spur
   unverändert.
5. **Wurzel.** `traegerHolidayTrail` (`api/serve_mounts_traeger.go`)
   übersetzt nur in das Vokabular des Ports von `traeger-view`; kein Owner
   importiert den anderen.
6. **Register.** Der Konstruktor `NewHolidayAccessTrail` hebt das
   Compose-Budget von `modules/enrollment/compose` um genau 7 Zeilen. Die
   beiden Methoden von `traegerHolidayTrail` stehen mit dieser Entscheidung im
   Pass-through-Register der Wurzel.

## Verworfene Optionen

- **Lesen in einer Träger-Transaktion je Anfrage:** zwei zusätzliche
  Anweisungen pro Seite und eine neue Stelle im Register der
  Träger-Transaktionen. Das Query-Budget darf nicht steigen.
- **Ferieneinträge zusätzlich in `audit.data_access_log` schreiben:** Das
  Protokoll ist dort je Schule; die Anmeldungen haben keine Schule (ADR 0114
  Punkt 6).

## Folgen

- Eine neue Aktion in `enrollment.holiday_registration_audit` erscheint im
  Protokoll; ohne Bezeichnung in `data_labels.go` zeigt das Portal
  „Sonstige personenbezogene Daten“.
- Bei vielen Ferieneinträgen liest eine späte Seite aus beiden Spuren bis zu
  `offset + limit` Zeilen.
