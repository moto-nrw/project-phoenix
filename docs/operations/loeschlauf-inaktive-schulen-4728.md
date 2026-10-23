# Löschlauf nach Deaktivierung oder Soft Delete prüfen

Slice 4728-a, Teil von #4728 und #3846. Keine vollständige Mandantenlöschung
(#3837), keine neue Frist und kein neuer Schalter.

## Stand und lokaler Nachweis

**Im Code umgesetzt:** Die nächtlichen DSGVO-Jobs bekommen eine eigene
Schulliste einschließlich inaktiver und soft-gelöschter Schulen. Die übrigen
Jobs behalten ihre bisherigen Listen. Je Schule gelten weiter ihre Settings,
die Tenant-Transaktion mit RLS und der dauerhafte Erfolgsvermerk in
`platform.cleanup_runs`. Eine vorhandene Notbremse
(`gdpr.data_cleanup_enabled = false`) bleibt wirksam. Ohne Erfolgsvermerk
wartet eine Schule auf ihre eingestellte Minute; verpasste Läufe mit älterem
Erfolgsvermerk werden beim nächsten Tick nachgeholt. Zurückgegebene Fehler
setzen keinen Erfolgsvermerk und hindern andere Schulen nicht am Lauf.

**Lokal geprüft:** `backend/modules/schedulerruntime/compose/cleanup_schools_test.go`
prüft mit synthetischen Daten und der Rolle `phoenix_auth` aktive, inaktive
und soft-gelöschte Schulen, eigene Fristen und Registry-Voreinstellungen,
die Notbremse einer anderen Schule, Nachholen nach einem Neustart sowie
Settings- und Transaktionsfehler. Ausführen aus `backend/`:

```sh
../scripts/run-go-toolchain.sh go test -tags=testhooks ./modules/schedulerruntime/compose -run 'Test(NightlyCleanup|Cleanup.*Failure)' -count=1
```

**Produktiv nachgewiesen: offen.** Dieser Slice greift weder auf Produktion
noch auf Staging zu. Das Deployment und der folgende Nachweis sind
Mensch-Schritte. Die [Fristenliste](../reference/loeschfristen.md) ist die
fachliche Quelle; die lokalen Tests belegen keinen produktiven Stand.

## Mensch-Schritte nach dem Deployment

1. Release und Head-SHA festhalten. Vor dem ersten Lauf prüfen, ob die
   bestehenden Fristen und bewusst gesetzten Notbremsen noch gelten. Neu
   erfasste Schulen können beim nächsten fälligen Lauf ältere Daten löschen.
   Keine Notbremse allein für diesen Nachweis ausschalten und keine echte
   Schule zu Testzwecken deaktivieren oder löschen.
2. Vor und nach dem nächsten eingestellten Termin die folgende reine
   Zählabfrage über den freigegebenen Datenbankzugang ausführen. Sie gibt
   keine Schulnamen, Personeninhalte oder Zugangsdaten aus. Keine
   Datenbank-Zugangsdaten oder ungefilterten Logs in PRs übernehmen.
3. Bei inaktiven und soft-gelöschten Schulen mit eingeschaltetem Lauf müssen
   die fälligen Jobs nach ihrem Termin einen aktuellen Erfolgsvermerk haben.
   Gewollte Notbremsen getrennt prüfen. Der Worker meldet einen Lauf mit
   Schulfehler als `failed`, nicht als Gesamterfolg. Fehler beheben und den
   Nachhollauf abwarten; Erfolgsvermerke niemals von Hand setzen.
4. Zeitpunkt, Release, aggregierte Zählwerte, Ergebnis und offene Fehler im
   internen Betriebsnachweis festhalten. Erst danach den produktiven Nachweis
   dieses Slices als erbracht markieren. Vollständige Mandantenlöschung und
   Sicherungskopien bleiben die separaten Verfahren #3837 und #3836.

```sql
WITH jobs(job_id) AS (
    VALUES ('visit-cleanup'), ('timetable-cleanup'),
           ('time-tracking-cleanup'), ('student-change-log-cleanup'),
           ('pwa-usage-cleanup'), ('staff-message-cleanup')
), schools AS (
    SELECT id,
           CASE WHEN deleted_at IS NOT NULL THEN 'soft-deleted'
                WHEN NOT active THEN 'inactive' ELSE 'active' END AS state
    FROM platform.schools
)
SELECT schools.state, jobs.job_id,
       count(*) AS schools,
       count(run.succeeded_at) AS with_success,
       count(*) FILTER (
           WHERE (run.succeeded_at AT TIME ZONE 'Europe/Berlin')::date =
                 (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date
       ) AS succeeded_today
FROM schools CROSS JOIN jobs
LEFT JOIN platform.cleanup_runs AS run
       ON run.tenant_id = schools.id AND run.job_id = jobs.job_id
GROUP BY schools.state, jobs.job_id
ORDER BY schools.state, jobs.job_id;
```

Die Zählung allein belegt weder die Löschung jeder Datenart noch, ob eine
Schule bewusst angehalten wurde. Abweichungen werden anhand der bestehenden
Settings und der Job-Fehlermeldungen untersucht, ohne Personeninhalte in den
Nachweis zu kopieren. In der isolierten Demo-Umgebung gilt dieselbe Änderung
auch für stillgelegte Demo-Schulen (ADR 0029); der stündliche Demo-Betrieb
für Besuchernamen bleibt ein eigener Löschweg.
