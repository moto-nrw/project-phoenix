# Backend-Testsuite: Performance-Analyse

Stand: 2026-09-01. Alle Zahlen in dieser Datei sind auf dieser Maschine gemessen, nicht geschätzt. Schätzungen sind ausdrücklich als solche markiert.

## Messaufbau

| | |
|---|---|
| Hardware | 16 Kerne (Apple Silicon, darwin/arm64), 64 GB RAM |
| Toolchain | go1.27.0 aus dem Devbox-Pin (identisch mit `scripts/run-go-toolchain.sh`; das Skript selbst wurde von einem lokalen Hook blockiert, deshalb wurden PATH auf `.devbox/nix/profile/default/bin` und `GOTOOLCHAIN=local` inline gesetzt und die Go-Version gegen `go.mod` verifiziert) |
| Test-DB | `postgres-test`-Container (Postgres 17, Port 5433), Template `phoenix_test_e987fd2edf72` warm |
| Suite | 193 Packages, davon 164 mit Test-Binaries; 24 638 Tests (14 610 Top-Level-Funktionen), 5 Skips |
| Messregeln | Messläufe mit `-count=1`; Zeiterfassung mit `/usr/bin/time -l`; JSON via `go test -json` bzw. `gotestsum --jsonfile` |

Durchgeführte Läufe (alle mit warmem Compile-Cache; ein echter Kalt-Compile wurde nicht gemessen, dazu unten mehr):

| Lauf | Kommando | Wall | User-CPU | Sys-CPU |
|---|---|---|---|---|
| A | `go test -count=1 -run '^$' -p 6 -parallel 8 ./...` (null Testkörper) | 86,2 s | 96,9 s | 25,4 s |
| C | `gotestsum -- -count=1 -p 6 -parallel 8 ./...` (volle Suite, Parameter wie `test-backend.sh`) | 99,6 s | 159,1 s | 95,5 s |
| D | `GOMAXPROCS=2 go test -p 4 -parallel 8 ./...` (Parameter wie `test-changed.sh`, frische Run-ID) | 114,1 s | 145,4 s | 79,9 s |
| E | identisch zu D, gleiche Run-ID wie D | 140,9 s (0 Cache-Treffer, Anomalie, siehe Befund 1) | | |
| F | identisch zu E, direkt danach | **3,1 s** (164/164 Packages `(cached)`) | 1,9 s | 2,4 s |

Einzelmessungen der Test-DB-Lebenszyklus-Kosten:

| Schritt | Kosten |
|---|---|
| Bootstrap-Handshake (Template warm) | 0,12 bis 0,29 s |
| Template-Neuaufbau kalt (Drop + alle Migrationen) | 6,3 s (der Kommentar "~25s" in `test/db_clone.go` ist lokal veraltet) |
| Clone anlegen (`CREATE DATABASE ... TEMPLATE`, 25 MB) | 63 ms |
| Clone droppen | 16 ms |
| Sweep (`go run ./internal/testdb/cmd/sweep`, nichts zu tun) | 0,13 s |

## Gesamtbild: Wo die Zeit wirklich hingeht

Die Zerlegung Lauf C minus Lauf A ist eindeutig:

| Kostenblock | Wall-Anteil am Volllauf (99,6 s) | Beleg |
|---|---|---|
| (a) Compile | im Warmzustand nahe 0 (Cache); **Re-Link ist NICHT gecacht** und läuft bei jedem `go test`-Aufruf neu: 1,5 bis 2,2 s Wall, ~3,2 s CPU pro großem Binary (gemessen an `api/auth`, 77 MB) | zwei aufeinanderfolgende `go test -c ./api/auth`: 2,16 s und 1,54 s |
| (b) Test-DB-Setup | vernachlässigbar: Bootstrap 0,3 s einmal pro Lauf, Clone 63 ms pro Package (lazy, nur bei tatsächlichem DB-Zugriff) | Einzelmessungen oben |
| (c) Testkörper | **~13 s Wall** (37,8 s aggregierte Binary-Zeit über alle 164 Packages) | Summe der Package-Differenzen Lauf C minus Lauf A |
| Fixer Per-Binary-Overhead (Link + Erst-Exec + Start) | **~86 s Wall**, der Boden der Suite bei null Tests | Lauf A |

