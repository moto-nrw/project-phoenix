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
einschließlich Imports und Dokumentation. Der nur noch von Test-Roots genutzte lokale Konstruktor entfällt mit
8 Zeilen; Test-Roots bauen ihren lokalen Hub direkt. Nach ADR 0057 Punkt 5
steigt das Budget `modules/delivery/compose` damit netto von 361 auf 385 und
das Gesamtbudget von 12758 auf 12782. Bestehende Logik wandert nicht aus der Messung. Diese
Entscheidung erlaubt keine neue Policy-Regel, keine zusätzliche Ausnahme und
keine Änderung an Tabellen, Ownern, Baseline oder Composition-Inventar.

Betriebsbedingungen und Cutover stehen im
[Worker-Runbook](../operations/worker-lease-2726.md#realtime-across-processes-5457).

## Kanal als nicht vertrauenswürdiger Invalidierungsauslöser

LISTEN/NOTIFY ist kein authentifizierter Inhaltskanal. Der Umschlag enthält
Schule, Adressierung, eine zufällige Hub-Instanz-ID, einen erlaubten Event-Typ
und ausschließlich benötigte numerische Invalidierungs-IDs sowie feste
Invalidierungsgründe. Empfänger rekonstruieren daraus neue Ereignisse;
unbekannte Typen und Inhaltsfelder werden nicht übernommen. Freitext,
Anwesenheitsnotizen, Namen, Statusinhalte und Links werden nie veröffentlicht.
Die numerischen IDs optimieren nur das Nachladen, sie gewähren keinen Zugriff.
Alle Nachladeanfragen bleiben an den authentifizierten Tenant gebunden.
`notification` trägt Inhalt, den Browser direkt anzeigen; über den Kanal geht
deshalb nur die ID einer gespeicherten Kopie (siehe Ergänzung zu #5504).

Der Hub stellt zuerst lokal zu und veröffentlicht zusätzlich über eine
begrenzte Queue mit 128 Einträgen. Volle Queues und Transportfehler werden
ohne Payload protokolliert. After-Commit-Hooks warten weder auf den Pool noch
auf NOTIFY. Listener ignorieren ihre eigene Hub-Instanz-ID. Beim Shutdown
wird die Queue abgebrochen; auch dieser mögliche Verlust wird durch Polling
abgeglichen. Ein vorbereitetes Statement mit `pg_notify($1, $2)` verwendet
echte Protokoll-Bindings, da der installierte Treiber gewöhnliche
`ExecContext`-Argumente interpoliert. PostgreSQL kann Bind-Werte abhängig von
seiner Logging-Konfiguration dennoch protokollieren; sie enthalten deshalb
weiterhin keinerlei Freitext oder angezeigten Inhalt.

Die gemeinsame Lebensdauer von Publisher und Subscription benötigt weitere
8 reine Konstruktionszeilen. Deliverys Compose-Budget beträgt damit 393 statt
385, das Gesamtbudget 12790 statt 12782. Policy, Baseline und
Composition-Inventar bleiben unverändert.

Der Architekturprüfer verfolgt dafür lokal einmalig gebundene vorbereitete
Standardbibliotheks-Statements bis zu ihrem konstanten SQL-Text und wendet
dieselben Tabellen-/Owner-Prüfungen wie bei direkter Ausführung an. Dynamisches
SQL, unbekannte Statement-Parameter, Neuzuweisungen und Adress-Escapes bleiben
unaufgelöst und schlagen weiterhin fehl. Der Regressionstest umfasst erlaubte
NOTIFY-/Owner-Abfragen sowie fremde Tabellen und diese unaufgelösten Fälle;
es entsteht keine Policy-Ausnahme für den Transport.

## Bereitschaft des Listeners beim API-Start

`pgdriver.Listener.Listen` schreibt die LISTEN-Befehle nur auf den Socket;
es wartet nicht auf die Bestätigung des Servers. Der Konstruktor bestätigt
die Anmeldung deshalb über eine inhaltsfreie, zufällige Probe auf dem
zusätzlichen PostgreSQL-Kanal `moto_realtime_ready_v1`. Beide LISTEN-Befehle
laufen auf derselben Verbindung und werden dort geordnet verarbeitet. Erst
der Empfang der eigenen Probe beweist die wirksame Anmeldung am Ereigniskanal.
Eine Probe auf der separaten Pool-Verbindung kann die Anmeldung überholen;
nur dieser Start-Handshake wird innerhalb der vorhandenen fünf Sekunden
wiederholt. Danach meldet sich der Listener vom Bereitschaftskanal ab.
Bereitschaftsproben werden niemals an Browser weitergeleitet. Ein Test hält
LISTEN-Pakete an der TCP-Grenze zurück und weist nach, dass der Konstruktor
bis zur tatsächlichen Bereitschaft wartet. Ereignisse selbst werden nicht
wiederholt; Verlust bei späteren Verbindungsabbrüchen bleibt durch Polling
abgedeckt. Worker ohne Listener führen keinen Handshake aus.

Für `staff_message` bleibt die numerische Absender-Konto-ID als `source`
erhalten. Der Teamchat benötigt sie, damit ein anderer Tab desselben Senders
nicht durch ein unnötiges Nachladen unbeabsichtigt einen Lesecursor vorzieht.
Andere Quellen sowie nicht numerischer Freitext bleiben ausgeschlossen.
Die Statement-Analyse zählt auch Zuweisungen an `range`-Ziele: Eine dort
ersetzte Statement-Variable darf nicht weiter anhand ihres ursprünglichen
konstanten SQL-Texts freigegeben werden.

## Ergänzung 2026-11-02: Benachrichtigungen werden nachgeladen (#5504)

Der SSE-Kanal von Delivery speichert jede In-App-Benachrichtigung eines
relayenden Hubs in `platform.realtime_notifications`: Schule, Zielgruppe,
die bereits aufgelösten Konten oder Erziehungsberechtigten, die Portale und
die anzeigefertige Nutzlast. Das Signal enthält nur Schule, Adressierung und
die ID dieser Kopie. Der empfangende Prozess liest die Kopie in der
Tenant-Transaktion der genannten Schule unter RLS und zeigt sie nur dort an,
wo Signal und Kopie dieselbe Zielgruppe, dieselbe Gruppe und dieselben
Empfänger nennen; bei Konten gilt die Schnittmenge. Unbekannte, fremde,
anders adressierte oder ältere als zwei Minuten alte IDs zeigen nichts an.
Inhalt aus dem Signal wird nie übernommen. Kann die Kopie nicht gespeichert
werden, bleibt die Benachrichtigung im eigenen Prozess; die lokale Zustellung
hängt nie vom Kanal ab. Die nächtliche Delivery-Aufbewahrung löscht
abgelaufene Kopien, die Schullöschung kennt die Tabelle.

Topics und `active_group_id` werden beim Senden und Empfangen auf eine
Aktivgruppen-ID oder `edu:<ID>` normalisiert; anderes wird verworfen. Eine
volle Queue zählt verworfene Signale und warnt höchstens alle 30 Sekunden,
statt pro Signal einen Fehler an den Aufrufer zu geben. Transport- und
Empfangsfehler werden mit Ursache, aber ohne Payload protokolliert.
