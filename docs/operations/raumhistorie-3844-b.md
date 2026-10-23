# Raumhistorie bei ausgeschaltetem Protokoll

Slice 3844-b, Refs #3844, Teil von #3846. Geprüfte Code-Basis:
`ce867abd86` (`origin/development`, 23.10.2026).

## Erhebung vor der Umsetzung

Einstellung: `gdpr.attendance_log_enabled`, Voreinstellung `false` in
`backend/modules/settings/internal/defaults/gdpr.go`.

| Funktion | Fundstelle unter `backend/` | Braucht geschlossene Besuche bei ausgeschaltetem Protokoll? |
|---|---|---|
| Besuchsliste, Gruppe, einzelner Besuch | `modules/studentpresence/inbound/presence/visits_handlers.go`, `presence_queries.go` | Tagesbezug bleibt möglich. Ältere Abschlüsse sind nicht erforderlich und werden nicht mehr gelesen. |
| Aktueller Ort, Raumkapazität, Raumwechsel, Abholung | `modules/studentpresence/internal/application/presence/visit_reads.go`, `presence_queries.go`, `visit_service.go` | Offene Besuche reichen. Sie bleiben unabhängig vom Alter erhalten. |
| Heutige Raum-/Angebotsnamen in Übersichten | `modules/studentpresence/internal/application/presence/visit_names.go` | Liest bereits nur heutige Eintritte. Die Tagesdaten bleiben bis zum Nachtlauf erhalten. |
| Tagesabschluss und Kiosk-Timeout | `modules/studentpresence/internal/application/presence/session_timeout.go`, `session_groups.go` | Listen enthalten auch Abschlüsse, verarbeitet werden hier nur offene Besuche. Alte Abschlüsse sind nicht erforderlich. |
| An-/Abmeldung über Kiosk, Mitarbeiter-UI und Stundenplan | `modules/studentpresence/internal/application/presence/visit_helpers.go`, `attendance_service.go`; `modules/studentpresence/internal/adapters/postgres/visit.go` | Aktuelle Besuche und Tagesanwesenheit bleiben erhalten. Der Schreibvertrag ändert sich nicht. Keine PyrePortal-Änderung. |
| Tagesauswertung und Export | `modules/presenceprojection/http/day_log_handlers.go`, `day_log_export.go` | Bei ausgeschaltetem Protokoll bereits gesperrt. Tagesanwesenheit hat eine eigene Aufbewahrung. |
| „Wo war das Kind“, Verlauf und Export | `modules/presenceprojection/http/attendance_history_handlers.go`, `attendance_history_export.go` | Bei ausgeschaltetem Protokoll bereits gesperrt. |
| Raumauslastung und Statistikexport | `modules/studentpresence/internal/application/statistics/service.go`, `modules/settings/statistics_retention.go` | Kein notwendiger operativer Tagesablauf. Der Raumteil wird bei ausgeschaltetem Protokoll gesperrt, auch bevor gelöscht wurde. Andere Statistikbereiche bleiben unverändert. |

Ergebnis: Kein untersuchter operativer Ablauf benötigt geschlossene Besuche
vor dem heutigen Tag. Tagesdaten und offene Besuche bleiben gemäß Entscheidung
vom 23.10.2026 erhalten. Das ist keine Feststellung einer Rechtsgrundlage.

## Im Code umgesetzt

1. Der Nachtlauf löst die Einstellung je Schule auf. Ausgeschaltet zählt das
   Besuchsende vor heutiger Berliner Mitternacht, nicht eine rollende
   24-Stunden-Frist oder die Einwilligungsfrist. Offene Besuche bleiben.
2. Ausschalten und Zurücksetzen auf `false` löschen ältere Abschlüsse sofort.
   Einstellung, Löschungen und Löschprotokoll stehen in derselben Transaktion.
   Ein Audit- oder Löschfehler darf keinen erfolgreichen Wechsel hinterlassen.
3. Besuchslisten, Gruppenlisten und Einzelabrufe beschränken abgeschlossene
   Besuche auf heute. Ein Fehler beim Einstellungslesen liefert keine Historie.
   Offene Besuche sind davon getrennt. Raumstatistiken lesen ausgeschaltet
   keine Raumdaten.
4. Vorschau und Löschstatistik verwenden dieselbe Auswahl wie der Löschlauf.
   `audit.data_deletions` enthält `visit_retention`, Anzahl, Schulbezug und
   Metadaten mit `attendance_log_enabled: false` und `completed_before`.

Keine Änderung an Gesundheitsdaten, Einwilligungsnachweisen, Stimmungsdaten,
Tagesanwesenheitsfristen oder produktiven Einstellungen. Keine vollständige
Erfüllung von #3844. Bei eingeschaltetem Protokoll gelten die bestehenden
Einwilligungs-/Schulfristen weiter.

## Produktiv noch nachzuweisen

Nur eine berechtigte Person führt diese Schritte in der freigegebenen
Betriebsumgebung aus. Dieses Dokument führt keine Requests aus.

1. Deployment und freigegebenen Umfang mit dem Verantwortlichen bestätigen.
   Erforderlichkeit der Tageshistorie und Rechtsgrundlage dokumentieren.
2. Je betroffener Schule prüfen, dass der nächtliche Löschlauf eingeschaltet
   ist und erfolgreich läuft. Das gilt auch für inaktive Schulen.
3. Mit freigegebenen synthetischen Daten das Ausschalten und Zurücksetzen
   prüfen: ältere abgeschlossene Besuche verschwinden, heutige und offene
   bleiben, Löschprotokoll und Schulbezug stimmen.
4. Nach dem nächsten Nachtlauf Löschung und Audit prüfen. Fehler des Nachtlaufs
   beheben; bloßes Ausblenden ist kein Nachweis physischer Löschung.
5. Verantwortlichen-Nachweis ohne Personendaten außerhalb dieses PRs ablegen.

Backups und Wiederherstellung bleiben Gegenstand der getrennten Verfahren zu
#3836 und #3837. Ein Deployment allein belegt diese Betriebsmaßnahmen nicht.
