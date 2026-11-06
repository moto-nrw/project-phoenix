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
| `accepted mails stayed without any provider event` | Mails haben die 24 Stunden seit der Annahme ohne Ergebnis-Ereignis überschritten. Die Zahl am Ende nennt nur die neu hinzugekommenen: Jede Mail zählt einmal, auch wenn sie noch bis zu 28 Tage geprüft wird. | Einzelne Mail: in Lettermint über `provider_message_id` nachsehen; dort kann auch ein Ereignis stehen, dessen Namen moto nicht kennt. Hohe oder wachsende Zahl: Lettermint meldet keine Ereignisse, Status von Lettermint prüfen. |
| `read the webhook state` oder `read the events of mail` | Die Team-API antwortet nicht wie erwartet. | Token `LETTERMINT_TEAM_API_TOKEN`, `LETTERMINT_WEBHOOK_ID` und Status von Lettermint prüfen. |
| `provider rate limit reached` | Lettermint hat den Lauf mit 429 beendet. Der nächste Lauf macht mit denselben Mails weiter. | Einmalig: nichts tun. Bei jedem Lauf: Die Grenze der Team-API reicht nicht für die offenen Mails, Limit mit Lettermint klären. |
| `read the open mails` | Die Datenbank hat dem Abgleich nicht geantwortet. Der Lauf hat keine Mail geprüft. | Datenbank prüfen. Andere Gründe in derselben Zeile gelten trotzdem. |

Eine Mail, die Lettermint nicht kennt, löst diesen Alarm nicht aus. Sie schreibt
einmal die Warnung `the provider does not know an accepted mail` mit `mail_id`.
Häufen sich diese Warnungen, zuerst das Format der gespeicherten
`provider_message_id` prüfen.

Der Abgleich erkennt keine Blocklisten: Die Ereignisse der Team-API enthalten
die Antwort des Zielservers nicht. Ein Hard Bounce aus dem Abgleich zeigt deshalb
nur den Status `failed` und setzt keine Adresse auf die Sperrliste der Schule.
Nur der Webhook mit der Antwort des Zielservers sperrt. Eine während eines
Ausfalls wirklich ungültige Adresse steht deshalb erst auf der Liste, wenn der
Webhook das Ereignis nachliefert; bis dahin hält Lettermint sie über seine eigene
Sperre zurück.

## Warnung `email accepted without a provider id`

Lettermint hat eine Mail angenommen, aber die Antwort 250 enthält kein
`id=<uuid>`. Der Abgleich kann diese Mail nicht prüfen, sie hängt allein am
Webhook. Die Zeile nennt `message_id` und `template`, nie die Adresse. Tritt sie
bei jeder Mail auf, hat sich das Antwortformat geändert: Der Abgleich läuft dann
leer, ohne dass der Alarm anschlägt.

## Alarm „Security mail over fallback“

Sicherheitsmails gehen über das Postfach aus `EMAIL_FALLBACK_*`. Das Feld
`reason` nennt den Auslöser: `provider_send_failed` (SMTP bei Lettermint
fehlgeschlagen) oder `provider_reported_failure` (Lettermint hat die Mail
angenommen und dann eine Sperre, einen Blocklisten-Bounce oder eine Ablehnung
gemeldet).

Eine Sicherheitsmail bekommt auf dem Notweg eigene Versuche, und das nur einmal.
Danach landet sie bei einem Fehler als `dead_letter`, sie geht nicht zurück zu
Lettermint. Mails mit dem Stand `delivered` oder `complained` gehen nie erneut
raus.

1. Status von Lettermint und Blocklisten der Versand-IPs prüfen.
2. Das Postfach ist nicht für große Mengen gedacht. Andere Mails nutzen den
   Notweg nie. Bei einem SMTP-Fehler warten sie in der Warteschlange. Hat
   Lettermint sie schon angenommen und meldet dann einen Blocklisten-Bounce,
   warten sie nicht: Sie enden als `failed` und werden nicht erneut versendet.
   Die Adresse bleibt frei.

Als Blocklisten-Bounce zählt, in dieser Reihenfolge:

1. Antwort mit Adress-Status (5.1.1, 5.1.2, 5.1.3, 5.1.4, 5.1.6, 5.1.10): Die
   Adresse ist schuld, auch wenn der Text „listed in“ enthält.
2. Antwort, die eine Blockliste, die sendende IP oder eine Reputation nennt
   (zum Beispiel `5.7.606 banned sending IP`, `low reputation`): Die Adresse
   bleibt frei, die Sicherheitsmail nimmt den Notweg.
3. Sonst gilt das Ereignis selbst.

## Webhook-Secret wechseln

`EMAIL_WEBHOOK_SECRETS` nimmt mehrere Werte, kommagetrennt. Jeder gültige reicht.
Die Reihenfolge verhindert Lücken:

1. Neues Secret in Lettermint erzeugen, das alte dort noch nicht entfernen.
2. Neues Secret per `sops edit` hinten an `EMAIL_WEBHOOK_SECRETS` anhängen und
   deployen. moto akzeptiert jetzt beide.
3. In Lettermint auf das neue Secret umstellen.
4. Mindestens 14 Stunden warten: So lange laufen Wiederholungen, die noch mit dem
   alten Secret signiert sein können.
5. Altes Secret aus `EMAIL_WEBHOOK_SECRETS` entfernen und deployen.

Ohne konfiguriertes Secret antwortet der Webhook mit 503. Das schreibt nur die
Warnung `email webhook has no signing secret`, keinen Fehler und kein
Sentry-Ereignis; Lettermint wiederholt und schaltet den Webhook nach 72 Stunden
ab.

## Sperren

Eine Sperre nach Hard Bounce gilt bei Lettermint teamweit und läuft nicht ab.
Eine Sperre nach einer Beschwerde betrifft die ganze Route, nur der Support von
Lettermint hebt sie nach einer Prüfung auf. moto hebt sie nie automatisch auf.

Beim Aufheben löscht moto bei Lettermint nur, wenn die Adresse auf der Liste der
Schule steht oder die Schule selbst eine Mail an genau diese Adresse hat. Sonst
könnte eine Schule die teamweite Sperre einer fremden Adresse löschen. Hat die
Adresse bei Lettermint eine `manual`- oder `unsubscribe`-Sperre, löscht moto
nichts und meldet das. Ohne `EMAIL_PROVIDER=lettermint` leert das Aufheben nur die
Liste der Schule.
