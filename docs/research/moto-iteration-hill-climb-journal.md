# Hill-Climb-Journal: moto-Dev- und Feedback-Loop

Stand: 31.10.2026. Ausführung des [Startprompts](moto-iteration-hill-climb-prompt.md)
auf Grundlage der [Bestandsaufnahme](moto-iteration-hill-climb-2026-10-31.md).
Testplatz: eigener Worktree `perf-iteration-hill-climb` mit wt-Ports (Frontend
3010, Backend 8090), eigenem Compose-Projekt und eigener Test-Run-ID.
Ausgangsrevision `6851990948`. Maschine: Darwin arm64, 16 CPUs, 64 GB; Go 1.27.3,
Node 24.21.0, pnpm 10.34.5. Last-Durchschnitt während der Messungen 9 bis 31,
weil parallel fremde Agent-Sessions liefen. Einzelwerte sind daher Größenordnungen,
keine Mikrobenchmarks.

## Messwerkzeug

`scripts/dev-bench.sh` misst Stufen der Task-Bank und schreibt TSV plus Metadaten
(Revision, Dirty-Diff-Hash, Last, Versionen, Heavy-Status) nach `tmp/dev-bench/`.
Spalten: Wandzeit, Wartezeit am Heavy-Gate (`queue_s`), CPU-Sekunden des
Prozessbaums, Peak-RSS des Baums, Plattenwachstum des Worktrees, Status.
Ein fehlgeschlagener Schritt lässt den Lauf mit Exit 1 scheitern; Sonden werden
auch bei Abbruch entfernt (`scripts/dev-bench.test.mjs`, 8 Tests).

| Aufgabe | Stufe | Inhalt |
| --- | --- | --- |
| Frischer Worktree | `fresh-worktree` | `git worktree add`, `devbox run bootstrap`, `doctor`, Entfernen |
| Go-Blatt | `go-leaf` | Sondendatei in `modules/workforce/http/timetracking`, `test-changed.sh --fast` |
| Go-zentral | `go-central` | Sondendatei in `sharedkernel/calendar`, `test-changed.sh --fast` |
| Frontend | `fe-change` | Kommentar in `src/lib/date-helpers.ts`, `test-changed.sh` und `pnpm run check` |
| Browser | `browser-flow` | `e2e/planung-navigation.spec.ts` gegen den eigenen nativen Stack |

Jede Änderungsstufe läuft als `change` und unverändert als `repeat`, damit
Cache-Treffer getrennt sichtbar sind. Änderungsstufen verweigern einen
schmutzigen Baum, weil uncommittete Dateien die Testauswahl verändern.

## Baseline

Baseline 1 (`20261031-083049`) war durch eine uncommittete Workflow-Datei
verfälscht: Sie erzwang den vollen Vitest-Lauf. Sie dient nur als Beleg für
Gate-Wartezeiten. Baseline 2 (`20261031-092243`, Revision `f825e86a0f`, sauber,
`--heavy`) für die Frontend-Stufen. Die Go-Zeilen stammen aus Baseline 3
(Revision `dd5b02a0c7`, Last 9 bis 24): Baseline 2 maß `go-central-change` mit
236 s und 126 von 129 gecachten Paketen, weil die Sonde den Test-Cache nicht
invalidierte (Runde 7).

| Stufe | Wand s | Warten s | CPU s | Peak MB | Auswahl |
| --- | ---: | ---: | ---: | ---: | --- |
| go-leaf-change | 129,1 | 34,8 | 246,6 | 2456 | 2 Go-Pakete, 0 gecacht, Frontend-Guards |
| go-leaf-repeat | 23,8 | 0,1 | 58,3 | 1321 | 2/2 gecacht, Guards erneut |
| go-central-change | 417,6 | 0,1 | 923,2 | 9216 | 170 Pakete, 129 mit Tests, 0 gecacht |
| go-central-repeat | 70,0 | 0,1 | 64,0 | 1308 | 128 gecacht |
| fe-change-tests-change | 98,8 | 0,7 | 409,3 | 2487 | Vitest inkrementell, 495 Dateien, 11 189 Tests |
| fe-change-check-change | 9,0 | 0,1 | 10,5 | 2772 | Lint, Typen, Format |
| fe-change-tests-repeat | 98,2 | 0,0 | 407,1 | 2466 | identisch, kein Ergebnis-Cache |
| fe-change-check-repeat | 7,5 | 0,0 | 4,2 | 1643 | tsbuildinfo warm |