Die Suite ist also nicht "langsame Tests", sondern **fixer Per-Package-Overhead**: 24 638 Tests kosten zusammen nur 13 s mehr Wall-Zeit als gar keine Tests.

Der Per-Binary-Overhead zerlegt sich weiter:

- **Re-Link bei jedem Aufruf**: Go cacht Testergebnisse und Objektdateien, aber nicht das gelinkte Test-Binary. Jeder `-count=1`-Lauf linkt alle 164 Binaries neu. Das erklärt den hohen Sys-Anteil (95 s in Lauf C).
- **Erst-Exec-Strafe unter macOS**: Der erste Start eines frisch gelinkten Binaries kostet 0,89 s Wall bei nur 0,03 s CPU (Signaturprüfung/Page-in des ad-hoc-signierten Binaries). Der zweite Start desselben Binaries: 0,01 s. Bei 164 frischen Binaries sind das ~145 s, die sich bei `-p 6` auf ~24 s Wall verteilen.
- **Startup unter Contention**: Ein leeres `internal/sliceutil`-Binary läuft solo in 0,35 s, unter `-p 6` im Suitelauf in 1,86 s; `api/*`-Binaries liegen dort bei 3,1 bis 4,0 s.

CPU vs. Wall: Lauf C nutzt 255 s CPU über 99,6 s Wall auf 16 Kernen, also nur ~16 % Auslastung. Die DB-Packages warten (I/O), die CPU-Zeit steckt in Link und Binary-Start.

## Top-Listen

### Teuerste Packages nach Testkörper-Kosten (Package-Wall im Volllauf minus Leerlauf)

| Delta | Volllauf | Leerlauf | Package |
|---|---|---|---|
| 13,2 s | 15,8 s | 2,6 s | internal/architecture |
| 10,9 s | 13,6 s | 2,7 s | services/parent |
| 8,3 s | 10,4 s | 2,1 s | test |
| 8,3 s | 10,8 s | 2,6 s | database/migrations |
| 6,7 s | 9,0 s | 2,3 s | services/platform |
| 5,6 s | 7,8 s | 2,2 s | services/schedule |
| 5,5 s | 7,7 s | 2,2 s | database/repositories/schedule |
| 5,5 s | 8,1 s | 2,6 s | database/repositories/platform |
| 5,4 s | 7,5 s | 2,1 s | services/auth |
| 5,3 s | 7,3 s | 2,0 s | services/enrollment |

Wichtig: Die Zahlen im Volllauf enthalten Contention. `services/parent` braucht solo 3,7 s statt 13,6 s, `services/platform` solo 4,0 s statt 9,0 s. `database/migrations` streut stark (6,3 bis 27,6 s über drei Solo-Läufe).

### Teuerste Einzeltests (Volllauf, Top-Level)

| Zeit | Test |
|---|---|
| 14,6 s | internal/architecture.TestCanonicalArchitectureRatchetMatchesCommittedBaseline |
| 11,5 s | internal/architecture.TestCompositionLegacyCallerInventory |
| 6,9 s | database/repositories/schedule.TestTimetableConflictAckRepository_CapPrunesOldest |
| 6,5 s | test.TestHermeticTestPatterns |
| 6,0 s | services/auth.TestFactoryOperatorMFARefusesChallengeWithoutSMTP |
| 4,8 s | internal/architecture.TestCheckReportsDeterministicSemanticLocations |
| 4,2 s | test.TestCalendarFixtureClockRatchet |

Verteilung aller 14 610 Top-Level-Tests: Median < 10 ms, p90 = 0,16 s, p99 = 1,04 s, Maximum 14,6 s. Nur 168 Tests brauchen 1 s oder mehr. Die Testkörper sind gesund.

## Befunde

