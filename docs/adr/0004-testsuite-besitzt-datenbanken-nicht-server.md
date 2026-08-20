# Die Testsuite besitzt ihre Datenbanken, nicht den Postgres-Server

Bis August 2026 hatte die Backend-Testsuite keinen Lebenszyklus-Endpunkt:
Package-Clones (`phx_test_pkg_<SHA1(Pfad)>`) wurden nur beim nächsten Lauf
desselben Pfads gedroppt, nie beim Teardown — pro Worktree und Umbenennung
leakte je Package eine Datenbank für immer (Stand 19.08.2026: 1092 verwaiste
Clones, 33 GB). Gleichzeitig verhinderten `t.Setenv("APP_ENV", "test")` und
`viper.Set(...)` in `SetupTestDB` jede Intra-Package-Parallelität, jeder
Test-Aufruf öffnete einen frischen 3-Conn-Pool (2665 pro Suite-Lauf), und die
Suite war DB-gebunden statt CPU-gebunden (`services/active`: 80s Wandzeit bei
4,5s CPU). Die CI-Kosten (Blacksmith, 8-vCPU-Runner, Ø 3,8 min test-backend
bei ~1.200 Runs/Monat) machten jede Testminute zum direkten Kostenfaktor
(#2419).

Entscheidung (Umsetzung: ein Issue #2419, zwei PRs — erst Lebenszyklus, dann
mechanischer Sweep über alle Testdateien):

- **Der lokale `postgres-test`-Container bleibt langlebig.** Die Suite besitzt
  die *Datenbanken* darin, nicht den Server: `go test` ist selbst-
  initialisierend (Container bei Bedarf starten, Template `phoenix_test` per
  Migrations-Hash prüfen und neu bauen), ein Make-/devbox-Target liefert nur
  Komfort obendrauf (gotestsum, sofortiges Teardown). CI behält Service-
  Container und Snapshot-Cache unverändert.
- **Prozess-einmaliges Setup in einem `sync.Once`** innerhalb von
  `SetupTestDB`. Das TestMain pro Package kam später doch — es trägt den
  per-Test-Tenant-Schalter und das Leftover-Gate — und mit ihm der
  Clone-Teardown: jedes Binary droppt seinen Clone am Ende selbst, ein
  nacktes `go test ./...` räumt also genauso auf wie der Wrapper. Der Sweep
  des Wrappers macht nur noch GC; abgebrochene Läufe (Panik, Ctrl-C, Kill)
  fängt weiterhin die Generation-GC, die über alle Worktrees läuft.
- **Isolation pro Test über Fixtures, nicht über Transaktions-Rollback.**
  Jeder Test erzeugt seinen eigenen Tenant statt des fixen Bootstrap-Tenants 1;
  der geteilte Package-Clone bleibt. Rollback-pro-Test scheitert an Code, der
  selbst Transaktionen und RLS-Rollen öffnet (`TenantTxMiddleware`).
- **Tests laufen parallel, sofern sie es können.** `t.Parallel()` ist der
  Normalfall, nicht die Ausnahme: der Sweep hat es allen Top-Level-Tests
  gegeben und dort wieder entfernt, wo der Test es nachweislich nicht
  verträgt. Fünf Gründe bleiben übrig, jeder direkt über dem Test benannt:
  prozess-globaler Zustand (Env, viper, Settings-Registry, `os.Stdout`),
  Schema-Änderungen (Migrationstests), tenant-übergreifende Sweeps, die im
  Servicetest ohne Tenant-Transaktion laufen und deshalb an RLS vorbeisehen,
  Query-Budget-Messungen am geteilten Pool und absichtliche Lock-Contention
  (ein Test hält eine Zeilensperre und erwartet, dass die zweite Transaktion
  darauf blockiert). Statt einer Fluchtluke gibt es
  einen schrumpfenden Zähler: `serialTestBaseline` in
  `test/hermetic_verification_test.go` friert die Restmenge pro Package ein,
  neue Tests müssen parallel sein. Per-Row-DELETE-Cleanups (`defer Cleanup*`)
  werden ersatzlos entfernt, die Clone-Isolation macht sie redundant.
- **Ein Pool pro Package, bemessen an der eigenen Parallelität.** Die
  Poolgröße kommt aus `-test.parallel` plus Reserve, nicht aus GOMAXPROCS:
  ein Test, der eine Tenant-Transaktion hält und darin eine zweite öffnet,
  braucht zwei Verbindungen gleichzeitig, und ohne Reserve blockieren sich
  `-parallel` solcher Tests gegenseitig bis zum Timeout. Wrapper und CI
  pinnen zusätzlich `-p 4 -parallel 8`, damit das serverseitige Budget
  (p × (Pool + 1)) unter den 100 Verbindungen eines Standard-postgres:17
  bleibt.

  Das rührt an die Abgrenzung des Issues ("kein CI-Umbau nötig"): die
  CI-Workflow-Zeile und `max_connections` im Compose-Beispiel ändern sich mit.
  Bewusst so — die Verbindungsobergrenze ist die eine Größe, die lokaler Lauf
  und CI sich teilen, und ein Pool, der zur Parallelität passt, wäre ohne sie
  in CI ein "too many clients" statt eines Timeouts. Am CI-Aufbau
  (Service-Container, Snapshot-Cache) ändert sich nichts.
- **Leftover-Detection ist ein Gate — gemessen gegen den eigenen Start, im
  Testprozess.** Jeder Clone hält seinen Startzustand selbst fest
  (`testdb.SnapshotSharedBaseline`, direkt nach dem Bootstrap); jedes
  Test-Binary vergleicht am Ende seiner eigenen Tests dagegen
  (`testpkg.Run(m)` aus dem TestMain) und lässt das PACKAGE scheitern, wenn
  Zeilen übrig sind. Der Maßstab ist ausdrücklich nicht „keine Zeilen": gezählt wird nur,
  was **außerhalb der Tenants dieses Laufs** liegt (Tenant-ID unter
  `testdb.TenantIDBase` oder gar kein Tenant). Was ein Test in seinen eigenen
  Tenant schreibt, sieht kein anderer Test und stirbt mit dem Clone — 210
  Tenants pro Clone sind das gewollte Ergebnis, kein Befund. Weil der
  Startzustand im Clone gemessen wird statt gegen das Template, sind die
  Bootstrap-Fixtures per Konstruktion Teil von „Start" und das Gate ändert
  sich nicht, wenn sie später verschwinden. `testdb.LeftoverAllowlist` duldet
  die Paare, die es bei Einführung schon gab (tenant-lose Tabellen: Accounts,
  Operator-Portal, RBAC-Katalog); sie darf nur schrumpfen.
  `PHX_TEST_LEFTOVERS=1` zeigt zusätzlich die geduldeten. Sequence-Offsets
  bleiben zunächst als Flake-Detektor und werden gestrichen, sobald die Suite
  auf frischen Clones grün ist.

  Die Kosten sind eine Abfrage pro Package beim Beenden (gemessen 30-70 ms,
  alle Tabellen in einem Round-Trip), nicht eine pro Test — deshalb sitzt das
  Gate am Package-Ende. Es benennt damit das PACKAGE, nicht den einzelnen
  Test; wer den Verursacher sucht, lässt das Package seriell mit
  `PHX_TEST_LEFTOVERS=test go test -parallel 1` laufen, dann prüft dieselbe
  Vergleichslogik nach jedem einzelnen Test. Eine Attribution pro Test unter
  Parallelität gibt es bewusst nicht: pg_stat und Zeilenzählung sind
  datenbankweit, ein parallel laufender Nachbar wäre nicht vom eigenen Test
  zu trennen.

Abgewogen: Ein ephemerer Postgres pro Lauf hätte „Start = Ende" trivial wahr
gemacht, kostet aber pro Lauf den Template-Neubau (~25s) oder einen eigenen
Cache dafür; der langlebige Container mit Datenbank-GC liefert dieselbe
Garantie auf Datenbank-Ebene ohne diesen Preis. TestMain pro Package hätte
auch nackte `go test`-Aufrufe aufräumen lassen, wurde aber gegen 62
Boilerplate-Dateien getauscht, weil die GC den Rest erledigt. Zielwerte:
CI test-backend unter 90 Sekunden, lokaler Volllauf ≈ 2 Minuten; die Frage
4-vCPU- statt 8-vCPU-Runner wird erst nach dem Umbau gemessen, nicht vorab
entschieden.
