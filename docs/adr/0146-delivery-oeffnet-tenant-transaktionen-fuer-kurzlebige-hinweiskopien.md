---
status: accepted
---

# Delivery öffnet Tenant-Transaktionen für kurzlebige Hinweiskopien

Entscheidung vom 2026-11-02 für #5504, ergänzend zu ADR 0145. Der
Hintergrund-Worker speichert Hinweise nach dem Commit des Erzeugers. Die
empfangende API hat keinen Request mit einer bereits geöffneten
Tenant-Transaktion. Beide brauchen deshalb eine eigene, ausdrücklich auf die
adressierte Schule begrenzte Transaktion. Eine administrative Verbindung oder
ein Inhalt im nicht authentifizierten Signal wären keine sicheren Alternativen.

Genau zwei neue Command-Grenzen in Delivery sind zulässig:

1. `sseChannel.relayable` speichert die unveränderliche Kopie in der
   Tenant-Transaktion der adressierten Schule unter RLS. Der Speicherversuch
   einschließlich Verbindungsaufnahme ist auf 250 ms begrenzt. Ein Fehler
   lässt den Hinweis lokal; es gibt keine dauerhafte Zustellgarantie.
2. `sseChannel.loadRelayed` liest die höchstens zwei Minuten alte Kopie in der
   Tenant-Transaktion der Signalschule unter RLS. Signal und gespeicherte
   Adressierung müssen übereinstimmen. Eltern-Hinweise speichern zusätzlich
   die Schüler-IDs im JSON. Vor Remote-Zustellung prüft die bestehende
   `permittedGuardians`-Grenze erneut `parent_portal.access`. Fehlende
   Prüfinformationen und Fehler verwerfen die Zustellung.

Die exakten Stellen und Begründungen stehen in
`backend/test/realtime_notification_transaction.go` und im Transaktionsregister.
Die Ausnahme gilt weder für weitere Stellen noch für geänderte Begründungen.
SQL bleibt im bestehenden Postgres-Adapter von Delivery; Owner und
Importregeln ändern sich nicht.

Die Bindung des Stores im bestehenden `compose.NewTenantSSEChannel` benötigt
11 zusätzliche Compose-Zeilen. Dafür steigt das Budget von
`modules/delivery/compose` von 393 auf 404 und die Summe von 12790 auf 12801
(ADR 0057 Punkt 5). Andere Budgets und Ausnahmeregister bleiben unverändert.