### Befund 1 (der größte Hebel): `test-changed.sh` zerstört den Go-Test-Result-Cache bei jedem Aufruf

`scripts/test-changed.sh:47` würfelt pro Aufruf eine neue `PHX_TEST_RUN_ID`. Jedes DB-Test-Binary liest diese Variable (über `testdb.RunID()`), damit wandert sie in den Cache-Key, und **jedes DB-Package läuft bei jedem Aufruf komplett neu, auch wenn sich nichts geändert hat**.

Beleg: `go test ./services/parent` mit gleicher Run-ID zweimal: 3,7 s, dann `(cached)`. Mit neuer Run-ID: wieder 3,0 s voll. Auf Suitenebene: Lauf F (identisch zu D, stabile Run-ID) = **3,1 s statt 114,1 s**, alle 164 Packages `(cached)`.

Anomalie: Lauf E (gleiche Run-ID wie D, aber D lief in einer anderen Shell-Umgebung) traf den Cache nicht. Wahrscheinliche Ursache ist ein Umgebungs-Fingerprint-Unterschied zwischen den beiden Aufrufkontexten; in identischer Umgebung (E gegen F, und die Package-Einzelversuche) trifft der Cache zuverlässig. Vor dem Umbau gehört die Trefferquote in der echten Hook-/CI-Umgebung nachgemessen.

Korrektur nach Prüfung der Go-Quelle (`cmd/go/internal/test/test.go`): `GOMAXPROCS` und `-p` sind KEINE Cache-Key-Inputs. Die echten Splitter zwischen den Aufrufwegen sind die von den Binaries per `os.Getenv` gelesenen Variablen, konkret `PHX_TEST_TEMPLATE` (von `test-backend.sh` gesetzt, von `test-changed.sh` ursprünglich nicht) und das `-test.parallel`-Flag. Das erklärt auch die Lauf-E-Anomalie plausibler als der ursprüngliche GOMAXPROCS-Verdacht. Diagnose im Zweifel: `GODEBUG=gocachetest=1` nennt beim Miss die Ursache.

### Befund 2: Die Affected-Selektion ist für Kernänderungen faktisch die volle Suite

Reverse-Abhängigkeits-Closure (Import-Graph inkl. Test-Imports, gemessen über `go list`):

| Geändertes Package | Betroffene Packages (von 193) |
|---|---|
| models/base, models/users, services/active, services/schedule, database/repositories/users, test | **146** |
| api/common | 50 |
| api/iot/checkin | 5 |

Ursache: Das geteilte Fixture-Package `test` importiert über 30 interne Packages (alle Repositories, alle Models, services/config, auth, email, realtime, tenant), und praktisch jedes DB-Test-Package importiert `test`. Damit zieht jede Änderung an Models, Repositories oder zentralen Services die 146er-Closure, und der "changed-only"-Loop ist ein Volllauf. Da die Änderung die 146 Binaries auch recompiliert, hilft der Result-Cache aus Befund 1 innerhalb einer Fix-Iteration an einem Kern-Package nicht; er hilft beim Wiederholungslauf, bei Test-only-, Doku- und Blattpackage-Änderungen und beim Pre-Push nach grünem lokalem Lauf.

### Befund 3: Fixer Per-Binary-Overhead dominiert (Link nicht gecacht + macOS-Erst-Exec)

Zahlen im Gesamtbild oben. Konsequenz: Jede Maßnahme, die die Zahl der tatsächlich neu gebauten und gestarteten Binaries senkt (Befunde 1 und 2), wirkt ~7x stärker als jede Optimierung an den Testkörpern (13 s von 99,6 s).

Gegenprobe `-ldflags=-w` (ohne DWARF): Binary 77 auf 62 MB, Link-Zeit im Rauschen (1,5 bis 2,9 s), Erst-Exec unverändert 0,91 s. **Kein Gewinn, verworfen.**

### Befund 4: Die Ratchet-Tests sind die teuersten CPU-Verbraucher

