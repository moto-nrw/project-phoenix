# Nächtliche Löschung schlägt wiederholt fehl

Alarm `nightly-cleanup-failing` (#6327) aus
[`alerting/nightly-cleanup.yml`](../grafana/provisioning/alerting/nightly-cleanup.yml).
Er meldet einen nächtlichen Löschjob, der bei einer Schule mindestens dreimal
in Folge fehlgeschlagen ist. Ein Alarm gilt für genau eine Schule und einen Job.

## Verhalten des Workers

Jeder nächtliche Löschjob (`visit-cleanup`, `timetable-cleanup`,
`time-tracking-cleanup`, `student-change-log-cleanup`, `pwa-usage-cleanup`,
`staff-message-cleanup`) führt je Schule eine Fehlerserie in
`platform.cleanup_runs` (`consecutive_failures`, `last_failed_at`) neben dem
letzten Erfolg (`succeeded_at`).

| Fehlschläge in Folge | Wartezeit bis zum nächsten Versuch |
|---:|---|
| 1 | 1 Stunde |
| 2 | 2 Stunden |
| 3 | 4 Stunden |
| 4 | 8 Stunden |
| 5 | 16 Stunden |
| ab 6 | 24 Stunden |

- Die Wartezeit gilt auch zur eingestellten Uhrzeit und über den Tageswechsel.
  Ein neuer Tag setzt die Serie nicht zurück.
- Die Serie steht in der Datenbank. Ein Neustart des Workers setzt sie fort.
- Nur ein erfolgreicher Lauf setzt die Serie auf 0.
- Ein nicht lesbarer Schalter `gdpr.data_cleanup_enabled` und eine nicht
  lesbare oder ungültige Uhrzeit `gdpr.data_cleanup_time` zählen als
  Fehlschlag (`failed_steps=run`). Ohne Uhrzeit ist der nächste Versuch fällig,
  sobald die Wartezeit abgelaufen ist.
- Jede Schule und jeder Job hat eine eigene Serie. Ein Fehler hält keine andere
  Schule und keinen anderen Job auf.
- Ein fehlgeschlagener Schritt der Hauptlöschung (`visit-cleanup`) macht nur
  seine eigenen Änderungen rückgängig. Die übrigen Schritte werden gespeichert,
  der Lauf bleibt fällig (#6288).

## Signale

| Logzeile (`msg`) | Level | Wann | Felder |
|---|---|---|---|
| `nightly cleanup failing repeatedly` | ERROR | einmal je fehlgeschlagenem Lauf ab dem 3. Fehlschlag in Folge | `tenant_id`, `job_id`, `consecutive_failures`, `failed_steps`, `next_attempt_at`, `error` |
| `nightly cleanup recovered` | INFO | erster Erfolg nach mindestens einem Fehlschlag, auch wenn der Alarm-Fehlschlag nie gespeichert wurde | `tenant_id`, `job_id`, `consecutive_failures` (0), `previous_failures` |
| `cleanup step failed` | ERROR | jeder fehlgeschlagene Schritt, jeder Lauf | `tenant_id`, `step`, `error` |

`failed_steps` nennt die fehlgeschlagenen Schritte der Hauptlöschung, z. B.
`feedback,retention`. `run` bedeutet: Der ganze Lauf ist gescheitert, etwa an
Einstellungen, an der Transaktion oder am Speichern des Ergebnisses. Die Zeilen
enthalten nur IDs, keine Namen.

## Prüfabfrage

Die Alarmregel nimmt je Schule und Job den letzten Wert von
`consecutive_failures` der beiden ersten Zeilen aus den letzten 30 Stunden: höchstens
24 Stunden Wartezeit plus bis zu 2 Stunden Laufzeit liegen zwischen zwei
Fehlerzeilen. Sie alarmiert ab 3. In Grafana Explore (Loki):

```logql
max by (tenant_id, job_id) (last_over_time({env="prod",service="server"} |= "nightly cleanup" | json | msg=~"nightly cleanup (failing repeatedly|recovered)" | keep tenant_id, job_id, consecutive_failures | unwrap consecutive_failures [30h]))
```

Read-only in der Datenbank, nur mit dem Admin-Zugang des Betriebs:

```sql
SELECT tenant_id, job_id, succeeded_at, consecutive_failures, last_failed_at
FROM platform.cleanup_runs
WHERE consecutive_failures > 0
ORDER BY consecutive_failures DESC;
```

## Alarm bearbeiten

1. Aus dem Alarm Schule (`tenant_id`) und Job (`job_id`) ablesen.
2. In Loki die letzte Zeile `nightly cleanup failing repeatedly` dieser Schule
   lesen: `failed_steps`, `error`, `next_attempt_at`.
3. Die Zeilen `cleanup step failed` derselben Schule zeigen den genauen Fehler
   je Schritt.
4. Ursache beheben, z. B. eine nicht lesbare Einstellung der Schule oder ein
   Datensatz, an dem der Schritt scheitert. Daten nur nach fachlicher Prüfung
   ändern.
5. Auf den nächsten Versuch warten (`next_attempt_at`). Einen Lauf nicht von
   Hand in Production auslösen.

## Erholung

Der erste erfolgreiche Lauf schreibt `nightly cleanup recovered` mit
`consecutive_failures=0`. Damit ist der letzte Wert 0 und der Alarm löst sich
bei der nächsten Auswertung (alle 5 Minuten).

Wird der Job für die Schule nicht mehr ausgeführt (Schalter
`gdpr.data_cleanup_enabled` aus, Modul abbestellt), kommt keine neue Zeile. Der
Alarm endet dann nach 30 Stunden ohne Zeile (`noDataState: OK`). Die Serie in
`platform.cleanup_runs` bleibt bis zum nächsten Erfolg stehen.

## Nach dem Deployment prüfen

1. Zuerst das Backend mit Migration 1.15.834 deployen, dann die Regel mit
   `scripts/deploy-grafana-provisioning.sh <ssh-host> alerting/nightly-cleanup.yml`.
2. Im Grafana-Log keinen Provisioning-Fehler sehen; die Regel steht im Ordner
   `Phoenix Alerts`.
3. Die Zustellung nur in Staging prüfen: eine Kopie der Regel mit
   `env="staging"` anlegen und dort eine Schule mit unlesbarer Einstellung
   `gdpr.data_cleanup_timeout_minutes` dreimal fehlschlagen lassen. Die
   Benachrichtigung über den bestehenden Alarmkanal bestätigen, danach die
   Einstellung reparieren und das Ende des Alarms prüfen. Die Production-Regel
   nicht zum Testen ändern oder pausieren.

Die LogQL-Abfrage prüft
[`scripts/nightly-cleanup-alerts.integration.test.mjs`](../../scripts/nightly-cleanup-alerts.integration.test.mjs)
gegen ein isoliertes Loki: ein Alarm je Schule und Job, auch 27 Stunden nach dem
letzten Fehlschlag, Ende durch die Erholungszeile, keine Wirkung von
Schrittfehlern, Staging-Zeilen und Zeilen älter als 30 Stunden.
