# Tägliche verschlüsselte Sicherung

Slices 3836-a und 3836-b, Refs #3836. Keine Aussage hier bestätigt den produktiven Stand.
Release-Snapshots bleiben getrennte Rollback-Punkte, keine Tagessicherung.

## Stand und Grenzen

**Im Code umgesetzt:** Online-Dump der einzelnen Datenbank `postgres` mit
Schema, Daten und ACLs, Rollen ohne Passwörter, gesamtes Upload-Volume samt
versteckten Dateien, SHA-256-Manifest, age-Verschlüsselung, S3-Transport mit
vollständigem Download-/Hashvergleich und altersbasierte Bereinigung.
Ein täglicher systemd-Timer arbeitet unabhängig von Deployments. Jeder
Teilfehler liefert einen Fehlerstatus; erst nach Prüfung und Bereinigung
erscheint `backup_result=ok step=complete`. Eigene Grafana/Loki-Regeln erkennen
fehlenden Erfolg seit 26 Stunden und Fehler der letzten 30 Minuten. Fehlerausgaben von Werkzeugen
werden nicht weitergegeben, weil sie SQL, Dateinamen oder Secrets enthalten
können. Die Abschlusszeile benennt nur den fehlgeschlagenen Schritt.

**Lokal getestet:** `devbox run -- node --test scripts/daily-backup.integration.test.mjs`
nutzt einen isolierten PostgreSQL-17-Cluster ohne Netzwerk, ein lokales
rclone-Ziel und ein zur Laufzeit erzeugtes Wegwerf-Schlüsselpaar. Es prüft
Wiederherstellung nach Volume-Verlust, Rollen-Dumps ohne Passwörter, anschließend
SCRAM-Anmeldung mit neu gesetzten Passwörtern aller drei LOGIN-Rollen, Mandantentrennung
durch RLS, Dateien einschließlich versteckter Dateien, Altersbereinigung und
Fehlerfälle. Es testet keinen echten S3-Anbieter und keine produktiven Daten.

