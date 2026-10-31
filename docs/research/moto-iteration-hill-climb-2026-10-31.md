# Schnellere moto-Iteration: Environment und Hill Climb

Stand: 31.10.2026. Untersuchung, keine Änderung an App, Tooling oder Infrastruktur.
Lokale Ausgangsrevision: `6851990948`. Bestandsaufnahme und `doctor` wurden
ausgeführt; neue Performance-Benchmarks und GitHub-Laufzeitauswertungen nicht.

**Entscheidung:** Den bestehenden nativen Dev-Loop zu einem zuverlässig messbaren
Agent-Environment ausbauen. Zuerst Bootstrap und Messbarkeit, dann parallele
Verifikation, anschließend den jeweils nachgewiesenen Engpass optimieren.
Kein neuer Orchestrator, Build-Cache-Dienst oder Maschinenkauf ohne Messbeleg.

Der direkt verwendbare [Startprompt](moto-iteration-hill-climb-prompt.md) setzt
diese Empfehlung in einen Arbeitsauftrag für einen neuen Chat um.
Die [externe Recherche](moto-iteration-external-research.md) enthält Primärquellen
und die Grenzen der Theo-Zuschreibung.

## 1. Was aktuell bekannt ist

Ziel ist mehr korrekt gelandete Arbeit und früheres Produktfeedback bei mehreren
Agents auf einem Rechner. Der Nutzer kennt bereits Parallelisierung, Tests und
Caches als Hebel. Unbekannt ist, welcher Teil des gesamten Loops heute begrenzt.

| Nr. | Status | Befund und Beleg |
| --- | --- | --- |
| K1 | Beobachtet | Lokaler Mac: 16 logische CPUs, 64 GiB RAM; rund 688 GiB verfügbar. Ermittelt mit `sysctl` und `df`; keine Messung von Speicher- oder CPU-Druck. |
| K2 | Beobachtet | `git worktree list --porcelain` registriert 358 Worktrees. Das beweist weder deren Nutzung noch deren belegten Speicher. |
| K3 | Beobachtet | Native App-Prozesse, minimale Startmodi, Worktree-Ports und Bootstrap existieren bereits: [Operations](../agents/operations.md), [.wtconfig](../../.wtconfig), [Bootstrap](../../scripts/bootstrap-dev.sh). |
| K4 | Beobachtet | `~/bin/agent-heavy` besitzt zwei maschinenweit pro Nutzer geteilte Zulassungsslots. Er begrenzt weder CPU noch RAM, garantiert kein FIFO und erfasst keine unwrapped Commands. |
| K5 | Beobachtet | Go-Wrapper teilen stabile Worktree-Run-IDs mit Overlap-Lock, Template-Bootstrap und auf macOS höchstens zwei Package-Worker bei `-parallel 8`: [Run-ID](../../scripts/test-run-id.sh), [Budget](../../scripts/go-package-workers.sh), [Changed-Runner](../../scripts/test-changed.sh). |
| K6 | Beobachtet | Vitest verwendet Threads, maximal vier Worker und drei Testprojekte. VM-Pools und deaktivierte Isolation wurden laut dokumentierten Versuchen wegen RAM und Korrektheit verworfen: [Konfiguration](../../frontend/vitest.config.ts). Das sind historische Messungen, nicht neu reproduziert. |
| K7 | Beobachtet | Fail-closed Frontend-Selektion samt Dateisystem-Guards und Backend-Abhängigkeitsselektion existieren bereits. Auch Pre-Push cached erfolgreiche statische Checks mit Eingabefingerprints: [Frontend-Runner](../../scripts/test-frontend-changed.mjs), [Pre-Push](../../scripts/pre-push.sh), [Kontext](../../.claude/README.md). |
| K8 | Beobachtet | Standard-E2E und Guide-Erzeugung verwenden fest `localhost:3000` und lokale Server-Wiederverwendung. Synthetische Netzwerk-E2E haben dagegen eigene Ports und Lifecycle-Isolation: [E2E](../../frontend/playwright.config.ts), [Guides](../../frontend/playwright.guides.config.ts), [Netzwerk](../../frontend/playwright.network.config.ts). Port-Konflikte wurden hier nicht zur Laufzeit reproduziert. |
| K9 | Beobachtet | [dev-bench.sh](../../scripts/dev-bench.sh) misst Wall/CPU und gesampelten Prozessbaum-RSS. Es löscht für den Typecheck `.tsbuildinfo`, verändert kurz `backend/main.go` und protokolliert fehlgeschlagene Mess-Commands ohne deren Exitcode aus `measure()` weiterzugeben. Ein grüner Skript-Exit allein beweist daher keinen erfolgreichen Benchmark. |
| K10 | Beobachtet | `devbox run doctor` endet mit Exit 1: `frontend/.next/types/routes.d.ts` fehlt. Toolchain, Modulversion, Frontend-Abhängigkeiten, Browser-CLI, Compose und Docker bestehen. [Doctor](../../scripts/check-dev-environment.mjs) und Bootstrap passen grundsätzlich zusammen; warum die generierte Datei hier fehlt, ist nicht untersucht. |
| K11 | Beobachtet | Historische [Performance-Analyse](../perf/test-performance-analysis.md) beschreibt große Cache-/Link-/Fixture-Graph-Kosten. Spätere [Environment-Dokumentation](../development-environment.md) und aktuelle Wrapper enthalten bereits Verbesserungen. Alte Befunde sind keine offenen Aufgabenliste. |
| K12 | Hypothese | Mehr gleichzeitig schreibende Agents erhöhen den Durchsatz nur, solange Verifikation, Review, gemeinsame Dateien und Integration mithalten. Aktuelle Queue-, Review- und Merge-Zeiten wurden nicht erhoben. |