Aus Baseline 1: `fresh-bootstrap` 26,1 s Wand, 59,5 CPU-s, 836 MB Peak,
1362 MB scheinbare Größe des neuen Worktrees; `doctor` 2,5 s.

Beobachtete Wartezeiten am Heavy-Gate über alle Läufe: 9,8 bis 490 s, Median
etwa 78 s. Ein Slot-Sampler (alle 5 s, 38 min) fand nie einen freien Slot:
28 % der Slot-Zeit hielt ein komplett gewrappter `quorum babysit` bei etwa einer
CPU-Sekunde pro Minute, 48 % andere Jobs, 24 % dieser Testplatz.

PR-Pipeline, 623 gemergte PRs in 14 Tagen (nur lesend über `gh`): Median
Öffnen→Merge 47 min, letzter Push→Merge 20 min, CI bis `ci-gate` 5,6 min bei
3 bis 12 s Queue. Die Dauer treibt Nacharbeit: 0 Commits nach Ready 15 min,
6 oder mehr 411 min. PRs mit Backend und Frontend: Median 218 min.
CI-Fehlschläge in PRs (366 Läufe, 17. bis 31.10.): `frontend-build` 144 (alle
13 Stichproben Bundle-Gates), Backend-Shards 116, Seed-Smoke 59,
`frontend-quality` 45, davon 37 knip.

## Runden

### Runde 0: Messung, die scheitern kann

Hypothese: Die vorhandene `dev-bench.sh` meldete auch fehlgeschlagene Schritte
als Messwert und mischte Gate-Wartezeit in die Laufzeit. Vertrag: kein grüner
Messwert ohne grünen Befehl; Sonden und Dateien werden immer wiederhergestellt.
Ergebnis: behalten (`7c60c6b387`). Fehlerfälle, Abbruch per SIGTERM (Exit 130,
Datei byte-genau wiederhergestellt) und Dirty-Tree-Verweigerung sind getestet.

### Runde 1: E2E prüft den eigenen Server

Hypothese: Playwright nutzte fest Port 3000 und `reuseExistingServer`; in einem
Worktree mit Port 3010 testete es den Server eines fremden Checkouts und
meldete grün. Messgröße: welcher Server beantwortet die Testanfragen.
Vorher: Anfragen an den fremden Server auf 3000, Tests grün. Nachher: Anfragen an
3010. CI setzt `PORT` nicht und nutzt weiter 3000. Ergebnis: behalten
(`4bf8e56b5b`, Doku `d197f2a19e`).

Folgewirkung, erst im Browser-Flow sichtbar: Der Sitzungs-Cache des Test-Logins
ist nach Origin geschlüsselt. Pro Worktree ist er beim ersten Lauf kalt; alle
Playwright-Worker loggen sich dann gleichzeitig mit MFA ein und überschreiten
das Limit von drei Codes pro Konto in 15 Minuten (`identity.mfa_blocked`, 7 von
13 Tests rot). Siehe Runde 5.

### Runde 2: knip im schnellen Pre-Push-Pfad

Hypothese: Seit knip aus Pre-Push entfernt wurde (`d0b32821e2`, 25.10.), stieg
die Zahl der knip-Fehler in CI von 0 auf 36 in sechs Tagen; jeder kostet einen
CI-Umlauf von etwa 6 min plus Nacharbeit. Erlaubter Preis: bis 10 s pro
ungecachtem Frontend-Push. Messung: knip 6,2 bis 6,6 s, 3,3 CPU-s, 0,7 GB.
Probe: ein ungenutzter Export lässt den Pre-Push mit Exit 1 scheitern und
besteht ohne ihn. Der Fingerprint-Cache von Pre-Push greift weiter.
Ergebnis: behalten (`5018e17a7f`). Nicht gemessen: Rückgang der CI-Fehler; das
braucht einige Tage Daten.