**Produktiv nachzuweisen:** Einrichtung, tägliche Ausführung, separater
deutscher Rechenzentrumsstandort, Zugangsrechte, Schlüsselverwahrung,
Aufbewahrung einschließlich etwaiger Objektversionen, erste echte
Wiederherstellung und AVV/TOM-Abgleich. Alarmzustellung und Bearbeitung fehlen
als produktiver Nachweis; `OnFailure` schreibt ein zur neuen Regel passendes
Journal-Ereignis. Alloy-Anbindung und Zustellung muss ein Mensch einrichten
und Ende-zu-Ende prüfen (siehe unten).
Release-Snapshots haben eine eigene [Altersbereinigung](release-backup-rollback.md#snapshot-retention-slice-3836-c)
(3836-c); deren Timer muss separat eingerichtet und nachgewiesen werden.
Slice 3837-d ergänzt die unabhängige Löschungsliste und deren sperrende
Restore-Prüfung. Einrichtung und getrennte Sicherung sind Mensch-Schritte
im [Löschverfahren](tenant-erasure.md#unabhängige-löschungsliste-einrichten-3837-d-mensch),
kein Bestandteil des täglichen DB-/Upload-Bundles.

Die Skripte sind ein **Gesamtsystem-Restore**, kein Mandantenexport und keine
selektive Löschung. Ein geteilter PostgreSQL-Cluster, zusätzliche Datenbanken
oder externe Tablespaces werden abgewiesen. Die Quell-App bleibt beim Backup
online. Datenbank-Snapshot und Upload-Archiv sind nicht atomar: parallele
Dateilöschungen können zu fehlenden Referenzen führen. Beim Restore muss der
Mensch deshalb zusätzlich DB-Dateiverweise und tatsächliche Dateien abgleichen;
ein fehlerhafter Abgleich verhindert die Freigabe. Änderungen an Dateien
während des Archivierens können den Sicherungslauf abbrechen.

## Einrichtung durch einen Menschen

Die Einrichtungshilfe `bash scripts/daily-backup-setup-wizard.sh` führt durch
diese Schritte. Sie greift auf keine Server zu, fragt keine Secrets ab und
installiert oder aktiviert nichts. Der Mensch führt die Befehle aus.

1. **Offline-Schlüssel:** Auf einem getrennten, vertrauenswürdigen Rechner
   `umask 077; age-keygen -o /geschuetzter/offline-pfad/backup-identity` ausführen.
   Den öffentlichen Empfänger mit `age-keygen -y /geschuetzter/offline-pfad/backup-identity`
   ermitteln. Nur diesen öffentlichen Wert auf den Quellhost übertragen.
   Private Identität mindestens doppelt offline, verschlüsselt und mit
   benannten Berechtigten verwahren. Wiederbeschaffung praktisch prüfen.
   Niemals in Quellhost, Backupziel, CI, Git, Tickets oder Nachweistext kopieren.
   Bei Rotation alte Identitäten bis zum Ablauf ihrer letzten Sicherung behalten.
2. **Getrenntes Ziel:** S3-kompatiblen Speicher an einem anderen deutschen
   Rechenzentrumsstandort anlegen. Anbieter, Region, konkreten Standort und
   dessen Trennung vom Quellstandort im geschützten Betriebsnachweis bestätigen.
   Eigener Bucket/Präfix je Umgebung, keine Kundennamen in Objektnamen.
   HTTPS-Endpunkt, separate technische Zugangsdaten, nur notwendige Rechte
   (Liste, Schreiben, Lesen zur Prüfung, Löschen im Backup-Präfix).
   Administrative Wiederherstellungszugänge getrennt und offline verwahren.
   Ein kompromittierter Quellhost darf nicht alle Generationen vernichten:
   anbieterseitige unveränderbare Aufbewahrung mit getrennten Adminrechten
   einrichten und testen. Sperren dürfen das Löschen abgelaufener Bundles nicht
   verhindern. Versionierung, Delete-Marker, abgebrochene Multipart-Uploads und
   Replikate in die maximale Gesamtfrist von 90 Tagen einbeziehen; Lifecycle
   und endgültige Löschung nachweisen, nicht nur das Verschwinden aus der Liste.
   Das Skript prüft diese Anbieterregeln nicht.
3. **Konfiguration nur über SOPS:**
   [Vorlage](daily-backup.env.example) in ein eigenes, vom Betreiber verwaltetes
   SOPS-Dokument außerhalb dieses Repos übernehmen. Nicht `environments/*.sops.env`
   ändern. Dies ist ein eigener Hostdienst, keine neue App-Variable und keine
   Änderung der Compose-Allowlist. Der Mensch materialisiert daraus
   `/etc/moto-backup/backup.env` (root, 0600) mit SOPS. Kein Klartext in Git,
   Shell-History, Chat oder Journal. Die privaten age-Backupidentitäten gehören
   auch nicht in dieses Dokument. Rclone erhält ausschließlich die aufgeführten
   `RCLONE_CONFIG_MOTO_*`-Werte aus der Umgebung, keine Konfigurationsdatei.
4. **Werkzeuge und Pfade:** Bash, Docker/Compose, Python 3.11+, age, rclone und
   Linux `findmnt` installieren; im Repo sind Python, age und rclone durch
   Devbox erfasst. Geprüfte Versionen für den Host festhalten. `PATH` explizit
   auf installierte Werkzeuge setzen. `BACKUP_COMPOSE_FILE` ist der absolute
   Pfad zur bestehenden Compose-Datei; daneben liegt ihre vorhandene `.env`,
   die Compose nur liest und die nicht gesichert wird. `BACKUP_PROJECT` ist
   exakt deren Compose-Projektname, `BACKUP_UPLOADS_VOLUME` das tatsächliche
   Upload-Volume (gegen den Mount von `server` prüfen).
   `BACKUP_TARGET` hat die Form `moto:bucket/umgebungs-praefix`;
   Provider, HTTPS-Endpunkt, Region und beide Zugangswerte sind Pflicht.
   `BACKUP_RETENTION_DAYS` ist eine ganze Zahl 1 bis 90, ohne impliziten Default.
   `BACKUP_WORK_DIR` ist der konfigurierbare Klartext-Arbeitsbereich, bei S3
   zwingend tmpfs (z. B. `/run/moto-backup`); kein Klartext-Swap, keine Core-Dumps.
   `BACKUP_CIPHER_DIR` ist ein eigener privater Pfad auf einem Disk-Dateisystem
   (z. B. `/var/lib/moto-backup-cipher`), nicht unterhalb des Klartextpfads.
   Der Dienst legt beide Standardpfade mit 0700 an. Bei anderen Pfaden muss der
   Betreiber Verzeichnis, Rechte und tmpfs-Mount selbst vorbereiten.
   Das äußere Tar wird direkt nach age gestreamt: beim Backup verbleiben nur
   Dump, Rollen und Upload-Archiv im Klartextbereich. Chiffrat und vollständiger
   Prüfrückdownload liegen auf Disk. Beim Restore liegen dort nur das Chiffrat,
   im Klartextbereich vorübergehend äußeres Tar und extrahierte Nutzdateien.
   Nach dem Entschlüsseln bzw. Extrahieren werden die nicht mehr benötigten
   Bundles sofort entfernt.

   **Platz vor dem ersten Lauf dimensionieren:** Unkomprimierte DB-Größe,
   Gesamtgröße der Uploads und Wachstum bis zum nächsten Lauf im geschützten
   Betriebsverfahren messen, nicht aus stark komprimierten Bundles ableiten.
   Als konservative Ausgangsreservierung je Bereich mindestens
   `2 × (DB-Größe + Upload-Größe) + 512 MiB` einschließlich Wachstumsreserve
   ansetzen; bei vielen kleinen Dateien Archiv-Overhead zusätzlich messen.
   `BACKUP_MIN_FREE_BYTES` muss diese Reservierung als positive Bytezahl enthalten.
   Sie muss auch für das größte noch aufbewahrte Restore-Bundle reichen.
   Vor Dump oder Download prüft das Skript den freien Platz beider Bereiche;
   bei gemeinsamem Dateisystem verlangt es die doppelte Reservierung.
   Unzureichender Platz führt zu Exit ≠ 0 und
   `backup_result=failed step=workspace_capacity` (bzw. `restore_result=failed`).
   Die Reservierung ist kein Quota-Lock: konkurrierende Nutzung oder stärkeres
   Wachstum kann trotzdem abbrechen und muss wie jeder Teilfehler bearbeitet
   werden. tmpfs-Limit und verfügbarer RAM müssen zur Reservierung passen;
   andernfalls einen ausreichend großen geschützten tmpfs bereitstellen,
   nicht einfach einen kleinen `/run`-Mount verwenden.

   Private Arbeitsverzeichnisse werden bei Erfolg, Fehler und SIGTERM entfernt.
   Das ist Entfernen von Dateien, kein garantierter physischer Overwrite auf SSDs;
   Klartext bleibt deshalb auf tmpfs ohne unverschlüsselten Swap.
   Nach Stromausfall/SIGKILL auf Reste beider Pfade und `.release-operation.lock`
   prüfen. Nur verwaiste Dateien entfernen, keinen Lock eines laufenden
   Backups/Deployments. Alle Betriebswerkzeuge serialisieren über denselben
   Release-Lock. Ein Konflikt ist ein Fehler, kein übersprungener Erfolg.
5. **Timer installieren:** `daily-backup.sh`, `daily_backup.py` und für die
   getrennte Listenpflege `compact-erasure-ledger.py` zusammen
   unter `/usr/local/lib/moto-backup/` installieren (root, nicht durch
   App-Nutzer schreibbar). Units aus `scripts/systemd/` nach
   `/etc/systemd/system/` kopieren. `systemd-analyze verify` über alle drei
   Units ausführen, `systemctl daemon-reload`, dann zunächst
   `systemctl start moto-daily-backup.service`. Ergebnis im Journal und
   verschlüsseltes Objekt am Ziel prüfen. Erst nach erfolgreichem Restore-Test
   `systemctl enable --now moto-daily-backup.timer` ausführen.
   `systemctl list-timers moto-daily-backup.timer` kontrollieren: täglich
   02:15 UTC plus höchstens fünf Minuten Zufallsversatz, verpasster Lauf wird
   nachgeholt. Bis zur nachgewiesenen Alarmzustellung muss ein benannter Mensch
   täglich Ergebnis und Frische prüfen, Fehler bearbeiten und dies dokumentieren.
   Die folgenden Alarmregeln ersetzen diesen Übergangsschritt erst nach Abnahme.

Asymmetrisches age statt gemeinsam auf dem Host abgelegtem Entschlüsselungs-
passwort trennt Daten und Schlüssel bei Hostverlust (ADR 0085). Der öffentliche Schlüssel
erlaubt Verschlüsselung, nicht Entschlüsselung. age und SHA-256 beweisen aber
nicht die Herkunft eines Bundles: den verschlüsselten SHA-256-Wert und das
Erstellungsdatum im getrennt geschützten Sicherungskatalog ablegen und vor
einem Restore vergleichen. Der Empfänger ist kein Signierschlüssel.
Tool-Verträge: [age](https://github.com/FiloSottile/age),
[rclone copyto](https://rclone.org/commands/rclone_copyto/),
[S3-Umgebungskonfiguration](https://rclone.org/s3/).

## Ausfallerkennung und Bearbeitung (Mensch-Schritte)

**Im Repo vorbereitet, nicht produktiv nachgewiesen:**
`monitoring/grafana/provisioning/alerting/daily-backup.yml` ergänzt die alten
cron-Regeln unverändert. Der eigene Stream `{env="prod",service="daily-backup"}`
verhindert, dass ein alter DB-Dump einen fehlenden neuen Sicherungslauf verdeckt.
`absent_over_time` meldet auch einen noch nie angelegten Stream als fehlend.
Ein leerer, gesunder Abfragevektor ist deshalb `OK`; Abfragefehler sind `Alerting`.
Die zweite Regel zählt Fehler seit 30 Minuten, auch wenn später ein Erfolg folgt.
Beide nutzen die bestehende Standard-Notification-Policy. Keine Contact Points,
Empfänger oder Notification Policies werden provisioniert.

1. **Bestand und Journalzugriff prüfen:** Ein berechtigter Mensch ermittelt im
   serverlokalen `/root/monitoring/docker-compose.yml` die Alloy-Version, den
   Konfigurationspfad und den vorhandenen `loki.write`-Komponentennamen.
   Die Basis-Compose und Alloy-Konfiguration liegen nicht im Repo. Das folgende
   Beispiel setzt den Dienstnamen `alloy`, `/etc/alloy/config.alloy` und
   `loki.write.default` voraus; Namen an den geprüften Bestand anpassen, keine
   zweite Loki-Verbindung oder neuen Zugang anlegen. Host-Journalzugriff prüfen:
   persistentes Journal aktivieren, mehr als 26 Stunden Journal-/Loki-Aufbewahrung
   sicherstellen, Alloy-Cursor über Containerneustarts persistent speichern.
   Der vorhandene Alloy-Nutzer braucht Leserechte, z. B. über die numerische
   Host-Gruppe `systemd-journal`; nicht durch weltlesbare Journale lösen.
   Für containerisiertes Alloy folgende Mounts **ergänzen**, vorhandene erhalten:

   ```yaml
   services:
     alloy:
       volumes:
         - /var/log/journal:/var/log/journal:ro
         - /run/log/journal:/run/log/journal:ro
         - /etc/machine-id:/etc/machine-id:ro
   ```

   Der Dienst und der Failure-Hook verwenden `SYSLOG_IDENTIFIER=moto-daily-backup`.
   Zusätzlich muss `_UID=0` übereinstimmen: Dieses vertrauenswürdige Journalfeld
   beschränkt die Quelle auf Root-Prozesse. Der Bezeichner allein ist fälschbar;
   ein lokaler Nicht-Root-Prozess könnte sonst einen Sicherungserfolg vortäuschen.
   Der explizite `logger --tag` ist nötig, weil der Failure-Hook unter einer
   instanziierten anderen Unit läuft. Nicht nur `_SYSTEMD_UNIT` des Hauptdiensts
   filtern. Nach Installation der geänderten Units `systemd-analyze verify`
   und `systemctl daemon-reload` ausführen.
2. **Alloy ergänzen:** Diesen Ausschnitt in die bestehende Konfiguration
   übernehmen. Keine Journal-Metadaten oder freien Werkzeugausgaben nach Loki
   weitergeben. Die enge Zeilenprüfung lässt nur den festen Ergebnisvertrag zu.

   ```alloy
   loki.source.journal "moto_daily_backup" {
     matches = "SYSLOG_IDENTIFIER=moto-daily-backup _UID=0"
     max_age = "27h"
     labels = { env = "prod", service = "daily-backup" }
     forward_to = [loki.process.moto_daily_backup.receiver]
   }

   loki.process "moto_daily_backup" {
     stage.match {
       selector = "{service=\"daily-backup\"} !~ \"^backup_result=(ok|failed) step=[a-z_]+( missing=[A-Z_]+)?$\""
       action = "drop"
     }
     forward_to = [loki.write.default.receiver]
   }
   ```

   Leerzeichen verknüpfen die beiden Journalbedingungen mit UND, siehe
   [Alloy: Journal matches](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.journal/#journal-matches).
   Root-Prozesse bleiben eine Vertrauensgrenze; der Filter beweist keinen
   erfolgreichen Restore und schützt nicht vor einem kompromittierten Root-Zugang.

   Den Ausschnitt lokal mit Alloy 1.21.2 validiert (nur Syntax/Komponenten,
   ohne Journal). Die vollständige Konfiguration mit der installierten Version
   prüfen. `validate` ist in älteren Alloy-Versionen nicht verfügbar; dann
   zunächst ein separat geprüftes Upgrade vorbereiten, nicht die Prüfung
   überspringen. Beispiel im vorhandenen Compose-Verzeichnis:

   ```bash
   docker compose run --rm --no-deps --entrypoint /bin/alloy alloy \
     validate /etc/alloy/config.alloy
   docker compose up -d alloy
   ```

   Bei einem Fehler nicht aktivieren; die vorherige Konfiguration behalten.
   Validierung prüft Syntax/Komponenten, nicht Journalrechte oder Zustellung.
   Ein tatsächlich erfolgreicher Sicherungslauf muss im Journal und in Grafana
   Explore unter dem neuen Stream sichtbar sein. Keine synthetischen
   Erfolgsmeldungen in den produktiven Stream schreiben.
3. **Regeln und Zustellung abnehmen:** Nur `alerting/daily-backup.yml` in das
   vorhandene Grafana-Provisioning übernehmen und Grafana neu laden/starten.
   Bestehende Contact Points und die Standard-Policy samt etwaiger Label-Routen
   durch den Betreiber prüfen, nicht ersetzen. Grafana-Provisionierungsfehler
   ausschließen. Auf einer isolierten Testinstallation mit synthetischen Logs
   die beiden Regeln und die bestehende Zustellung Ende-zu-Ende prüfen:
   fehlender/über 26 Stunden alter Erfolg, Skriptfehler, Failure-Hook sowie
   Wiederanlauf von Alloy/Grafana. Ein fehlendes EnvironmentFile oder ein
   nicht startbares Skript muss über `OnFailure` erkannt werden. Ein komplett
   ausgefallener Host/Alloy liefert keine Failure-Zeile: dafür greift spätestens
   die 26-Stunden-Regel; bestehende Host-/Server-Down-Regeln bleiben bestehen.
   Keine Zustellung behaupten, solange Empfänger und Bearbeitung nicht bestätigt
   sind. Zeiten von Auslösung, Zustellung, Übernahme und Abschluss sowie den
   technischen Ergebnisstatus ohne Rohlogs im geschützten Betriebsnachweis ablegen.
4. **Bearbeitung festlegen:** Betreiber benennt zuständige Rolle und Vertretung
   außerhalb des Repos. Bei Alarm Übernahme bestätigen, Timerstatus, letzten
   verifizierten Erfolg und den bereinigten `step` im Journal prüfen. Bei
   `missing=<VAR>` nur den genannten Wert im Betreiber-SOPS-Verfahren ergänzen;
   keine Werte oder Werkzeugdiagnosen in Tickets kopieren. Speicherplatz,
   Transport, Schlüssel-Empfänger und Retention je Fehlerschritt prüfen.
   Ursache beheben, beaufsichtigten Lauf starten und vollständigen Erfolg samt
   Zielprüfung bestätigen. Bei weiterhin fehlender Sicherung nach dem
   Vorfallverfahren eskalieren, nicht nur den Alarm stummschalten.
   Die Fehlerregel kann nach 30 Minuten zurückgehen, ohne dass die Ursache
   behoben ist; Abschluss nur nach bestätigtem Sicherungserfolg dokumentieren.

Lokaler Regressionstest:
`devbox run -- node --test scripts/daily-backup-alerts.integration.test.mjs`.
Er wertet die provisionierten LogQL-Ausdrücke in Loki 3.4.3 ohne Netzwerk aus;
er prüft weder die serverlokale Alloy-Konfiguration noch Notification-Routing.
Quellen: [Journalquelle](https://grafana.com/docs/alloy/latest/reference/components/loki/loki.source.journal/),
[Alloy-Validierung](https://grafana.com/docs/alloy/latest/reference/cli/validate/),
[LogQL-Zeitfenster](https://grafana.com/docs/loki/latest/query/metric_queries/).

## Ablösung der bestehenden cron-Sicherung (Mensch-Schritte)

Die alten Regeln in `alerting/infrastructure.yml` suchen `Backup complete`
und `Backup FAILED` im Stream `service="backup"`. Das ist ein Hinweis auf ein
serverlokales Altverfahren, kein Nachweis seiner Eigenschaften. Ein Agent hat
weder den Produktionshost noch den Monitoring-Host dafür gelesen.

1. Betreiber inventarisiert Cron-/Timer-Einträge, Skriptpfad und -version,
   Zielanbieter/-standort, Verschlüsselung und Schlüsselzugriff, gesicherte
   Inhalte (insbesondere `.env`, Rollen-Hashes, Datenbank und Dateien),
   Aufbewahrung einschließlich Versionen/Replikaten und den tatsächlichen
   Log-Dateipfad. Befund nur im geschützten Betriebsnachweis festhalten,
   keine Skript-/Log-Kopien mit Secrets oder Personendaten ins Repo.
2. Beide Verfahren parallel betreiben, bis die neue Tagessicherung einschließlich
   Zieltrennung, Aufbewahrung und Zustellung abgenommen ist und daraus eine
   erste echte Wiederherstellung samt Löschprüfung erfolgreich dokumentiert ist.
   Alte Alarme bis dahin aktiv lassen. Ein grüner cron-Alarm bestätigt nicht
   die neue Tagessicherung.
3. Danach ausschließlich den identifizierten alten Sicherungsjob abschalten,
   nicht allgemeine Cron-Dienste oder Release-Snapshots. Altbestände gesperrt
   halten und spätestens 90 Tage nach ihrer jeweiligen Erstellung endgültig
   löschen, einschließlich Versionen, Replikaten und temporärer Klartextkopien.
   Bereits ältere Bestände nach Prüfung unverzüglich bereinigen; Abschaltung
   setzt die Frist nicht neu in Gang. Löschung nachweisen.
4. Erst danach alte Backup-Regeln gezielt über Provisioning entfernen
   (`deleteRules`: `orgId: 1`, UIDs `dfsla2b54bgg0a` und `dfsla2b8noa2od`),
   die beiden Regeldefinitionen aus `infrastructure.yml` nehmen und nur den
   alten Log-Collector stilllegen. Das ist ein späterer geprüfter Change,
   nicht Teil dieses PRs. Neue Regeln, Host-Alarme und Default-Policy erhalten.

## Wiederherstellung, App bleibt gesperrt

1. **Auf einer getrennten Maschine vorbereiten:** Der erste echte Restore-Test
   erfolgt niemals auf dem Produktionshost, auch nicht unter anderem
   Compose-Projektnamen. Dort benennt die Produktions-Compose das Upload-Volume
   fest `phoenix-production-uploads` und belegt Port `127.0.0.1:5432`;
   ein anderer Projektname isoliert diese Ressourcen nicht. Nie parallel zu
   einem Produktionsprojekt auf demselben Host testen.

   Auf der getrennten, zugriffsbeschränkten Maschine Docker/Compose und die
   Backup-Werkzeuge installieren. Keine öffentlichen App-Ports, keine Worker,
   Scheduler, Mail- oder Telemetrieverbindungen zulassen. Für den Test ausschließlich
   [daily-restore.compose.yml](daily-restore.compose.yml) verwenden, **nicht**
   `environments/production.compose.yml`. Die eigenständige Datei hat nur
   PostgreSQL 17, `restart: no`, einen expliziten eigenen DB-Volume-Namen und
   einen eigenen Loopback-Port. Es gibt keinen `server`-Dienst, den ein
   versehentliches `up` starten könnte. Bei anderer Backup-Hauptversion zunächst
   das Image der lokalen Kopie passend festlegen. Produktionsdateien bleiben
   unverändert.

   ```bash
   # Nur auf der getrennten Restore-Maschine, durch einen Menschen.
   set +x
   umask 077
   install -d -m 700 /srv/moto-restore
   install -m 600 docs/operations/daily-restore.compose.yml /srv/moto-restore/compose.yml
   sops --decrypt /geschuetzter/pfad/restore-drill.sops.env > /srv/moto-restore/.env
   chmod 600 /srv/moto-restore/.env
   ```

   Das separate Betreiber-SOPS-Dokument aus
   [daily-restore.env.example](daily-restore.env.example) und der Backup-Vorlage
   vorbereiten. Es enthält die vollständige Liste für diese Compose:
   `TZ`, `RESTORE_POSTGRES_PORT` (z. B. 55432),
   `RESTORE_DATABASE_VOLUME` (neuer Name) und `POSTGRES_PASSWORD`.
   Zusätzlich für das Restore-Skript alle Transport-/Arbeitsbereichswerte,
   `PHOENIX_AUTH_PASSWORD`, `PHOENIX_DEMO_PASSWORD` und den Identitätspfad füllen.
   Die drei Passwörter für den Test neu erzeugen, keine Produktionszugänge
   wiederverwenden. Werte für die folgenden Shell-Befehle shellkompatibel
   quotieren, nicht in die History eingeben. Die `.env` ist für die erforderliche
   Compose-Interpolation nötig, wird aber nie gesichert oder ausgegeben.

   ```bash
   set -a
   . /srv/moto-restore/.env
   set +a
   # Beispiel für 8 GiB tmpfs, vorab anhand der obigen Reservierung dimensionieren.
   # Nur frische, dedizierte Pfade, keine bestehenden Mounts überdecken.
   install -d -m 700 "$BACKUP_WORK_DIR" "$BACKUP_CIPHER_DIR"
   mount -t tmpfs -o size=8G,mode=0700,uid=0,gid=0,nosuid,nodev,noexec \
     tmpfs "$BACKUP_WORK_DIR"
   chmod 700 "$BACKUP_WORK_DIR" "$BACKUP_CIPHER_DIR"
   findmnt -n -o FSTYPE -T "$BACKUP_WORK_DIR"   # muss tmpfs sein
   findmnt -n -o FSTYPE -T "$BACKUP_CIPHER_DIR" # muss ein Disk-Dateisystem sein
   docker compose --env-file /srv/moto-restore/.env \
     -f /srv/moto-restore/compose.yml -p "$BACKUP_PROJECT" up -d --wait postgres
   ```

   `BACKUP_COMPOSE_FILE=/srv/moto-restore/compose.yml`, eine eigene
   `BACKUP_PROJECT`-Kennung und `BACKUP_UPLOADS_VOLUME` auf einen neuen,
   **noch nicht existierenden** Upload-Volume-Namen setzen. Das Skript erstellt
   dieses Volume erst beim Restore; es ist nicht an einen App-Dienst gebunden.
   Der DB-Cluster muss frisch sein: nur initdb-Rollen, leere Datenbank `postgres`.
   App-Image/Schema-Version aus dem getrennten Release-Inventar festhalten;
   Compose-Dateien, Zertifikate und Infrastruktur sind keine Bundle-Komponenten.
2. **Bundle wählen:** Im geschützten Sicherungskatalog Zeitpunkt und
   SHA-256-Wert des verschlüsselten Objekts prüfen. Restore-Zugang mit Leserecht
   auf genau dieses Sicherungsziel aus dem getrennten SOPS-Verfahren laden.
   Backup-Variablen aus der Vorlage auf die frische Umgebung ändern.
   `BACKUP_AGE_IDENTITY_FILE` auf die vom Menschen bereitgestellte private
   Identität setzen, nur für den Restore und niemals auf dem Quellhost.
   Schlüssel nicht in Befehlsargumente oder Protokolle schreiben.
3. **Restore ausführen:** Mit geladener Umgebung
   `bash scripts/restore-daily-backup.sh <daily-Zeitstempel-Zufallskennung.tar.age> /var/lib/moto-erasure/completed.ledger`
   ausführen. Das Skript lädt herunter, entschlüsselt lokal auf tmpfs, prüft
   Manifest, alle Prüfsummen, Archive und frisches Ziel vor dem ersten Schreibzugriff.
   Dann legt es Rollen aus dem passwortfreien Dump an, setzt die Passwörter
   für sämtliche bekannten LOGIN-Rollen neu und stellt Daten und Dateien her.
   Es startet keine App. `.daily-restore-review-required` hält den
   Sicherungszeitpunkt und die ausstehende Löschprüfung fest. Bei Teilfehler
   Exit-Code ungleich 0, Umgebung isoliert lassen und bei Importfehler frisch
   neu aufsetzen (bei `erasure_*` nach vollständigem Import Schritt 4 nutzen);
   keine teilweise wiederhergestellte Umgebung freigeben. Fehlt ein Pflichtwert,
   lautet die sichere Meldung z. B.
   `restore_result=failed step=configuration missing=PHOENIX_AUTH_PASSWORD app_start=blocked`.
   Sie nennt nur den Variablennamen, niemals den Wert.

   **Marker nach Abbruch:** Vor dem ersten Schreibzugriff entsteht neben der
   Compose-Datei `.daily-restore-review-required`. Bei einem späteren Fehler
   bleibt er absichtlich erhalten: Daten oder Rollen können bereits teilweise
   geschrieben sein. Nicht als Fehlerbehebung einfach löschen und weiterarbeiten.
   Nach Importfehler oder Stromausfall alle beteiligten Prozesse stoppen bzw. deren
   Ende prüfen, die isolierte fehlgeschlagene DB und das Upload-Volume verwerfen
   oder nach Vorfallverfahren getrennt halten. Erst wenn das Ziel nachweislich
   wieder frisch ist und kein anderer Restore läuft, darf ein berechtigter Mensch
   den alten Marker für einen neuen Versuch entfernen:

   ```bash
   # Nur nach obiger Prüfung auf der getrennten Restore-Maschine.
   rm -- /srv/moto-restore/.daily-restore-review-required
   ```

   Ein vorhandener Marker blockiert auch einen ansonsten frischen Versuch;
   das Skript überschreibt ihn nicht. Nach erfolgreichem Restore bleibt er bis
   zur vollständigen Freigabe aus Schritt 5 bestehen. Ein Fehler vor seiner
   Anlage braucht keinen Marker: Das Skript hat noch keine Zieldaten geschrieben.

   **Vollständige Zuordnung aus initdb und Migrationen:**

   | LOGIN-Rolle | Getrennt verwaltete SOPS-Variable | Herkunft |
   |---|---|---|
   | `postgres` | `POSTGRES_PASSWORD` | PostgreSQL-initdb / Compose |
   | `phoenix_auth` | `PHOENIX_AUTH_PASSWORD` | Migration `001014001_create_tenant_roles.go` |
   | `phoenix_demo` | `PHOENIX_DEMO_PASSWORD` | Migration `001015403_demo_runtime_role.go` |

   `phoenix_tenant` und `phoenix_admin` sind NOLOGIN und benötigen kein Passwort.
   Auch außerhalb der Demo existiert `phoenix_demo`; sein Wert ist erforderlich.
   Eng begrenzte Ausnahme für [persönliche Diagnosezugänge](diagnose-lesezugang-3840.md)
   und interne [Lösch-/Aufbewahrungszugänge](tenant-erasure.md): Ausschließliche
   Mitgliedschaft in **genau einer** der Gruppen `phoenix_diagnostics`,
   `phoenix_school_erasure` oder `phoenix_billing_retention` ohne ADMIN-/INHERIT-
   Option, mit SET-Option und ohne erhöhte Attribute, individuelle Einstellungen,
   direkte Rechte oder Objektbesitz wird akzeptiert. Zusätzliche Mitgliedschaften,
   auch in einer zweiten erlaubten Gruppe, bleiben verboten. Die Gruppen selbst
   müssen NOLOGIN/NOINHERIT sein, ohne erhöhte Attribute oder weitere
   Gruppenmitgliedschaften. Diese Zugänge werden vor dem Datenimport auf
   **NOLOGIN** gesetzt, nicht mit einem Passwort versehen.
   Nach dem Datenimport prüft der Restore zusätzlich sämtliche direkten
   Objektabhängigkeiten (einschließlich Spalten-/Default-Rechten und Besitz).
   Bei Abweichung scheitert er weiterhin bei `step=role_passwords`, die
   Freigabesperre bleibt bestehen. Nur die Anzahlen werden als
   `restore_diagnostics_disabled=<Anzahl>`,
   `restore_school_erasure_disabled=<Anzahl>` bzw.
   `restore_billing_retention_disabled=<Anzahl>` gemeldet, keine Login-Namen.
   Alle anderen unbekannten LOGIN-Rollen werden weiterhin abgewiesen.
   Vor Wiederholung deren Zweck/Mapping klären und das Verfahren gezielt erweitern.
   Persönliche Diagnosezugänge nach Restore **bewusst einzeln neu freischalten**:
   Auftrag und Schulwahl bestätigen, Rechte und MFA-Netzweg/HBA nach dem
   Diagnose-Runbook prüfen, Passwort neu setzen, erst danach LOGIN aktivieren.
   Nicht benötigte Zugänge bleiben gesperrt.
   **Lösch-/Aufbewahrungs-Logins ebenfalls bewusst einzeln neu freischalten:**
   Auftrag, Zuständigkeit und benötigte Gruppenrolle erneut prüfen. Keine
   kombinierte Mitgliedschaft für Löschung und Abrechnung vergeben. Mit dem
   administrativen Restore-Zugang direkte Rechte, Objektbesitz, Einstellungen
   und weitere Mitgliedschaften ausschließen, ein neues Passwort über das
   freigegebene Secret-Verfahren und `\password` setzen und erst danach LOGIN
   aktivieren. Für die Löschwiederholung vor App-Start die passende technische
   Anmeldung im bestehenden `DB_DSN` bereitstellen. Nach dem Wartungsfenster
   wieder sperren; nicht benötigte Zugänge bleiben NOLOGIN. Der Restore vergibt
   keine zusätzlichen Gruppenrechte und aktiviert keinen dieser Logins.
   `migrate` ersetzt diese Passwortsetzung
   nicht: bereits ausgeführte Migrationen laufen nach Restore nicht erneut.
   Das Restore-Skript ruft `psql -X -c '\password <Rolle>'` ohne TTY auf und
   liefert beide Passworteingaben ausschließlich über stdin. Keine Passwörter
   in Argumenten, SQL-Klartext, Shell-History oder Werkzeugausgaben.
   [PostgreSQL-Vertrag für `\password`](https://www.postgresql.org/docs/17/app-psql.html#APP-PSQL-META-COMMAND-PASSWORD).
   Das lokale Integrationstestverfahren erzwingt SCRAM und belegt die echte
   Anmeldung aller drei Rollen nach Restore, nicht nur deren Existenz.
4. **Löschungen vor Start erneut umsetzen (3837-d):** Vor dem Restore die
   **aktuelle unabhängige** `completed.ledger` aus dem getrennten Betriebsziel
   bergen und mit dem unabhängig verwahrten SHA-256-Dateiwert und den offenen
   Vorgängen abgleichen. Nicht aus der alten DB/Sicherung gewinnen und nicht
   neu initialisieren. Ablage außerhalb von DB-/Upload-Volumes, 0600 und nur
   für den berechtigten Betrieb les-/schreibbar. Der zusätzliche absolute
   Pfad ist ein Pflichtargument des Restore-Aufrufs, keine neue Env-Variable.
   Ein benannter Mensch muss Vollständigkeit und Aktualität bestätigen:
   eine Prüfsummenkette allein beweist das nicht.

   Nach DB- und Upload-Wiederherstellung validiert das Skript Format und
   gesamte Prüfsummenkette. Fehlende/beschädigte/unsichere Datei führt zu
   `restore_result=failed step=erasure_ledger app_start=blocked`.
   Für jede gelistete Schul-ID prüft es ausschließlich lesend, ob sie in
   `platform.schools` wieder existiert. Wenn ja, endet es mit
   `restore_result=failed step=erasure_replay_required app_start=blocked`.
   Die Markerdatei bleibt in beiden Fällen erhalten. Das ist ein vollständig
   importierter, aber **nicht freigegebener** Restore, kein Anlass, ihn auf
   derselben DB erneut zu starten oder den Marker als Fehlerbehebung zu löschen.

   Jede wieder vorhandene Schule mit `tenant erase` gemäß
   [Löschverfahren](tenant-erasure.md#2-probelauf-und-ausführung) erneut
   entfernen: gleiche Schul-ID und Vorgangsreferenz aus der Liste, gültige
   Operator-ID, geprüftes Upload-Volume und aktuelle Liste als Host-Bind-Mount.
   Die DB-Funktion muss auf dem restaurierten Schema vorhanden sein; auf
   älteren Sicherungen zuerst die geprüften Migrationen im isolierten
   Wartungsbetrieb nachholen. Kein Roh-SQL als Ersatz für die Löschung.
   Neue Quittungen und aktualisierte Liste getrennt sichern. Danach mit
   derselben Restore-Konfiguration, aber ohne Offline-Identität oder
   Passwortwerte für den Restore, die schreibfreie Prüfung wiederholen:

   ```sh
   python3 scripts/daily_backup.py verify-erasures \
     /var/lib/moto-erasure/completed.ledger
   ```

   Das funktioniert auf der bereits importierten DB, verlangt den bestehenden
   Review-Marker und ausschließlich `postgres` als laufenden Compose-Dienst.
   Erfolg: `restore_result=ok step=erasure_review_complete app_start=blocked`.
   Die Prüfung gibt keine Schul-/Personendaten aus und hebt keine Sperre auf.
   Sie prüft außerdem die Schul-ID-Sequenz. Wurde eine Schule erst **nach**
   dem Sicherungszeitpunkt angelegt und wieder gelöscht, fehlt ihre Wurzel im
   Restore, aber ihre ID darf nicht erneut vergeben werden. Bei
   `erasure_sequence_review_required` die höchste Schul-ID der bestätigten
   aktuellen Liste ermitteln. Im isolierten Wartungsbetrieb mit dem bestehenden
   administrativen Restore-Zugang die Sequenz ausschließlich vorwärts setzen
   (`highest_erased_school_id` als geprüfte interne numerische psql-Variable):

   ```sql
   SELECT setval(pg_get_serial_sequence('platform.schools', 'id'),
     GREATEST((SELECT last_value FROM platform.schools_id_seq),
              (SELECT COALESCE(max(id), 1) FROM platform.schools),
              :'highest_erased_school_id'::bigint), true);
   ```

   Danach `verify-erasures` erneut ausführen. Kein Schulinhalt wird dabei
   geändert. Den Prüfschritt auch auf anderen Restore-/Rollbackwegen vor
   erneuter Nutzung anwenden; die Liste darf nicht mit einem DB-Stand
   zurückgesetzt werden.
   Ihre Grenze ist die Schulwurzel: Dateien/Nebensysteme und Widerrufe bleiben
   im folgenden fachlichen Abgleich zu prüfen, auch wenn die Schulwurzel fehlt.
   Alle seit dem Sicherungszeitpunkt ausgeführten Löschungen anhand der
   Weisungen, Belege und Vorgänge abgleichen. Gesundheitsmarker/Widerrufe aus
   #4828/#4854 gemäß deren Verfahren wiederholen; die neue Liste erfasst nur
   abgeschlossene **Schullöschungen**, nicht Einzelpersonen oder Marker.
   Ist der Abgleich nicht vollständig möglich, bleibt die App gestoppt.
   Ein nächtlicher Fristenlauf ersetzt keine Einzel-Löschweisung.
   Zusätzlich DB-Dateiverweise/Dateien abgleichen, RLS mit mindestens zwei
   Schulen, Rollen/ACLs und zentrale Abläufe prüfen. Fällige Fristlöschungen
   im isolierten Betrieb vor Freigabe nachholen; der nächtliche Lauf setzt sie fort.
5. **Freigabe und Nachweis:** Erst nach erfolgreicher `verify-erasures`-Prüfung
   gegen die bestätigte aktuelle Liste, dokumentierter Prüfung aller Löschungen,
   neu gesetzten Betriebsgeheimnissen, Dateiabgleich und fachlicher Prüfung
   darf ein berechtigter Mensch die Markerdatei entfernen und die neue Umgebung
   gezielt starten. Auf der Testmaschine bleibt die App aus; die separate
   Compose-Datei enthält bewusst keine App. Ein späterer fachlicher App-Test
   benötigt eine separat geprüfte, netzisolierte Testinstallation mit genau
   diesem Upload-Volume und neuen Testgeheimnissen, ohne externe Seiteneffekte.
   Die Markerdatei ist eine Übergabehilfe, kein im Produkt
   durchgesetztes Startverbot. Kein bestehender Deployment-/Rollbackpfad darf
   die frische Umgebung vorher starten. Offline-Schlüssel entfernen und
   Klartextreste kontrollieren. Dauer, Zeitpunkt, Commit-/Image-Version,
   geprüfte Kriterien, Prüferrolle, Ergebnis und Abweichungen im geschützten
   Betriebsnachweis `backup-restore/YYYY-MM-DD/` ablegen. Keine Dumps, Secrets,
   Personendaten, konkreten Dateinamen oder SQL-Ergebnisse dort einfügen.
   Erste echte Wiederherstellung vor produktiver Zusage dokumentieren;
   anschließend risikobasiert und nach wesentlichen Änderungen wiederholen.

### Notfall-Restore auf dem Produktionshost (kein Testlauf)

Nur bei einem tatsächlichen Wiederherstellungsfall durch berechtigte Menschen:
Deployments, Timer und Autostarts sperren, alle Apps/Worker/Sidecars stoppen.
Kein paralleles Produktionsprojekt betreiben. Alten Cluster und Uploads nach
Vorfallverfahren isolieren; erst dann einen **frischen** Cluster derselben
Hauptversion und ein neues Upload-Volume bereitstellen. Nicht unkontrolliert
`down -v` auf dem vorhandenen Projekt ausführen.

Anders als beim Test kann die bestehende Produktions-Compose mit ihrem
festen Port und Upload-Namen erst genutzt werden, wenn die alten Ressourcen
nachweislich außer Betrieb sind. Ihre `.env` samt allen `:?`-Pflichtwerten aus
Betreiber-SOPS wiederherstellen, ohne sie aus Backups zu übernehmen. Der feste
Name `phoenix-production-uploads` darf vor Restore nicht mehr existieren;
vorhandene Vorfalldaten vorher getrennt sichern. `BACKUP_PROJECT` muss dem
späteren Produktionsprojekt entsprechen; nur `postgres` starten. Für die drei
LOGIN-Rollen genau die separat verwalteten/rotierten Werte bereitstellen, die
auch die anschließende App-Konfiguration verwendet. Insbesondere die App
verbindet sich als `phoenix_auth`, nicht als `postgres`.

Dann Schritte 2–5 einschließlich aller Löschungen und Freigabeprüfungen
abarbeiten. Den Offline-Schlüssel nur während dieser beaufsichtigten
Notfall-Wiederherstellung bereitstellen, anschließend wieder vom Host entfernen.
Erst nach Freigabe die regulären Dienste mit dem tatsächlich wiederhergestellten
Volume und passenden Images starten. Keine automatische Migration oder
Deployment-Pipeline darf die Löschprüfung überspringen. Dies ist ein
vorbereiteter Mensch-Schritt, kein hier ausgeführter Produktionsnachweis.

`BACKUP_TRANSPORT=local` mit absolutem Verzeichnis als `BACKUP_TARGET` ist nur
für synthetische lokale Tests vorgesehen und erfüllt keine Standorttrennung.
Auch lokal fehlen Pflichtwerte nicht stillschweigend. Niemals echte Daten
in die Testumgebung oder deren Testausgabe übernehmen.

## AVV/TOM und offene Freigabe

Vor externer Zusage Anhang 1 Ziff. 7 und Anhang 2 des AVV mit Anbieter,
getrenntem deutschem Standort, Zugangstrennung, endgültiger Aufbewahrung,
Alarmzustellung und Restore-Nachweis abgleichen. Stand dieses Slices:
Verschlüsselung/Schlüsseltrennung und Restore-Verfahren im Code, lokal getestet;
produktive Einrichtung und Nachweise offen. Das Legal-Repo bleibt unverändert.
