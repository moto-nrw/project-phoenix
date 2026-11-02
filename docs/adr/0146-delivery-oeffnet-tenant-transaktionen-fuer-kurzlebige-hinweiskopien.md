---
status: proposed
---

# Delivery öffnet Tenant-Transaktionen für kurzlebige Hinweiskopien

Vorschlag vom 2026-11-02 für #5504, ergänzend zu ADR 0145. Der
Hintergrund-Worker speichert Hinweise nach dem Commit des Erzeugers. Die
empfangende API hat keinen Request mit einer bereits geöffneten
Tenant-Transaktion. Beide brauchen deshalb eine eigene, ausdrücklich auf die
adressierte Schule begrenzte Transaktion. Eine administrative Verbindung oder
ein Inhalt im nicht authentifizierten Signal wären keine sicheren Alternativen.

Genau zwei neue Command-Grenzen in Delivery sind vorgesehen:

1. `sseChannel.relayable` speichert die unveränderliche Kopie in der
   Tenant-Transaktion der adressierten Schule unter RLS. Der Speicherversuch
   einschließlich Verbindungsaufnahme ist auf 250 ms begrenzt. Ein Fehler
   lässt den Hinweis lokal; es gibt keine dauerhafte Zustellgarantie.
2. `relayedContent.load` liest die höchstens zwei Minuten alte Kopie in der
   Tenant-Transaktion der Signalschule unter RLS. Signal und gespeicherte
   Adressierung müssen übereinstimmen. Für Staff-, Lehrkraft- und
   Eltern-Adressen prüft dieselbe Transaktion über Delivery's Projektion
   `platform.delivery_realtime_recipients` erneut die Regeln, die Web Push
   beim Versand anwendet: aktives Konto und aktive Schulzuordnung, Staff-
   bzw. Lehrkraft-Rolle, bei Erziehungsberechtigten die Guardian-Rolle und
   `parent_portal.access` für eines der Kinder, um die es geht. Eltern-Kopien
   speichern dafür die Schüler-IDs nur im JSON. Fehlende Prüfinformationen
   und Fehler verwerfen die Zustellung.

Der Nachlader hängt weder vom sendenden Kanal noch von anderen Ownern ab. Er
wird vollständig aus Datenbank, Unit of Work und Store gebaut und dem Hub bei
der Konstruktion übergeben, bevor der Listener startet (ADR 0059, 0068).

Die exakten Stellen und Begründungen stehen in
`backend/test/realtime_notification_transaction.go` und im Transaktionsregister.
Die Ausnahme gilt weder für weitere Stellen noch für geänderte Begründungen.
SQL bleibt im bestehenden Postgres-Adapter von Delivery; Owner und
Importregeln ändern sich nicht, Compose- und andere Budgets bleiben unverändert.

## Freigabe

Bis eine verantwortliche Person diesen Vorschlag annimmt, gelten die zwei
Grenzen nur als beantragt.
