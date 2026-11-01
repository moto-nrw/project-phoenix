---
status: accepted
---

# Echtzeit-Signale bleiben verlustbehaftet und laufen über PostgreSQL

Für [#5457](https://github.com/moto-nrw/project-phoenix/issues/5457) erweitert
Delivery Platform den vorhandenen LISTEN/NOTIFY-Mechanismus der
Mail-Outbox-Wakeups um einen eigenen Kanal für Echtzeit-Signale. Die Vorgaben
des Issues bleiben maßgeblich: keine neue Infrastruktur, Signale nach dem
Commit, schulgebundene Zustellung und Poll-Abgleich bei Verlust.

`realtime.Hub` behält die bestehende Gruppen-, Konto-, Portal- und
Schul-Adressierung. Ein Transport-Umschlag trägt zusätzlich die School-ID.
Der Hub kodiert und prüft die Umschläge; der PostgreSQL-Adapter transportiert
nur Zeichenketten über den bestehenden Pool. Jede API
hält einen eigenen Listener, verteilt empfangene Signale nur lokal und
veröffentlicht sie nicht erneut. Ein getrennter Worker veröffentlicht ohne
Listener. Der Prozess-Root schließt den Listener vor dem Pool. Konfiguration,
TLS- und Rollen-Einstellungen stammen unverändert vom vorhandenen Connector;
es gibt keinen weiteren DSN und keinen Env-Fallback.

Dies ist bewusst keine dauerhafte Outbox: Verbindungsabbrüche, volle Puffer
und übergroße Signale dürfen Hinweise verlieren. Sie dürfen weder die
Zielschule noch die Adressierung verändern. Der bestehende Poll-Abgleich
(verbunden fünf Minuten, sonst 60 Sekunden) bleibt die Rückfallebene.
Mail und Push bleiben unabhängig davon dauerhafte Zustellaufträge.

Die unadressierten Legacy-API-Aufrufe `BroadcastToAll` bleiben unverändert
lokal. Sie werden nicht über den Schulkanal verteilt. Ihre Vertragsmigration
ist [#5492](https://github.com/moto-nrw/project-phoenix/issues/5492), nicht Teil
der bereits schulgebundenen Worker-Produzenten.

Die Konstruktion bleibt gemäß ADR 0068 in Deliverys `compose`-Paket. Der neue
Konstruktor `NewPostgresRealtimeHub` benötigt genau 32 zusätzliche Zeilen,
einschließlich Imports und Dokumentation. Nach ADR 0057 Punkt 5 steigt dafür
das Budget `modules/delivery/compose` von 361 auf 393 und das Gesamtbudget
von 12741 auf 12773. Bestehende Logik wandert nicht aus der Messung. Diese
Entscheidung erlaubt keine neue Policy-Regel, keine zusätzliche Ausnahme und
keine Änderung an Tabellen, Ownern, Baseline oder Composition-Inventar.

Betriebsbedingungen und Cutover stehen im
[Worker-Runbook](../operations/worker-lease-2726.md#realtime-across-processes-5457).
