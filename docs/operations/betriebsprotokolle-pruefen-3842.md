# Betriebsprotokolle: lesende Prüfung

Slice 3842-f, Refs #3842, Teil von #3846. Stand: 23.10.2026.

## Zweck und Grenze

Dies ist eine **Checkliste für den berechtigten Menschen**, kein ausgeführter
Hosttest und kein Konfigurations- oder Löschskript. Der Slice greift weder auf
Produktion/Staging noch auf Monitoring-Hosts zu. Er ändert keine Umgebung,
Frist oder Berechtigung. Die [Fristentabelle](../reference/loeschfristen.md#betriebsprotokolle)
enthält Vorschläge, keine bereits wirksamen Vorgaben. Vor Anwendung müssen
Betriebsleitung und Datenschutzrolle sie bestätigen oder begründet ersetzen.

Erfolg bedeutet: Für jede Quelle sind effektive Werte, Zugriff, Datenumfang,
Kopien und tatsächlicher Löschweg belegt. Ein Konfigurationswert allein oder
ein nicht mehr auffindbarer Suchtreffer beweist keine physische Löschung.
Unbekannte oder nicht lesbare Werte bleiben **offen**, nie „bestanden“.

## Ablauf und erwartete Werte

1. **Prüfung vorbereiten.** Zuständige Person, Datum, Umgebung, ausgerollten
   Commit und installierte Dienstversionen intern erfassen. Produktion,
   Staging, Demo und Monitoring getrennt inventarisieren. Nur bestehende,
   auf die Aufgabe beschränkte Zugänge verwenden; keine Rechte für diesen
   Nachweis ausweiten. Fehlt ein solcher Zugang, ist das ein offener Punkt
   für Slice 3842-c. Keine Änderungen, Restarts, Vacuum-Läufe oder
   Löschtests in Produktion durchführen.
2. **journald-Konfiguration lesen.** Hauptdatei, aktive Drop-ins und gegebenenfalls
   Journal-Namespaces einschließlich Vorrangregeln prüfen. Erwartete
   Vorschlagswerte: `MaxRetentionSec=14day`, `MaxFileSec=1day`,
   `SystemMaxUse=1G`, `RuntimeMaxUse=256M`; Größenbudgets vor Freigabe an
   freiem Speicher und Logvolumen messen. `Storage`, `SystemKeepFree` und
   `RuntimeKeepFree` ebenfalls erfassen. `MaxRetentionSec=0`, ein bloß
   auskommentierter Wert oder nur ein Größenlimit erfüllen das Altersziel
   nicht. Rotation und Löschausführung einschließlich aktiver Dateien und
   technisch möglicher Verzögerung nachweisen. Weniger Speicher kann die
   Diagnosehistorie früher verkürzen; 14 Tage sind keine Mindestaufbewahrung.
3. **Docker-Treiber je laufendem Container lesen.** Für Backend, Frontend,
   PostgreSQL und Demo-Runtime wird `journald` mit umgebungsbezogenen Tags
   wie `prod-server`, `staging-server` und `demo-server` erwartet. Nicht nur
   den Daemon-Default oder die Compose-Vorlage prüfen. Ein begrenztes
   `docker inspect --format '{{.HostConfig.LogConfig.Type}}' <Container>`
   zeigt nur den Treiber; niemals vollständiges `docker inspect`, Env-Arrays
   oder gerendertes `docker compose config` als Nachweis ausgeben. Migrations-,
   Hilfs- und Monitoringcontainer gesondert erfassen, deren Treiber und
   Löschweg nicht aus den App-Tags ableiten.
4. **Caddy lokal am System prüfen.** Effektive Access- und Fehlerlog-Ausgaben
   samt importierten Snippets, Dienstargumenten und Overrides lesen, ohne
   vollständige Konfigurationen mit Zugangsdaten zu exportieren. Im
   [Repo-Beispiel](../../monitoring/caddy/Caddyfile.observability.example)
   stehen `roll_size 100MiB`, `roll_keep 10`, `roll_keep_for 168h` sowie
   Query-/Headerfilter und IP-Maskierung. Erwartet wird zusätzlich ein zur
   installierten Version passender, zeitgesteuerter Rotations-/Löschweg,
   der das 7-Tage-Ziel auch bei wenig oder keinem Verkehr erreicht.
   Aktive Datei und Archive getrennt prüfen. `roll_keep_for` allein reicht
   nicht: Archivbereinigung wird durch neue Dateien ausgelöst. Tokenpfade,
   weitere Header und separate Fehlerlogs auf unzulässige Inhalte prüfen;
   zur Filterprobe nur synthetische Marker in einer isolierten Testumgebung
   einsetzen. Keine echten URL-Pfade oder Header in den Nachweis kopieren.
5. **Loki und Alloy lesen.** Die tatsächlich geladene Konfiguration und
   Startparameter aus der serverlokalen Basis unter `/root/monitoring`
   ermitteln, nicht nur das Prometheus-Overlay. Bei unterstütztem
   TSDB-/BoltDB-Shipper-Index werden `compactor.retention_enabled=true`,
   `limits_config.retention_period=336h` und ein `24h`-Indexzeitraum als
   Ausgangswerte erwartet. Für Caddy-Streams höchstens `168h`; sämtliche
   Tenant-/Stream-Overrides auf längere oder deaktivierte Fristen prüfen.
   Compactor-Lauf, persistentes Arbeitsverzeichnis, eingerichteten
   `delete_request_store` und Speicherschreib-/Löschrechte bestätigen.
   `retention_delete_delay` und Laufintervalle separat dokumentieren und
   in die freigegebene tatsächliche Höchstaufbewahrung einrechnen.
   `max_query_lookback` ist keine Löschung. Bei anderem Index-/Versionsstand
   den passenden Löschweg belegen, nicht diese Werte blind übernehmen.
   Altbestand gesondert prüfen: Friständerungen erfassen laut aktueller
   Loki-Dokumentation nicht rückwirkend alle bereits aufgenommenen Logs.
   Alloy-WAL/Spool, Exporte und Speicherkopien ebenfalls inventarisieren.
6. **Sentry, PostHog und GitHub getrennt lesen.** Je aktivem Projekt und
   Datenart die tatsächliche Retention, Löschfähigkeit und Berechtigungen
   erfassen. Erwartete Vorschläge: Sentry 14 Tage, zulässige PostHog-Ereignisse
   höchstens 30 Tage, GitHub-Workflow-Logs 30 Tage; kürzere Artefaktfristen
   erhalten. Aufzeichnungen benötigen eine eigene kürzere Begründung oder
   Abschaltung. Tarif-/Dienstgrenzen als Lücke dokumentieren, nicht als
   Umsetzung ausgeben. Altbestand, Exporte, Traces, Anhänge, Profile und
   Release-Assets gesondert betrachten. Abschaltung neuer Erfassung löscht
   keine alten Daten. Zweck und zulässiger Betrieb der Telemetrie bleiben
   unter #3843 zu klären.
7. **Mail-Outbox prüfen.** Ausgerollte Delivery-Retention und erfolgreiche
   nächtliche Schulläufe sowie Worker-Läufe für Systemmails bestätigen.
   Erwartet: fällige abgeschlossene Ergebnisse nach 30 Tagen entfernt bzw.
   bei `guardian_welcome` inhaltsbereinigt; noch versuchte Zustellungen
   gesondert auf Alter und Hängenbleiben prüfen. Nur aggregierte Anzahlen
   und Laufstatus belegen, keine Empfänger, Nutzdaten, Providerantworten
   oder Fehlertexte exportieren. Fristen der SMTP-/Mailanbieter sind kein
   Ergebnis des DB-Löschlaufs und müssen separat belegt werden.
8. **Zugriff und Wirksamkeit belegen.** Für jede Quelle Reader, Admins,
   technische Konten und Exportrechte prüfen; Zweckbindung und Entzug
   bestätigen. Nur Anzahl/Rollen und interne Nachweis-ID festhalten, keine
   Namenslisten. Lösch- und Filterverhalten zunächst mit synthetischen Daten
   in isolierter Umgebung prüfen, einschließlich niedrigen Logvolumens,
   Neustart, Ausfall des Löschwegs und Wiederanlauf. Produktiv nur vorhandene
   Laufnachweise, Metadaten und aggregierte Altersbestände prüfen. Fehlende
   Logzufuhr und Löschfehler müssen erkennbar sein und den
   [Störungsablauf](stoerungen.md) auslösen. Das belegt noch keine vollständige
   Operator-Auditierung oder Sperre kritischer unprotokollierter Zugriffe
   (Slice 3842-b).

Die Vorschlagswerte sind keine vollständigen produktionsfertigen
Konfigurationen. Eine reine Lesekontrolle setzt keine davon um. Erforderliche
Änderungen separat freigeben, durchführen und erneut auf Wirksamkeit prüfen.
Bis dahin AK7 nicht als erfüllt markieren. Notwendige Abhilfe muss vor der
betroffenen Verarbeitung wirksam sein; ein offener Nachweis ist keine
Freigabe zur unveränderten Fortsetzung.

## Nachweis ohne Rohdaten

Je Quelle folgende Felder in einer geschützten Betriebsablage ausfüllen:

```text
Prüf-ID / Datum / prüfende Rolle / Umgebung / Commit / Dienstversion:
Quelle und Datenarten / Zweck / intern freigegebene Frist:
Effektive Alters-/Größenwerte / Rotation / maximale Löschverzögerung:
Kopien, Altbestand und deren Löschwege:
Lese-, Admin-, Exportrechte geprüft / Entzug geprüft: ja | nein | offen
Filterprobe mit synthetischen Daten / Löschprobe / produktiver Laufbeleg:
Ergebnis: belegt | Abweichung | offen | nicht vorhanden (begründet)
Abhilfe / verantwortliche Rolle / Termin / Entscheidung zur Verarbeitung:
Interne Nachweis-Referenz, keine Rohlogs oder Geheimnisse:
```

## Technische Quellen

Die Herstellerdokumentation erläutert die Grenzen, nicht den Hostzustand.
Vor Anwendung mit der installierten Version abgleichen:

- [systemd journald.conf](https://www.freedesktop.org/software/systemd/man/latest/journald.conf.html):
  Vorrang der Drop-ins, Alters-/Größenlimits und Dateirotation.
- [Caddy log](https://caddyserver.com/docs/caddyfile/directives/log):
  Rotation und an neue Dateien gebundene Archivbereinigung.
- [Loki Log retention](https://grafana.com/docs/loki/latest/operations/storage/retention/):
  Compactor, Indexvoraussetzungen, asynchrone Chunk-Löschung und Altbestand.
