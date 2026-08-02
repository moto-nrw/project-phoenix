# Check-in-Logging: Zweck, Zugriff, Aufbewahrung

Betrifft Issue #2062. Dieses Dokument hält fest, welche Felder der IoT-Check-in
auf Info-Ebene und höher schreibt, warum sie gebraucht werden, wer sie sehen
kann und wann sie verschwinden. Es ist die Grundlage für den Nachweis der
Datenminimierung nach Art. 5 Abs. 1 lit. c DSGVO für diesen Pfad.

## Was sich geändert hat

Vorher schrieb der detaillierte Check-in die Kiosk-Begrüßung als Feld
`message` auf Info-Ebene:

```json
{"level":"INFO","msg":"checkin complete","action":"checked_in","student_id":1450000001,
 "message":"Hallo Max!","visit_id":140000001,"room":"Gruppenraum 2"}
```

`message` enthielt den Vornamen des Kindes. Über journald landete die Zeile in
Loki und lag dort für die volle Aufbewahrungsdauer. Zusätzlich schrieb der Pfad
auf Info-Ebene `found student` mit der Schulklasse des Kindes.

Beides ist entfernt:

- `message` ist ersatzlos aus dem Info-Log gestrichen. Die Begrüßung steht
  weiterhin unverändert im HTTP-Response (`data.message`), den der Kiosk
  anzeigt. Die API hat sich nicht geändert, PyrePortal braucht keine Anpassung.
- `found student` (mit `class`) läuft jetzt auf Debug. Debug wird in
  Staging und Produktion durch `LOG_LEVEL=info` herausgefiltert.

## Verbleibende Felder auf Info-Ebene und höher

Referenz: `backend/api/iot/checkin/handlers.go`,
`backend/services/iot/checkin/checkin_service.go`.

| Feld | Wo | Zweck | Personenbezug |
|---|---|---|---|
| `student_id` | `checkin complete`, `binary mode checkin complete`, `checked in student`, `checked out student`, Fehlerpfade | Der einzige Anker, um einen gemeldeten Fehlscan (falscher Raum, doppelter Check-out, hängender Besuch) einem konkreten Vorgang zuzuordnen und ihn in der Datenbank nachzuvollziehen | Pseudonym. Ohne Datenbankzugriff nicht auflösbar |
| `visit_id` | `checkin complete`, `checked in/out student` | Verknüpft die Logzeile mit der Zeile in `active.visits`; nötig, um Doppelbesuche und nicht beendete Besuche zu untersuchen | Pseudonym |
| `staff_id` | Binary-Mode-Check-in, Fehlerpfade | Zuordnung, welche Betreuungskraft den Scan quittiert hat (Aufsichtsfrage) | Pseudonym |
| `action` | alle Abschluss-Logs | `checked_in` / `checked_out` / `transferred` / `checked_out_daily`; trägt die Diagnose ohne jeden Personenbezug | keiner |
| `room` / `room_name` / `activity_name` | Abschluss- und Kapazitäts-Logs | Kapazitäts- und Raumprobleme sind ohne Raumnamen nicht lesbar | keiner (Objekt, keine Person) |
| `rfid` | nur wenn ein Tag **nicht** aufgelöst werden konnte | Ein unbekannter Tag ist der häufigste Supportfall ("Karte geht nicht"); ohne die Tag-ID ist er nicht bearbeitbar | Zu diesem Zeitpunkt keiner: der Tag ist keiner Person zugeordnet. Derselbe Vorgang wird ohnehin als `audit`-Eintrag über den Unregistered-Tag-Scan-Service erfasst |
| `device_id` | Fehlerpfade, Ping | Gerätezuordnung bei Störungen einer einzelnen Station | keiner |

Namen, Klassen und Begrüßungstexte gibt es auf Debug (`found student`,
`student has active visit, performing checkout`, `performing check-in to room`,
`RFID tag belongs to person`). Debug ist in Staging und Produktion abgeschaltet.

### Zugriff

Die Logs laufen über journald des Hosts nach Loki und sind nur über Grafana
(`grafana.moto-app.de`, eigener Login) beziehungsweise mit Root-Zugang auf dem
Hetzner-Host lesbar. Loki und Prometheus sind nicht öffentlich exponiert
(localhost-gebunden, Zugriff ausschließlich über Caddy vor Grafana). Der
Personenkreis ist der Betriebszugang zum Monitoring-Host.

### Aufbewahrung