### Runde 3: Gate-Slots nicht für Warteschleifen

Befund aus dem Slot-Sampler, siehe Baseline. Änderung: Hinweis in
[quorum-review-loop.md](../agents/quorum-review-loop.md), nur `--test-cmd` zu
gaten (`ba4ea65e14`). Der Effekt hängt davon ab, dass Sessions dem folgen; er
ist nicht nachgemessen. Empfehlung für die globalen Agent-Regeln:
Orchestratoren und Warteschleifen nie in `agent-heavy` wickeln.

### Runde 4: Frischer Worktree bis zum laufenden Stack

Messgröße: manuelle Eingriffe von frischem Worktree bis zum ersten
Browser-Ergebnis. Vorher: zwei Eingriffe, jeder erst beim Start sichtbar,
`doctor` grün.

1. Next brach ab, weil `NEXT_PUBLIC_CATERER_HOSTNAME` in `.env` fehlte.
2. Das Backend brach ab, weil `TURNSTILE_SECRET_KEY` und `TURNSTILE_SITE_KEY`
   fehlten.

Dabei aufgefallen: Unter Devbox fehlt der Bash das Builtin `compgen`.
`dev-native.sh` übersah dadurch Variablen aus der Shell, und `.env` überschrieb
sie still, etwa `DB_DSN` oder Ports.

Lokal fehlten in 31 von 67 Worktrees Schlüssel aus der eigenen `.env.example`.
Änderungen: `doctor` nennt fehlende Schlüssel aus `.env.example` und
`backend/dev.env.example`, nur Namen, keine Werte (`a82ac61e8e`, Doku
`cda0e7aa2f`). `dev-native.sh` liest exportierte Namen aus `export -p`; ein
Regressionstest erzwingt eine Bash ohne `compgen` (`916e568773`).
Nachher: `doctor` meldet die Lücke vor dem Start in unter 3 s; mit ergänzten
Schlüsseln waren Backend und Frontend nach 10 s erreichbar. Ergebnis: behalten.

### Runde 5: Paralleler Erst-Login im Browser-Flow

Hypothese: Auf kaltem Sitzungs-Cache fordern alle Playwright-Worker gleichzeitig
einen MFA-Code für dasselbe Konto an; ab dem vierten Code in 15 Minuten antwortet
das Backend mit 429 (`identity.mfa_blocked`). Vertrag: echter UI-Login mit MFA
bleibt, Assertions unverändert. Änderung: Ein Worker loggt sich unter einer
Lock-Datei ein, die anderen warten und übernehmen dessen Sitzung; verwaiste Locks
verfallen nach 60 s (`09327291c8`). CI läuft mit einem Worker und ist nicht betroffen.

| Lauf | Cache | Ergebnis | Wand s | CPU s |
| --- | --- | --- | ---: | ---: |
| vorher | kalt | 6 grün, 7 rot mit 429 | 10,9 | 22,7 |
| nachher | kalt | 12 grün, 1 rot (siehe unten) | 31,5 | 45,7 |
| nachher | warm | 12 grün, 1 rot (siehe unten) | 31,2 | 27,8 |

Der verbleibende Fehler ist unabhängig von Port und Login: Der Test
„/substitutions stays its own page“ wartet auf `networkidle`, das lokal nie
eintritt, auch mit einem Worker; die Seite ist dann fertig gerendert. Das
Backend-Log zeigt offene SSE-Antworten von 25,7 s. Gemeldet als #5397. Die
Wandzeit von 31 s ist deshalb vom 30-s-Timeout dieses Tests bestimmt.

Frischer Worktree bis zum ersten Browser-Ergebnis, nach den Runden 4 und 5 und
mit warmen Caches: Bootstrap 26 s, `doctor` 3 s, Stack bereit nach 9 bis 10 s,
Seed 120 s, Browser-Flow 31 s; zusammen etwa 3 min ohne manuellen Eingriff außer
dem Ergänzen der von `doctor` genannten Env-Schlüssel. Der Seed ist der größte
Posten. Ergebnis: behalten.

