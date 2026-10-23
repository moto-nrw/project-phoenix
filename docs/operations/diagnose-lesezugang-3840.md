# Diagnose-Lesezugang ohne Finanz- und Gesundheitsdaten

Stand: 23.10.2026, Slice 3840-f, Teil von #3840 und #3846.

**Im Code umgesetzt:** Migration `1.15.488` legt `phoenix_diagnostics` als
`NOLOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`
an. Die Rolle ist kein Mitglied anderer Rollen und wird nicht an App-Rollen
vergeben. Die Migration enthält weder LOGIN-Zugänge noch Zugangsdaten.
Die vorausgesetzte Migration `1.15.481` ersetzt PUBLIC-Rechte auf
`auth.mfa_overrides` und dessen Sequenz durch ausdrückliche Grants an
`phoenix_auth`, `phoenix_tenant`, `phoenix_admin` und `phoenix_demo`.
`1.15.488` verändert diese Rechte weder bei Up noch bei Down; die RLS-Policies
bleiben erhalten.
**Produktiv nachzuweisen:** Rollout, persönliche Zugänge, effektive Rechte,
Ablösung bisheriger Superuser-Diagnose und Entzug alter Zugänge. In diesem
Slice wurden ausschließlich lokale synthetische Testdaten verwendet.

## Erlaubter Umfang und Grenzen

Die Positivliste umfasst nur `id`, `tenant_id`, `created_at`, `updated_at` in
`facilities.rooms`, `education.groups` und `activities.groups`. Damit lassen
sich Existenz, Anzahl, Schulzuordnung und Aktualisierung von Räumen,
OGS-Gruppen und Angeboten prüfen. Namen und Freitext sind nicht erforderlich.
Spalten-GRANTs statt Tabellen-GRANTs verhindern, dass neue Spalten automatisch
lesbar werden. Neue Tabellen erhalten keine Rechte; es gibt keine Default-GRANTs.

Alle übrigen Anwendungstabellen und Views bleiben ausgeschlossen. Das umfasst
insbesondere:

- Finanzdaten von Personal und Eltern, Betreuungsprofile mit `health_info`;
- das gesamte `audit`-Schema, insbesondere Personal-/Finanzänderungen,
  Kinderänderungen und Zugriffsprotokolle, auch bei maskierten Alt-/Neuwerten;
- `users.student_data_change_requests`, `config.setting_audit`, Anmeldedaten
  unter `enrollment`, Krankheitstage und Personalabwesenheiten unter `active`;
- Notizen, Nachrichten, Konten, Zugangsdaten, Dateien und Dokumentmetadaten.
- Löschaufträge und Abrechnungsstatistik unter `platform`, einschließlich
  `school_erasures`, `billing_key_date_counts` und `live_billing_key_date_counts`.

Die ganze Tabelle auszuschließen schützt auch vor Gesundheitsangaben in
Freitext und unbekannten Protokollkopien. Der
[Datenkatalog](../reference/datenkatalog.md) bleibt das Bestandsverzeichnis;
eine neue Datenart erfordert keine wachsende Ausschlussliste. Der Katalogtest
prüft stattdessen alle effektiven Anwendungsspalten gegen die zwölf erlaubten
Spalten, einschließlich Views und PUBLIC-Rechten. Er prüft außerdem, dass
Tabellen-/Schreibrechte, Sequenzen und aufrufbare Anwendungsfunktionen mit
`SECURITY DEFINER` ausgeschlossen sind. Das ersetzt keine Prüfung der produktiven Rechte.

**RLS bleibt aktiv, ohne BYPASSRLS.** Die bestehenden PUBLIC-Policies der drei
Tabellen vergleichen `tenant_id` mit `app.current_tenant_id`. Ohne Schul-ID
sind keine Zeilen sichtbar; eine Transaktion liest nur die gewählte Schule.
Das ist eine interne Betriebsrolle, kein schulgebundener Kundenzugang:
Berechtigte können die Schul-ID selbst wählen. RLS verhindert hier vergessene
Filter, nicht die bewusste Wahl einer anderen Schule. Diagnose nur mit
dokumentiertem Auftrag und für die dafür freigegebene Schule ausführen.
Keine Mitgliedschaft in `phoenix_tenant`, `phoenix_admin`, `phoenix_auth`,
`phoenix_demo`, `phoenix_school_erasure`, `phoenix_billing_retention` oder
privilegierten PostgreSQL-Rollen ergänzen.

