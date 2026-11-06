# Zustellung über Lettermint

Die Logzeilen enthalten keine Empfängeradressen. Der Abgleich läuft alle zehn
Minuten im Worker (`email-delivery-reconciliation`) und nur mit
`EMAIL_PROVIDER=lettermint`.

## Alarm „Email delivery alarm“

Das Feld `error` nennt den Grund:

| Text | Bedeutung | Aktion |
|---|---|---|
| `email webhook is disabled at the provider` | Lettermint hat den Webhook nach 72 Stunden Fehlern abgeschaltet. Ereignisse aus dieser Zeit kommen nie an. | Ursache der Fehler beheben (Antworten von `/webhooks/email/delivery/lettermint`, Secret), dann den Webhook im Dashboard einschalten. |
| `the provider reported events the webhook never delivered` | Der Abgleich fand einen Stand, den der Webhook nicht gebracht hat. | Zustellprotokoll des Webhooks in Lettermint prüfen. Der Stand der Mails ist bereits korrigiert. |
| `accepted mails stayed without any provider event` | Eine angenommene Mail hat nach 24 Stunden kein Ereignis. | Die Mail in Lettermint über `provider_message_id` nachsehen. |
| `read the webhook state` oder `read the events of mail` | Die Team-API antwortet nicht wie erwartet. | Token `LETTERMINT_TEAM_API_TOKEN`, `LETTERMINT_WEBHOOK_ID` und Status von Lettermint prüfen. |

Der Abgleich erkennt keine Blocklisten: Die Ereignisse der Team-API enthalten
die Antwort des Zielservers nicht. Während eines Webhook-Ausfalls kann ein
Blocklisten-Bounce deshalb eine Adresse auf die Sperrliste der Schule setzen.
Nach so einem Ausfall die in dieser Zeit neu gesperrten Adressen prüfen.

## Alarm „Security mail over fallback“

Sicherheitsmails gehen über das Postfach aus `EMAIL_FALLBACK_*`. Das Feld
`reason` nennt den Auslöser: `provider_send_failed` (SMTP bei Lettermint
fehlgeschlagen) oder `provider_reported_failure` (Lettermint hat die Mail
angenommen und dann eine Sperre, einen Blocklisten-Bounce oder eine Ablehnung
gemeldet).

1. Status von Lettermint und Blocklisten der Versand-IPs prüfen.
2. Das Postfach ist nicht für große Mengen gedacht. Andere Mails nutzen den
   Notweg nie und warten in der Warteschlange.

## Sperren

Eine Sperre nach Hard Bounce gilt bei Lettermint teamweit und läuft nicht ab.
Eine Sperre nach einer Beschwerde betrifft die ganze Route, nur der Support von
Lettermint hebt sie nach einer Prüfung auf. moto hebt sie nie automatisch auf.
