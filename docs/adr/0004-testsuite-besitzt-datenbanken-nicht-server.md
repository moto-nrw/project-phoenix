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
- **Kein TestMain in den 62 Packages.** Das Prozess-einmalige Setup wandert in
  ein `sync.Once` innerhalb von `SetupTestDB`; Clone-Teardown übernimmt der
  Wrapper nach dem Gesamtlauf, eine Generation-GC (läuft über alle Worktrees)
  fängt abgebrochene Läufe. Ein nacktes `go test ./pkg` darf seinen Clone bis
  zur nächsten GC liegen lassen.
- **Isolation pro Test über Fixtures, nicht über Transaktions-Rollback.**
  Jeder Test erzeugt seinen eigenen Tenant statt des fixen Bootstrap-Tenants 1;
  der geteilte Package-Clone bleibt. Rollback-pro-Test scheitert an Code, der
  selbst Transaktionen und RLS-Rollen öffnet (`TenantTxMiddleware`).
- **Alle Packages werden parallel-sicher — ohne Fluchtluke.** Keine
  Serial-Allowlist; sture Tests werden repariert. Per-Row-DELETE-Cleanups
  (`defer Cleanup*`) werden ersatzlos entfernt, die Clone-Isolation macht sie
  redundant. Ein Pool pro Package (≈ GOMAXPROCS, gedeckelt) ersetzt die
  per-Test-Pools.
- **Leftover-Detection ist Diagnose, kein Gate**: Opt-in per Env-Flag,
  Standard aus. Die Hermetik kommt aus dem Lebenszyklus, nicht aus der
  Detection. Sequence-Offsets bleiben zunächst als Flake-Detektor und werden
  gestrichen, sobald die Suite auf frischen Clones grün ist.

Abgewogen: Ein ephemerer Postgres pro Lauf hätte „Start = Ende" trivial wahr
gemacht, kostet aber pro Lauf den Template-Neubau (~25s) oder einen eigenen
Cache dafür; der langlebige Container mit Datenbank-GC liefert dieselbe
Garantie auf Datenbank-Ebene ohne diesen Preis. TestMain pro Package hätte
auch nackte `go test`-Aufrufe aufräumen lassen, wurde aber gegen 62
Boilerplate-Dateien getauscht, weil die GC den Rest erledigt. Zielwerte:
CI test-backend unter 90 Sekunden, lokaler Volllauf ≈ 2 Minuten; die Frage
4-vCPU- statt 8-vCPU-Runner wird erst nach dem Umbau gemessen, nicht vorab
entschieden.
