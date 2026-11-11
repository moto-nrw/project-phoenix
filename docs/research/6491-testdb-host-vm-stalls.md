# #6491: I/O-Timeouts beim Mandantenaufbau am gemeinsamen Testserver

Messstand: `origin/development` @ `133c223179`, 11.11.2026. macOS/arm64,
16 logische CPUs, OrbStack-VM mit 16 vCPUs und 16 GB, Postgres 17.11/alpine.
Fremde Agentenlast lief durchgehend weiter (Load-1 zwischen 4 und 35).

## Ergebnis

`EnsureTestTenant` scheiterte nicht in Postgres und nicht am Verbindungspool.
Der Wartepfad ist die TCP-Strecke vom Mac in die Docker-VM. Diese Strecke steht
für alle Verbindungen zugleich still, während Schreib- und Flush-Spitzen der
VM-Platte laufen. Die Spitzen erzeugt vor allem der Klon-Churn des Testservers:
Jeder Klon (`CREATE DATABASE … STRATEGY FILE_COPY`) schreibt rund 3.400 Dateien
mit etwa 42 MB, sein `DROP` löscht sie wieder, und beide erzwingen insgesamt
drei Checkpoints. `fsync=off` verhindert das nicht: Dateianlage, Löschung und
Discard laufen trotzdem über das Dateisystem der VM-Platte. Läuft die
5-s-Fixture-Frist während eines Stillstands ab, meldet der Treiber
`read tcp [::1]:…->[::1]:5433: i/o timeout`.

Fix: Der Testserver hält seine Daten in einem tmpfs der VM, wie der
CI-Service. Mit identischer Klonlast blieben die Stillstände aus.

## Messaufbau

Alle Proben am gemeinsamen Server waren rein lesend und protokollierten weder
SQL-Text noch Zugangsdaten.

| Probe | Ort | Inhalt |
| --- | --- | --- |
| Host-Probe | macOS-Prozess, `[::1]:5433` | `SELECT 1` alle 100 ms auf einer festen Verbindung, neue Verbindung pro Sekunde, 10-ms-Ticker für Pausen des eigenen Prozesses |
| VM-Probe | Container im Netz-Namespace des Testservers, `127.0.0.1:5432` | Dasselbe ohne den Host-Pfad, dazu pro Sekunde `pg_stat_activity` (nur Wait-Events, Datenbank, Alter), `pg_locks`/`pg_blocking_pids`, `pg_stat_checkpointer`, `pg_stat_wal`, `pg_stat_io` |
| Blockgerät | Container ohne Netz | `/sys/block/vdb/stat` alle 200 ms: Schreibwartezeit, Flushes, Discards |
| Client-Overlay | temporäres `-overlay` für `backend/test/fixtures.go` | `EnsureTestTenant` getrennt nach Pool-Erwerb (`sql.DB.Conn`), frischer oder wiederverwendeter Verbindung, `pg_backend_pid()`, Organization-INSERT, School-INSERT, `sql.DBStats`-Wartezähler, Pausen des Testprozesses |

Die Probe-Quellen waren Wegwerfcode und sind nicht Teil des PR.

## Beobachtungen am gemeinsamen Server

70 Minuten VM-Probe, davon 32 Minuten mit Host-Probe, ohne eigenen Eingriff in
fremde Läufe:

- Host-Pfad: Stillstände von 1.259, 656, 2.491, 760, 602 und 767 ms.
- VM-Pfad im selben Zeitraum: `SELECT 1` höchstens 6,6 ms, die
  `pg_stat_activity`-Probe höchstens 40 ms in 4.240 Abfragen, kein Timeout.
- Weder der Host-Prozess noch der VM-Ticker pausierten in diesen Sekunden.
- Client-Backends in Klon-Datenbanken warteten nur in erwarteten Testzuständen
  (`pg_sleep`, Relation- und Transaktionssperren von Nebenläufigkeitstests).
  Lange Wartezeiten lagen in der Wartungsdatenbank: `IO/CopyFileWrite`,
  `IPC/CheckpointStart`/`CheckpointDone` und `ProcSignalBarrier` beim
  Klonen und Löschen, bis 10 s.
- Server-Log: 1.124 Klon-Lebenszyklen in 15 Minuten mit je drei erzwungenen
  Checkpoints (`immediate force wait flush-all` und zweimal
  `immediate force wait`). Kein Checkpoint brauchte länger als 0,05 s.

Um 06:05:03 standen zwei Host-Proben gegen **zwei verschiedene
Postgres-Server** gleichzeitig 2.491 ms still. Um 06:11:29, 06:13:29 und
06:14:17 standen `[::1]`, `127.0.0.1` und die Container-IP eines eigenen
Kontrollservers sowie die Host-Probe des gemeinsamen Servers in derselben
Millisekunde still. Port-Weiterleitung, IPv6 und Postgres scheiden damit aus.