- `internal/architecture` (17 s solo, 43,6 s User + 54,9 s Sys): shellt `go list -json ./...` aus und lädt das gesamte Modul mit `packages.Load` (Typprüfung). Der Großteil der CPU liegt in Kindprozessen; das In-Prozess-Profil ist GC-dominiert (`runtime.madvise` 29 %, Scan/Alloc weitere ~25 %), also allokationslastige AST-Arbeit. Läuft im Loop nur bei Änderungen an `internal/architecture`/`policy.json`, kostet aber jeden Volllauf und CI-Lauf ~16 s.
- Package `test` (9,1 s solo, 20,7 s User): CPU-Profil zeigt 43 % rohe Syscalls, 33 % `path/filepath.WalkDir`, 21 % `test.walkGoFilesRaw`. **Jeder Ratchet-Test läuft den kompletten Backend-Baum erneut ab und öffnet jede Datei neu.** Dieses Package liegt in der 146er-Closure und läuft damit bei fast jeder Kernänderung mit.

### Befund 5: Produktionscode-Hotspots

Ausdrücklich geprüft, wie gefordert: **Es gibt keinen nennenswerten CPU-Hotspot im Produktionscode.** Die DB-Packages sind Warte-dominiert (services/parent: 840 ms Samples auf 2,5 s Profil-Dauer, 58 % davon `syscall.rawsyscalln`; services/platform analog). Kryptografie: Argon2 taucht mit 40 ms von 610 ms Samples in services/platform auf, mehr nicht. Die einzigen CPU-Fresser sind Test-Infrastruktur (Befund 4).

### Hypothesen-Check (jede einzeln belegt oder widerlegt)

| # | Hypothese | Ergebnis |
|---|---|---|
| 1 | Passwort-Hashing pro Test | **Widerlegt.** `test/argon2.go` setzt billige Testparameter, `test/fixtures.go` cacht Hashes pro Passwort. Profil: 40 ms Argon2 in services/platform. |
| 2 | Fixture-Aufbau mit Einzel-INSERTs | **Ohne messbare Wirkung.** Per-Test-Tenant = 2 bis 3 INSERTs. Alle Testkörper zusammen: 13 s Wall für 24 638 Tests. Batching lohnt nicht. |
| 3 | Fehlendes `t.Parallel()` | **Widerlegt.** 14 733 `t.Parallel()`-Aufrufe auf 14 741 Testfunktionen, in allen Top-Packages 1:1. |
| 4 | `time.Sleep`/Polling | **Widerlegt als Kostentreiber.** 35 Treffer; die scheinbaren Monster (3 min, 2 h, 18 h in services/scheduler) laufen in `testing/synctest` mit Fake-Zeit. Echte Sleeps: ~20 Stellen mit 10 bis 250 ms. |
| 5 | Volle Migrations-/Template-Kosten für Packages ohne DB-Bedarf | **Widerlegt.** Setup ist lazy: ohne DB-Zugriff entsteht kein Clone (Lauf A belegt das). Clone kostet 63 ms, Template-Handshake 0,3 s pro Lauf. `-short` existiert bereits zentral in `SetupTestDB`. |
| 6 | Redundante Suite-Setups pro Testfunktion | **Widerlegt.** Ein Pool pro Binary (`db_clone.go`), Clone lazy und einmalig, `SetupTestDB` pro Test ist ein Once-Zugriff. |

Alle sechs vorab vermuteten Kostentreiber sind es nicht. Die tatsächlichen sind Befunde 1 bis 4.

## Maßnahmenplan (nach Ersparnis pro Aufwand)