### Runde 6: CI-Fehlerklassen seit dem aktuellen Bundle-Gate

Die Auswertung der Baseline umfasst überwiegend die Zeit vor dem Delta-Gate
(#5285, 29.10.) und `98608f2ee9` (30.10.). Nachmessung für 30.10. 10:00 UTC bis
31.10. 11:00 UTC mit `gh run list --workflow main.yml --event pull_request`,
fehlgeschlagenen Jobs aus der Jobs-API und `gh run view --job ID --log`:
62 PR-Läufe, 13 fehlgeschlagen.

| Klasse | Läufe | Branches | Lokal abfangbar |
| --- | ---: | ---: | --- |
| Bundle-Obergrenze (absolut, kein Delta) | 5 | 3 | kaum: 13 bis 201 Bytes über der Grenze, macOS und Linux weichen ab |
| `deadcode` | 5 | 1 | ja, aber 33 s, 70 CPU-s, 11,7 GB Peak-RSS |
| Backend-Tests | 4 | – | `test-changed.sh` |
| knip | 3 | 2 | ja, Runde 2 |
| Seed-Smoke, Core-Action fehlt | 2 | 1 | ja, `TestFullProductionRouterGolden` im `api`-Paket |

4 der 5 Bundle-Fehler trafen `/[tenant]/reset-password`: `development` liegt
wenige Bytes unter `referenceBytes` plus 2 %, sodass jede PR mit wenigen Bytes
Wachstum gemeinsamen Codes rot wird. Das ist eine Policy-Frage (Puffer,
Warnschwelle, Neubasierung) und in #5396 nachgetragen; nicht autonom geändert.

Verworfen: `deadcode` in Pre-Push. Ein Branch pro Tag rechtfertigt 11,7 GB
Spitzenspeicher pro Backend-Push nicht, solange mehrere Agents den Rechner
teilen. Auffällig ist eher, dass derselbe Fehler auf einem Branch fünfmal in
Folge gepusht wurde.

### Runde 7: Sonde, die den Test-Cache wirklich invalidiert

Befund aus dem ersten Parallellauf: Im zweiten Lauf eines Worktrees meldete die
Go-„Änderung“ beide Pakete als gecacht. Go cached Testergebnisse nach dem Inhalt
des Test-Binaries; der Linker entfernte die ungenutzte Sondenfunktion, das
Binary blieb gleich. Probe an `sharedkernel/calendar`: alte Sonde mit neuem Wert
`(cached)`, neue Sonde (`init()` schreibt den Wert in eine Paketvariable) läuft
neu. Fix mit Regressionstest (`dd5b02a0c7`). Folge: Baseline 2 unterschätzte
`go-central-change` (236 statt 418 s, 651 statt 923 CPU-s, 6,4 statt 9,2 GB);
die Tabelle oben ist korrigiert. Der erste Parallellauf ist verworfen.

### Runde 8: Kontrollierter Parallellauf 1/2/4

Aufbau: vier frische Worktrees auf derselben Revision (`dd5b02a0c7`), jeder
startet `dev-bench.sh go-leaf` (Änderung und Wiederholung) gleichzeitig, ohne
Heavy-Gate (vom Nutzer freigegeben), gemeinsamer Test-DB-Server. Skript:
`tmp/hill-climb/par.sh`, Ergebnisse `tmp/par/results.tsv`. Last vor dem Start 9
bis 16 durch fremde Sessions.

| Agents | Batch-Wand s | Ergebnisse/min | Änderung je Agent s | CPU-s je Änderung | Peak MB je Agent |
| ---: | ---: | ---: | --- | --- | --- |
| 1 | 135,8 | 0,44 | 112 | 208 | 2170 |
| 2 | 141,8 | 0,85 | 105 bis 116 | 201 bis 215 | 1713 bis 2328 |
| 4 | 178,5 | 1,34 | 86 bis 106 | 148 bis 198 | 1891 bis 2247 |

Für eine leichte Go-Aufgabe skaliert der Durchsatz bis vier Agents fast linear;
CPU und RAM je Ergebnis bleiben gleich. Mit zwei Gate-Slots hätten vier Agents
etwa 2 × 136 s gebraucht statt 178 s. Bei `go-central-change` (923 CPU-s,
9,2 GB) wären vier gleichzeitige Läufe etwa 37 GB; dort schützt das Gate.
Folgerung: Die Gate-Grenze sollte nach erwarteter Last gewichten, nicht nur
Jobs zählen. `agent-heavy` ist ein nutzerlokales Werkzeug und wurde nicht geändert.

Falsches Rot bei vier Agents: Ein Agent scheiterte an
`listen tcp 127.0.0.1:10026: bind: address already in use` in
`TestDemoAdminEnrollsAndSignsInAgainWithoutExternalMail`. Parallele Worktrees
teilen diesen festen Port; das ist die zweite Ursache von #5395 und betrifft
jede parallele Agent-Arbeit, die `api` testet. Siehe Runde 9.

### Runde 9: Gemeinsamer Demo-Code-Port

Hypothese: `TestDemoAdminEnrollsAndSignsInAgainWithoutExternalMail` scheitert,
sobald ein anderer Prozess auf dem Rechner `127.0.0.1:10026` hält. Rot
reproduziert mit einem Hilfsprozess, der den Port mit `SO_REUSEADDR` hält (wie
Go). Python ohne diese Option scheitert an TIME_WAIT-Resten, Go nicht; das
verdeckte den Effekt zunächst. Während der Messung war der Port zweimal kurz
von fremden Testprozessen belegt. Fix: Der Bind ist der prozessübergreifende
Lock, der Test wartet bis zu einer Minute darauf; Assertions unverändert.
Nachher grün nach 7,1 s Wartezeit. PR #5404. Offen in #5395: der
lastabhängige `mfa status unavailable` und die feste E-Mail bei `-count>1`.

### Runde 10: Gewichtetes Heavy-Gate und Rest von #5395

Nach Freigabe des Nutzers hat `~/bin/agent-heavy` jetzt 4 Einheiten: Standard-Jobs
belegen 2, `--light` 1. Wartet ein schwerer Job länger als 20 s, sperrt er neue
leichte Zulassungen. 7 Verhaltenstests (isoliertes State-Verzeichnis), dreimal
grün; die alte Version scheitert an 5. Die globalen Agent-Anweisungen nennen
`--light` für mittlere Jobs. Nicht nachgemessen: Wartezeiten mit dem neuen Gate.

#5395: Die feste E-Mail-Adresse brach `-count>1` (duplicate key), behoben in
#5410. `mfa status unavailable` ließ sich nicht reproduzieren: zwei gleichzeitige
Läufe des ganzen `api`-Pakets aus zwei Worktrees, bei Last 6 und bei Last 38
(14 CPU-Burner), jeweils grün.

### Runde 11: Seed und CI-Pfad, kein billiger Hebel

Seed auf dem nativen Stack, je mit vorherigem `migrate reset` (6 bis 9 s):

| Lauf | Seed s | Last |
| --- | ---: | --- |
| alle vier Profile | 106,0 / 89,4 | 10 / 5 |
| nur `vollbetrieb` | 88,5 / 81,0 | 9 / 5 |

Der Seeder arbeitet sequenziell über die HTTP-API: rund 4 600 Requests pro Lauf;
die Summe der Server-Latenzen entspricht der Wandzeit. Größter Posten
`manual-approved-enrollments` mit 291 ms pro Aufruf, etwa 17 s pro Lauf.
Hypothese Seed-Cache geprüft: `pg_dump` der geseedeten DB 0,4 s (3,4 MB),
`pg_restore` 4,3 s. Verworfen trotz 20-facher Beschleunigung: 36 von 361
Worktrees wurden in 7 Tagen geseedet, etwa 5 Seeds pro Tag; ein Cache gilt nur
für denselben Berliner Tag, dieselbe Backend-Revision und dieselben Env-Schlüssel
und braucht Rollen vor dem Restore. Ersparnis wenige Minuten pro Tag.

CI-Pfad (15 grüne PR-Läufe von `main.yml`): Zuletzt fertig wurden `backend-lint`
(bis 524 s, Schritt `golang ci lint` 193 s im Mittel, 476 s maximal),
`Seed & simulate smoke` (bis 337 s), `CI and release script suites` (bis 259 s)
und `frontend-build` (bis 211 s). Der Lint-Cache wird bereits in #5409 (anderer
Autor, am 31.10. gemergt) angegangen. Der Restore-Shard (167 s) läuft nur bei
Änderungen an Backup-, Release- oder Workflow-Dateien. Der Seed-Smoke ist ein
bewusster Integrationstest. Kein Eingriff.

