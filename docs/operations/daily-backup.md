# Tägliche verschlüsselte Sicherung

Slice 3836-a, Refs #3836. Keine Aussage hier bestätigt den produktiven Stand.
Release-Snapshots bleiben getrennte Rollback-Punkte, keine Tagessicherung.

## Stand und Grenzen

**Im Code umgesetzt:** Online-Dump der einzelnen Datenbank `postgres` mit
Schema, Daten und ACLs, Rollen ohne Passwörter, gesamtes Upload-Volume samt
versteckten Dateien, SHA-256-Manifest, age-Verschlüsselung, S3-Transport mit
vollständigem Download-/Hashvergleich und altersbasierte Bereinigung.
Ein täglicher systemd-Timer arbeitet unabhängig von Deployments. Jeder
Teilfehler liefert einen Fehlerstatus; erst nach Prüfung und Bereinigung
erscheint `backup_result=ok step=complete`. Fehlerausgaben von Werkzeugen
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
noch (3836-b); das `OnFailure`-Template schreibt nur einen lokalen Journal-Eintrag.
Release-Snapshot-Altersgrenzen (3836-c) und die Wiederholungsliste für
Löschungen (3837-d) sind nicht Teil dieses Slices.

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
5. **Timer installieren:** `daily-backup.sh` und `daily_backup.py` zusammen
   unter `/usr/local/lib/moto-backup/` installieren (root, nicht durch
   App-Nutzer schreibbar). Units aus `scripts/systemd/` nach
   `/etc/systemd/system/` kopieren. `systemd-analyze verify` über alle drei
   Units ausführen, `systemctl daemon-reload`, dann zunächst
   `systemctl start moto-daily-backup.service`. Ergebnis im Journal und
   verschlüsseltes Objekt am Ziel prüfen. Erst nach erfolgreichem Restore-Test
   `systemctl enable --now moto-daily-backup.timer` ausführen.
   `systemctl list-timers moto-daily-backup.timer` kontrollieren: täglich
   02:15 UTC plus höchstens fünf Minuten Zufallsversatz, verpasster Lauf wird
   nachgeholt. Bis 3836-b muss ein benannter Mensch täglich Ergebnis und
   Frische prüfen, Fehler bearbeiten und dies dokumentieren. Das ist eine
   Übergangsanweisung, kein Nachweis bereits funktionierender Alarmierung.

Asymmetrisches age statt gemeinsam auf dem Host abgelegtem Entschlüsselungs-
passwort trennt Daten und Schlüssel bei Hostverlust (ADR 0085). Der öffentliche Schlüssel
erlaubt Verschlüsselung, nicht Entschlüsselung. age und SHA-256 beweisen aber
nicht die Herkunft eines Bundles: den verschlüsselten SHA-256-Wert und das
Erstellungsdatum im getrennt geschützten Sicherungskatalog ablegen und vor
einem Restore vergleichen. Der Empfänger ist kein Signierschlüssel.
Tool-Verträge: [age](https://github.com/FiloSottile/age),
[rclone copyto](https://rclone.org/commands/rclone_copyto/),
[S3-Umgebungskonfiguration](https://rclone.org/s3/).

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
   `bash scripts/restore-daily-backup.sh <daily-Zeitstempel-Zufallskennung.tar.age>`
   ausführen. Das Skript lädt herunter, entschlüsselt lokal auf tmpfs, prüft
   Manifest, alle Prüfsummen, Archive und frisches Ziel vor dem ersten Schreibzugriff.
   Dann legt es Rollen aus dem passwortfreien Dump an, setzt die Passwörter
   für sämtliche bekannten LOGIN-Rollen neu und stellt Daten und Dateien her.
   Es startet keine App. `.daily-restore-review-required` hält den
   Sicherungszeitpunkt und die ausstehende Löschprüfung fest. Bei Teilfehler
   Exit-Code ungleich 0, Umgebung isoliert lassen und frisch neu aufsetzen;
   keine teilweise wiederhergestellte Umgebung freigeben.

   **Vollständige Zuordnung aus initdb und Migrationen:**

   | LOGIN-Rolle | Getrennt verwaltete SOPS-Variable | Herkunft |
   |---|---|---|
   | `postgres` | `POSTGRES_PASSWORD` | PostgreSQL-initdb / Compose |
   | `phoenix_auth` | `PHOENIX_AUTH_PASSWORD` | Migration `001014001_create_tenant_roles.go` |
   | `phoenix_demo` | `PHOENIX_DEMO_PASSWORD` | Migration `001015403_demo_runtime_role.go` |

   `phoenix_tenant` und `phoenix_admin` sind NOLOGIN und benötigen kein Passwort.
   Auch außerhalb der Demo existiert `phoenix_demo`; sein Wert ist erforderlich.
   Weitere LOGIN-Rollen werden bei `step=role_passwords` abgewiesen, nicht
   unbemerkt passwortlos freigegeben. Vor Wiederholung deren Zweck/Mapping klären
   und das Verfahren gezielt erweitern. `migrate` ersetzt diese Passwortsetzung
   nicht: bereits ausgeführte Migrationen laufen nach Restore nicht erneut.
   Das Restore-Skript ruft `psql -X -c '\password <Rolle>'` ohne TTY auf und
   liefert beide Passworteingaben ausschließlich über stdin. Keine Passwörter
   in Argumenten, SQL-Klartext, Shell-History oder Werkzeugausgaben.
   [PostgreSQL-Vertrag für `\password`](https://www.postgresql.org/docs/17/app-psql.html#APP-PSQL-META-COMMAND-PASSWORD).
   Das lokale Integrationstestverfahren erzwingt SCRAM und belegt die echte
   Anmeldung aller drei Rollen nach Restore, nicht nur deren Existenz.
4. **Löschungen vor Start erneut umsetzen:** Seit dem Sicherungszeitpunkt
   ausgeführte Löschungen je Schule anhand der **außerhalb der Sicherung**
   aufbewahrten Weisungen, Löschbelege und Vorgänge abgleichen. Auch Dateien,
   Mandantenlöschungen, Widerrufe und Folgeaufträge berücksichtigen. Wiederholung
   mit berechtigtem, mandantenbezogenem Verfahren ausführen und nachweisen.
   Solange die Wiederholungsliste aus 3837-d fehlt, verantwortet ein benannter
   Mensch diesen manuellen Abgleich. Ist er nicht vollständig möglich, bleibt
   die App gestoppt. Ein nächtlicher Fristenlauf ersetzt keine Einzel-Löschweisung.
   Zusätzlich DB-Dateiverweise/Dateien abgleichen, RLS mit mindestens zwei
   Schulen, Rollen/ACLs und zentrale Abläufe prüfen. Fällige Fristlöschungen
   im isolierten Betrieb vor Freigabe nachholen; der nächtliche Lauf setzt sie fort.
5. **Freigabe und Nachweis:** Erst nach dokumentierter Prüfung aller Löschungen,
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