Systemkatalog-Metadaten bleiben nach PostgreSQL-Standard sichtbar. Die Rolle
hat außerdem die standardmäßige Statistikansicht der Erweiterung
`pg_stat_statements`, jedoch keine Mitgliedschaft in `pg_read_all_stats`:
SQL-Texte und Query-IDs anderer DB-Nutzer bleiben nach PostgreSQL ausgeblendet.
Nur diese Erweiterungsansichten sind vom Anwendungsspalten-Katalogtest
ausgenommen, nicht beliebige zukünftige Erweiterungen.
Die Rolle ist kein Schutz vor DB-Superusern, Host-/Dateizugriff, gestohlenen Dumps oder
kompromittierten App-Zugängen. Sie gewährt keine Upload- oder Backuprechte.
Andere Diagnosefragen benötigen eine gesondert geprüfte, datenarme Abfrage
oder Projektion, keinen pauschalen Superuser-Ersatz.

## Persönlichen Zugang einrichten und alte Diagnose ablösen

Durch den berechtigten Betrieb in einer freigegebenen Admin-Sitzung durchführen.
Keine Ausführung durch Agents auf Produktion oder Staging. Verbindung und
TLS-Serverprüfung nach dem betrieblichen Zugangskonzept herstellen; keine
Zugangsdaten in Kommandozeilen, Dateien dieses Repos, Chat oder Nachweise kopieren.

### Vorbedingungen: Version und persönlicher Zugangsweg

Dieses Runbook benötigt **PostgreSQL ≥17**: `inherit_option` und `set_option`
gibt es seit 16, das abgefragte Recht `MAINTAIN` erst seit 17.
`environments/production.compose.yml` verwendet `postgres:17-alpine`; das ist
kein Nachweis der laufenden Produktionsversion. Vor Einrichtung und Nachweis
in `psql` prüfen und bei `false` abbrechen:

```sql
SELECT current_setting('server_version_num')::integer >= 170000 AS supported_version;
```

Der Betrieb entscheidet ausdrücklich über den MFA-geschützten Netzweg, die
freigegebene Datenbank, den persönlichen Rollennamen, die vom DB-Server gesehene
Quelladresse und die vertrauenswürdige CA. Diese Werte im geschützten
Betriebsauftrag festhalten, nicht in diesem Repository. Keine öffentliche
DB-Freigabe und kein neues Host-SSH-Verfahren aus diesem Runbook ableiten.

1. Einen bestehenden MFA-geschützten VPN-/DB-Proxy-Zugang verwenden. Fehlt er,
   Einrichtung durch den Betrieb als Voraussetzung beauftragen, nicht durch
   eine direkte Internetfreigabe ersetzen. Firewall nur für dessen tatsächliche
   DB-seitige Quelladresse und den PostgreSQL-Port freigeben.
2. In der Admin-Sitzung mit `SHOW hba_file;` die tatsächlich aktive Datei
   ermitteln. Der Betrieb pflegt dort vor allgemeineren Regeln einen Eintrag
   nach folgendem Schema; Platzhalter bewusst durch die freigegebenen Werte
   ersetzen, für IPv4 eine einzelne Adresse mit `/32`, für IPv6 mit `/128`:

   ```text
   hostssl <Datenbank> <persönliche_Rolle> <DB-seitige_Quelladresse/CIDR> scram-sha-256
   ```

   Kein `trust`, keine Gruppenerweiterung auf App-/Adminrollen, kein Netzbereich
   `0.0.0.0/0` oder `::/0`. Nach der Regel für diese persönliche Rolle deren
   sonstige TCP-Wege mit `host all <persönliche_Rolle> 0.0.0.0/0 reject` und
   `host all <persönliche_Rolle> ::/0 reject` sperren. Auch lokale Regeln auf
   Umgehungen prüfen. Die erste passende HBA-Regel gewinnt.