| Prio | Maßnahme | Gemessene Basis | Erwartete Ersparnis | Aufwand | Risiko |
|---|---|---|---|---|---|
| 1 | **Stabile `PHX_TEST_RUN_ID` in `test-changed.sh`** (deterministisch pro Worktree, z. B. Hash des Worktree-Pfads, statt `/dev/urandom`); zusätzlich die `GOMAXPROCS`-Divergenz zu den anderen Aufrufwegen beseitigen, damit alle Wege denselben Cache füttern | 114,1 s auf 3,1 s bei unveränderter Selektion (Lauf D auf F) | Wiederholungs-, Test-only- und Blattänderungs-Läufe fallen auf Sekunden; Quorum-/Gate-Doppelläufe werden fast gratis | 2 Zeilen Skript | **Verhaltensändernd**: `(cached)` heißt "nicht neu ausgeführt". Der Cache-Key deckt gelesene Dateien und Env ab, nicht externen DB-Zustand; die hermetische Architektur macht das vertretbar, aber es ist eine Gate-Entscheidung. Anomalie aus Lauf E vorher in der Zielumgebung nachmessen. |
| 2 | **Zweistufiger Loop-Modus**: lokal zuerst nur direkt geänderte Packages plus direkte Importer (typisch < 10), volle 146er-Closure erst vor Push bzw. im Gate; mindestens aber Fail-fast-Reihenfolge (geänderte Packages zuerst) | Closure-Messung: 146/193 für jede Kernänderung; Volllauf 100 bis 114 s | Fix-Iteration an einem Kern-Package von ~114 s auf grob 10 bis 25 s (Schätzung: Overhead von < 10 statt 146 Binaries) | Skript-Logik, überschaubar | **Verhaltensändernd** fürs Gate (weniger wird pro Iteration geprüft); als rein lokaler Schnellmodus mit vollem Lauf vor Push risikoarm |
| 3 | **`-p` erhöhen + `max_connections` im Test-Container anheben** (aktuell begrenzt die 100er-Grenze auf `-p 6`; `-p 10` braucht ~130 Verbindungen) | CPU-Auslastung im Volllauf nur ~16 %; `-p 4` auf `-p 6` brachte bereits 114 auf 99,6 s | Schätzung: Volllauf-Boden von 86 s Richtung 50 bis 60 s | Compose-Config des Test-Containers + eine Zahl in `test-backend.sh`/`test-changed.sh` | Risikofrei (reine Test-Infrastruktur); auf schwächeren Maschinen validieren |
| 4 | **Gemeinsamer Datei-Walk/Parse-Cache für die Ratchet-Tests im Package `test`** (einmal Baum lesen, alle Ratchets teilen sich das Ergebnis) | Profil: 33 % WalkDir, 21 % walkGoFilesRaw, 43 % Syscalls; Package-Delta 8,3 s | Schätzung: 5 bis 6 s pro Volllauf, und das Package liegt in fast jeder Loop-Selektion | Teständerung, mittel | Risikoarm, aber Teständerung: **nur nach Freigabe** (`.claude/rules/no-test-modifications.md`) |
| 5 | `internal/architecture` aus dem heißen Pfad halten (läuft ohnehin nur bei eigenen Änderungen in der Selektion; im CI ggf. als eigener Job parallelisieren) | 17 s solo, ~98 s CPU inkl. Kindprozesse | Nur CI-/Volllauf-Wirkung | klein | Risikofrei |
| 6 | `-ldflags=-w` für Test-Binaries | gemessen: kein signifikanter Gewinn | keine | | verworfen |

Nicht adressierbar ohne Plattformwechsel: die macOS-Erst-Exec-Strafe (~0,9 s pro frischem Binary) und das fehlende Link-Caching in Go. Beide sprechen zusätzlich für Maßnahmen 1 und 2 (weniger frische Binaries pro Iteration).

## Empfehlung: `test-changed.sh` als pre-push-Hook

Vertretbar ist ein pre-push-Hook aus meiner Sicht erst, wenn der typische Lauf **p50 unter ~15 s und p95 unter ~60 s** bleibt; darüber wird er routinemäßig mit `--no-verify` umgangen und ist dann schlimmer als kein Hook.

Heutiger Stand: Kernänderungen kosten 100 bis 140 s pro Lauf (Läufe C bis E), jedes Mal, auch direkt nach einem grünen lokalen Lauf. Das ist zu langsam.

Was dafür noch fehlt:

