# Server- und Postgres-Fehler

Zwei Regeln aus `grafana/provisioning/alerting/server-errors.yml` (#2953):

| Regel | Quelle | Schwelle |
|---|---|---|
| Server error spike | `{env="prod",service="server"}`, `level="ERROR"` | mehr als 10 Zeilen in 5 Minuten |
| Postgres errors | `{env="prod",service="postgres"}`, `ERROR:` oder `PANIC:` | mehr als 5 Zeilen in 5 Minuten |

## Was zählt, was nicht

Der Server rollt Tenant-Transaktionen auch hinter 401, 404 oder 410 zurück.
Seit #2953 schreibt er dafür keine ERROR-Zeile mehr; die Request-Zeile von
slog-chi trägt den Status auf WARN. Eine Zeile `runtime operation failed` mit
`outcome=transaction_failure` erscheint nur noch hinter einem 5xx und enthält
`method`, `route`, `path` und `status`. Die Regel filtert zusätzlich
`transaction_failure` ohne 5xx-Status heraus, damit ein älteres Image den
Alarm nicht auslöst.

`FATAL:` im Postgres-Log zählt absichtlich nicht: das Herunterfahren beim
Deploy beendet Verbindungen mit `FATAL: terminating connection`, und
Anmeldefehler stehen als `FATAL: password authentication failed`. Beides ist
ein eigenes Signal, kein Query-Fehler.

## Alarm bearbeiten

1. In Grafana Explore die Zeilen der letzten 15 Minuten öffnen. Beim
   Server-Alarm nach `route` und `status` gruppieren, beim Postgres-Alarm nach
   dem Text hinter `ERROR:`.
2. `correlation_id` der ERROR-Zeile gegen die Request-Zeile joinen, wenn der
   Kontext fehlt.
3. `permission denied for schema ...` oder `relation ... does not exist` nach
   einem Deploy: Migration und Rollen prüfen (`deploy-remote.sh`, Exit-Codes
   im Root-`CLAUDE.md`).
4. `deadlock detected` oder `could not serialize access`: der Server
   wiederholt diese Transaktionen selbst. Nur handeln, wenn die Zahl über
   Stunden steigt; dann die betroffene Route im Server-Log suchen.
5. Ein einzelner Client mit vielen 4xx ist kein Fehler. Taucht er trotzdem im
   Alarm auf, läuft ein Image von vor #2953.

## Bekannte Grenzen

- Die Loki-Labels `service="server"` und `service="postgres"` folgen den
  journald-Tags `prod-server` und `prod-postgres` aus
  `environments/production.compose.yml`. Bei einer Änderung am Alloy-Mapping
  beide Regeln anpassen.
- `response_write_failure` (Client hat die Verbindung abgebrochen) bleibt
  ERROR und zählt. Steigt das dauerhaft, die Schwelle oder die Einstufung
  im Server prüfen.
- Staging ist nicht abgedeckt; die Regeln sind bewusst auf `env="prod"`
  begrenzt wie die Buchungs-Audit-Regeln.