3. Mit `SELECT count(*) FROM pg_hba_file_rules WHERE error IS NOT NULL;`
   auf **0** Fehler prüfen, dann `SELECT pg_reload_conf();` muss `true` liefern.
   Eine zweite Admin-Sitzung für Rücknahme offen halten. Den vorherigen
   Regelstand nur in der geschützten Betriebskonfiguration sichern.
4. Nach Anlage unten vom freigegebenen Netzweg mit `psql -X`,
   `sslmode=verify-full` und der freigegebenen CA verbinden. Der Zielname muss
   zum Serverzertifikat passen. Passwort nur verdeckt eingeben. In dieser
   Sitzung `SELECT ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid();`
   auf `true` prüfen und den untenstehenden Rechtenachweis ausführen.
   Eine Verbindung von einem nicht freigegebenen Weg muss scheitern.

### Rolle anlegen und prüfen

1. Migration ausrollen. Den persönlichen Rollennamen im Beispiel ersetzen.
   Einen eigenen Zugang ohne weitere Mitgliedschaften und Objektbesitz anlegen:

   ```sql
   CREATE ROLE diagnose_person LOGIN NOINHERIT NOSUPERUSER
       NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
   GRANT phoenix_diagnostics TO diagnose_person;
   ```

   Dieses Verfahren setzt das standardmäßige CONNECT über PUBLIC voraus.
   Vor dem Verbindungsversuch als Admin mit
   `SELECT has_database_privilege('diagnose_person', current_database(), 'CONNECT');`
   auf `true` prüfen. Bei `false` abbrechen: Der Betrieb muss einen gesonderten
   Zugangs-/Restore-Entwurf für diese Datenbank freigeben, nicht PUBLIC pauschal
   erweitern. Ein Grant an die Gruppe hilft beim NOINHERIT-Login nicht.
   Keine Rechte direkt an den persönlichen Zugang vergeben, auch kein CONNECT:
   solche Zugänge lehnt der Restore ab.
2. Ein neues Geheimnis im Passwortmanager erzeugen und nur im verdeckten
   interaktiven `psql`-Dialog setzen: `\password diagnose_person`.
   Niemals ein Passwort als SQL-Literal, Shell-Argument oder Chattext verwenden.
   Den persönlichen Zugang im vorhandenen MFA-geschützten Betriebszugangsweg
   bereitstellen; DB-Passwortauthentisierung allein ist kein MFA-Nachweis.
3. Mit dem persönlichen Zugang neu verbinden. Die Rechteprüfung unten ausführen,
   **nicht** bloß `SET ROLE` in einer Superuser-Sitzung. Nur bei genau erwarteten
   Rechten freigeben. Danach eine freigegebene Schul-ID in `psql` eingeben:

   ```sql
   \prompt 'Freigegebene Schul-ID: ' school_id
   BEGIN READ ONLY;
   SET LOCAL ROLE phoenix_diagnostics;
   SELECT set_config('app.current_tenant_id', :'school_id', true);
   SELECT count(id) FROM facilities.rooms;
   COMMIT;
   ```

   Das Beispiel zur Inhaltsdiagnose gehört nicht in den inhaltsfreien
   Rechtenachweis. In produktiven Nachweisen keine Kundendaten/Zeilenzahlen
   oder Screenshots von Abfrageergebnissen ablegen.
4. Bisherige Superuser-Diagnoseprofile und geteilte Zugangsdaten aus dem
   Diagnoseablauf entfernen. Alle bisherigen Berechtigungen und separaten
   Host-/Dateizugänge inventarisieren und unnötige Zugänge entziehen.
   Administrative Migrationen und Notfälle behalten einen getrennten,
   persönlichen, begründeten und protokollierten Adminprozess. Nicht den
   noch für Migrationen benötigten technischen Zugang unkontrolliert sperren.
5. Bei Ende des Auftrags Mitgliedschaft entziehen, persönlichen Zugang auf
   NOLOGIN setzen und vorhandene Diagnose-Sitzungen gezielt beenden. Ein
   Rechteentzug allein beendet bestehende Sitzungen nicht. Passwortmanager-
   Freigabe und Zugangsweg ebenfalls entziehen; Termin und interne Referenz
   dokumentieren.

### Nach einer Wiederherstellung

