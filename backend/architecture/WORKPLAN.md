# Backend-Zielarchitektur #2580

Offene Tickets, geordnet nach Vergebbarkeit statt nach Chronologie. Erledigtes steht nicht hier:
`gh issue list --search "2580 in:body" --state closed`.

Stand 21.09.2026 · Ratchet 684 · Composition 641 · Policy-Epoche 18 · 346 Regeln mit
`convert it to exact debt` · 86.664 LOC unter `modules/*/legacy`

Summenprobe: 252 + 61 + 96 + 43 + 28 + 19 + 185 = 684 = `wc -l backend/architecture/legacy.jsonl`.
Geht sie nicht auf, ist eine Zeile hier veraltet.

## Entscheidungen

- [ ] [#3387](https://github.com/moto-nrw/project-phoenix/issues/3387) Legacy-Guardian-Felder auf den Schüler-Endpunkten — blockt #2760
- [ ] [#3417](https://github.com/moto-nrw/project-phoenix/issues/3417) Zyklen-Ziel und Plattform-Dreieck — entschieden in ADR 0032; offen: Kante `organization-tenancy → delivery-platform` lösen, DoD-Satz in #2580, Teilmengen-Check für `cycles`
- [x] [#3414](https://github.com/moto-nrw/project-phoenix/issues/3414) Projection-Owner `operator-dashboard-view` bleibt (ADR 0033)
- [ ] Gehört der Träger-Strang [#2809](https://github.com/moto-nrw/project-phoenix/issues/2809) / [#2821](https://github.com/moto-nrw/project-phoenix/issues/2821) zu #2580?

## Jetzt vergebbar · 252 Keys

- [ ] [#2733](https://github.com/moto-nrw/project-phoenix/issues/2733) services/enrollment, repositories/enrollment — 68
- [ ] [#2742](https://github.com/moto-nrw/project-phoenix/issues/2742) services/education, repositories/education, api/groups, api/admin — 63
- [ ] [#2728](https://github.com/moto-nrw/project-phoenix/issues/2728) services/users — 42
- [ ] [#2734](https://github.com/moto-nrw/project-phoenix/issues/2734) api/enrollment — 32
- [ ] [#2738](https://github.com/moto-nrw/project-phoenix/issues/2738) api/common — 23
- [ ] [#2727](https://github.com/moto-nrw/project-phoenix/issues/2727) database/repositories/users — 15
- [ ] [#2729](https://github.com/moto-nrw/project-phoenix/issues/2729) models/users — 9

## Legacy-Nester auflösen · 86.664 LOC, 0 Keys

- [ ] [#3424](https://github.com/moto-nrw/project-phoenix/issues/3424) `modules/timetable/legacy/timetableplanning` — 23.300 LOC, blockt #2732
- [ ] [#3422](https://github.com/moto-nrw/project-phoenix/issues/3422) `modules/studentpresence/legacy` — 13.969 LOC
- [ ] [#3413](https://github.com/moto-nrw/project-phoenix/issues/3413) `modules/workforce/legacy/timetracking` — 13.511 LOC
- [ ] [#3420](https://github.com/moto-nrw/project-phoenix/issues/3420) `workflows/parentportal/legacy` — 5.924 LOC, blockiert durch #3351
- [ ] [#3418](https://github.com/moto-nrw/project-phoenix/issues/3418) `modules/workforce/legacy/shiftplanning` — 5.336 LOC, blockt #2747, #2750
- [ ] [#3427](https://github.com/moto-nrw/project-phoenix/issues/3427) `modules/careplan/legacy/carelifecycle` — 4.902 LOC, neu durch #3350 am 18.09., teilt `careexitview` mit #3432
- [x] [#3410](https://github.com/moto-nrw/project-phoenix/issues/3410) `modules/careplan/legacy` Wurzelpaket aufgelöst — 874 LOC, 15 Keys weniger (644 → 629)

## Schuld sichtbar machen · 346 Regeln

- [ ] [#3421](https://github.com/moto-nrw/project-phoenix/issues/3421) `inbound-parent.*` zu exaktem Debt konvertieren — 34 Regeln
- [ ] [#3416](https://github.com/moto-nrw/project-phoenix/issues/3416) `issue`-Feld an Policy-Regeln und `rules.stale`
- [ ] [#3423](https://github.com/moto-nrw/project-phoenix/issues/3423) Die acht neuen Quality-Ratchets auf null fahren

## api/students · 61 Keys

- [x] [#3351](https://github.com/moto-nrw/project-phoenix/issues/3351) `modules/careplan/legacy/careschedule` auflösen — 9.561 LOC (nativ in Care Plan, ADR 0030)
- [ ] [#3352](https://github.com/moto-nrw/project-phoenix/issues/3352) Status-Tage und Präsenz-Reads → `modules/studentpresence`
- [ ] [#3353](https://github.com/moto-nrw/project-phoenix/issues/3353) Offering-Change- und Pickup-Entscheidungen → Owner-Commands
- [ ] [#3354](https://github.com/moto-nrw/project-phoenix/issues/3354) Stammdaten- und Elternantrags-Reviews → Owner-Module
- [ ] [#3356](https://github.com/moto-nrw/project-phoenix/issues/3356) Settings-, Listenexport-, Messaging-, IoT-, Aktivitäts- und Schulstruktur-Kanten
- [ ] [#2731](https://github.com/moto-nrw/project-phoenix/issues/2731) Carrier — 61

## Identity · 96 Keys

- [ ] [#3487](https://github.com/moto-nrw/project-phoenix/issues/3487) `legacy/jwt` aus dem `legacy/`-Pfad umbenennen — entschieden in ADR 0031: das Paket bleibt der Session-Adapter, der Umzug läuft zuletzt nach den Carriern und blockt nichts
- [ ] [#3230](https://github.com/moto-nrw/project-phoenix/issues/3230) `api/auth` → `modules/identityaccess/inbound/auth`
- [ ] [#3231](https://github.com/moto-nrw/project-phoenix/issues/3231) Identity-Handler aus `api/operator`
- [ ] [#2736](https://github.com/moto-nrw/project-phoenix/issues/2736) Carrier api/auth, api/operator — 41
- [ ] [#2725](https://github.com/moto-nrw/project-phoenix/issues/2725) Carrier `modules/identityaccess/legacy/usercontext` — 55, aus 46 Kompatibilitätsregeln konvertiert (keine neue Kante). Auflösung wartet auf Owner-Contracts: Staff-/Teacher-Mitgliedschaft (`school-membership`, an #2753 ausrichten), eigene Gruppen und Vertretungen (`school-structure`), beaufsichtigte und aktive Gruppen (`student-presence`). Front-Tickets: #3498, #3499, #3500, danach #3501 (Auflösung)

## Storage

- [ ] [#2759](https://github.com/moto-nrw/project-phoenix/issues/2759) Cutover `users.students` — DB-Hälfte durch PR #3384, offen ist der Caller-Switch
- [ ] [#3432](https://github.com/moto-nrw/project-phoenix/issues/3432) Drei Projection-Grants von `users.students` umhängen — blockt #2760
- [ ] [#2760](https://github.com/moto-nrw/project-phoenix/issues/2760) Contract `users.students` — blockiert durch #2759, #3387, #3432
- [ ] [#2755](https://github.com/moto-nrw/project-phoenix/issues/2755) Backfill `users.students_guardians`
- [ ] [#2756](https://github.com/moto-nrw/project-phoenix/issues/2756) Cutover `users.students_guardians`
- [ ] [#2757](https://github.com/moto-nrw/project-phoenix/issues/2757) Contract `users.students_guardians`
- [ ] [#2753](https://github.com/moto-nrw/project-phoenix/issues/2753) Cutover `users.staff`
- [ ] [#2754](https://github.com/moto-nrw/project-phoenix/issues/2754) Contract `users.staff`
- [ ] [#2762](https://github.com/moto-nrw/project-phoenix/issues/2762) Cutover Activity Instances und Participants
- [ ] [#2763](https://github.com/moto-nrw/project-phoenix/issues/2763) Contract Activity Instances und Participants
- [ ] [#2719](https://github.com/moto-nrw/project-phoenix/issues/2719) Contract Enrollment Request-Child

## Worker und Scheduler · 28 Keys

- [ ] [#2726](https://github.com/moto-nrw/project-phoenix/issues/2726) Worker hinter erneuerbarem DB-Lease
- [ ] [#2746](https://github.com/moto-nrw/project-phoenix/issues/2746) services/scheduler — 28, blockiert durch #2726

## Laufzeit

- [ ] [#3415](https://github.com/moto-nrw/project-phoenix/issues/3415) `validate-ticket` in CI, Evidenzvertrag durchsetzbar machen — blockt #3021
- [ ] [#3411](https://github.com/moto-nrw/project-phoenix/issues/3411) Checkpoint-Workload ausweiten — blockt #3021
- [ ] [#3412](https://github.com/moto-nrw/project-phoenix/issues/3412) Rückzugs-Warteschlange: Paginierung und Query-Budget
- [ ] [#3419](https://github.com/moto-nrw/project-phoenix/issues/3419) IoT-Fehlertexte im Golden, `apiErrors.ts`-Pfad korrigieren
- [ ] [#3021](https://github.com/moto-nrw/project-phoenix/issues/3021) Runtime-Checkpoint 3

## Blockiert · 43 Keys

- [ ] [#2732](https://github.com/moto-nrw/project-phoenix/issues/2732) api/timetable — 43, blockiert durch #3424

## Spur A · 19 Keys

- [ ] [#2706](https://github.com/moto-nrw/project-phoenix/issues/2706) Dokument-Rendering — 19, blockiert durch #2727, #2729, #2731

## Endkette · 185 Keys

- [ ] [#2750](https://github.com/moto-nrw/project-phoenix/issues/2750) root api, cmd, main composition — 68
- [ ] [#2748](https://github.com/moto-nrw/project-phoenix/issues/2748) shared test und E2E composition — 63
- [ ] [#2743](https://github.com/moto-nrw/project-phoenix/issues/2743) repository Factory — 18
- [ ] [#2747](https://github.com/moto-nrw/project-phoenix/issues/2747) service Factory — 32
- [ ] [#2751](https://github.com/moto-nrw/project-phoenix/issues/2751) Legacy-Composition löschen, leeren Ratchet beweisen — 4
- [ ] [#2745](https://github.com/moto-nrw/project-phoenix/issues/2745) API-Aggregat — 0
- [ ] [#2749](https://github.com/moto-nrw/project-phoenix/issues/2749) Scheduler-Setter, breite Test-Composition — 0