Die Logaufbewahrung wird auf dem Monitoring-Host konfiguriert
(`/root/monitoring/`, nicht in diesem Repository). Sie ist unabhängig von der
fachlichen Aufbewahrung der Anwesenheitsdaten, die über die
`gdpr.data_cleanup_*`-Einstellungen und `audit.data_deletions` läuft.

Die konkrete Dauer steht in der Loki-Konfiguration auf dem Host
(`limits_config.retention_period` beziehungsweise die `compactor`-Sektion) und
ist aus diesem Repository nicht prüfbar. Vor jeder Aussage nach außen dort
nachsehen.

## Altbestand in Loki

Ab dem Deployment dieser Änderung schreibt kein Check-in mehr einen Vornamen
auf Info-Ebene. Alle betroffenen Zeilen stammen daher aus der Zeit **vor** dem
Deployment und laufen spätestens eine volle Aufbewahrungsperiode danach ab.

Betroffene Zeilen finden (Grafana Explore, Loki-Datenquelle):

```logql
{env="prod"} | json | msg = "checkin complete" | message != ""
```

Wenn die Daten früher weg sollen, statt auf den Ablauf zu warten: Lokis
Delete-API löscht stream- und zeitraumbezogen, nicht feldbezogen — sie entfernt
also die kompletten Logzeilen im gewählten Zeitfenster. Sie setzt
`compactor.retention_enabled: true` und `deletion_mode: filter-and-delete`
voraus; beides ist auf dem Host zu prüfen, bevor der Aufruf abgesetzt wird:

```bash
# auf dem Monitoring-Host, Zeitstempel als Unix-Sekunden
curl -XPOST -G 'http://localhost:3100/loki/api/v1/delete' \
  --data-urlencode 'query={env="prod"} |= "checkin complete"' \
  --data-urlencode "start=<unix-ts>" \
  --data-urlencode "end=<unix-ts>"
```

Dashboards und Alert-Regeln liegen auf dem Monitoring-Host
(`/root/monitoring/grafana/provisioning/`) und sind aus diesem Repository nicht
einsehbar. Vor dem Löschen dort gegenprüfen, ob eine Regel oder ein Panel auf
`message` filtert; die bekannten Loki-Regeln arbeiten über `msg` und
HTTP-Status, wären also nicht betroffen.

Exporte: Es gibt keinen automatisierten Loki-Export aus diesem Repository. Falls
manuell CSV/JSON aus Grafana gezogen wurde, gehören diese Dateien mit in die
Löschung.

## Was das absichert

| Prüfung | Ort |
|---|---|
| Ein echter Check-in und Check-out über den Router; kein Info-Record enthält Vor- oder Nachnamen, Begrüßung oder ein `message`-Attribut; `action`, `student_id`, `visit_id`, `room` bleiben vorhanden; die HTTP-Antwort trägt die Begrüßung weiterhin | `backend/api/iot/checkin/gdpr_logging_test.go` |
| Dasselbe für den Binary-Mode-Pfad | `backend/api/iot/checkin/gdpr_logging_internal_test.go` |
| Statischer CI-Ratchet: kein Log-Aufruf ab Info-Ebene darf backendweit `FirstName`, `LastName` oder `GreetingMsg` lesen; im IoT-Baum zusätzlich keine namenstragenden Log-Keys | `backend/test/gdpr_log_pii_ratchet_test.go` |

## Bekannte Randfälle

- **`DB_DEBUG=true`** hängt den BUN-Query-Hook ein. Der protokolliert langsame
  Abfragen (ab 5 ms) auf Warn-Ebene mit den ersten 200 Zeichen des fertigen
  SQL — inklusive eingesetzter Werte, also möglicherweise Namen. Der Schalter
  ist eine reine Entwicklungshilfe und darf in Staging und Produktion nicht
  gesetzt werden.
- Der Request-Logger protokolliert weder Request- noch Response-Bodies
  (`WithRequestBody: false`, `WithResponseBody: false` in `api/base.go`). Die
  Begrüßung im Response taucht deshalb an keiner anderen Stelle im Log auf.
- `POST /api/iot/staff/rfid` protokolliert die RFID-Tag-ID einer
  Mitarbeiterzuordnung auf Info-Ebene (`api/iot/data/rfid_handlers.go`). Das
  betrifft keine Kinderdaten und liegt außerhalb dieses Issues, ist aber bei
  einer künftigen Durchsicht der Personal-Endpunkte einen Blick wert.
