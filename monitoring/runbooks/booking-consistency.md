# Buchungs-Konsistenz-Audit

Das tägliche Audit prüft die buchungsgeführte Betreuung pro Mandant. Die Logs
enthalten nur Mandanten-IDs, Audit-Datum und aggregierte Zähler. Sie enthalten
keine Namen, Kind-IDs oder andere personenbezogene Daten.

## Signale

| Feld | Bedeutung | Aktion |
|---|---|---|
| `pickup_projection_missing_days` | Gebuchter Betreuungstag ohne gültige effektive Abholzeit | Buchung, Angebots-Gehzeit und individuelle Abweichung prüfen. |
| `approved_without_required_offering` | Genehmigtes Kind ohne erforderliches, auswählbares Betreuungsangebot | Anmeldung und wirksame Angebotsbuchung prüfen. |
| `approved_without_optional_offering` | Optionales Angebot wurde nicht gewählt | Nur prüfen; kein Fehler und kein Alarm. |
| `total_findings` | Summe der beiden handlungsfähigen Zähler | Nicht pauschal auf einen bekannten Bestand alarmieren. |

## Alarm bearbeiten

1. Im Dashboard **Phoenix Booking Consistency** Mandant, Audit-Datum und
   betroffene Kategorie ablesen.
2. Die Mandanten-ID der zuständigen Schule zuordnen.
3. Den aktuellen Wert mit dem unten dokumentierten Ausgangswert vergleichen.
4. Buchungen, Angebotszeiten und individuelle Zeitabweichungen read-only prüfen.
5. Daten nur nach fachlicher Bestätigung durch die zuständige Schule ändern.
6. Nach einer fachlich bestätigten Korrektur den Ausgangswert in Alert-Regel und
   Runbook gemeinsam anpassen. Der Alarm löst sich auf, sobald der Wert nicht
   mehr über diesem Ausgangswert liegt beziehungsweise der technische Fehler endet.

## Bekannter Ausgangswert

Der read-only Production-Lauf vom 29.08.2026, noch vor dem Deployment des
reduzierten Logvertrags, enthielt bei den weiterhin gültigen Feldern:

| Mandant | `pickup_projection_missing_days` | `approved_without_required_offering` | `approved_without_optional_offering` |
|---|---:|---:|---:|
| `4` | 10 | 0 | 1 |
| `5` | 5 | 0 | 3 |

Die übrigen erfolgreichen Mandanten meldeten für diese Felder null. Die alten
`total_findings`-Werte sind kein Ausgangswert, weil sie noch die drei entfernten
Zähler enthielten. Den Ausgangswert nach dem Backend-Deployment mit dem ersten
neuen Lauf bestätigen; erst danach Datenabweichungen fachlich bearbeiten. Die
Alert-Regel codiert die beiden von null abweichenden handlungsfähigen Werte
explizit. Für alle anderen Mandanten und Kategorien gilt null.

## Technischer Fehler

- `booking consistency audit failed`: Scheduler- und Datenbankfehler im
  Gesamtlauf prüfen.
- `operation=booking-consistency-audit`: Der angegebene Mandant ist
  fehlgeschlagen; die übrigen Mandanten wurden weiter verarbeitet.
- Kein Ergebnis seit 26 Stunden: Production-Server, Scheduler und Loki prüfen.
  Danach muss für jeden aktiven Mandanten genau ein Ergebnis-Log vorhanden sein.

## Nach dem Deployment prüfen

1. Zuerst den Backend-Stand mit dem reduzierten Logvertrag deployen.
2. Im neuesten Production-Lauf müssen nur diese Zähler vorkommen:
   `pickup_projection_missing_days`, `approved_without_required_offering`,
   `approved_without_optional_offering` und `total_findings`.
3. Prüfen, dass pro aktivem Mandanten ein Ergebnis vorliegt und keine
   personenbezogenen Felder enthalten sind.
4. Einen technischen Testfehler ausschließlich in Staging auslösen und die
   Benachrichtigung über den bestehenden Grafana-Alarmkanal bestätigen.
5. Den 26-Stunden-Alarm in Staging mit einer temporär verkürzten Testregel
   prüfen. Die Production-Regel nicht zum Testen pausieren oder verändern.