1. **Maßnahme 1** (stabile Run-ID). Damit ist der pre-push nach einem grünen lokalen Lauf ein 3-bis-10-s-Cache-Durchlauf, und nur ungetestete Änderungen laufen wirklich. Das allein macht den Hook realistisch.
2. Nachmessen der Cache-Trefferquote in der lefthook-Umgebung (wegen der Umgebungs-Anomalie aus Lauf E; ein Env-Fingerprint-Unterschied zwischen Shell und Hook würde den Effekt still zunichte machen).
3. Der Frontend-Teil von `test-changed.sh` (`pnpm install --frozen-lockfile` + vitest bei jeder Frontend-Berührung) wurde hier nicht vermessen und braucht vor einem Hook-Einsatz dieselbe Betrachtung.
4. Optional Maßnahme 3, damit auch der Worst Case (kalter Cache nach Rebase) unter eine Minute fällt.

## Umsetzung (2026-09-01, Branch perf/backend-test-loop)

Maßnahmen 1 bis 4 sind umgesetzt und nachgemessen:

- **Maßnahme 1** (`scripts/test-run-id.sh`, stabile Run-ID pro Worktree + Overlap-Lock, `PHX_TEST_TEMPLATE` auch in `test-changed.sh`, `GOMAXPROCS`-Override entfernt, `-parallel 8` gepinnt): unveränderter Wiederholungslauf der vollen Suite über `test-backend.sh`: **3,5 s statt ~100 bis 140 s** (164/164 Packages `(cached)`). Nach einer Änderung nur am `test`-Package: 22,7 s (162 cached, Rest neu). Lock-Fallback verifiziert: bei lebendem fremdem Halter Wegwerf-ID plus Warnung, fremdes Lock bleibt stehen.
- **Maßnahme 2** (`test-changed.sh --fast` / Selektor `--direct`): Depth-1-Importer statt transitiver Closure, ohne das automatisch angehängte `test`-Package; Default-Modus byte-identisch, per Fixture-Kette in `backend-affected-packages_test.sh` gepinnt.
- **Maßnahme 3**: `test-backend.sh` läuft `-p 10` (lokal `max_connections=300`, Kommentar korrigiert), `test-changed.sh` cappt `-p` bei 8. Messwert: Volllauf mit kaltem Ergebnis-Cache 111 s bei `-p 10` gegenüber 99,6 s bei `-p 6` in der Analyse — im Rahmen der Lauf-zu-Lauf-Streuung, kein belegter Gewinn; der eigentliche Hebel ist der Cache.
- **Maßnahme 4** (`backend/test/source_index_test.go`): ein gemeinsamer, pro Root einmal gebauter Datei-Index ersetzt die Baum-Walks von `walkGoFilesRaw` und den drei Ad-hoc-Walkern in `hermetic_verification_test.go`. Solo-Zeit des `test`-Packages: **6,7 s statt 9,1 s**; `scanTree`/`scanRatchetPattern` und die AST-Ratchets sind bewusst eine spätere PR.
- **Vorbedingung**: der PreToolUse-Hook `guard-absolute-rules.sh` blockte jede Skript-Ausführung und damit die hier vorgeschriebenen Wrapper; er vettet jetzt git-getrackte Repo-Skripte (Testtabelle in `.claude/hooks/guard-absolute-rules_test.sh`, auch als CI-Step).

## Rohdaten

JSON- und Zeitdateien der Läufe A, C, D, E, F sowie CPU-/Mem-Profile der Top-5-Packages liegen im Session-Scratchpad (`runA_setup_only.json`, `runC_full.json`, `runD_time.txt`, `*.cpu`, `*.mem`); sie sind flüchtig. Reproduktion: die Kommandos stehen in der Tabelle im Messaufbau, alle mit `-count=1` außer D/E/F (die bewusst das reale `test-changed.sh`-Verhalten ohne `-count=1` nachstellen).

Hinweis zur Hygiene nach den Messungen: keine Clone-Leichen (`phx_test_pkg_*` = 0), Template neu aufgebaut und korrekt gestempelt, Sweep gelaufen.