### Runde 12: Query-Budget-Register als eigenes Paket

Frequenz statt Einzelbefehl: 83 von 383 gemergten Backend-PRs der letzten 14
Tage änderten Produktionsdateien von `backend/test`, 157 Mal eine Datei unter
`test/query-budgets/`. Das Register war per `//go:embed` Teil des Pakets `test`.
Jede Budget-Änderung baute deshalb alle 174 Importeure neu und wählte sie aus,
obwohl nur 40 Pakete `AssertQueryBudget` aufrufen.

- Hypothese: Ein eigenes Paket neben den JSON-Dateien beschränkt Auswahl und
  Test-Cache-Invalidierung auf die Budget-Aufrufer.
- Zielmetrik: Laufzeit und CPU von `test-changed.sh --fast` nach einer
  Budget-Änderung.
- Vertrag: dieselben Budgets, Pfade, Prüfregeln und Ratchets; jeder Aufruf
  bleibt für `TestQueryBudgetRatchet` sichtbar.
- Abbruch: Architektur-Check rot oder ein Aufrufer fällt aus der Auswahl.

Ergebnis (#5430), je ein Lauf unter Last 5 bis 21:

| | vorher | nachher |
| --- | ---: | ---: |
| Pakete | 174 | 42 |
| Wandzeit s | 571 | 358 |
| CPU-s | 1752 | 1261 |

Beide Läufe waren grün. Den Rest bestimmen `internal/architecture` (101 s; liest
die Budgets bewusst), `api` (90 s) und `organizationtenancy/compose` (82 s).
Behalten.

### Runde 13: Paket-Worker auf macOS, `-p 4` verworfen

`go-package-workers.sh` begrenzt macOS auf 2 Paket-Binaries, weil gleichzeitige
Clones auf Docker Desktops virtueller Platte normale Fixture-Queries aushungern.
A/B mit Sonde in `sharedkernel/calendar`, abwechselnd (Zeit im Gate):

| `-p` | Wandzeit s | CPU-s |
| --- | ---: | ---: |
| 2 | 415 / 387 | 988 / 765 |
| 4 | 239 / 249 | 790 / 780 |

Ein einzelner Lauf ist mit `-p 4` 38 % schneller. Gegenprobe mit zwei
gleichzeitigen `-p 4`-Läufen auf demselben Test-DB-Server, so viel wie das
gewichtete Gate zulässt: Ein Lauf scheiterte, weil `CreateTestAccount` nach 45 s
mit `read tcp …:5453: i/o timeout` abbrach. Das ist genau der dokumentierte
Fehlermodus. Abbruchkriterium erreicht, Grenze bleibt 2. Eine Lockerung
bräuchte eine Grenze pro Test-DB-Server statt pro Lauf.

### Runde 14: CI-Fehler nach der Bundle-Neubasierung

Nachmessung für 31.10. 12:45 bis 22:00 UTC (nach #5400), gleiche Methode wie
Runde 6: 54 PR-Läufe, 9 fehlgeschlagen, alle im Frontend. Ein Lauf kann mehrere
Klassen haben.

| Klasse | Läufe | Branches | Vom bestehenden Pre-Push abgedeckt |
| --- | ---: | ---: | --- |
| knip, ungenutzte Exporte | 4 | 2 | ja |
| oxlint (SWR-Schlüssel, doppelter Bezeichner) | 3 | 3 | ja |
| Guard-Inventar `--check-guards` | 3 | 2 | nein |
| Bundle | 2 | 2 | nein; ein echtes PR-Delta (+45 KB), eine Grenze 144 Bytes über |
| depcruise, Zyklus | 1 | 1 | nein |
| Vitest `dom-2` | 1 | 1 | nein |

7 der 9 Läufe scheiterten an einer Prüfung, die der Pre-Push schon ausführt; diese
Pushes liefen also ohne den Hook (andere Rechner, `PHX_PREPUSH=skip` oder
`--no-verify`). Auf 2 von 4 Branches scheiterte derselbe Job zweimal in Folge.
Zwei Änderungen:

- `--check-guards` läuft jetzt im schnellen Frontend-Gate des Pre-Push
  (etwa 3 s). Probe: eine neue Testdatei mit `node:fs` ohne Inventareintrag
  scheitert lokal mit derselben Assertion wie in CI.
- `scripts/ci-failures.mjs` zeigt für den letzten Lauf eines Branches die roten
  Jobs und Schritte, deren Fehlerzeilen (höchstens 30 plus 12 Zeilen Ende) und
  den lokalen Befehl, und warnt, wenn der vorige Push im selben Job scheiterte.
  An #4767 gegen echte Logs geprüft: 5 rote Jobs auf etwa 70 Zeilen.

Beifund: #5430 benennt `testpkg.AssertQueryBudget` um. Offene PRs, die neue
Aufrufer hinzufügen, scheitern nach dem Merge der Basis am Build (#4767).

### Runde 15: `deadcode` lokal, verworfen

Über 233 rote PR-Läufe (25. bis 31.10.) scheiterten 11 an `deadcode`, auf 6
Branches; einer davon fünfmal hintereinander. Kosten lokal (Last 18 bis 30):

| Modus | Wandzeit s | CPU-s | Peak-RSS |
| --- | ---: | ---: | ---: |
| Tests (`-test`) | 30 | 55 | 11,9 GB |
| Produktion | 10 | 16 | 4,4 GB |
| Tests, `GOMEMLIMIT=3GiB` | 53 | 194 | 8,2 GB |
| Produktion, `GOMEMLIMIT=3GiB` | 9 | 22 | 3,2 GB |

Der Live-Heap im Testmodus liegt über 3 GiB; das Limit erzwingt nur GC-Arbeit.
Pro Backend-Push 12 GB bei mehreren Agents auf einer Maschine sind nicht
vertretbar. Stattdessen nennt `ci-failures.mjs` für `backend-quality` den
lokalen Befehl `scripts/check-deadcode.sh`, damit nach einem roten Lauf nicht
erneut ungeprüft gepusht wird.

### Runde 16: Go-Testausgabe für Agents begrenzt

Ein gescheitertes Paket gibt alles aus, was sein Binary geschrieben hat; einmal
494 MB aus `api`. `test-changed.sh` schrieb das per `tee` in die Ausgabe des
Agents. Jetzt gehen live nur Paketergebnisse und Build-Fehler durch; ein Fehler
ergänzt die Zusammenfassung, die jetzt auch `t.Fatal`-Meldungen enthält, und
behält die volle Ausgabe gzip-komprimiert unter `test-changed-go.log.gz`.
Probe mit einem Test, der 20 000 Zeilen schreibt und scheitert: vorher
20 000 Zeilen live, nachher 14 Zeilen mit Meldung; Log 52 KB komprimiert.

## Verworfen

- Frontend-Guards bei Backend-Änderungen verkleinern: Die 45 Guard-Dateien sind
  ein bewusst fail-closed Inventar von IO-Grenzen. Kürzen würde den Testvertrag
  ändern, nicht nur Kosten senken.
- `-p 4` auf macOS (Runde 13): einzeln 38 % schneller, unter paralleler Last
  DB-Timeouts.
- `deadcode` im Pre-Push (Runde 15): 30 s und 11,9 GB pro Backend-Push.

## Unknown Unknowns

- Ein grüner lokaler E2E-Lauf bewies vor Runde 1 nichts über den eigenen Code.
- `doctor` prüfte Werkzeuge, nicht die Konfiguration, an der der Start scheitert.
- Devbox liefert eine Bash ohne Programmable Completion; Skripte, die mit
  `/bin/bash` getestet werden, laufen in der Praxis unter einer anderen Bash.
- `devbox run -- CMD ARGS` setzt die Argumente zu einem Shell-String zusammen;
  `$VAR` in Argumenten wird vor dem Zielbefehl expandiert.
- Das Heavy-Gate hat zwei Slots ohne FIFO. Die Wartezeit hängt mehr davon ab,
  wer einen Slot hält, als von der eigenen Arbeit.
- Lastabhängige Flakes: `TestDemoAdminEnrollsAndSignsInAgainWithoutExternalMail`
  scheiterte 2/2 bei Last 18 bis 25 und bestand bei Last 12 (#5395); ein
  fehlgeschlagenes `api`-Paket schrieb 494 MB Log. Ein tsgo-Panic
  (`ScriptKind must be specified`) trat in 1 von 6 Frontend-Läufen auf,
  vermutlich parallel zu einer pnpm-Installation. Später ein V8-GC-Absturz in
  Node 24.21.0 (`Check failed: !IsFreelistEntry()`) in den Vitest-Guards, bei
  Wiederholung grün. Zwei native Abstürze an einem Tag unter hoher Last; nicht
  gemeldet, weil keiner reproduzierbar war.
- Parallele Agents teilen feste Loopback-Ports. Ein Port-Konflikt sieht aus wie
  ein Testfehler, nicht wie ein Infrastrukturfehler.

## Extrapolation

Die größten Hebel liegen nicht in einzelnen Befehlen, sondern in Rückläufen:
Ein vermeidbarer CI-Fehler kostet einen Umlauf von etwa 6 min und verlängert die
PR-Dauer stärker als jeder lokale Sekundengewinn (0 gegen 6+ Nacharbeits-Commits:
15 gegen 411 min). Daraus folgt eine Reihenfolge: zuerst Fehlerklassen, die CI
häufig und lokal billig finden lässt (knip, Bundle-Gates #5396), dann
Startfehler des Testplatzes, erst danach reine Laufzeit.

## Nächster belegter Engpass

1. Wirkung des gewichteten Gates nachmessen (Runde 10): Wartezeit am Gate und
   Durchsatz mit gemischten leichten und schweren Jobs.
2. Zentrale Go-Änderung: 418 s und 923 CPU-s für 129 Pakete im Schnellmodus.
   Häufigster Auslöser (Budget-Dateien) in Runde 12 entschärft; übrig bleiben
   echte Änderungen an `test/fixtures*.go` und Shared-Kernel-Paketen. Eine
   DB-Parallelitätsgrenze pro Test-Server statt pro Lauf würde `-p 4` erlauben.
3. Vitest bei zentralen Frontend-Dateien: 495 Dateien, 409 CPU-s, ohne
   Ergebnis-Cache bei unveränderter Wiederholung. Setup 31 s und Environment
   60 s der Vitest-Zeit sind der größte Posten.
4. Frontend-Guards bei reinen Go-Änderungen: 45 Dateien, 15,7 s bei jeder
   Wiederholung, auch wenn Go alles aus dem Cache liefert.
5. Pushes ohne Pre-Push-Hook (Runde 14): 7 von 9 roten Läufen hätte der Hook
   abgefangen. Unklar ist, ob Hooks auf anderen Rechnern installiert sind oder
   übersprungen werden; erst messen, dann entscheiden.

Erledigt aus dieser Liste: Bundle-Obergrenzen nahe der Basis (#5400, Neubasierung
nach Entscheidung des Nutzers in #5396), Heavy-Gate nach Last gewichtet (Runde 10),
Query-Budget-Paket (Runde 12), CI-Fehlerbericht und Guard-Inventar im Pre-Push
(Runde 14), begrenzte Go-Testausgabe (Runde 16). Verworfen: Seed-Cache
(Runde 11), `-p 4` auf macOS (Runde 13), `deadcode` im Pre-Push (Runde 15).