## 2. Empfohlenes Environment

### Bestehende Basis behalten

| Ebene | Konkreter Vorschlag | Nachweis, dass es funktioniert |
| --- | --- | --- |
| Reproduzierbarer Einstieg | Devbox, `wt`, Bootstrap und beide Doctor-Arten weiterverwenden. Im vorgesehenen Benchmark-Worktree Bootstrap ausführen, anschließend Tools und Isolation prüfen. | Frischer Worktree erreicht ohne manuelle Reparatur einen erfolgreichen Bootstrap und Doctor; Ports und Compose-Identität sind unabhängig. |
| Viele Agents, begrenzte schwere Arbeit | Schreib-/Recherche-Agents von Test-/Build-Slots trennen. Mit vorhandenen zwei Heavy-Slots starten. 1, 2 und 4 aktive Arbeitsabläufe messen, erst danach Kapazität ändern. | Gesamtzahl korrekt abgeschlossener Aufgaben pro Zeit steigt; Queue-Wartezeit, CPU-Sekunden, RSS und interaktive Latenz bleiben innerhalb vorher vereinbarter Grenzen. |
| Zustandsisolation | Pro Aufgabe Worktree, mutable Build-Ausgaben, Browser-Profil/Login, Report-Verzeichnis und Datenzustand isolieren. Vorhandene Port-Leases und Test-DB-Clones nutzen. | Zwei verschiedene lokale Worktrees führen denselben repräsentativen UI-/API-Check gleichzeitig aus und treffen nachweislich ihren eigenen Code und Zustand. |
| Caches | Go-Build-/Modul-Cache und pnpm-Store wiederverwenden; `.next`, TS-Inkrementaldateien und Testartefakte pro Worktree belassen. Cache-Schlüssel und Invalidierung prüfen. | Unveränderte Wiederholung spart Arbeit; eine relevante Source-, Toolchain-, Env- oder Migration-Änderung führt weiterhin zum notwendigen Check. |
| Kleine Feedback-Stufen | Benannter Test → lokaler Fast-Loop → vorgeschriebene Checks → CI. `agent-browser` für lokales Erkunden, Playwright für reproduzierbare Flows, vorhandene Netzwerk-/Layout-Suites für ihre begrenzten Verträge. | Ein bekannter Fehler wird in der billigsten geeigneten Stufe erkannt. Synthetische Tests werden nicht als Beleg für reale DB-/Auth-Integration ausgegeben. |