Der [Tagesbackup-Restore](daily-backup.md) setzt persönliche LOGIN-Rollen nur
dann automatisch auf **NOLOGIN**, wenn sie ausschließlich Mitglied von
`phoenix_diagnostics` ohne ADMIN-/INHERIT-Option sind, SET ROLE erlaubt ist und
keine erhöhten Attribute, individuellen Einstellungen, direkten Rechte oder
Objektbesitz vorliegen. Andere unbekannte LOGIN-Rollen blockieren den Restore.
Die Prüfung direkter Rechte erfolgt nach dem Datenimport; bei Fehler bleibt
die Freigabesperre bestehen. Die Ausgabe nennt nur die Anzahl gesperrter
Diagnosezugänge, keine Rollennamen.

Nach Restore bleiben diese Zugänge gesperrt. Vor bewusster erneuter Freigabe
je Zugang Auftrag und Schulwahl bestätigen, Netzweg/HBA prüfen, Rechte in der
Admin-Sitzung vorprüfen und ein neues Passwort im verdeckten `\password`-Dialog
setzen. Erst dann `ALTER ROLE diagnose_person LOGIN;` ausführen, mit dem
persönlichen Zugang neu verbinden und vor Freigabe den vollständigen Nachweis
unten führen. Nicht benötigte Zugänge bleiben NOLOGIN; ein Backup
reaktiviert keine abgelaufenen Aufträge.

## Inhaltsfreier produktiver Rechtenachweis

Mit dem persönlichen Zugang ausführen; nur die Ergebnisse der folgenden
Katalogabfragen in einem geschützten Betriebsnachweis ablegen. Sie lesen keine
Finanz-, Gesundheits- oder sonstigen Kundeninhalte. Code-/Deploymentversion,
Datum, zuständige Betriebsfunktion und interne Auftragsreferenz ergänzen.
Persönliche Namen und Zugangsdaten bleiben außerhalb von Repository und PR.

```sql
-- Beide Rollen: keine erhöhten Attribute. Nur die persönliche Rolle hat LOGIN.
SELECT rolcanlogin, rolinherit, rolsuper, rolcreatedb, rolcreaterole,
       rolreplication, rolbypassrls
FROM pg_roles WHERE rolname IN (session_user, 'phoenix_diagnostics');

-- Genau eine Mitgliedschaft für den persönlichen Zugang, keine für die Gruppe.
SELECT member = 'phoenix_diagnostics'::regrole AS group_membership,
       roleid::regrole::text AS granted_role, admin_option, inherit_option, set_option
FROM pg_auth_members
WHERE member IN (session_user::regrole, 'phoenix_diagnostics'::regrole);

-- Erwartet: exakt die zwölf oben genannten Spalten, auch für den LOGIN-Zugang.
SET ROLE phoenix_diagnostics;
SELECT n.nspname, c.relname, a.attname,
       has_column_privilege(session_user, c.oid, a.attnum, 'SELECT') AS login_can_read,
       has_column_privilege(current_user, c.oid, a.attnum, 'SELECT') AS group_can_read
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum > 0 AND NOT a.attisdropped
WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND NOT EXISTS (SELECT FROM pg_depend d JOIN pg_extension e ON e.oid = d.refobjid
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid
                    AND d.deptype = 'e' AND e.extname = 'pg_stat_statements')
  AND (has_column_privilege(session_user, c.oid, a.attnum, 'SELECT')
       OR has_column_privilege(current_user, c.oid, a.attnum, 'SELECT'))
ORDER BY 1, 2, 3;

-- Erwartet: keine Zeilen. Prüft auch direkte Rechte des persönlichen Zugangs.
SELECT n.nspname, c.relname
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
  AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
  AND NOT EXISTS (SELECT FROM pg_depend d JOIN pg_extension e ON e.oid = d.refobjid
                  WHERE d.classid = 'pg_class'::regclass AND d.objid = c.oid
                    AND d.deptype = 'e' AND e.extname = 'pg_stat_statements')
  AND (has_table_privilege(session_user, c.oid, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
       OR has_table_privilege(current_user, c.oid, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
       OR has_any_column_privilege(session_user, c.oid, 'INSERT,UPDATE,REFERENCES')
       OR has_any_column_privilege(current_user, c.oid, 'INSERT,UPDATE,REFERENCES'));

-- Erwartet: keine Zeilen. Keine Sequenznutzung oder Schemaänderung.
SELECT c.oid::regclass::text
FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
WHERE n.nspname NOT LIKE 'pg_%' AND CASE WHEN c.relkind = 'S' THEN
    has_sequence_privilege(session_user, c.oid, 'USAGE,SELECT,UPDATE')
    OR has_sequence_privilege(current_user, c.oid, 'USAGE,SELECT,UPDATE')
    ELSE false END;
SELECT nspname FROM pg_namespace
WHERE nspname NOT LIKE 'pg_%' AND nspname <> 'information_schema'
  AND (has_schema_privilege(session_user, oid, 'CREATE')
       OR has_schema_privilege(current_user, oid, 'CREATE'));

-- Erwartet: keine Zeilen. Keine Umgehung durch Anwendungsfunktionen.
SELECT p.oid::regprocedure::text
FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE p.prosecdef AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'
  AND ((has_schema_privilege(session_user, n.oid, 'USAGE') AND has_function_privilege(session_user, p.oid, 'EXECUTE'))
       OR (has_schema_privilege(current_user, n.oid, 'USAGE') AND has_function_privilege(current_user, p.oid, 'EXECUTE')));

-- Erwartet: RLS und FORCE jeweils true, für genau die drei Tabellen.
SELECT c.oid::regclass::text, c.relrowsecurity, c.relforcerowsecurity
FROM pg_class c
WHERE c.oid IN ('facilities.rooms'::regclass, 'education.groups'::regclass, 'activities.groups'::regclass);
RESET ROLE;
```

