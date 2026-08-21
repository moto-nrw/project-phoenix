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
- **Handshake einmal pro Lauf, nicht pro Binary.** Server erreichbar und
  Template zum Migrations-Hash gebaut prüft der Wrapper einmal
  (`internal/testdb/cmd/bootstrap`) und reicht den Template-Namen über
  `PHX_TEST_TEMPLATE` weiter; gemessen kosteten die beiden Schritte über 93
  Binaries 2,6 s summiert gegen ~0,2 s für den einen Aufruf. Ohne die
  Variable macht jedes Binary beides selbst — das ist es, was ein nacktes
  `go test ./...` selbst-initialisierend hält.

- **Ein Pool pro Package, bemessen an der eigenen Parallelität.** Die
  Poolgröße kommt aus `-test.parallel` plus Reserve, nicht aus GOMAXPROCS:
  ein Test, der eine Tenant-Transaktion hält und darin eine zweite öffnet,
  braucht zwei Verbindungen gleichzeitig, und ohne Reserve blockieren sich
  `-parallel` solcher Tests gegenseitig bis zum Timeout. Wrapper und
  Post-Merge-CI pinnen zusätzlich `-p 6 -parallel 8`, damit das serverseitige Budget
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
  bleiben, solange die generischen Fixture-Cleanups mit ihren
  tabellenübergreifenden `id = ?`-Armen noch Aufrufer haben.

- **Teardown pro Zeile ist die Ausnahme, nicht die Regel.** Eine Zeile, die
  einem Tenant gehört, stirbt mit dem Clone und ist für keinen anderen Test
  sichtbar — ihr Teardown ist reine DB-Last. 5120 solcher Aufrufe sind
  entfernt (587 bleiben, `cleanupCallBaseline` friert sie ein). Drei Formen
  überleben mit Grund: eine Zeile in einer **tenant-losen** Tabelle
  (auth.accounts, RBAC-Katalog, Plattform/Operator-Tabellen), die das
  Leftover-Gate sonst zählt; ein **Zustands-Reset zwischen Subtests** eines
  Tests, wo ein Unique-Index oder eine tenant-weite Zählung sonst kippt (der
  bessere Weg ist `testpkg.OwnTenant` für diesen Subtest); und das Löschen,
  das **selbst der Testschritt** ist (eine ID, die der Code als fehlend melden
  muss). Gemessen: Volllauf 85 s → 53 s.

- **Subtests dürfen einen eigenen Tenant beanspruchen.** Der Normalfall bleibt
  „Subtests teilen den Tenant des Elterntests" — richtig, solange der Vater die
  Fixtures baut, die sie lesen. Für die andere häufige Form (eine Tabelle von
  Subtests, die je dieselbe Art Zeile anlegen und dann tenant-weit
  assertieren) gibt es `testpkg.OwnTenant(t)` / `OwnCtx(t)`: der längste
  Präfix von `t.Name()`, der das beansprucht hat, bestimmt den Tenant, und die
  Fixtures des Subtests folgen dorthin.

- **Klonen läuft unter dem SHARED Lebenszyklus-Lock.** `CREATE DATABASE …
  TEMPLATE` nimmt auf der Quelle nur eine ShareLock, und zwei Cloner schreiben
  nie dieselbe Ziel-Datenbank (der Name trägt Run-ID und Package) — was das
  exklusive Lock hier fernhalten muss, ist der Template-Neubau, der die
  kopierte Datenbank droppt, und die Generations-GC, die Clones droppt. Die GC
  nimmt das exklusive Lock deshalb nur noch per `pg_try_advisory_lock`: ist es
  belegt, wird sie übersprungen (ein toter Clone kostet Platz bis zum nächsten
  Binary, dem Sweep oder dem nächsten Lauf) statt sich vor und hinter die
  Cloner in die Warteschlange zu stellen. Gemessen waren es 6,0 s summiert
  über 93 Binaries, alle hinter einem Lock serialisiert.

  Die Kosten sind eine Abfrage pro Package beim Beenden (gemessen 30-70 ms,
  alle Tabellen in einem Round-Trip), nicht eine pro Test — deshalb sitzt das
  Gate am Package-Ende. Es benennt damit das PACKAGE, nicht den einzelnen
  Test; wer den Verursacher sucht, lässt das Package seriell mit
  `PHX_TEST_LEFTOVERS=test go test -parallel 1` laufen, dann prüft dieselbe
  Vergleichslogik nach jedem einzelnen Test. Eine Attribution pro Test unter
  Parallelität gibt es bewusst nicht: pg_stat und Zeilenzählung sind
  datenbankweit, ein parallel laufender Nachbar wäre nicht vom eigenen Test
  zu trennen.

