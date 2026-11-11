# Schließtag-Hinweise: hängende bestätigte Aufträge

Der Task `holiday-closure-watchdog` liest die Auftragszahlen jede Minute in
seiner eigenen administrativen Transaktion mit 30 Sekunden Timeout. Er läuft
unabhängig von `holiday-closure-notifications` und verändert keine Daten.
Er liest auch Aufträge, deren Schule inzwischen den Träger gewechselt hat.

Als überfällig zählen:

- `blocked` sofort, auch ohne weiteren Verarbeitungsversuch;
- `pending` seit mehr als 15 Minuten fällig;
- `leased` mit seit mehr als 15 Minuten abgelaufener Lease;
- `prepared` oder `unknown` seit mehr als 15 Minuten ohne geklärten Versand.

15 Minuten lassen gegenüber der fünfminütigen Verarbeitungs-Lease und dem
minütlichen Aufholen Zeit für mehrere normale Läufe. Das ist eine technische
Alarmgrenze, keine Wartezeit für den Elternhinweis. Unbestätigte Vorschläge,
`needs_confirmation` und bekannte Versandfehler mit einem regulären Nachholknopf
lösen diesen Alarm nicht aus.

Grafana wertet jede Minute den neuesten Zähler `overdue` aus dem Log
`holiday closure watchdog checked` aus. Ein Wert über null löst den Alarm aus.
Auch ohne laufenden Worker wertet Grafana weiter aus: Fehlt die erfolgreiche
Prüfung zehn Minuten lang, löst `holiday-closure-watchdog-missing` aus.
Abfragefehler und fehlende Daten lösen beim Heartbeat ebenfalls Alarm aus.
Die bestehenden Grafana-Kontakte werden verwendet.

## Alarm bearbeiten

1. Bei fehlender Prüfung Worker-Lease, Task-Registrierung, Datenbank und
   Loki prüfen. `holiday closure watchdog failed` nennt den technischen Fehler.
   Nach Wiederherstellung muss jede Minute ein neuer Prüfwert erscheinen.
2. Bei überfälliger Arbeit die betroffenen IDs und Zustände lesend aus
   `calendar.holiday_closure_events` ermitteln. Keine Empfänger, Payloads oder
   Familiennamen in Logs oder Tickets übernehmen.
3. Bei `pending` oder abgelaufener Lease den Verarbeitungsjob und dessen
   Fehlermeldungen prüfen. Nach Behebung übernimmt der normale Worker.
   Bei `blocked` zuerst die Ursache beheben, danach denselben Hinweis im
   Schul-Portal oder Träger-Büro über „Eltern informieren“ wieder aufnehmen.
4. Bei `prepared` die Delivery-Queue und den Provider prüfen. Bei `unknown`
   den tatsächlichen Transportausgang nach dem
   [Mail-Runbook](email-delivery.md) klären. Keine neue Mail und keinen neuen
   Vorgang anlegen, keinen unklaren Ausgang auf Verdacht zurücksetzen.
5. Nach der Behebung müssen der nächste Prüfwert null und der Heartbeat
   aktuell sein. Beide Alarme lösen sich dann auf.

## Aufbewahrung und Wiederaufnahme

Fehlgeschlagene und im Ausgang unklare Schließtag-Mails behalten ihren Inhalt
über die technische 30-Tage-Frist hinaus, damit derselbe Auftrag nachholbar
bleibt. Erfolgreiche Mails behalten danach nur Versandstatus, Idempotenzschlüssel
und `registration_ids` für die vorhandene Anmeldungslöschung; Empfänger,
Mailinhalt und Providerdaten werden entfernt. Die fachliche Anmeldungslöschung
bleibt maßgeblich. Ein offener Auftrag mit erneuter Bestätigung wird nicht
allein wegen seines Alters entfernt.

## Nach dem Deployment prüfen

1. Jede Minute einen Prüfwert ohne personenbezogene Angaben erwarten.
2. In einer isolierten Testumgebung einen fälligen Auftrag altern lassen,
   den Verarbeitungsjob anhalten und den Überfälligkeitsalarm prüfen.
3. Den Watchdog anhalten und nach zehn Minuten den fehlenden Heartbeat prüfen.
4. Nach Wiederanlauf beide Auflösungen und den vorhandenen Alarmkontakt prüfen.
   Die tatsächliche Benachrichtigung der produktiven Kontakte ist durch lokale
   Tests nicht belegt.
