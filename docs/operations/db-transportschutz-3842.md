# DB-Transportschutz: Bewertung und Betriebsnachweis

Slice **3842-d**, AK3 von [#3842](https://github.com/moto-nrw/project-phoenix/issues/3842),
Epic #3846. Repo-Abgleich vom 24.10.2026 gegen `origin/development`,
Commit `755ccc706520fde78a5946581e9207a8b5660eaa`.

## Ergebnis und Aussagegrenze

**Bewertet, nicht produktiv nachgewiesen:** Für die aktuellen Produktions-TCP-Wege
ist kein gleichwertiger Ersatz der Gegenstellenprüfung belegt. Abhilfe ist
`sslmode=verify-full` mit vertrauenswürdiger CA und passendem Servernamen.
Eine begründete Same-Host-Alternative bleibt möglich, aber erst nach dem unten
beschriebenen Wirksamkeitsnachweis und einer dokumentierten Risikoentscheidung.
`require` allein und das Wort „Docker-intern“ genügen dafür nicht.

Diese Änderung enthält nur Dokumentation und eine angeglichene Projektregel.
Keine Serverzugriffe, Requests an Produktions-/Staging-Hosts, Secret-Lektüre,
Zertifikate, Konfigurationsänderungen oder Rollouts. Notwendige Abhilfe ist
**vor der betroffenen Verarbeitung** erforderlich; dies ist keine Freigabe des
aktuellen Betriebs. AK3 bleibt bis zum Betriebsnachweis offen.

## Im Repository belegte Netz- und Clientpfade

Die drei Compose-Dateien definieren je eine PostgreSQL-17-Instanz und ein
umgebungseigenes `driver: bridge`-Netz, **nicht** `internal: true`. Alle DB-Ports
sind an Host-Loopback veröffentlicht, nicht an alle Hostadressen. Der Host und
weitere Teilnehmer desselben Netzes bleiben mögliche DB-Clients. Staging und
Produktion teilen laut [Betriebsdoku](../agents/operations.md#environment-management-sops)
einen Host, Demo hat eine eigene VM; der laufende Aufbau wurde nicht geprüft.

| Verbindungsweg | Repo-Beleg und Transport | Bewertung / offener Nachweis |
|---|---|---|
| Produktions-App und eingebetteter Worker → `postgres:5432` | [Production Compose](../../environments/production.compose.yml): App-DSN `sslmode=require`, ohne `sslrootcert`; DB `ssl=on`, Minimum TLS 1.2. | Verschlüsselung verlangt, Serveridentität nicht geprüft. `verify-full` vorbereiten; gleichwertiger Netzschutz derzeit nicht belegt. |
| Staging-/Demo-App → jeweiliges `postgres:5432` | [Staging](../../environments/staging.compose.yml), [Demo](../../environments/demo.compose.yml): App-DSN `sslmode=disable`; kein DB-TLS-Mount/-Startparameter. | Kein TLS im App-Pfad konfiguriert. Nur bei belegter Isolation und geeigneter Datenherkunft als Same-Host-Alternative bewertbar, keine Freigabe für identifizierbare Produktivdaten in Test/Demo. |
| Migration und Preflight → DB | In allen Compose-Dateien separater `migrate`-Job im jeweiligen Netz; privilegierter `DB_DSN` aus SOPS. [Deploy-Skript](../../scripts/deploy-remote.sh) startet beide als Jobs. | Ziel, TLS-Modus und CA aus dem Repo ohne Entschlüsselung nicht feststellbar. Gleicher Schutzbedarf wie App, wegen Privilegien besonders relevant; beide Jobpfade prüfen. |
| Demo-Laufzeit → DB | `demo-runtime` teilt den Netzwerk-Namensraum von `server`; eigener `DEMO_DB_DSN` und Rolle `phoenix_demo`. | Endpoint/TLS unbekannt, nicht vom App-DSN ableiten; beim Betriebsnachweis gesondert erfassen. |
| Optionaler eigenständiger Worker → DB | [Worker-Runbook](worker-lease-2726.md): gleiche Konfiguration wie `serve`; in den drei Compose-Dateien kein eigener Worker-Dienst. | Eingebetteter Worker nutzt App-Pfad; falls separat betrieben, dessen effektiven DSN und Netz separat prüfen. |
| Release-/Tagesbackup und Restore → DB | [Release-Skript](../../scripts/release-backup.sh) und [Tagesbackup](../../scripts/daily_backup.py): `compose exec ... postgres` mit `pg_dump`, `pg_dumpall`, `psql`, `pg_restore`, ohne `-h`; PostgreSQL-Env-Allowlist enthält keine `PGHOST`-/`PGSERVICE`-Umleitung. | Lokaler Unix-Socket im DB-Container, kein TCP-Netztransport: TLS hier nicht erforderlich. Docker-/Hostrechte bleiben hochprivilegiert (3842-c); Laufzeitabweichungen prüfen. Backup-Übertragung und Speicherschutz separat unter #3836, nicht durch DB-TLS belegt. |
| Host-/Diagnosezugang → DB | Loopback-Mappings: Produktion `127.0.0.1:5432`, Staging `:5433`, Demo `:5434`, jeweils Containerport 5432. Compose-Kommentar nennt einen Tunnel; [Diagnose-Runbook](diagnose-lesezugang-3840.md#vorbedingungen-version-und-persönlicher-zugangsweg) verlangt einen freigegebenen MFA-Netzweg und `verify-full`. | Weder Tunnel noch VPN/Proxy sind als aktiv belegt. Keine öffentliche DB-Freigabe. Für externe Diagnose `verify-full`, CA/SAN und vollständigen Weg nachweisen; TLS-terminierende Proxys erfordern Bewertung beider Teilstrecken. |

App-/CLI-DSNs werden in
[`database_config.go`](../../backend/modules/transactionruntime/internal/adapters/database/database_config.go)
ausgewählt; `GetServeDSN`/`GetDemoDSN` tauschen die Rolle samt Passwort, nicht
die TLS-Parameter. Der Connector in
[`db.go`](../../backend/modules/transactionruntime/internal/adapters/database/db.go)
übergibt den DSN an `pgdriver.WithDSN`. Die Version steht in
[`backend/go.mod`](../../backend/go.mod): `pgdriver v1.3.0`.

### Netzgrenzen und Zertifikate

- App, Frontend, DB und bei Ausführung Migration teilen das Umgebungsnetz.
  [Prometheus](../../monitoring/prometheus-compose.yml) tritt zusätzlich
  `production_production` **und** `staging_staging` bei. Es ist kein vorgesehener
  DB-Client, kann im gemeinsamen Netz aber grundsätzlich den DB-Port erreichen.
  Ein kompromittierter Netzteilnehmer kann DB-Verbindungen versuchen;
  Umleitung/Mitlesen hängt zusätzlich von seinen Rechten und dem Host ab.
- Ein Bridge-Netz ist keine kryptografische Serverauthentisierung. Loopback
  begrenzt die Host-Portfreigabe, nicht Zugriffe im Bridge-Netz. Nach
  [Docker-Dokumentation](https://docs.docker.com/engine/network/port-publishing/)
  gibt es vor Docker 28.0.0 außerdem eine L2-Erreichbarkeitslücke bei
  Loopback-Publishing. Version, Routing, Firewall und zusätzliche Netzanschlüsse
  sind daher Teil des Betriebsnachweises, nicht aus YAML abzuleiten.
- Produktion mountet `/etc/docker/postgres/ssl/certs` nur in den DB-Container:
  `server.crt`, `server.key`, `ca.crt`. Die App und `migrate` haben keinen
  CA-Mount. Eine serverseitige CA-Datei schafft kein Vertrauen beim Client.
  Aussteller, SAN, Ablauf, Schlüsselrechte und Rotation sind unbekannt.
  [`config/ssl/postgres/create-certs.sh`](../../config/ssl/postgres/create-certs.sh)
  ist ausdrücklich für Entwicklung, kein Produktionszertifikat/-nachweis.
- `ssl=on` erlaubt laut
  [PostgreSQL](https://www.postgresql.org/docs/17/ssl-tcp.html) auch Klartext,
  solange passende `hostssl`-/Ablehnungsregeln ihn nicht ausschließen. Die
  produktive HBA-Datei ist nicht im Compose referenziert. Die lokale Vorlage
  unter `config/ssl/postgres/` beweist deshalb keine aktive Produktionsregel.

## Schutzentscheidung je Pfad

Bei `pgdriver v1.3.0` bedeutet `require` ohne `sslrootcert` TLS mit
`InsecureSkipVerify`; `verify-full` prüft Zertifikatskette und Zielnamen.
`require` mit expliziter Root-CA verhält sich wie `verify-ca`, **ohne**
Namensprüfung. Das entspricht nicht `verify-full`. Maßgeblich ist der
[Treiberquelltext](https://github.com/uptrace/bun/blob/driver/pgdriver/v1.3.0/driver/pgdriver/config.go),
nicht eine angenommene Übernahme aller libpq-Einstellungen.
Für `psql` gilt die [libpq-TLS-Dokumentation](https://www.postgresql.org/docs/17/libpq-ssl.html).

1. **Produktions-TCP, Migration und externe Diagnose:** `verify-full` ist die
   konkrete Abhilfe, solange der alternative Schutz nicht nachgewiesen ist.
   Angesichts sensibler Kinder-, Personal- und Finanzdaten ist eine unbestätigte
   Gegenstelle bei weiteren Netzteilnehmern nicht als gleichwertig einzustufen.
   Ein VPN/Tunnel allein beweist nicht die letzte Strecke zur richtigen DB.
2. **Mögliche Same-Host-Alternative:** Alle Strecken bleiben auf einem verwalteten
   Host; DB-Port nur für erforderliche Dienste und freigegebene lokale Zugänge
   erreichbar; keine unkontrollierten Frontend-/Monitoring-/Fremdcontainer im
   DB-Netz; keine unsicheren Routing-/Proxywege oder Containerrechte. Betrieb
   belegt wirksame Filter, Netzmitgliedschaft, Host-/Dockerrechte und Datenumfang.
   Technische Verantwortung und Datenschutzrolle dokumentieren Begründung,
   Restrisiko, Gültigkeitsbereich und erneute Prüfung. Aktuelles Compose allein
   erfüllt diesen Nachweis **nicht**; `internal: true` allein wäre ebenfalls
   kein Ersatz für eine Zugriffsbeschränkung innerhalb des Netzes.
3. **Staging/Demo:** `disable` ist keine Verschlüsselungsmaßnahme. Die Bewertung
   eines lokal begrenzten Weges setzt die obigen Grenzen und getrennte Zugänge
   sowie synthetische/anonymisierte Test-/Demodaten voraus. Fehlt der Nachweis,
   Weg beschränken oder TLS samt Gegenstellenprüfung herstellen. Auch der
   Vertragsabgleich unten bleibt offen.
4. **Lokale Unix-Sockets:** Kein separater Netzpfad; Schutz durch Container-,
   Dateisystem- und Hostrechte statt TLS. Das legitimiert weder reguläre
   Superuser-Diagnose noch unbeschränkten Zugriff auf Sicherungen.

**Restrisiken:** `verify-full` schützt nicht gegen einen kompromittierten Host,
gestohlene CA-Schlüssel, berechtigte missbräuchliche DB-Nutzung oder Datenabfluss
aus einem kompromittierten Client. Same-Host-Schutz hängt besonders von
Host-/Dockerrechten und Netzmitgliedschaft ab. Bei weiteren Clients, anderem
Routing oder einem eigenen Staging-Server (3842-j) neu bewerten; eine lokale
Ausnahme gilt nicht automatisch über Hostgrenzen hinweg.

## Lokale Prüfung dieses Doku-Slices

Am 24.10.2026 liefen beide SQL-Abfragen unten erfolgreich auf einem kurzlebigen
lokalen PostgreSQL-17-Container (`--network none`, keine veröffentlichten Ports,
DB-Daten nur im tmpfs, keine echten Zugangsdaten). Eine synthetische TCP-Sitzung
und eine Unix-Socket-Sitzung wurden getrennt gezählt; beide erwartungsgemäß
ohne TLS. Container danach entfernt. Das prüft Abfragesyntax und Klassifikation,
**nicht** TLS, Negativtests oder einen Betriebszustand außerhalb dieser Testinstanz.

## Mensch-Schritte: lesender Betriebsnachweis

**Nur durch berechtigten Betrieb in einer gesondert freigegebenen Sitzung.**
Die folgenden Prüfungen wurden in Produktion/Staging/Demo nicht ausgeführt.
Keine neuen Hostzugänge aus diesem Dokument ableiten. Rohkonfiguration,
Adressen, Rollennamen, SQL-Inhalte und Secrets bleiben in der geschützten
Betriebsablage. In GitHub nur Datum, Umgebung, Prüferrolle, Ergebnis und interne
Nachweis-ID festhalten. Dockerrechte sind nicht „read-only“; der lesende Befehl
ändert nichts an ihrer hohen Berechtigung.

1. **Effektive Netzwege prüfen.** Je Umgebung im richtigen Compose-Verzeichnis:

   ```bash
   docker version --format '{{.Server.Version}}'
   docker compose ps --format json
   docker compose port postgres 5432
   ss -ltn
   ```

   Nur erwartete Loopback-Bindings 5432/5433/5434 zulassen; IPv6, zusätzliche
   Compose-Overrides, Proxy-/VPN-Weiterleitungen, Docker-Direct-Routing und
   Firewallregeln mitprüfen. `ss` allein erfasst nicht alle Docker-NAT-Wege.
   Mit gezieltem `docker network inspect <freigegebenes_Netz>` Mitglieder
   einschließlich Monitoring vergleichen, keine vollständigen Container- oder
   Env-Inspects exportieren. Erreichbarkeit von nicht erlaubten Netzen separat
   bestätigen; nicht durch Testverbindungen mit echten Zugangsdaten prüfen.
2. **Clientpfade erfassen.** App, beide Migrationsaufrufe, gegebenenfalls Worker,
   Demo-Laufzeit, Backup/Restore und Diagnose einzeln dem tatsächlichen Ziel,
   Transportmodus und Truststore zuordnen. DSNs nur geschützt prüfen, niemals
   ausgeben. Kurzlebige Jobs nicht allein wegen einer leeren Momentaufnahme
   als geprüft markieren. Aktive HBA-Regeln, TLS-Minimum und Zertifikatsstatus
   unter zeitlich begrenzter berechtigter Administration prüfen; keine Rechte
   der regulären Diagnoserolle dafür erweitern.
3. **Verschlüsselung datenarm prüfen.** In einer freigegebenen Admin-Sitzung
   mit ausreichender Statistiksicht, ohne Abfrage von Nutzdaten oder Querytext:

   ```sql
   SELECT CASE WHEN a.client_addr IS NULL THEN 'local_socket'
               WHEN a.usename = 'phoenix_auth' THEN 'application_tcp'
               WHEN a.usename = 'phoenix_demo' THEN 'demo_tcp'
               ELSE 'other_tcp' END AS path,
          s.ssl, s.version, s.bits, count(*) AS connections
   FROM pg_stat_activity AS a
   LEFT JOIN pg_stat_ssl AS s USING (pid)
   WHERE a.backend_type = 'client backend' AND a.pid <> pg_backend_pid()
   GROUP BY 1, 2, 3, 4
   ORDER BY 1, 2;
   ```

   Die eigene nach `verify-full` aufgebaute Diagnoseverbindung separat prüfen:

   ```sql
   SELECT ssl, version, bits FROM pg_stat_ssl WHERE pid = pg_backend_pid();
   ```

   Erwartung für TLS-Pfade: `ssl=true`, `bits>0`, mindestens TLS 1.2. NULL-Werte
   bleiben unbekannt. `ssl=false` für
   bestätigte Unix-Sockets ist kein TLS-Fehler. Andere Klartextwege benötigen
   Abhilfe oder den dokumentierten Alternativnachweis. Keine Zeile bedeutet
   **keinen Nachweis**. [pg_stat_ssl](https://www.postgresql.org/docs/17/monitoring-stats.html#MONITORING-PG-STAT-SSL-VIEW)
   belegt Verschlüsselung, **nicht** den Clientmodus oder dessen CA-/Namensprüfung.
4. **Gegenstellenprüfung nachweisen.** Für jeden verwendeten Client zunächst
   lokal mit Test-DB und synthetischen Zugangsdaten prüfen: richtige CA und
   richtiger SAN erfolgreich; falscher Name und eine gültige, aber fremde
   Test-CA müssen an der Zertifikatsprüfung scheitern. Eine explizit fehlende
   oder unlesbare CA-Datei muss vorab einen Konfigurationsfehler liefern.
   Bloßes Weglassen des CA-Parameters garantiert keinen Fehler, weil der
   System-Truststore greifen kann. Keinen
   Produktiv-DSN an einen Testserver senden. Danach im freigegebenen Netzweg
   die positive Verbindung und geschützte Clientkonfiguration bestätigen;
   Negativproben dort nur mit erfundenen Wegwerfzugangsdaten, isolierter
   Umgebung und ohne Passwort-/Clientzertifikats-/Serviceprofile. Ein DNS-,
   Timeout- oder Authentisierungsfehler zählt nicht als Zertifikats-Negativtest.
   `psql -w` unterbindet nur Nachfragen, nicht die Nutzung einer Passwortdatei.
   Bei `psql` im Tunnel muss
   `host` zum SAN passen; `hostaddr` kann den freigegebenen lokalen Endpunkt
   auswählen ([libpq](https://www.postgresql.org/docs/17/libpq-connect.html#LIBPQ-PARAMKEYWORDS)).
   `hostaddr` nicht ungeprüft als Go-Treiberoption übernehmen.
5. **Entscheidung erfassen.** Je Pfad Ergebnis, Abweichung, Verantwortliche,
   Abhilfe und Termin dokumentieren. Keine Freigabe bei fehlendem Schutznachweis.
   Unbekannte oder unsichere Pfade vor betroffener Verarbeitung beschränken
   oder korrigieren; nach Änderungen erneut prüfen.

## Mensch-Schritte: TLS-Abhilfe vorbereiten und ausrollen

Kein ausführbares Deploy-Skript und kein geänderter Deploy-Pfad in diesem PR.
Die folgenden Änderungen brauchen einen eigenen geprüften Auftrag und dessen
Freigabe; erforderliche Schutzmaßnahmen dadurch nicht aufschieben:

1. Betrieb lässt je DB ein Serverzertifikat durch eine vertrauenswürdige CA
   ausstellen. SAN muss den tatsächlichen Client-Zielnamen enthalten, intern
   aktuell `postgres`; Diagnoseziel und getrennte Umgebungen berücksichtigen.
   Schlüssel bleiben außerhalb Git, CA-Privatschlüssel außerhalb der App-/DB-
   Container. Für Produktion bestehender Serverpfad:
   `/etc/docker/postgres/ssl/certs/{server.crt,server.key,ca.crt}`.
   Schlüsselbesitz/-rechte nach PostgreSQL-Vorgaben prüfen; Ablaufüberwachung,
   Erneuerung und verantwortliche Rolle festlegen.
2. Nur das öffentliche CA-Vertrauensmaterial außerhalb Git provisionieren und
   read-only in **jeden** TCP-Client einbinden, etwa unter
   `/etc/moto/db-tls/ca.crt` im Container. App-Endpoint dann
   `postgres://phoenix_auth@postgres:5432/postgres?sslmode=verify-full&sslrootcert=/etc/moto/db-tls/ca.crt`.
   Maintenance-/Demo-Endpoints nur über SOPS durch berechtigte Menschen ändern,
   niemals privilegierten DSN in den Server kopieren. Staging/Demo benötigen
   bei Wahl von TLS zuvor Serverzertifikat, Mount und aktiviertes TLS.
3. HBA-Regeln für betroffene TCP-Rollen/Quellen mit `hostssl` und gezielter
   Klartextablehnung vor allgemeineren Regeln absichern; Socket-Backup und
   Healthchecks berücksichtigen. Kein pauschales `trust` oder Öffnen ins Internet.
   Zertifikatswechsel benötigen je Änderung Reload/Neustart des DB-Dienstes;
   neue Mounts/Startparameter eine Neuerstellung. Clients mit neuer CA/DSN neu
   starten, damit nicht nur alte Poolverbindungen gemessen werden.
4. Erst lokal mit Testmaterial positiv/negativ pro Client prüfen, dann den
   freigegebenen Rollout einschließlich Preflight, Migration und neuer
   App-Verbindungen durchführen. Netz-/Statistiknachweis oben wiederholen;
   bei Fehlern betroffene Verarbeitung sperren/beschränken, nicht still auf
   `require`/`disable` zurückfallen. Frühere geprüfte Konfiguration nur
   zurücknehmen, wenn deren Schutz weiterhin nachgewiesen ist.

## Vorschlag AVV/TOM und Abschluss

Gelesene Grundlage: [rechtliche Einordnung](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/research/14-dsgvo-mindestumfang.md),
[AVV Anhang 1 Ziff. 5](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/03-avv.md)
und [interne Checkliste](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/intern/checkliste.md).
Der AVV-Entwurf sagt derzeit, interne DB-Verbindungen seien verschlüsselt.
Das ist für die App-Pfade von Staging/Demo im Repo nicht belegt und beschreibt
lokale Socketpfade nicht treffend. Eine Risikoentscheidung ändert keine Zusage.

**Textvorschlag für 3842-i, erst nach Entscheidung und Wirksamkeitsnachweis:**

> Datenbankverbindungen über Hostgrenzen und externe Diagnosewege sind
> verschlüsselt und prüfen die Identität der Gegenstelle. Für Verbindungen
> innerhalb desselben verwalteten Hosts dokumentiert moto je Pfad TLS mit
> Gegenstellenprüfung oder einen risikogerecht gleichwertigen Schutz durch
> wirksame Netz- und Zugriffsbeschränkung. Lokale Unix-Sockets sind durch
> Betriebssystem- und Containerrechte geschützt. Änderungen an Netzwegen,
> Clients und Vertrauensmaterial lösen eine erneute Prüfung aus.

Kein Legal-Repo wurde geändert. AK3: Repo-Bewertung und konkrete Abhilfe
vorbereitet; effektive Topologie, TLS-/Gegenstellenprüfung oder Alternativschutz
und Vertragsabgleich bleiben offen. Die übrigen AK und die Abschlussentscheidung
stehen im [Gesamtbericht](sicherheitshaertung-3842.md).
