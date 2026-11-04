# Externe Evidenz für den moto-Iterationsloop

Stand: 2026-10-31. Recherche, keine umgesetzte Infrastrukturänderung.
Primärquellen tragen die technischen Aussagen. „Folgerung“ bezeichnet einen
Vorschlag für moto, keinen gemessenen Geschwindigkeitsgewinn.

## Theo: Was sich tatsächlich belegen lässt

Theos eigener [T3-Code-PR #24635](https://github.com/pingdotgg/t3code/pull/24635)
vom 25./26. Oktober beschreibt wiederkehrende Tool-Fehler und einen Eval-Runner
mit 88 kleinen synthetischen Threads. Die Agent-Instruktionen und Tool-Schemas
stammen aus dem getesteten Checkout. Ein Simulator liefert Shell-Ergebnisse,
ohne Shell-Befehle auf dem Host auszuführen. CI prüft Typen und Grader, ruft
aber keine kostenpflichtigen Modelle auf. Das sind Angaben des Autors, kein
unabhängiger Nachweis eines bestimmten Produktivitätsgewinns.

Die [Eval-Anleitung im Originalrepo](https://raw.githubusercontent.com/pingdotgg/t3code/main/evals/README.md)
verlangt Fälle aus echten Fehlern, einen Fehlschlag vor dem Fix, Kontrollfälle,
mehrere Samples und einen Vergleich mit demselben Modell. Der
[Tool-Use-Runner](https://raw.githubusercontent.com/pingdotgg/t3code/main/evals/mcp-tool-use/README.md)
unterstützt Baseline-Vergleiche, Stopp bei Regression und Kostenberichte.

**Übertragbares Prinzip:** nicht „Agent soll beliebig lange optimieren“, sondern
echte Reibung in prüfbare Fälle verwandeln, eine Änderung machen, gegen eine
stabile Baseline vergleichen und Regressionen verwerfen. Für deterministische
moto-Testskripte sind normale Tests und Ressourcenmessungen die ersten Grader;
Modell-Evals braucht erst das nichtdeterministische Agent-Verhalten.

Das recherchierte Originalvideo
[How To Give Claude Unlimited Usage](https://www.youtube.com/watch?v=NBCM_ZoWZo8)
war ohne verwertbaren Transkripttext erreichbar. Detaillierte Aussagen daraus
werden hier deshalb nicht als verifiziert dargestellt. Der obige PR ist die
stärkere direkt lesbare Primärquelle. Der folgende moto-Loop ist eine eigene
Anwendung des Prinzips, kein angeblich wörtlicher „Theo-Prompt“.

## Offizielle Verträge und ihre Folgerungen

| Bereich | Verifizierter Vertrag | Folgerung für moto |
|---|---|---|
| Go-Buildcache | Der [Go-Cache](https://go.dev/src/cmd/go/internal/help/helpdoc.go) ist für parallele Go-Aufrufe geeignet; normales Arbeiten verlangt kein regelmäßiges Leeren. | Gemeinsamen warmen Cache erhalten. Cache-Diagnostik statt pauschalem `go clean`. Der gemeinsam nutzbare Cache beweist nicht, dass beliebige Testdaten oder Buildausgaben gemeinsam beschreibbar sind. |
| Go-Tests | [`go test`](https://go.dev/src/cmd/go/internal/test/test.go) cached erfolgreiche Pakettests unter bestimmten Flags und Eingaben. `-count=1` deaktiviert den Testresultatcache. `-parallel` begrenzt Paralleltests innerhalb einer Testbinary; `-p` steuert zusätzlich Pakete. | Die vorhandenen Grenzen zusammen mit mehreren Agent-Prozessen messen. Für Tests gegen externe DB-Zustände Cache-Gültigkeit ausdrücklich prüfen; ein Go-Cache-Hit beweist nicht, dass ein externer Dienst erneut getestet wurde. Nicht wahllos `-count=1` entfernen. |
| pnpm 10 | Die [pnpm-10-Konfiguration](https://pnpm.io/10.x/settings) beschreibt Store und Importmethoden. Store und Installation auf verschiedenen Dateisystemen verhindern Hardlinks. `enableGlobalVirtualStore` ist in dieser Version experimentell und hat ESM-/Hoisting-Grenzen. | Den vorhandenen Store und tatsächliche Importmethode prüfen, bevor neue Cache-Technik eingeführt wird. Ein globaler Virtual Store ist ein begrenztes Experiment, kein notwendiger Standard. Kein gemeinsam beschreibbares `node_modules` per Worktree-Symlink erzwingen. |
| Vitest 4 | Die [v4-Performance-Anleitung](https://v4.vitest.dev/guide/improving-performance) beschreibt Isolation, Worker-Pools und `experimental.fsModuleCache`. Isolation abschalten setzt sauberen gemeinsamen Zustand voraus. [`related`](https://v4.vitest.dev/guide/cli) erfasst statisch erkennbare Imports, nicht beliebig dynamische Importpfade. | Die vorhandenen Threads und Isolationsgrenzen nicht als neue Optimierung verkaufen. Import-/Setup-Kosten messen. Experimentellen Dateisystemcache gezielt prüfen. Den fail-closed Repository-Selektor nicht durch nacktes `related` ersetzen. |
| Playwright | Die [Auth-Anleitung](https://playwright.dev/docs/auth) empfiehlt getrennte Accounts pro Worker, wenn Tests gemeinsamen Serverzustand ändern. Browserkontexte isolieren nicht automatisch die Daten im Backend. | Eindeutige Ports allein reichen nicht. Für parallele moto-Flows auch Schule, Nutzer, DB-Daten, Auth-State und Artefaktpfade isolieren. Vorhandene Isolation wiederverwenden und mit zwei mutierenden Flows nachweisen. |
| GitHub Actions | Die [Cache-Referenz](https://docs.github.com/en/actions/reference/workflows-and-actions/dependency-caching) beschreibt Schlüssel, Restore-Fallbacks und Branch-Sichtbarkeit. [Concurrency](https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency) erlaubt gezieltes Ersetzen überholter Läufe. | Restore-/Uploadzeit gegen eingesparte Arbeit messen. Workflow und Branch/PR in Concurrency-Gruppen berücksichtigen, damit unabhängige PRs sich nicht gegenseitig abbrechen. Pflichtchecks nicht wegoptimieren. |

Versionsgrenze: Das lokale Manifest nennt Go 1.27.3, pnpm 10.34.5 und
Vitest `^4.1.12`. Die unversionierte Vitest-Webseite beschreibt inzwischen v5.
Insbesondere deren `doctor`-Befehl und Cache-Defaults sind keine belegten
Fähigkeiten des hier installierten v4-Setups. Maßgeblich bleiben Lockfile,
installierte CLI und die passende Dokumentationsversion.

## Was kein automatischer nächster Schritt ist

Eine [GitHub Merge Queue](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue)
kann veraltete Branch-Rechecks reduzieren. Sie setzt passende Repository-/Plan-
Berechtigungen und `merge_group`-Trigger für erforderliche Actions-Checks voraus.
Das ist eine Option nach Nachweis eines Merge-Engpasses, keine pauschale
Empfehlung, bestehende Review- oder Integrationsregeln zu ersetzen.

Ebenso sind zusätzliche Worker, eine neue Testplattform und Remote-Runner
Hypothesen. Eine kürzere Einzellaufzeit kann mehr Gesamt-CPU, höheren
Speicherdruck und längere Warteschlangen für andere Agents verursachen.
Der richtige Vergleich umfasst deshalb Einzelarbeit **und** parallele Arbeit.

## Mess- und Übergabeform

[Anthropics Eval-Leitfaden](https://www.anthropic.com/research/demystifying-evals-for-ai-agents)
unterscheidet Ergebniszustand, Trace, Grader sowie Capability- und
Regressionsfälle. Wiederholte Versuche helfen, Modellstreuung sichtbar zu machen.
Für moto folgt daraus: Ein Agent-Bericht ist kein Erfolgskriterium. Prüfbar sind
Exitcode, tatsächlich gewählte Tests, erzeugte Ergebnisse, isolierter DB-Zustand
und erhaltene Schutzchecks.

Die [Harness-Erfahrungen für lange Agent-Läufe](https://www.anthropic.com/research/effective-harnesses-for-long-running-agents)
stützen kleine verifizierte Schritte, nutzbare Browserwerkzeuge und persistente
Übergaben. Eine zusätzliche Agent-Plattform folgt daraus nicht. Vorhandene
Start-/Stop-, Test- und Browserbefehle sollten leicht auffindbar und zuverlässig
sein; Logs und Messdaten müssen eine Folgesitzung weiterarbeiten lassen.

**Vorgeschlagener kleinster Loop:** repräsentative Aufgabe festlegen → gültige
Baseline messen → einen nachgewiesenen Engpass ändern → identische Aufgabe und
Schutzchecks wiederholen → nur belegte Verbesserung behalten. Pro Messung
Revision/Dirty-State, Toolversionen, Konfiguration, Cachezustand, Last,
Parallelität, Exitcode, Wall-/CPU-Zeit und Peak-RSS festhalten. Fehlversuche und
ausgelassene Checks separat führen; sie sind keine schnellen erfolgreichen Läufe.
