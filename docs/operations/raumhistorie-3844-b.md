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

### Ergänzung der Erhebung, Runde 3

Die folgenden internen Lesewege nutzen **nicht** den Schutz von `/active/visits*`.
Sie können bei ausgeschaltetem Protokoll ältere Abschlüsse bis zur Löschung
lesen. Die Fundstellen beziehen sich auf den PR-Stand nach Runde 2.

| Leseweg | Fundstelle unter `backend/` | Einordnung nach Löschung |
|---|---|---|
| Aufsichts-/Terminliste, `/api/timetable/operations/instances/{id}/roster` und `/active-groups/{id}/roster` | `modules/timetable/internal/application/operations_roster.go:133` | Liest Besuche eines Termins ohne Tagesgrenze. `rosterRow` liefert Besuchs-ID und Eintrittszeit. Reguläre Status-/Anwesenheitsdaten bleiben in `schedule.instance_students`; reine Besuchsinformationen und nicht gespiegelte Spontanteilnahmen können entfallen. |
| Wochenplan eines Kindes, `GET /api/timetable/student/{id}/week` | `modules/timetable/internal/application/timetable_data_student_week.go:70` | Liest Besuche auch abgeschlossener Termine und liefert IDs, Eintritt und Austritt. Gespiegelte Teilnahme bleibt separat unter `EnrolledByDate`; `VisitsByActiveGroup` verliert gelöschte Besuche. Keine vollständige Gleichwertigkeit durch den Mirror. |
| Tageslisten, Vorschau/Export/Abgleich | `modules/classday/internal/application/slotlist_slots.go:211` | Besuche dienen als zusätzliche Anwesenheitsevidenz für gestartete Termine. Reguläre Teilnahmen bleiben im Terminroster; nur besuchsgestützte Evidenz fällt nach Löschung weg. |
| Tageslisten-Auswahl und Mengen | `modules/classday/internal/application/slotlist_options.go:169` | Gleicher Rückfallweg für besuchsgestützte Teilnahme, etwa bei abweichendem Mirror-/Altbestand. Die Klassifikation kann sich nach Löschung ändern, wenn die entsprechende Rosterzeile fehlt oder keinen Anwesenheitsstatus trägt. |
| Raum-Sitzungsverlauf | `modules/studentpresence/internal/adapters/postgres/room_history.go:32` | Zählt unterschiedliche Kinder pro Raumsitzung, liefert hier keine einzelnen Besuchszeiten oder Kinder. Diese Anzahl kann nach Löschung sinken; Sitzungs-/Aufsichtsdaten bleiben separat bestehen. Kein Ersatz durch den Attendance-Mirror in dieser Abfrage. |

`attendance_mirror.go:19` und `attendance_mirror_batch.go` unter
`modules/timetable/internal/application/` spiegeln reguläre An-/Abmeldungen
in `schedule.instance_students`. Fehlende Terminzuordnungen sind jedoch
zulässige No-ops. Der Mirror belegt deshalb den Erhalt regulärer Teilnahme,
nicht den Erhalt jeder Besuchsinformation oder jedes Altbestands. Ein Umbau
dieser Lesewege ist nicht Teil dieser Nacharbeit. Die pauschale Aussage, kein
Leseweg brauche ältere Besuche, ist durch die Erhebung **nicht** belegt.
Tagesdaten und offene Besuche bleiben gemäß Entscheidung vom 23.10.2026
erhalten. Das ist keine Feststellung einer Rechtsgrundlage.

## Im Code umgesetzt

1. Der Nachtlauf löst die Einstellung je Schule auf. Ausgeschaltet zählt das
   Besuchsende vor heutiger Berliner Mitternacht, nicht eine rollende
   24-Stunden-Frist oder die Einwilligungsfrist. Offene Besuche bleiben.
2. Ausschalten und Zurücksetzen auf `false` löschen ältere Abschlüsse sofort.
   Einstellung, Löschungen und Löschprotokoll stehen in derselben Transaktion.
   Ein Audit- oder Löschfehler darf keinen erfolgreichen Wechsel hinterlassen.
   Der Hook greift auch bei `gdpr.data_cleanup_enabled=false`: Ausschalten
   ist eine bewusste Entscheidung der Schule. Die Notbremse hält nur automatische
   Läufe an, nicht diese ausdrückliche Löschung, wie beim Ausschalten von Fotos.
3. Die HTTP-Lesewege `/active/visits*` (Listen, Gruppen, Einzelabruf) beschränken abgeschlossene
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

