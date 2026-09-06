# Backend-Refactor: Tracker-Bestand und Arbeitsreihenfolge

**Stand:** 04.09.2026, 13:57 Uhr (Europe/Berlin)\
**Quelle:** GitHub Issues und GitHub REST API für `moto-nrw/project-phoenix`.\
**Scope:** Backend-Refactor, Architektur-Ratchet und angrenzende Planungs-Issues. PyrePortal- und andere Cross-Repo-Umstellungen sind nicht Teil der empfohlenen Arbeitswelle.

## Kurzurteil

1. **Die Backend-Wayfinder-Arbeit ist fertig.** [#2557](https://github.com/moto-nrw/project-phoenix/issues/2557) hat 11 native Sub-Issues; alle sind geschlossen. `Not yet specified` ist leer. Die Map ist noch offen, obwohl ihr eigenes Ziel erreicht ist.
2. **[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) ist der maßgebliche Backend-Migrationsplan.** Der native Baum enthält 128 Nachfahren: 41 geschlossen, 87 offen. Davon sind 84 offene Blätter, also die kleinsten Implementierungstickets im Plan, und drei offene Zwischenknoten.
3. **Die GitHub-Fortschrittsanzeige von 75 % ist irreführend.** Sie zählt nur die zwölf direkten Kinder von #2580. Zwei geschlossene Zwischenknoten enthalten zusammen noch 20 offene Kinder: [#2667](https://github.com/moto-nrw/project-phoenix/issues/2667) ist geschlossen, obwohl 5 von 11 Kindern offen sind; [#2679](https://github.com/moto-nrw/project-phoenix/issues/2679) ist geschlossen, obwohl 15 von 17 Kindern offen sind.
4. **Die 84 Tickets sind nicht „die Ratchets“.** Das eigentliche Architektur-Ratchet wurde mit [#2583](https://github.com/moto-nrw/project-phoenix/issues/2583) gebaut und geschlossen. Die meisten offenen Tickets verschieben fachliche Capabilities oder bauen Legacy-Composition ab. [#2751](https://github.com/moto-nrw/project-phoenix/issues/2751) ist der abschließende Null-Nachweis und hat noch 13 offene Blocker.
5. **Jetzt drei parallele PRs starten:** [#2683](https://github.com/moto-nrw/project-phoenix/issues/2683), [#2694](https://github.com/moto-nrw/project-phoenix/issues/2694) und [#2700](https://github.com/moto-nrw/project-phoenix/issues/2700). Ein vierter aktueller Frontier-Task überschneidet sich entweder in denselben Paketen oder führt in den vorerst ausgeschlossenen IoT-/PyrePortal-Ast.

## Unmittelbarer PR-Stand

- [PR #3003](https://github.com/moto-nrw/project-phoenix/pull/3003) setzt #2681 um. Backendtests, Lint und Seed-Smoke sind grün; der verbindliche Architecture-Ratchet-Check ist rot. Der Fehler ist keine bloß veraltete Branch-Baseline: CI meldet 68 Policy-Lockerungen, darunter neue erlaubte Care-Plan-Adapter-Kanten. Diesen PR zuerst korrigieren und mergen, bevor die nächste Backend-Welle beginnt.
- [PR #3000](https://github.com/moto-nrw/project-phoenix/pull/3000) setzt #2975 um. Alle Checks einschließlich Performance-Trace und Render-Counter sind grün. Das ist Tristans Abnahme-Spur, kein eigener Backend-WIP-Slot.

## Gemessener Ratchet-Fortschritt

Das Architektur-Legacy-Manifest startete im Aktivierungscommit vom 28.08. mit 3.663 Einträgen. Auf `development` stehen am 04.09. noch 2.618 Einträge: 1.045 entfernt, also 28,5 Prozent. Der lokale deterministische Check ist grün.

Die übrigen codebasierten Backend-Ratchets sind bereits bei null tolerierter Schuld: Service-Queries, fünf Handler-Layer-Muster, Handler-Komplexität, Model-Ceremony, Data-Layer-Konsolidierung, Helper-Konsolidierung, Calendar-Clock-Ausnahmen und Repository-Methoden ohne Produktionscaller haben leere Allowlists. Die Seeder-Coverage-Liste enthält begründete semantische Ausnahmen, aber `seedCoverageDebt` ist leer.

```bash
scripts/backend-architecture.sh check
# backend architecture ratchet passed: 2618 legacy violation(s) remain
```

## Verifizierter Status der genannten Issues

| Issue | Erstellt | Status | Verifizierter Befund |
|---|---:|---|---|
| [#1179](https://github.com/moto-nrw/project-phoenix/issues/1179) | 31.03.2026 | offen | Altes Architektur-Epic ohne native Sub-Issues oder Dependencies. Zuletzt am 12.07. aktualisiert. Die sechs ausdrücklich verlinkten Issues #575, #584, #585, #586, #1142 und #1144 sind geschlossen, aber fast alle Phasen- und DoD-Checkboxen in #1179 sind offen. |
| [#2557](https://github.com/moto-nrw/project-phoenix/issues/2557) | 23.08.2026 | offen | Backend-Wayfinder-Map. Alle 11 nativen Kinder #2558–#2568 sind geschlossen; `Not yet specified` ist leer. Funktional abgeschlossen. |
| [#2580](https://github.com/moto-nrw/project-phoenix/issues/2580) | 24.08.2026 | offen | Aktuelle Spec und Root des blockers-first Migrations-DAG. 12 direkte Kinder, 116 Enkel, keine tiefere Ebene; 87 Nachfahren sind offen. |
| [#2683](https://github.com/moto-nrw/project-phoenix/issues/2683) | 28.08.2026 | offen | **Ausführbar.** Alle fünf nativen Blocker (#2664–#2668) sind geschlossen. Blockiert elf offene Folgetickets. |
| [#2698](https://github.com/moto-nrw/project-phoenix/issues/2698) | 28.08.2026 | offen | **Blockiert.** Offene Blocker: #2676, #2677, #2691, #2692, #2693. Das Ticket pinnt ausdrücklich PyrePortal-Wire-Verträge und gehört damit nicht in die jetzige Welle. |
| [#2742](https://github.com/moto-nrw/project-phoenix/issues/2742) | 28.08.2026 | offen | **Blockiert.** Offene Blocker: #2683, #2688, #2711. |
| [#2751](https://github.com/moto-nrw/project-phoenix/issues/2751) | 28.08.2026 | offen | **Spätes End-Gate.** 13 offene Blocker: #2719, #2725, #2726, #2743, #2745, #2747–#2750, #2754, #2757, #2760, #2763. |
| [#2973](https://github.com/moto-nrw/project-phoenix/issues/2973) | 03.09.2026 | geschlossen | App-Shell-Requests; geschlossen am 04.09. um 08:39 UTC. |
| [#2974](https://github.com/moto-nrw/project-phoenix/issues/2974) | 03.09.2026 | geschlossen | Client-Bundle; geschlossen am 04.09. um 10:57 UTC. |
| [#2975](https://github.com/moto-nrw/project-phoenix/issues/2975) | 03.09.2026 | offen | Render-Kaskaden; `theitger` zugewiesen, keine offenen Blocker. |
| [#2976](https://github.com/moto-nrw/project-phoenix/issues/2976) | 03.09.2026 | geschlossen | RSC-Prefetches; geschlossen am 04.09. um 09:47 UTC. |

**Folgerung:** Frontend-Performance benötigt aus Yannicks Sicht keine aktive Spur: Nur #2975 ist offen und bereits Tristan zugewiesen. Nach dessen Abschluss bleibt die erneute Messung gegen die Baseline als Abnahme, kein weiterer Refactor-Block.

## Was „Backend-Ratchets“ im Tracker derzeit bedeutet

Die offene Titelsuche `ratchet in:title` liefert nur:

- [#2751](https://github.com/moto-nrw/project-phoenix/issues/2751): Endpunkt der Architektur-Migration, nicht jetzt ausführbar.
- [#2946](https://github.com/moto-nrw/project-phoenix/issues/2946): eigenständige Kalender-Testschuld, erstellt am 02.09., `priority: medium`, unzugewiesen und ohne native Blocker.

Die bereits erledigte Ratchet-Arbeit umfasst unter anderem #2583 sowie die inzwischen geschlossenen Einzel-Ratchets #2840, #2841, #2842, #2846, #2848 und #2851. Deshalb sollten zwei Listen getrennt bleiben:

1. **Ratchet-Hygiene:** kleine, eigenständige Guardrail-Issues wie #2946.
2. **Backend-Zielarchitektur:** der Capability-/Composition-DAG unter #2580, dessen Ratchet die verbleibenden Legacy-Kanten misst.

## Aktuelle Frontier von #2580

Alle acht Tickets sind seit dem 28.08. offen, Yannick zugewiesen und haben **null offene native Blocker**. Das Erstellungsdatum liefert innerhalb dieses Blocks daher keine sinnvolle Reihenfolge; der Entsperr-Effekt und Paketkonflikte tun es.

| Rang | Issue | Offene Folgetickets, die es direkt blockiert | Empfehlung |
|---:|---|---:|---|
| 1 | [#2683](https://github.com/moto-nrw/project-phoenix/issues/2683) – Activities/Timetable | 11 | Jetzt. Größter direkter Entsperr-Effekt; entsperrt unter anderem #2684–#2686 und ist Voraussetzung von #2742. |
| 2 | [#2694](https://github.com/moto-nrw/project-phoenix/issues/2694) – Enrollment Intake | 10 | Jetzt, eigener PR-Owner. |
| 3 | [#2687](https://github.com/moto-nrw/project-phoenix/issues/2687) – Workforce | 8 | Nach #2683 oder mit enger Abstimmung: Beide ändern `services/schedule` und `database/repositories/schedule`. |
| 4 | [#2700](https://github.com/moto-nrw/project-phoenix/issues/2700) – Reminder Delivery | 5 | Jetzt, eigener PR-Owner; öffnet Worker-/Scheduler-Abbau. |
| 5 | [#2676](https://github.com/moto-nrw/project-phoenix/issues/2676) – Device Fleet | 4 | Später gemäß Scope-Entscheidung. Es öffnet den Ast zu #2698 und #2739 und ändert IoT-/Device-Code. |
| 6 | [#2681](https://github.com/moto-nrw/project-phoenix/issues/2681) – Care Requests | 4 | Bereits als PR #3003 begonnen. Zuerst dessen Policy-Lockerungen beheben und mergen; #2694 erst danach starten, weil beide `api/parent` und `services/parent` ändern. |
| 7 | [#2675](https://github.com/moto-nrw/project-phoenix/issues/2675) – Parent Messaging | 2 | Später; teilt Notification-/Users-Flächen mit anderen Frontiers. |
| 8 | [#2674](https://github.com/moto-nrw/project-phoenix/issues/2674) – Staff Messaging | 0 | Niedrigster DAG-Nutzen; später. |

### Praktische PR-Belegung

**Vor Welle A:** PR #3003 reparieren und mergen.

**Welle A, drei parallele Owner:**

1. Yannick: #2683, weil es die breiteste und architektonisch zentralste Kante öffnet.
2. Delegieren: #2694 an einen Owner für Enrollment.
3. Delegieren: #2700 an einen Owner für Delivery/Scheduler.

**Kein vierter Backend-Refactor-PR in derselben Welle.** #2687 kollidiert mit #2683 in den Schedule-Paketen; #2674/#2675 berühren Notifications, die auch #2700 ändert; #2676 führt in den zurückgestellten Device-/IoT-Ast. Ein vierter Slot kann Frontend-Performance-Abnahme oder nicht überlappende Produktarbeit tragen.

**Welle B nach den Merges von #2683 und #2694:** #2687 starten und die dann neu entsperrten Tickets gegen die aktuelle Frontier priorisieren. Keine statische Reihenfolge bis #2751 festschreiben.

## Planungsflächen bereinigen

### #1179 nicht als zweite Roadmap ausführen

**Verifiziert:** #1179 ist fünf Monate älter als #2557, hat keine native Hierarchie und beschreibt noch eine gemeinsame Backend-/Frontend-Zielkette. #2557 und #2580 beschränken ihren Scope dagegen ausdrücklich auf das Go-Backend und enthalten die späteren Architekturentscheidungen samt ausführbarem DAG.

**Folgerung:** #1179 ist eine historische Klammer, kein paralleler Arbeitsstrom. Entweder als durch #2557/#2580 und die Frontend-Seam-Arbeit ersetzt schließen oder den Body auf reine Links zu den heutigen Roadmaps kürzen.

### #2397 vor Umsetzung gegen #2580 abgleichen

[#2397](https://github.com/moto-nrw/project-phoenix/issues/2397) entstand am 18.08., also vor der Backend-Wayfinder-Map. Es ist offen, unzugewiesen, hat keine native Parent-/Dependency-Kante und trägt nur die Textzeile `Part of #1179`. Sein großes Ein-Issue-Scope – Education Groups, Staff PIN, Attendance History, Legal Documents, Delegate-/Model-Aufräumung und mehrere Ratchets – überschneidet sich mit den später erzeugten Capability- und Contract-Tickets unter #2580.

**Folgerung:** #2397 nicht zusätzlich als einen großen PR starten. Zuerst jeden noch gültigen Befund einem #2580-Ticket zuordnen; nur echte Lücken als kleine neue Kinder von #2580 anlegen. Danach #2397 als ersetzt schließen. Das verhindert doppelte Änderungen an denselben Seams.

## Wo Wayfinder noch nötig ist

- **Backend-Zielarchitektur:** kein neuer Wayfinder-Lauf. #2557 hat alle elf Entscheidungen abgeschlossen; #2580 ist bereits die daraus entstandene Spec und Migration.
- **Backend-Frontier:** ebenfalls kein Wayfinder. Hier entscheidet der native Dependency-DAG; die nächste Arbeit ist Implementierung.
- **Frontend-/BFF-Seam:** vor Beginn einmal kurz neu validieren. [#2341](https://github.com/moto-nrw/project-phoenix/issues/2341) wurde am 16.08. erstellt, also vor #2557, und hat bereits zwölf offene native Kinder (#2342–#2353). Es braucht keine neue Ideensammlung, aber einen Abgleich seiner Annahmen mit der inzwischen beschlossenen Backend-Zielarchitektur. Wenn die Entscheidungen halten, ist #2342 die unblocked Frontier; wenn nicht, Wayfinder nur für die strittigen Entscheidungen öffnen.
- **Neuer Frontend-Refactor nach der Seam:** dafür eine eigene Map erst nach dem BFF-Abgleich. Nicht in #1179 oder #2580 mischen.

## Abfragen und Zählweise

Primärquellen waren die Issue-Seiten oben und diese GitHub-API-Abfragen:

```bash
# Native Kinder und deren Status
gh api --paginate repos/moto-nrw/project-phoenix/issues/2580/sub_issues
gh api --paginate repos/moto-nrw/project-phoenix/issues/<child>/sub_issues

# Native Blocker und entsperrte Tickets
gh api --paginate repos/moto-nrw/project-phoenix/issues/<number>/dependencies/blocked_by
gh api --paginate repos/moto-nrw/project-phoenix/issues/<number>/dependencies/blocking

# Wayfinder-Flächen
gh issue list --state all --label wayfinder:map --limit 1000
gh issue list --state all --search \
  'label:wayfinder:research OR label:wayfinder:prototype OR label:wayfinder:grilling OR label:wayfinder:task' \
  --limit 500

# Offene Ratchet-Titel
gh issue list --state open --search 'ratchet in:title' --limit 500
```

Die Fortschrittszahlen zählen **native** Sub-Issues rekursiv. Textverweise wie `Part of #1179` zählen nicht als Parent-Kante. `issue_dependencies_summary.blocked_by` zählt nur offene Blocker; die konkreten Listen stammen aus `dependencies/blocked_by` und wurden nach `state=open` gefiltert.