**Zuerst lokal bleiben.** Eine separate Linux-Verifikationsmaschine ist eine
spätere Vergleichsoption, falls aktuelle Messungen macOS-Link-/Startkosten oder
lokale Contention als Engpass bestätigen. Dann dasselbe Aufgabenpaket inklusive
Transfer, Queue und Artefaktrückgabe vergleichen. Erst danach Hardware oder
Remote-Cache wählen. Kein konkretes Produkt und keine Kostenersparnis sind hier
bereits nachgewiesen.

Für Browser-Fehler braucht der Agent passende, redigierte Evidenz: sichtbarer
Zustand, Konsole, Backend-Log und bei geeigneten Suites Trace. Die vorhandenen
Netzwerk-E2E schalten sensible Artefakte absichtlich ab. Diese Schutzentscheidung
bleibt erhalten; kein pauschales Einschalten von Traces, Cookies oder Requestlogs.

## 3. Den richtigen Hill Climb bauen

Hier meint Hill Climb einen **verifizierbaren Verbesserungszyklus**, keine
offene Aufforderung, endlos Optimierungen zu erfinden:

`Baseline → Engpass → Hypothese → eine Änderung → gleicher Vergleich → behalten/verwerfen`

Theos [T3-Code-PR #24635](https://github.com/pingdotgg/t3code/pull/24635)
und die Eval-Anleitungen im Originalrepo belegen reale Tool-Fehler als Fälle,
Kontrollen, wiederholte Samples und Baseline-Vergleiche. Das ist die passende
Inspiration. Die genaue Quellenlage steht in der externen Recherche.
Die folgende Ausgestaltung ist unser Vorschlag für moto, kein Theo-Zitat.

### Messvertrag

1. **Arbeitslast festlegen:** ein frischer Worktree, eine Go-Blattänderung,
   eine zentrale Go-Änderung, eine React-/Route-Änderung und ein echter
   Browser-Flow. Historische Fehler eignen sich als feste Aufgaben. Zusätzlich
   einzelne zurückgehaltene Fälle prüfen, damit der Loop nicht nur seinen
   eigenen Benchmark optimiert.
2. **Zwei Betriebsarten trennen:** isolierter Vorher/Nachher-Vergleich und
   kontrollierte Mehragent-Last. In beiden Revision, Dirty-Diff, Toolchain,
   Konfiguration, Testauswahl, Datenzustand und andere Last protokollieren.
   Baseline und Kandidat unterscheiden sich nur durch die getestete Änderung.
3. **Warmth explizit machen:** frischer Worktree mit warmen gemeinsamen Caches,
   unveränderte Wiederholung und Source-Änderung sind verschiedene Fälle.
   Echtes Cold-Verhalten nur mit separaten task-eigenen Cache-Verzeichnissen
   messen, nicht mit globalem Cache-Cleaning. Wiederholungszahl passend zu Kosten
   und Streuung wählen, zunächst mindestens drei gepaarte Läufe für Zeitvergleiche.
4. **Ergebnisse vollständig erfassen:** Exitcode, ausgeführte/übersprungene Tests,
   Cache-Treffer, Wall/CPU-Sekunden, Peak-Prozessbaum-RSS, Queue-Wartezeit,
   Disk-Zuwachs und Ressourcen-Cleanup. Wenige Läufe als Median und Spannweite
   ausweisen, keine belastbaren p95 daraus behaupten. `--simulate-ram-gb` ändert
   nur Budgets und simuliert keinen echten RAM-Druck.
5. **Annahme entscheiden:** vorab relevante Zielgröße und Regressionsgrenzen
   festlegen. Nur Gewinne oberhalb der gemessenen Streuung bei unveränderten
   Verträgen behalten. CPU/RAM gegen Wall-Zeit explizit abwägen. Gescheiterte oder
   unklare Versuche dokumentieren und nur eigene Änderungen zurücknehmen.

Das Benchmark-Skript muss vor automatischer Nutzung Fehler verlässlich melden und
den Ausgangszustand erhalten. Bis dahin einzelne Commands mit gesichertem
Exitstatus im isolierten Benchmark-Worktree messen. Nicht den Main-Checkout oder
eine fremde laufende Session für den Hot-Reload-Probe verändern.

### Erfolg nicht mit PR-Zählen verwechseln

PRs/Woche bleiben ein nützliches Ergebnismaß, aber keine alleinige Zielfunktion.
Zusätzlich Zeit bis erstes korrektes Feedback, Review-ready → Merge,
Review-Nacharbeit und nach Merge entdeckte Fehler betrachten. Deployment und
Nutzerfeedback separat messen, wenn die Daten verfügbar sind. Das entspricht
dem DORA-Prinzip, Durchsatz zusammen mit Instabilität zu betrachten, ohne unsere
PR-Metriken als DORA-Deployment-Metriken auszugeben.
Quelle: [DORA-Metriken](https://dora.dev/guides/dora-metrics/).

Für schnelleres menschliches Feedback liefert jede UI-Änderung zusätzlich einen
reproduzierbaren lokalen Einstieg: passende synthetische Rolle und Daten,
Worktree-URL, kurzer Prüfablauf und passende Screenshots. Gewünschtes Verhalten
und eine konkrete offene Produktfrage gehören dazu. Bei bereits autorisierten
Preview-Umgebungen denselben Ablauf dort nutzen; hier wird keine neue Umgebung
veröffentlicht. Feedback als bestätigte Beobachtung oder offene Entscheidung
zur nächsten Änderung zurückführen. Ein schneller Testlauf allein belegt noch
keinen schnelleren Nutzerfeedback-Zyklus.

Eine kleine Sammlung echter Agent-Aufgaben prüft außerdem Tool-Auswahl,
Fehlererkennung und tatsächlichen Endzustand, nicht bloß erfolgreich gestartete
Commands. Stabile Umgebungen und getrennte Trials sind wichtig, weil gemeinsame
Ressourcenknappheit mehrere Versuche zugleich verfälschen kann.
Quelle: [Anthropic: Agent-Evals](https://www.anthropic.com/research/demystifying-evals-for-ai-agents).

## 4. Unknown unknowns, nach Wirkung priorisiert

1. **Fremden Code erfolgreich testen** (hoch, K3/K8).
   Ein Browser-Kontext isoliert keinen Server. Festes `:3000` plus Reuse kann
   in mehreren Worktrees eine falsche Zuordnung erlauben. Billiger Check:
   zwei Worktrees mit unterscheidbaren Antworten starten, Serveridentität
   prüfen, nur task-eigene Instanzen nutzen. Der konkrete Fehler ist noch
   nicht reproduziert.
2. **Mess- und Bootstrap-Fehler als Agent-Problem behandeln** (hoch, K9/K10).
   Eine fehlende generierte Datei oder ein nur protokollierter Testfehler
   entwertet den Vergleich. Check: frischer Bootstrap sowie absichtlich
   fehlschlagender Benchmark-Command mit erwarteter Fehlerweitergabe.
3. **Die Warteschlange wandert zu dir** (hoch, K4/K12).
   Mehr Agents können mehr offene PRs statt mehr gelandeter Arbeit erzeugen.
   Check: bei einer kleinen PR-Stichprobe aktive Arbeit, Heavy-Slot-Warten,
   CI, Review und Merge-Konflikte getrennt erfassen. Zunächst keine zusätzliche
   dauerhafte Telemetrieplattform bauen.
4. **Speicher sparen kann mehr CPU kosten** (mittel, K5/K6/K11).
   Häufiges Bereinigen nimmt warmen Caches ihren Nutzen; weniger Worker können
   dagegen Gesamtdurchsatz verbessern. Check: wiederholte echte Aufgaben mit
   CPU-Sekunden und Disk-Zuwachs vergleichen, nicht nur Ordnergrößen.
   pnpm-Store-Pruning nicht im normalen Arbeitsloop.
5. **Registrierte Worktrees sind kein aufräumbarer Müll** (mittel, K2/K3).
   Viele können Dependency-Bäume, Watcher, DBs und Netzwerke mitbringen. Check:
   `~/bin/agent-worktrees REPO --output NEW_REPORT` als Bericht nutzen und
   Session-Eigentum, Dirty-/Unpushed-Arbeit und persistente Ressourcen prüfen.
   APFS-Clones und Hardlinks machen aufsummiertes `du` zudem zu keinem sicheren
   Beleg für tatsächlich freigebbaren physischen Speicher. Hier wurde nichts
   gelöscht oder als verwaist eingestuft.

## 5. Extrapolate knowledge

| Ableitung | Grundlage / Sicherheit | Günstiger Gegencheck |
| --- | --- | --- |
| Das nächste Upgrade ist eher ein verlässlicher Nachweis als ein weiteres Tool. | K3–K10; hoch: Werkzeuge existieren, der Startcheck ist tatsächlich rot. | Einen frischen Worktree bis zum eigenen erfolgreichen UI-/API-Nachweis führen. |
| Die optimale Zahl schreibender Agents ist nicht die optimale Zahl gleichzeitiger Volltests. | K4–K6/K12; mittel: aktuelle Gesamtlast fehlt. | 1/2/4 Arbeitsabläufe bei zunächst unverändert zwei Heavy-Slots vergleichen. |
| Ein leicht langsamerer Einzeltest kann mehr Arbeit pro Stunde erlauben. | K4–K6/K11; mittel: historisch belegte Einzeltradeoffs, neue Mehragent-Messung offen. | Gesamt-Completion-Zeit plus Hot-Reload-Latenz bei geringerer interner Parallelität messen. |
| Neue globale Caches könnten weniger bringen als richtige Cache-Eingaben und kleinere Abhängigkeitsgraphen. | K5/K7/K11; mittel. | Miss-Ursachen und Zahl betroffener Packages nach einer zentralen Änderung erheben, nicht alte 146-Package-Zahlen übernehmen. |
| Nach behobener Verifikationsreibung können Review und Integration zum Engpass werden. | K2/K12; spekulativ, keine aktuellen PR-Zeitdaten. | Nach jeder Runde denselben End-to-End-Zeitstrahl prüfen und Arbeit-in-Fortschritt an den neuen Engpass anpassen. |

**Modell:** Der Durchsatz gehört der gesamten Kette, nicht dem schnellsten
Einzelcommand. Gemeinsame wiederverwendbare Daten dürfen warm bleiben; mutabler
Aufgaben- und Serverzustand braucht klare Eigentümer. Mehr Agents helfen, wenn
sie unabhängige Arbeit erledigen und verlässliche Nachweise produzieren.
Der nächste Versuch folgt dem gemessenen Engpass, nicht einem festen Toolplan.

**Überprüfbare Vorhersage:** In der ersten Optimierungsrunde sollte die
Bootstrap-/Serverzuordnung weniger manuelle Eingriffe brauchen. In der nächsten
Lastmessung sollte begrenzte schwere Parallelität entweder den Gesamtdurchsatz
verbessern oder als Hypothese verworfen werden. Ohne Messung kein Gewinnversprechen.

## 6. Reihenfolge und Grenzen

1. Dedizierten Benchmark-Worktree bootstrappen; Doctor und Serverzuordnung prüfen.
2. Fehlerfeste Baseline und kleine repräsentative Aufgabenbank herstellen.
3. Parallel ausführbare Browser-/API-Verifikation nachweisen.
4. Unter 1/2/4 Arbeitsabläufen den größten belegten Zeit-/CPU-/RAM-Engpass verbessern.
5. PR-/Review-/Produktfeedback einbeziehen; Remote-Ausführung erst bei Bedarf vergleichen.

**Geprüft:** aktueller Code und Konfiguration, lokale Hardware/Freiplatz,
registrierte Worktree-Anzahl, Heavy-Gate-Status und -Implementierung, Doctor,
externe Primärquellen. **Nicht geprüft:** aktuelle Suite-Laufzeiten, Cache-Größen,
aktive Worktree-Eigentümer, gleichzeitige E2E-Läufe, GitHub-Queue-/Review-Zeiten
oder tatsächlicher Nutzerfeedback-Zyklus. Hier gestartete App-Server, Container
und Browser-Sessions: keine. Bestehende Ressourcen und warme Caches bleiben erhalten.