**Zuständigkeit:** Eine von der Betriebsleitung beauftragte moto-Operatorperson
mit bestehendem Operatorzugang sowie freigegebenem Container-/DB-Lesezugang
prüft den Betrieb. Die Schuladministration (`config:manage`) entscheidet über
das Anwesenheitsprotokoll mit dem Verantwortlichen. Nur der moto-Operator darf
die automatische Löschung anhalten. Keine zusätzlichen Rechte für den Nachweis
vergeben. Die folgenden Befehle sind vorbereitete Mensch-Schritte, hier nicht
gegen Produktion/Staging ausgeführt; keine Hostnamen oder Secrets eintragen.

### 1. Einstellungen und Laufbereitschaft prüfen

Im Operatorportal: **Schulen > betroffene Schule > Einstellungen > Datenschutz
> Datenbereinigung**. Dort effektiven Wert von „Alte Daten automatisch löschen“
(`gdpr.data_cleanup_enabled`, Standard `true`) und „Uhrzeit für das Löschen“
(`gdpr.data_cleanup_time`, Standard `02:00`, Berlin) prüfen, auch bei inaktiven
Schulen. `false` hält automatische Läufe dieser Schule an. Im Schulportal unter
**Einstellungen > Datenschutz** den Wert des Anwesenheitsprotokolls prüfen.
Zusätzlich muss der Backend-Scheduler laufen (`CLEANUP_SCHEDULER_ENABLED` darf
nicht `false` sein). Nur diese Werte prüfen, niemals ganze Umgebungen ausgeben.
Deployment-Commit, Schulbezug und freigegebene Rechtsgrundlage intern erfassen.

### 2. Trockenlauf im Container

Im geprüften Deployment-Verzeichnis mit dessen bestehender Compose-/Env-Auswahl:

```sh
docker compose run --rm --no-deps -T migrate ./main cleanup visits --dry-run
```

Erwartet: `DRY RUN MODE - No data will be deleted`, je Schule
`Total visits to delete`, ältester Zeitpunkt und Anzahl betroffener Kinder.
Kein `--verbose`: keine Kind-IDs in den Nachweis übernehmen. Der Aufruf ist
mandantenübergreifend und läuft auch bei angehaltenem Nachtlauf. Er löscht
nichts, ersetzt aber weder Nachtlauf noch Ausschalt-Hook. Nicht `--dry-run`
weglassen; `cleanup visits preview` wäre eine echte Löschung. Die
Produktions-Compose nennt den Wartungsdienst `migrate`; nur er erhält den
privilegierten DB-Zugang. Der Befehl überschreibt dessen Standardkommando,
er führt **keine Migration** aus. Das Image muss dem ausgerollten Backend
entsprechen. Nicht `exec server` verwenden oder dessen Anwendungsrolle erweitern.
Ohne bestehende freigegebene Compose-/Env-Auswahl bleibt der Trockenlauf offen.

### 3. Lesender Mengen- und Laufnachweis

Den folgenden SQL-Block als `/tmp/raumhistorie-nachweis.sql` speichern. Für
`school_id` die intern bestätigte numerische Schul-ID setzen; keine Namen.
Mit bestehendem berechtigtem Lesezugang ausführen, im Compose-Standard etwa:

```sh
docker compose exec -T postgres psql -X -v ON_ERROR_STOP=1 \
  -U postgres -d postgres -v school_id=123 < /tmp/raumhistorie-nachweis.sql
```

`123` ist ein Platzhalter, keine Test-/Produktivschule. Keine DSN ausgeben.
Die Transaktion erzwingt nur lesende Abfragen; `app.current_tenant_id` setzt
zusätzlich den Schulkontext für RLS. Jede Abfrage bleibt explizit schulbezogen.

```sql
BEGIN READ ONLY;
SELECT set_config('app.current_tenant_id', :'school_id', true);
SELECT count(*) FILTER (WHERE exit_time IS NOT NULL AND exit_time <
         (date_trunc('day', now() AT TIME ZONE 'Europe/Berlin')
          AT TIME ZONE 'Europe/Berlin')) AS alte_abschluesse,
       count(*) FILTER (WHERE exit_time IS NULL) AS offene_besuche,
       count(*) FILTER (WHERE exit_time >=
         (date_trunc('day', now() AT TIME ZONE 'Europe/Berlin')
          AT TIME ZONE 'Europe/Berlin')) AS heutige_abschluesse
FROM active.visits WHERE tenant_id = :'school_id'::bigint;
SELECT date_trunc('minute', deleted_at) AS zeitpunkt,
       count(*) AS protokolleintraege, sum(records_deleted) AS geloeschte_besuche
FROM audit.data_deletions
WHERE tenant_id = :'school_id'::bigint AND deletion_type = 'visit_retention'
  AND deleted_at >= now() - interval '2 days'
GROUP BY 1 ORDER BY 1;
SELECT succeeded_at FROM platform.cleanup_runs
WHERE tenant_id = :'school_id'::bigint AND job_id = 'visit-cleanup';
ROLLBACK;
```