CI-PRs verwenden dieselbe Reverse-Dependency-Auswahl wie
`scripts/test-changed.sh` und erzeugen keine Coverage: Seit SonarCloud nur auf
Pushes nach `development`/`main` läuft, hatte die PR-Coverage keinen Verbraucher.
Änderungen an der Test-Infrastruktur erzwingen nur den Volllauf der betroffenen
Suite; CI-Workflow-Änderungen erzwingen beide. Pushes fahren beide Vollläufe mit
Coverage für SonarCloud. Reine Markdown- und nicht testrelevante
Backend-Konfigurationsänderungen starten keine Tests.
Lint- und Build-Jobs laufen dabei nur für den tatsächlich geänderten Stack;
die Sonar-bedingte Gegen-Suite startet ausschließlich ihre Tests mit Coverage.
Reine `_test.go`-Änderungen laufen nur im eigenen Package; nur geänderter
Produktionscode zieht die transitiven Importierer nach sich. Eingebettete
Produktions-Assets (Locales, Export-Schriften/-Logo) zählen wie Produktionscode;
Goldens und E-Mail-Templates laufen in ihrem besitzenden Test-Package.

Der lokale Changed-Loop nutzt höchstens die Hälfte der erkannten CPUs und
maximal vier Package-Binaries. `GOMAXPROCS` pro Binary wird so berechnet, dass
auch zusammen höchstens die halbe Maschine belegt wird; `-parallel` ist auf
acht begrenzt. CI und der explizite Volllauf-Wrapper behalten für Post-Merge-Coverage
`-p 6 -parallel 8`. Changed-only-PRs laufen auf dem 4-vCPU-Runner mit `-p 4`
statt den doppelten Minutenpreis des 8-vCPU-Runners zu zahlen. Vitest nutzt
lokal ebenfalls höchstens die Hälfte beziehungsweise vier Worker, in CI aber
alle CPUs des isolierten Runners.

Der eigenständige Seed-&-Simulate-Smoke läuft nur bei Backend-Produktionscode,
seinen eingebetteten/runtime-geladenen Assets oder CI-Workflow-Änderungen.
Frontend-only-Pushes und reine Backend-Teständerungen starten seinen
4-vCPU-Runner nicht.

Abgewogen: Ein ephemerer Postgres pro Lauf hätte „Start = Ende" trivial wahr
gemacht, kostet aber pro Lauf den Template-Neubau (~25s) oder einen eigenen
Cache dafür; der langlebige Container mit Datenbank-GC liefert dieselbe
Garantie auf Datenbank-Ebene ohne diesen Preis. TestMain pro Package hätte
auch nackte `go test`-Aufrufe aufräumen lassen, wurde aber gegen 62
Boilerplate-Dateien getauscht, weil die GC den Rest erledigt. Zielwerte:
CI test-backend unter 90 Sekunden, lokaler Volllauf ≈ 2 Minuten. Die
Changed-only-PR-Last rechtfertigt den 4-vCPU-Runner; der instrumentierte
Post-Merge-Volllauf bleibt auf 8 vCPUs.
