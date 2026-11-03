# Test-Performance: Messplatz

Startprompt: [Test-Hillclimb](test-performance-hill-climb-prompt.md).
Dieser Messplatz hält den gemeinsamen Prüfer und das Messprotokoll fest.
Die ausgeführten Baselines, Defektproben und Runden stehen im
[Experimentjournal](test-performance-hill-climb-journal.md).

## Ziel und Entscheidung

Primär zählt die Zeit von einer realistischen Änderung bis zum **korrekten
Testergebnis**, lokal und auf dem relevanten CI-Pfad. Ein 4×-Speedup ist das
Stretch-Ziel gegenüber einer neuen, gültigen Baseline. Berichte lokale Vollsuite,
Änderungsfälle und CI getrennt. Ein schnellerer Teiljob belegt keinen schnelleren
PR-Check.

Korrektheit ist eine Eintrittsbedingung, kein verrechenbarer Score. Vergleiche
danach Wandzeit, CPU-Sekunden, Prozessbaum-RSS, Flakes und CI-Runner-Zeit.
Behalte die nicht dominierten Varianten: Eine Variante dominiert eine andere,
wenn sie bei gleicher belegter Fehlererkennung in keiner relevanten Metrik
schlechter und in mindestens einer besser ist. Bei einem echten Tradeoff bleibt
die Entscheidung sichtbar; zusätzliche Runner kaufen Zeit, sparen aber nicht
automatisch Arbeit oder Geld.

## Start

Alle Befehle ab Repo-Root. Der Toolchain-Runner lädt die eingecheckten Werkzeuge.
Die Messung nutzt automatisch `~/bin/agent-heavy`, falls vorhanden: `quick`
belegt einen leichten Slot, Vollsuite und CI-Projekte die schwere Zulassung. Den
Mess-Orchestrator nicht zusätzlich in das Gate packen: Nur der jeweilige
Testcommand belegt Slots. Keine App-Server starten. Vitest benötigt keine
Datenbank; Backendtests initialisieren ihren Testserver über den vorhandenen
Lifecycle. Für isolierte Backendversuche die private Test-DSN vererben, niemals
in Commands, Logs oder diese Dokumentation schreiben.

```bash
scripts/run-go-toolchain.sh node scripts/test-perf.mjs --help
scripts/run-go-toolchain.sh node --test scripts/test-perf.test.mjs scripts/dev-bench.test.mjs
scripts/run-go-toolchain.sh node scripts/test-perf.mjs run quick smoke
```

`smoke` hat bewusst den Cache-Zustand `uncontrolled` und lässt sich nicht als
Baseline vergleichen. `warm` ist eine dokumentationspflichtige Angabe des
Ausführenden, keine automatische Cache-Erwärmung.

## Feste Workloads

[Manifest](../../scripts/test-perf-workloads.json), für Baseline und Kandidat
identisch und aus derselben Kopie des Evaluators ausgeführt:

| Profil | Verwendung |
| --- | --- |
| `quick` | Sechs vorhandene API-/Logik-/DOM-/UI-Suites für kurze Experimente. Kein repräsentativer Gesamtscore. |
| `full` | Vollständige lokale Vitest-Suite als unabhängige Breitenkontrolle. |
| `ci-node` | Aktuelle Node-Projekte mit CI-Console-Option. |
| `ci-dom-1`, `ci-dom-2` | Aktuelle DOM-Projekte, auf zwei Vitest-Shards aufgeteilt. |
| `backend-full` | Gesamte Go-Suite über den bestehenden Wrapper mit `-count=1 ./...`, Bootstrap und Leftover-Gate. |
| `backend-architecture`, `backend-export`, `backend-lifecycle`, `backend-smoke` | Feste gezielte Go-Pakete über denselben Wrapper, jeweils ohne Ergebnis-Cache. |

Die CI-Profile bilden nur die Vitest-Befehle ab, nicht Runner-Start, Installation,
Browser-Layouttests oder den Workflow-DAG. Bei Workflow-Änderungen zuerst das
Manifest separat versionieren und beide Seiten neu messen. Layout-, Netzwerk-,
Backend-, Architektur- und übrige Pflichtchecks bleiben erhalten.

Jeder Trial schreibt in ein neues Verzeichnis unter `tmp/test-perf/`:

- `metadata.json`: Revision, Dirty-Diff-Hash, ungetrackte Input-Hashes, Toolchain,
  Workload-/Evaluator-Hash, relevante Konfiguration als Hash und Startlast.
- `vitest.json`: Originalreport mit Testnamen, Status und Einzeldauern.
- `measure/`: Original-Logs, TSV und Metadaten des bestehenden
  [Ressourcenmessers](../../scripts/dev-bench.sh).
