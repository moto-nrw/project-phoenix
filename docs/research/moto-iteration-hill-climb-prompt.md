# Startprompt: moto Iteration Hill Climb

Diesen Prompt in einen neuen Chat im Repository kopieren. Er autorisiert dort
die Untersuchung und schrittweise Implementierung, nicht pauschal Deployments,
Merge, Datenlöschung oder unbegrenzte externe Ausgaben.

```text
Verbessere den Entwicklungs- und Feedback-Loop für moto / Project Phoenix.
Arbeite als evidenzgetriebener Hill Climb: messen, einen Engpass verbessern,
gegen dieselbe Aufgabe vergleichen, gute Änderungen behalten, dann neu priorisieren.
Ich möchte mehr korrekt gelandete Arbeit und schnelleres Nutzerfeedback,
mehr gleichzeitig nützliche Agents und weniger CPU-, RAM- und Disk-Arbeit pro Ergebnis.
Nicht bloß mehr offene PRs oder schnellere Einzelcommands.

Arbeite autonom an reversiblen Änderungen und führe sie durch Verifikation.
Du darfst unabhängige Untersuchungen und klar abgegrenzte Implementierungen an
Subagents delegieren. Weise ihnen Dateieigentum zu; verhindere konkurrierende
Edits und doppelte schwere Checks. Agent-Anzahl und Test-Worker sind getrennte Budgets.
Nutze vorhandene Autorisierung und die Repository-Regeln für PRs und Reviews.
Merge, Deployment, kostenpflichtige Infrastruktur und persistente Datenlöschung
brauchen entsprechende ausdrückliche Autorisierung, sofern sie noch nicht vorliegt.

1. Ausgangslage verifizieren

Lies AGENTS.md und die darin verlangten lokalen und betroffenen Bereichsregeln.
Nutze $unknown-unknowns und $extrapolate-knowledge bei der Bestandsaufnahme und
erneut, wenn Messungen neue Engpässe zeigen. Trenne Beobachtung und Hypothese.
Lies docs/research/moto-iteration-hill-climb-2026-10-31.md und
docs/research/moto-iteration-external-research.md als Hinweise, nicht als aktuellen
Wahrheitsstand. Prüfe aktuellen Code, installierte Versionen und vorhandene Arbeit.

Es gibt bereits Devbox, wt, native Dev-Modi, Doctor, agent-browser, Playwright,
Go-Test-DB-Templates/Clones, stabile Run-IDs, begrenzte Worker, Affected-Tests,
Pre-Push-Caches und scripts/dev-bench.sh. Erweitere sie statt Parallelsysteme zu bauen.
Prüfe zuerst frischen Bootstrap/Doctor und verlässliche lokale Serveridentität.
Bei der Recherche fehlten generierte Next-Routentypen; Standard-E2E nutzte fest
Port 3000. Prüfe, ob das noch gilt. Historische Performance-Befunde sind teils behoben.

2. Einen belastbaren Mess- und Testplatz herstellen

Nutze einen eigenen verwalteten Worktree, lokale synthetische Daten und die
kleinsten benötigten Dienste. Prüfe Ports, Compose-Identität, Test-DB-Zustand,
Build-Ausgaben, Browser-Login/Profile, Reports und Lifecycle auf Isolation.
Nutze agent-browser aus Devbox für lokale Browserarbeit. Keine Produktions-Requests.
Stelle einen einfachen, dokumentierten Weg vom frischen Worktree zu einem eigenen,
nachweislich richtigen UI-/API-Test her. Zuerst bestehende Commands zusammensetzen;
neue Befehle nur für nachgewiesene wiederkehrende Lücken hinzufügen.

Prüfe das Messwerkzeug selbst: fehlgeschlagene Checks müssen fehlschlagen,
Abbrüche müssen eigene Ressourcen aufräumen und Ausgangsdateien erhalten.
dev-bench.sh protokollierte Fehler ohne sie zuverlässig als Exitstatus weiterzugeben,
löschte Inkrementaldateien und veränderte für Hot Reload kurz eine Source-Datei.
Behebe oder isoliere diese Effekte, bevor du es als automatischen Verifier nutzt.

3. Baseline und Zielfunktion festlegen

Wähle ein kleines festes Aufgabenpaket aus realer Arbeit:
frischer Worktree, Go-Blattänderung, zentrale Go-Änderung, React-/Route-Änderung
und echter Browser-Flow. Verwende zusätzlich zurückgehaltene Kontrollfälle.
Erfasse Zeit bis erstes korrektes Feedback, Zeit bis review-ready, Wartezeit auf
Heavy-Slot/CI/Review, CPU-Sekunden, Peak-Prozessbaum-RSS, Cache-Treffer,
Disk-Zuwachs, Fehler/Flakes und manuelle Eingriffe. Werte PR-Durchsatz zusammen
mit Nacharbeit und nach Merge entdeckten Fehlern aus. Miss Produktfeedback und
Deployment getrennt, soweit lesender Zugriff und vorhandene Daten es erlauben.

Trenne isolierte Vorher/Nachher-Läufe von kontrollierter Mehragent-Last.
Dokumentiere Revision, Dirty-Diff, Toolchain, Konfiguration, Datenzustand,
Testumfang und Cache-Wärme. Vergleiche frischen Worktree mit warmen geteilten
Caches, unveränderte Wiederholung und echte Source-Änderung separat.
Nutze mindestens drei gepaarte Läufe für erste Zeitvergleiche; bei zu großer
Streuung gezielt weiter messen. Berichte Median/Spannweite, keine p95 aus Mini-Samples.
Globale warme Caches bleiben erhalten; Cold-Versuche bekommen eigene Cache-Pfade.

Führe Volltests, Builds und große Analysen über ~/bin/agent-heavy -- COMMAND aus,
falls installiert, mit dem äußersten Toolchain-/Environment-Command innerhalb
des Gates. Reads, kleine Checks und Dev-Server bleiben außerhalb. Teste zunächst
1, 2 und 4 Arbeitsabläufe bei bestehender Heavy-Kapazität, bevor du Limits erhöhst.
Berücksichtige verschachtelte Worker, Language-Server, DB-Pools und Browser.

4. In überprüfbaren Runden verbessern

Wähle je Runde den größten durch Messung belegten Engpass. Recherchiere dafür
gezielt aktuelle Primärquellen mit Websearch; lies die Doku passend zu den
installierten Versionen. Halte Quelle und Datum fest. Theo ist Inspiration für
verifizierbare Ziele und iterative Verbesserung, kein Ersatz für eigene Evidenz.

Formuliere VOR der Änderung: Befund, Hypothese, Zielmetrik, relevante Mindestwirkung,
zulässige Tradeoffs, Korrektheitsvertrag und Abbruch-/Rücknahmekriterium.
Implementiere eine zusammenhängende Änderung. Vergleiche Baseline und Kandidat
unter denselben Bedingungen. Behalte nur belastbare Verbesserungen ohne
Vertragsverlust. Bei Unklarheit messe gezielt nach oder nimm nur eigene Änderungen
zurück. Dokumentiere verworfene Versuche, damit andere Agents sie nicht wiederholen.

Prüfe je nach Evidenz Bootstrap, Server-/Testisolation, Abhängigkeitsgraphen,
Cache-Invalidierung und Wiederverwendung, wiederholte Install-/Analysearbeit,
DB-/Fixture-Kosten, verschachtelte Parallelität, Browser-Feedback sowie
Review-/Integrationsreibung. Cache-Größen allein sind kein Optimierungsziel.
Vergleiche Remote-/Linux-Verifikation erst, wenn ein lokaler Engpass sie rechtfertigt,
einschließlich Übertragung, Wartezeit, Kosten und Artefaktrückgabe.

Erhalte Tenant-Isolation, RLS, Auth, Testverträge, Fixture-/Architekturregeln,
globale Guards, unabhängige Erwartungswerte und vorgeschriebene CI-/Review-Gates.
Kürzere Testauswahl ist nur zulässig, wenn ihre Abdeckung begründet ist und die
erforderlichen Abschlusschecks bestehen. Ein gecachtes Grün braucht korrekte
Invalidierung; ein ausgefallener oder übersprungener Check ist kein Erfolg.
Synthetische UI-Checks ersetzen keine reale API-/DB-Integration.
Nutze redigierte Fehlerartefakte und erhalte bestehende Trace-/Secret-Schutzregeln.

5. Fortschritt vererbbar machen und liefern

Führe ein schlankes Experimentjournal mit Revisionen, Commands, Ergebnissen,
Messgrenzen, Entscheidung und nächstem belegten Engpass. Trenne Rohartefakte von
knapper Entscheidungsnotiz; Tokens und Logs sollen nicht unbegrenzt wachsen.
Neue Erkenntnisse gehören an den passenden bestehenden Einstiegspunkt, nicht
als wiederholte Globalanweisung in alle Agent-Kontexte.

Liefere kleine, unabhängig prüfbare Änderungen mit den vorgeschriebenen Checks.
Für UI-Arbeit liefere einen reproduzierbaren Einstieg mit synthetischer Rolle,
Daten, Worktree-URL, kurzem Prüfablauf und Screenshots sowie eine konkrete
Feedbackfrage. Nutze vorhandene autorisierte Preview-Wege, wenn passend.
Führe erhaltenes Produktfeedback in den nächsten prüfbaren Versuch zurück.
Prüfe bei neuen Edits oder veränderter Basis die betroffenen Nachweise erneut;
wiederhole identische gültige Checks nicht ohne Grund. Frage nur bei materiell
fehlender Entscheidung oder fehlender Autorisierung, nicht nach jeder Runde.
Nach drei Experimenten bewerte erneut End-to-End-Nutzen, verbleibenden Engpass
und Aufwand. Bei weiterem belegtem Nutzen fortfahren; bei einem Plateau zuerst
Messung, Ziel und Engpassannahme prüfen, statt beliebig neue Tools einzuführen.

Beende task-eigene Dienste und entferne disposable Artefakte. Prüfe Eigentum
vor Cleanup; bewahre fremde Sessions, ungesicherte Arbeit, warme Caches und
persistente Daten. Worktree-Berichte sind keine Löschfreigabe.
Berichte knapp: was konkret besser ist, vorher/nachher, welche Checks bestanden,
was unbewiesen bleibt, was noch läuft und welcher nächste Versuch begründet ist.

Beginne jetzt mit aktueller Bestandsaufnahme und einer ausführbaren Baseline.
Bleibe nicht bei einer Empfehlungsliste stehen. Versprich kein dauerhaftes
Weiterlaufen außerhalb der tatsächlichen Laufzeit und liefere bei Unterbrechung
einen überprüfbaren Fortsetzungspunkt.
```