Die zwölf Spalten allein genügen nicht: ausgehende Rollenmitgliedschaften,
erhöhte Attribute oder zusätzliche Rechte des LOGIN-Zugangs verhindern die
Abnahme. Bei Abweichungen Zugang sperren und Rechteursache beheben, nicht die
Positivliste still erweitern. Migration und Katalogtests auf der freizugebenden
Version lokal prüfen; nach Änderungen an Rollen, Views, Funktionen, Schema-
oder Default-Rechten und nach Restore den Betriebsnachweis erneuern.

## Rücknahme und Nachweisgrenze

Vor Down persönliche LOGIN-Zugänge sperren und ihre Sitzungen beenden.
Down entzieht die zwölf Spaltenrechte und die drei Schema-USAGE-Rechte und
entfernt die Gruppenrolle atomar. Die persönlichen LOGIN-Rollen bleiben
bestehen und müssen separat verwaltet werden. Down lässt die MFA-Absicherung
aus `1.15.481` unverändert: keine neuen PUBLIC-Rechte und kein Entzug der
ausdrücklichen Laufzeitrechte. Unerwartete Grants, Objektbesitz oder Referenzen
in anderen Datenbanken desselben Clusters lassen Down
fehlschlagen, ohne Teilrücknahme. Keine Objekte mit `DROP OWNED` löschen:
Referenzen zuerst gezielt prüfen. Bei gemeinsam genutzten Testclustern die
Rolle nicht außerhalb des Test-Lebenszyklus löschen.

Die lokalen Tests belegen Positivliste, Zugriffssperren, RLS mit zwei Schulen,
fehlende Rechte für neue Tabellen/Spalten, persönliche Rollenwechsel und den
SQL-Up/Down-Rundlauf mit isoliertem Rollennamen. Sie beweisen weder produktiven
Rollout noch die Ablösung alter Superuser-Zugänge. #3840 AK3/AK4 sind dadurch
nur hinsichtlich des implementierten Diagnosezugangs teilweise abgedeckt;
Datenträger, Schlüssel und Backups bleiben separat nachzuweisen.

PostgreSQL-17-Verträge:
[Spaltenrechte](https://www.postgresql.org/docs/17/sql-grant.html),
[RLS](https://www.postgresql.org/docs/17/ddl-rowsecurity.html),
[Statistikzugriff](https://www.postgresql.org/docs/17/pgstatstatements.html),
[clusterweiter Rollenentzug](https://www.postgresql.org/docs/17/sql-droprole.html).