- `summary.json`: Metriken, vollständiges Ausführungsinventar, Last und Gültigkeit.

Vitests Ausführungsinventar enthält Dateipfade, vollständige Testnamen, Status
und Mehrfachvorkommen. Bestehende Skips werden sichtbar konserviert; zusätzliche
Skips, verlorene Tests und abweichende Inventare blockieren den Vergleich.
Backendprofile schreiben stattdessen den originalen `go.jsonl`-Report und prüfen
Run-/Terminalereignisse einschließlich Mehrfachvorkommen, Fehlern und Skips
streamend. Ihre Kontextmetadaten enthalten Go-Version, Locks, Wrapper und die
Git-Basisinputs: Auflösungserfolg des Targets und sämtliche unveränderlichen
Merge-Basen zu HEAD. Target-Tips vor/nach dem Trial werden separat dokumentiert.
Auch die tatsächlich gewählte Basis bleibt ein strikter Vergleichsinput, falls
Git mehrere beste gemeinsame Vorfahren liefert.
Basiswechsel während eines Trials oder zwischen Vergleichsseiten invalidieren
den Vergleich. Ein voranschreitender Target-Tip bei identischer Basis ist kein
anderer Ratchetinput; fehlende Basen bekommen keinen Ersatz für den echten Ratchet.

Ein Exitcode 0 ohne tatsächlich bestandene Tests ist ungültig. Fehler bleiben
in den Rohartefakten; ein während der Messung geänderter Checkout ist ungültig.
Der Vergleich prüft **keine Assertion-Stärke** und ersetzt keine Defektproben.

## Gültiger Vorher-/Nachher-Vergleich

1. Messwerkzeug und Workloads auf einem gemeinsamen Stand festhalten, getrennt
   von Optimierungsänderungen. Das verhindert, dass neue Harness-/Workflow-Dateien
   die Änderungsselektion der Baseline verfälschen. Keine fremde Arbeit committen.
2. Baseline- und Kandidatenstand explizit festhalten. Nutze dieselbe Kopie dieses
   Messwerkzeugs mit `--root CHECKOUT`, dieselben Tools, Lockfiles und relevante
   Konfiguration. Der strenge Vergleich lehnt abweichende Umgebungen ab, auch
   unterschiedliche ignorierte Env-Dateien. Untersuche solche Unterschiede,
   statt sie aus dem Prüfer zu entfernen.
3. Einen nicht gewerteten Warmlauf pro Seite ausführen. Danach unter vergleichbarer
   Last abwechselnd Baseline/Kandidat und Kandidat/Baseline messen. Neue Revisionen
   bekommen neue Trials. Anzahl aus Streuung und Aufwand ableiten; eine einzelne
   Probe ist nur ein Hinweis. Prüfschwelle und Ressourcenbudget vor den Trials
   im Journal festlegen. Keine p95 aus wenigen Messpunkten ableiten.
4. Gleich viele gepaarte Trials vergleichen. Die ausgegebenen Verzeichnisse
   einsetzen; Reihenfolge entspricht den Paaren:

   ```bash
   scripts/run-go-toolchain.sh node scripts/test-perf.mjs run quick baseline --root /ABS/BASE --cache-state warm
   scripts/run-go-toolchain.sh node scripts/test-perf.mjs run quick candidate --root /ABS/CANDIDATE --cache-state warm
   scripts/run-go-toolchain.sh node scripts/test-perf.mjs compare /ABS/BASE_TRIAL_1 /ABS/BASE_TRIAL_2 -- /ABS/CANDIDATE_TRIAL_1 /ABS/CANDIDATE_TRIAL_2
   ```

   Median, Spannweite, einzelne Paar-Speedups und Ressourcen stehen im Ergebnis.
   Gate-Wartezeit bleibt separat. Lastwerte helfen Störungen zu erkennen, beweisen
   aber keine identische Konkurrenzlast. Prozessbaum-RSS wird alle 0,5 s abgetastet;
   kurze Spitzen können fehlen. Wandzeit hat die Auflösung des Ressourcenmessers.
5. Einen gehaltenen Kandidaten auf `full`, realistischen Änderungsfällen und den
   betroffenen Pflichtchecks bestätigen. `quick` ist auf bekannte langsame Dateien
   konzentriert und darf die Auswahl nicht auf sich selbst optimieren.

