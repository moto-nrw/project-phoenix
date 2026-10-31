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
`--heavy`):

| Stufe | Wand s | Warten s | CPU s | Peak MB | Auswahl |
| --- | ---: | ---: | ---: | ---: | --- |
| go-leaf-change | 118,3 | 0,0 | 211,7 | 1836 | 2 Go-Pakete, Frontend-Guards |
| go-leaf-repeat | 24,8 | 77,2 | 57,6 | 1279 | 2/2 gecacht, Guards erneut |
| go-central-change | 236,3 | 0,1 | 650,8 | 6429 | 170 Pakete, 129 mit Tests, 126 gecacht |
| go-central-repeat | 84,8 | 20,9 | 135,1 | 2439 | 128 gecacht |
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

## Verworfen

- Frontend-Guards bei Backend-Änderungen verkleinern: Die 45 Guard-Dateien sind
  ein bewusst fail-closed Inventar von IO-Grenzen. Kürzen würde den Testvertrag
  ändern, nicht nur Kosten senken.
- Kontrollierter 1/2/4-Parallel-Lauf: nicht ausgeführt, weil er die Rechner
  fremder Sessions belastet hätte, solange beide Gate-Slots dauerhaft belegt
  waren. Ohne ihn ist die optimale Parallelität nicht belegt.

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
  vermutlich parallel zu einer pnpm-Installation; nicht gemeldet, nur einmal gesehen.

## Extrapolation

Die größten Hebel liegen nicht in einzelnen Befehlen, sondern in Rückläufen:
Ein vermeidbarer CI-Fehler kostet einen Umlauf von etwa 6 min und verlängert die
PR-Dauer stärker als jeder lokale Sekundengewinn (0 gegen 6+ Nacharbeits-Commits:
15 gegen 411 min). Daraus folgt eine Reihenfolge: zuerst Fehlerklassen, die CI
häufig und lokal billig finden lässt (knip, Bundle-Gates #5396), dann
Startfehler des Testplatzes, erst danach reine Laufzeit.

## Nächster belegter Engpass

1. Bundle-Obergrenzen in `frontend-build`: seit dem aktuellen Gate 5 von 13
   fehlgeschlagenen PR-Läufen, alle absolute Grenzen knapp über dem Ist (#5396).
   Wartet auf eine Policy-Entscheidung.
2. Vitest bei zentralen Frontend-Dateien: 495 Dateien, 409 CPU-s, ohne
   Ergebnis-Cache bei unveränderter Wiederholung. Setup 31 s und Environment
   60 s der Vitest-Zeit sind der größte Posten.
3. Seed beim ersten Browser-Lauf: 120 s von etwa 3 min bis zum ersten
   Browser-Ergebnis in einem frischen Worktree.
4. Frontend-Guards bei reinen Go-Änderungen: 45 Dateien, 15,7 s bei jeder
   Wiederholung, auch wenn Go alles aus dem Cache liefert.
