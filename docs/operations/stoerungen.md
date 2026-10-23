# Produktionsstörungen erkennen und bearbeiten

Slice 3845-a, Refs #3845, Teil von #3846. Stand: 23.10.2026.

## Stand und Zuständigkeit

**Im Code umgesetzt:** Die API bietet `GET /ready` auf dem API-Listener.
Die Probe prüft den laufenden API-Prozess und pingt dessen Datenbankpool mit
höchstens zwei Sekunden Wartezeit. Sie liest keine Mandantendaten. Erfolg:
200 mit `OK`; Fehler: 503 im gemeinsamen Fehlerformat, ohne DB-Details.
`/health` bleibt eine reine Prozessprobe. Das Frontend wird nicht mitgeprüft.

**Produktiv nachzuweisen:** Deployment, Probe-Ziel des Healthcheck-Containers,
Alarmzustellung und Vertretung sind in diesem Slice nicht am System geprüft.
Das [Monitoring-README](../../monitoring/README.md#provisioned-state-on-the-monitoring-host)
dokumentiert bereits provisionierte Regeln. Das ist kein neuer Zustellnachweis.
Die Container-Konfiguration und die Empfänger liegen nur auf dem Monitoring-Host.

Vor Inbetriebnahme hinterlegt die Betriebsleitung die Besetzung und erreichbaren
Kanäle in der geschützten Betriebsablage, nicht im Repository:

- **Betriebsdienst:** nimmt Alarme an, bestätigt Übernahme und führt das Störungsprotokoll.
- **Vertretung:** übernimmt bei ausbleibender Bestätigung; Betriebsleitung legt
  Reaktions- und Eskalationszeiten fest und prüft die Erreichbarkeit.
- **Technische Einsatzleitung:** grenzt Ursache ein und koordiniert Wiederherstellung.
- **Datenschutzrolle:** bewertet mögliche Datenschutzverletzungen und koordiniert
  erforderliche Meldungen nach dem geltenden Vorfallverfahren.
- **Betriebsleitung:** entscheidet über Kundeninformation und prüft Abschluss,
  offene Maßnahmen und SLA-Nachweise.

## Erkennungswege und Grenzen

Vorhandene Grafana-Regeln weiterverwenden, keine neuen Contact Points oder
Empfänger provisionieren:

- [`production.yml`](../../monitoring/grafana/provisioning/alerting/production.yml):
  `Production Server Down` zählt Loki-Erfolgsmeldungen
  `{env="prod", service="healthcheck"} |= "health check passed"` über fünf Minuten,
  Auswertung jede Minute, Haltezeit fünf Minuten. Auch fehlende Daten und
  Auswertungsfehler alarmieren. Ein Ausfall kann deshalb ungefähr zehn Minuten
  plus Probe-, Übertragungs- und Zustellzeit ungemeldet bleiben. Kurze Ausfälle
  können ohne Alarm enden. `DB Connection Error` erkennt nur den Text
  `connection refused`; `HTTP 5xx Spike` nur die konfigurierte Fehlerschwelle.
- [`infrastructure.yml`](../../monitoring/grafana/provisioning/alerting/infrastructure.yml):
  Host-Disk, RAM, CPU, Scrape-Ausfall, Backup, DB-Pool, Postgres-Verbindungen und
  Frontend-Event-Loop. Einige Regeln behandeln fehlende Daten/Auswertungsfehler
  als OK. Aktive Datenquellen und Labels vor Ort prüfen.
- [`server-errors.yml`](../../monitoring/grafana/provisioning/alerting/server-errors.yml):
  Server-/Postgres-Fehlerspitzen; [Fehler-Runbook](../../monitoring/runbooks/server-errors.md).
  Weitere fachspezifische Regeln und ihr Ausrollstand stehen im Monitoring-README.
- Supportmeldungen und gezielte Funktionsprüfungen ergänzen technische Alarme.
  Eine Meldung eröffnet denselben Bearbeitungsablauf wie ein Alarm.

Bekannte Lücken, die ein grünes `/ready` nicht ausschließt:

- **Worker/Scheduler:** ausgelassene oder fachlich falsche Läufe. Erwartete
  Laufnachweise prüfen, bei fehlendem Nachweis an den Betriebsdienst melden.
- **Mailzustellung:** SMTP-Annahme beweist keinen Posteingang. Zustellfehler und
  Rückläufer bearbeiten; Funktionsprobe nur mit freigegebenen Testadressen.
- **Einzelne Mandanten und Kiosks:** Berechtigungs-, RLS-, Daten- oder lokale
  Verbindungsfehler bleiben möglich. Supportmeldungen und freigegebene fachliche
  Prüfungen nutzen, ohne fremde Mandantendaten abzufragen.
- **Langsamkeit und Frontend:** DB-Ping beweist weder Antwortzeit der Fachrouten
  noch DNS, TLS, Reverse Proxy, Anmeldung oder Browser-Nutzbarkeit. Bestehende
  Metriken und Supportsignale prüfen. Beim Umstellen den bisherigen Netzpfad
  erhalten und separat festhalten, welche Komponenten er abdeckt.
- **Sicherheitsereignisse und Monitoring-Ausfall:** Missbrauch kann ohne 5xx
  auftreten; ein gemeinsamer Ausfall von Grafana/Loki/Alarmkanal kann stumm bleiben.
  Sicherheitsmeldungen gesondert bewerten, den Alarmweg regelmäßig und nach
  Änderungen testen. Betriebsleitung bewertet anhand dieser Tests, ob der
  bestehende Messpunkt ausreicht oder eine unabhängige Kontrolle nötig ist.

## Mensch-Schritt: bestehende Probe umstellen

Nur durch berechtigten Betrieb am System ausführen. Dieser Slice führt keine
Produktions-/Staging-Requests und keinen Serverzugriff aus.

1. Backend mit `/ready` ausrollen. Mit einer freigegebenen lokalen Prüfung am
   API-Listener bestätigen: 200 bei erreichbarer DB. Zuerst in einer isolierten
   Testumgebung DB-Unerreichbarkeit prüfen: 503, höchstens zwei Sekunden für den
   Ping, keine DB-Details. Keine produktive DB für einen Test abschalten.
2. Die server-lokale Compose-/Probe-Quelle für `monitoring-healthcheck-prod-1`
   und `monitoring-healthcheck-staging-1` finden und geschützt sichern. Bestehende
   Ziele, Netzpfad, Intervall, HTTP-Timeout und Erfolgskriterium feststellen, ohne
   Umgebungsvariablen oder Geheimnisse in Nachweise zu kopieren.
3. Beim API-Ziel nur `/health` durch `/ready` ersetzen. Kein Frontend-Ziel durch
   den API-Check ersetzen. HTTP-Timeout mit Abstand über zwei Sekunden setzen.
   Nur HTTP 200 darf eine `health check passed`-Zeile erzeugen; 503, Timeout und
   Verbindungsfehler dürfen keinen Erfolg melden. Vorhandene Labels `env` und
   `service="healthcheck"` beibehalten. Betroffenen Probe-Container neu erstellen.
4. In der Testumgebung Erfolg, Fehler, fehlende Probe-Daten und Wiederkehr
   prüfen. In Grafana Explore die zugehörigen Loki-Zeilen und in der Alarmhistorie
   Pending/Firing/Normal prüfen. Zustellung und Bestätigung durch die zuständige
   Rolle über den bestehenden Kanal belegen. Keine Empfänger im Repo hinterlegen.
5. Produktion ohne künstlichen Ausfall auf erfolgreiche `/ready`-Proben prüfen.
   Version, Zeitpunkt, Zielpfad/Netzabdeckung, Intervall/Timeout, getestete Fälle,
   Alarm-ID, Zustellnachweis und verbleibende Lücken geschützt dokumentieren.
   Bei Rücknahme die gesicherte Probe-Konfiguration wiederherstellen und
   ausdrücklich festhalten: `/health` erkennt keinen DB-Ausfall.

## Ablauf bei einer Störung

1. **Übernehmen:** Betriebsdienst bestätigt Übernahme im vereinbarten Kanal,
   eröffnet eine Vorfall-ID und erfasst erste Beobachtung, betroffene Funktionen
   und Umgebung. Ohne Bestätigung greift die hinterlegte Vertretung/Eskalation.
2. **Eingrenzen:** Alarmhistorie und Loki-Zeiten korrelieren; Umfang und letzte
   erfolgreiche/erste fehlgeschlagene Probe ermitteln. Fachliche Fehler und
   mögliche Sicherheitsereignisse getrennt bewerten. Bei Datenschutzverdacht
   sofort die Datenschutzrolle einschalten, nicht auf vollständige Diagnose warten.
3. **Beheben:** technische Einsatzleitung dokumentiert Entscheidungen,
   Änderungen und Wiederherstellung. Nur erforderliche, berechtigte Eingriffe;
   keine Personen, Tokens, Nutzdaten oder Rohlogs in öffentliche PRs übernehmen.
   Betriebsleitung koordiniert erforderliche Information der Betroffenen/Kunden.
4. **Verifizieren:** erste erfolgreiche Probe und fachliche Funktionsprüfung
   festhalten. Ein grüner DB-Ping allein schließt den Vorfall nicht ab.
5. **Abschließen:** Ursache, Beginn/Ende samt Unsicherheit, Auswirkungen,
   Verantwortungsrollen, Meldungen und Folgemaßnahmen prüfen. Fehlalarme und
   Erkennungslücken festhalten; Betriebsleitung bestätigt den Abschluss.

## Betriebs- und Ausfallzeiten für das SLA erfassen

Keine automatische SLA-Berechnung in diesem Slice. Maßstab ist die jeweils
vereinbarte Vertragsfassung. Aktueller [AGB-Entwurf, Anhang B](https://github.com/moto-nrw/legal/blob/main/vertragspaket/02-agb.md#anhang-b-service-level-und-support):
99 % im Jahresmittel innerhalb der Kernzeit, Montag bis Freitag 06:30–18:00,
ohne gesetzliche Feiertage am Standort, Schulferien eingeschlossen. Für die
Auswertung Zeitzone `Europe/Berlin` und den vertraglich relevanten Standort
festhalten. Das Sonderkündigungsrecht erfordert zusätzlich Monatswerte
(drei Monate unter 95 % innerhalb von sechs Monaten). Keine SLA-Gutschriften
aus diesem Verfahren ableiten.

Für jeden Vorfall einen geschützten Datensatz führen: Vorfall-ID, Umgebung,
betroffene Funktionen, Alarm-ID/Quelle, Beginn/Ende in UTC mit Berliner Ortszeit,
letzte erfolgreiche/erste fehlgeschlagene sowie letzte fehlgeschlagene/erste
erfolgreiche Probe, Auflösung der Messung, Datenlücken, Ursache und Nachweislinks.
Alarmbeginn ist wegen Fenster und Haltezeit **nicht** der Ausfallbeginn.
Unklare Grenzen als Zeitintervall dokumentieren, nicht als sekundengenauen Wert.
Überlappende Ausfälle für dieselbe Leistung später nur einmal zählen.

Grafana-Alarmhistorie und zugehörige Loki-Proben nach Vorfällen und vor Ablauf
der vor Ort bestätigten Aufbewahrung geschützt sichern. Export und lesbare
Wiederherstellung praktisch prüfen; fehlende/stumme Zeiträume sind **unbekannt**,
nicht automatisch verfügbar. Auch Betriebszeiträume ohne Vorfälle brauchen
einen vollständigen Probe-/Überwachungsnachweis. Rohlogs mit Personenbezug nicht
ungeprüft exportieren. Zuständige Rolle und Ablage im Betriebsnachweis benennen.

Wartung/Deploys als tatsächliche Unterbrechungen erfassen, auch bei erlaubten
Deploys zu jeder Zeit. Einen vertraglichen Ausschluss (angekündigte Wartung
außerhalb der Kernzeit, dringendes Sicherheitsupdate, Störung außerhalb des
Einflussbereichs) nur mit Begründung und Nachweis separat kennzeichnen, nicht
aus Rohzeiten löschen. Bei späterer Berechnung Kernzeit und begründete
Ausschlüsse anwenden; Messlücken und Unsicherheit mit ausweisen.

**Offen bis zum menschlichen Nachweis:** Rollenbesetzung und Eskalation,
Produktivumstellung, vollständiger Alarm-/Zustelltest, Aufbewahrungsdauer und
geschützte Sicherung der Messhistorie sowie ausreichende Abdeckung der genannten
Lücken. Erst danach lässt sich die Erfüllung der gesamten Issue-AKs beurteilen.