Vitests persistenter Sortiercache ist über `--cache=false` für alle Trials aus.
Shared Dependency-/Build-/OS-Caches bleiben unangetastet. Dieser Modus misst echte
Testausführung, nicht das Wiederverwenden eines grünen Ergebnisses. Produktive
Cachepfade zusätzlich separat prüfen. Frische Worktrees sind keine kalten
Maschinen; echte Cold-Versuche brauchen task-eigene Cachepfade.

Geänderte Dependencies, Toolchain, Workloads oder Prüfer sind neue Messbedingungen.
Dafür einen separat begründeten Versuchsaufbau und gemeinsame Kontrollläufe
anlegen. Inkompatible Werte nicht zu einem Speedup zusammenrechnen.

## Fehlererkennung und Änderungsselektion

Vor einer Optimierung der Selektion mindestens diese Fälle auf einem identischen,
isolierten Ausgangsbaum durchspielen. Der aktuelle Branch-Diff ist kein Ersatz
für jeweils genau eine kontrollierte Änderung.

| Fall | Nachweis |
| --- | --- |
| Blattmodul / einzelne UI-Komponente | Betroffene echte Tests laufen; unabhängige Fehlerprobe wird gefunden. |
| Zentrale Datei, z. B. Datumshelper oder Session-Abhängigkeit | Transitive Importer und IO-Guards bleiben erfasst. |
| Übersetzungs-JSON / generierter Katalog | Tatsächliche Import- und Generatorabhängigkeiten prüfen. |
| Help-Screenshot / Markdown | Lesende Guards behalten, Vollsuite nur bei begründeter Abhängigkeit. |
| Testfixture / Proxy-Routen-Inventar | Verbraucher und nicht im Importgraph sichtbare Zugriffe abdecken. |
| Backend-only, Rename, Delete, ungetracktes Input, Workflow/Runner | Bestehende Selector- und IO-Inventar-Verträge überprüfen. |

Das vorhandene `scripts/test-frontend-changed.test.mjs` ist der Einstieg, nicht
ein neues Parallelmodell. Eine kleinere Auswahl braucht unabhängige Defektproben:
reale, reversible Mutationen an geschütztem Verhalten müssen unter Baseline und
Kandidat rot werden, beim ausgewählten Lauf ebenso wie bei Volltests. Dazu gehören
betroffene Tenant-/Auth-Grenzen, veraltete Async-Antworten, Datums-/Timerverhalten
und der geänderte Selektionspfad. Wähle konkrete Seams aus echtem Code. Bewahre
Mutation, betroffenen Test und Ausgangsfehler als Evidenz; ein absichtlich
fehlschlagendes Mock allein belegt keine Produktionsfehlererkennung.

Mutation-Artefakte bleiben außerhalb des normalen Baums. Jede Mutation einzeln
anwenden und zurücknehmen, ohne fremde Änderungen anzufassen. Ein Mutant, den
schon die Baseline nicht erkennt, misst eine bestehende Qualitätslücke und ist
kein Nachweis gegen oder für den Kandidaten. Weniger redundante Tests sind erst
nach einer Assertion-/Verhaltenszuordnung und unveränderter Defekterkennung
vertretbar. Solche Inventaränderungen separat begründen und den gemeinsamen
Verifier auf beiden Ständen neu validieren. Gleiche Testzahl oder Coverage-%
allein beweist keine gleichbleibende Qualität.

## CI und Journal

CI separat an vergleichbaren PR-Typen und Runner-Klassen auswerten: Queue,
Workflow-Ende-zu-Ende, kritischer Pfad, Setup/Install/Browser/Vitest je Job,
Summe der Runner-Sekunden, Cachezustand, Fehlversuche und Reruns. Bestehende
Workflow-Daten, `scripts/ci-failures.mjs` und `.github/actions/ci-cost/` nutzen.
macOS-Messungen nicht als Linux-CI-Speedup ausgeben. Remote-Dispatch, Push und
kostenpflichtige Experimente brauchen eine dafür bestehende Autorisierung.

Das [Journal](test-performance-hill-climb-journal.md) hält nur Hypothese,
Revisionen, Commands/Artefaktpfade, Streuung, Qualitätsnachweis und Entscheidung.
Rohdaten bleiben lokal; keine Credentials oder vollständigen Env-Inhalte posten.
Nach drei Experimenten Nutzen und Engpass neu bewerten. Bei Plateau eine neue
Engpasshypothese prüfen und dann mit ehrlichem Reststand abschließen, falls kein
weiterer belegter Ansatz übrig bleibt. 4× verfehlt heißt nicht automatisch fertig.
Keine laufenden Dienste nötig. Nach Versuchen eigene Probes/temporäre Checkouts
und Jobs nach Eigentumsprüfung aufräumen; Ergebnisartefakte für den Handoff behalten.