Nur Mengen/Zeitpunkte sichern, keine Schüler-IDs, Rohmetadaten oder Besuchszeilen.
Bei ausgeschaltetem Protokoll müssen ältere Abschlüsse nach erfolgreichem Lauf
null sein. Keine Auditzeile bei null Löschkandidaten ist kein Fehler. Die
Laufmarke muss das letzte fällige Berliner Zeitfenster abdecken; sie wird erst
mit erfolgreicher Transaktion gespeichert. Ein Log allein ist kein Commitbeleg.

### 4. Nachtlauf, Fehler und Alarm prüfen

Grafana **Explore > Loki**, Backend-Stream der freigegebenen Umgebung wählen,
beispielsweise für das eingecheckte Produktionslabel:

```logql
{env="prod", service="server"} | json | tenant_id="123"
  |~ "running data cleanup for tenant|cleanup completed for tenant|cleanup failed for tenant|tenant operation failed, continuing to next tenant"
```

Schul-ID ersetzen. Erfolg: `cleanup completed for tenant` mit `records_deleted`
und aktuelle Laufmarke aus Schritt 3. Fehler: `cleanup failed for tenant` oder
`tenant operation failed, continuing to next tenant`; bei Letzterem `job_id`
auf `visit-cleanup` prüfen. Ohne Loki den gleichen begrenzten Zeitraum in den
Logs des Dienstes `server` prüfen. Nur Status, Zeitpunkt und Mengen übernehmen,
keine kompletten Fehlermeldungen mit möglichen Nutzdaten.

**Alarmgrenze:** Das Repo provisioniert aktuell nur HTTP-5xx-, DB-Verbindungs-
und Serverausfall-Alarme, keinen eigenen Löschlauf-Alarm. Ein vorhandener
produktiver Alarm ist daher separat nachzuweisen. Der zuständige Operator
prüft Alarmregel, Empfänger und synthetischen Alarmtest in der Testumgebung:
Fehler im obigen Stream sowie fehlende aktuelle Laufmarke nach Fälligkeit plus
Timeout (`gdpr.data_cleanup_timeout_minutes`, Standard 30 Minuten) müssen
auffallen. Auch ein still ausbleibender Lauf braucht einen Alarm oder eine
bis zur Einrichtung verbindlich zugewiesene tägliche manuelle Kontrolle von
Schritt 3. Fehlende Abdeckung als offenen Betriebsnachweis protokollieren.

### 5. Ausschalten ohne synthetische Produktivdaten nachweisen

1. Den vollständigen Verhaltenstest in einer isolierten Testschule auf lokaler
   Test-DB oder freigegebenem Staging durchführen: vorhandene synthetische
   Fixtures mit älteren/heutigen Abschlüssen und offenem Besuch verwenden.
   Auch `gdpr.data_cleanup_enabled=false` testen. Ausschalten bzw. Reset auf
   `false` muss nur ältere Abschlüsse samt Audit löschen. Reguläre
   Teilnahmeinformationen und die in der Erhebung genannten Grenzen prüfen.
   Keine synthetischen Besuche in Produktion anlegen und keine realen
   Besuchszeiten umdatieren.
2. Produktiv nur bei einer ohnehin freigegebenen Ausschaltentscheidung der
   Schule: Mengenabfrage aus Schritt 3 unmittelbar vorher/nachher ausführen
   und Audit-Zeitpunkt abgleichen. Nicht für diesen Test ein ausgeschaltetes
   Protokoll wieder einschalten. Bei null Altfällen ist der produktive
   Löschfall nicht beobachtet, nur Einstellung/Transaktion belegt; den
   Verhaltenstest der isolierten Umgebung separat referenzieren.
3. Nach dem nächsten fälligen Nachtlauf Schritt 3 und 4 wiederholen, auch für
   inaktive Schulen. Bei angehaltener Automatik deren Wiederaufnahme durch
   den Operator dokumentieren. Ausfälle können Löschung und die ungeschützten
   internen Lesewege verzögern; nur `/active/visits*` ist schon vorher begrenzt.
4. Datum, ausgerollten Commit, Rollen, Mengen, Lauf-/Auditzeitpunkt und offene
   Punkte im zugriffsbeschränkten Betriebsnachweis ablegen, keine Personendaten.

Backups und Wiederherstellung bleiben Gegenstand der getrennten Verfahren zu
#3836 und #3837. Ein Deployment allein belegt diese Betriebsmaßnahmen nicht.