## Kontrollschleife auf eigenen Servern

Eigene Container mit den Compose-Einstellungen des Testservers, eigenem Port und
eigenem Datenbereich, ohne neues Netzwerk. Das Template ist ein Dump des
aktuellen Test-Templates (38 MB). Acht Worker legen 30 s lang Klone mit
`FILE_COPY` an und löschen sie wieder.

| Datenbereich | Fenster à 30 s | Klon-Zyklen je Fenster | Create p50 | Drop p50 | Fenster mit Host-Stillstand ≥ 300 ms | VM-Pfad max |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Volume auf VM-Platte | 22 | 132–327 | 0,30–1,12 s | 0,37–0,80 s | 5 (bis 2.491 ms) | 15 ms |
| tmpfs | 8 | 3.119–3.533 | 0,050–0,055 s | 0,015–0,019 s | 0 (max. 36 ms) | 11 ms |
| ohne Klonlast | 8 | 0 | – | – | 0 (max. 212 ms) | 8 ms |

tmpfs schaffte also zehnmal mehr Klone pro Fenster und blieb trotzdem ohne
Stillstand.

In beiden Stillstandssekunden mit Blockgerät-Messung erreichte die aggregierte
Schreibwartezeit der VM-Platte das Maximum des gesamten Laufs:
57.122 ms (06:34:11) und 44.175 ms (06:45:52) in einer Sekunde. Dazu kamen je
zwei Flushes von zusammen 589 bzw. 345 ms und Discard-Bursts. Der Median lag
bei 34 ms pro Sekunde.

## Echte Testpakete: Volume gegen tmpfs

Gleiche Auswahl wie der Herkunftslauf von #6489 plus benachbarte Pakete, jeweils
gegen einen eigenen Server, nacheinander unter derselben Fremdlast:

```sh
TEST_DB_DSN=<eigener Kontrollserver> ~/bin/agent-heavy -- scripts/run-go-toolchain.sh \
  scripts/test-backend.sh -count=1 ./api ./database/migrations \
  ./modules/organizationtenancy/compose ./modules/enrollment/... \
  ./modules/timetable/... [./test]
```

`scripts/test-backend.sh` setzt `-p 2 -parallel 8`, `GOMAXPROCS=1`, `GOGC=50`.

| Lauf | Datenbereich | Tests | Gesamt | migrations | api | compose | Host-Stillstände ≥ 200 ms |
| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |
| A/B mit Overlay | Volume | 11.298 | 287,7 s | 79,5 s | 184,9 s | 170,2 s | 2 (max. 311 ms) |
| A/B mit Overlay | tmpfs | 11.298 | 204,7 s | 28,8 s | 152,3 s | 149,6 s | 0 (max. 56 ms) |
| Abschluss ohne Overlay | tmpfs (finale Compose-Werte) | 11.706 | 259,6 s | 27,5 s | 128,2 s | 144,5 s | 0 (max. 135 ms) |
| Abschluss ohne Overlay | Volume | 11.706 | 278,7 s | 69,1 s | 165,4 s | 148,6 s | 0 (max. 183 ms) |

Im Abschlusspaar lag die Schreibwartezeit der VM-Platte im p99 bei 24.783 ms
pro Sekunde (Volume) gegenüber 1.014 ms (tmpfs); der Rest stammt aus anderer
Last in derselben VM. Der Volume-Abschluss lief bei geringerer Fremdlast
(Load-1 4 bis 7) und blieb ohne Stillstand: Die Stillstände treten sporadisch
auf, nicht in jedem Lauf.

Das Template entstand auf einem leeren Server in 7 bis 12 s. Alle Läufe waren
grün. Das Overlay zeigte in keinem Lauf einen `EnsureTestTenant`-Aufruf über
500 ms. Pool-Wartezeit trat damit nicht auf; ein Pool-Timeout endet zudem mit
`context deadline exceeded`, nicht mit einem TCP-Lesefehler.

## Grenzen

- Ein 5-s-Ausfall wurde während der Messung nicht erneut gefangen. Der längste
  gemessene Stillstand dauerte 2,5 s. Die Zuordnung zum Issue-Fehler stützt sich
  auf identische Fehlerform, die Klonspitzen im Issue (271 Checkpoints in 20 s)
  und die unabhängigen Probe-Timeouts dort.
- Gemessen wurde nur OrbStack auf einem Rechner; das Innere der VM-Software
  wurde nicht untersucht. Andere Schreiblast in derselben VM kann den Host-Pfad
  weiter anhalten. tmpfs entfernt die Klonlast des Testservers als Auslöser.
- Der tmpfs zählt zum `mem_limit` des Containers. Am gemeinsamen Server lebten
  höchstens 34 Datenbanken gleichzeitig, davon rund 16 Templates und
  Systemdatenbanken. Ein frisch gestarteter tmpfs-Server hält nur die Templates
  der seither genutzten Migrations-Hashes.
