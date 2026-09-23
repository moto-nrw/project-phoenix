# Backend-Zielarchitektur #2580

Offene Tickets, geordnet nach Vergebbarkeit statt nach Chronologie. Erledigtes steht nicht hier:
`gh issue list --search "2580 in:body" --state closed`.

Stand 23.09.2026 · Ratchet 589 · Composition 623 · Policy-Epoche 25 · 96 Regeln mit
`convert it to exact debt` · 52.901 LOC unter `modules/*/legacy`

Summenprobe: 244 + 29 + 4 + 61 + 43 + 28 + 19 + 161 = 589 = `wc -l backend/architecture/legacy.jsonl`.
Geht sie nicht auf, ist eine Zeile hier veraltet.

## Entscheidungen

- [ ] [#3387](https://github.com/moto-nrw/project-phoenix/issues/3387) Legacy-Guardian-Felder auf den Schüler-Endpunkten — blockt #2760
- [ ] [#3417](https://github.com/moto-nrw/project-phoenix/issues/3417) Zyklen-Ziel und Plattform-Dreieck — entschieden in ADR 0032; offen: Kante `organization-tenancy → delivery-platform` lösen, DoD-Satz in #2580, Teilmengen-Check für `cycles`
- [x] [#3414](https://github.com/moto-nrw/project-phoenix/issues/3414) Projection-Owner `operator-dashboard-view` bleibt (ADR 0033)
- [ ] Gehört der Träger-Strang [#2809](https://github.com/moto-nrw/project-phoenix/issues/2809) / [#2821](https://github.com/moto-nrw/project-phoenix/issues/2821) zu #2580?

## Jetzt vergebbar · 244 Keys

- [ ] [#2733](https://github.com/moto-nrw/project-phoenix/issues/2733) services/enrollment, repositories/enrollment — 61 (`database/repositories/enrollment` gelöscht, 7 Keys weg; die 61 `services/enrollment`-Keys fallen über #3558 bis #3565)
- [ ] [#2742](https://github.com/moto-nrw/project-phoenix/issues/2742) services/education, repositories/education, api/groups, api/admin — 63
- [ ] [#2728](https://github.com/moto-nrw/project-phoenix/issues/2728) services/users — 42
- [ ] [#2734](https://github.com/moto-nrw/project-phoenix/issues/2734) api/enrollment — 32
- [ ] [#2738](https://github.com/moto-nrw/project-phoenix/issues/2738) api/common — 23
- [ ] [#2727](https://github.com/moto-nrw/project-phoenix/issues/2727) database/repositories/users — 14
- [ ] [#2729](https://github.com/moto-nrw/project-phoenix/issues/2729) models/users — 9

## Legacy-Nester auflösen · 51.401 LOC, 0 Keys

- [ ] [#3424](https://github.com/moto-nrw/project-phoenix/issues/3424) `modules/timetable/legacy/timetableplanning` — 20.326 LOC (Schnitt S6 Schulkalender erledigt: Zeiträume, Schließtage, Feiertage, Wochenzyklus und Dateframes beim Owner `modules/schoolcalendar`, Budget 24.299 → 23.093, `planexport/legacy` 410 → 359, Legacy-Summe 44.517 → 43.250, 3 Kompatibilitätsregeln weg statt konvertiert, Policy-Epoche 22 → 23, Ausnahme für die 7 Ersatzregeln per ADR 0038; Schnitt S4 Konflikterkennung und Personalplanung erledigt (#3550): Konflikte, Personalpool, Dienstplan-Abdeckung und Personalbedarf beim Owner `modules/timetable`, Budget 23.093 → 20.326, `planexport/legacy` 359 → 337, `supervisiondashboard/legacy` 729 → 722, Legacy-Summe 43.250 → 40.454, 4 Kompatibilitätsregeln weg statt konvertiert, Policy-Epoche 25 → 26, Ausnahme für die 5 Ersatzregeln per ADR 0038; offen: Schnitte S5, S2, S3, S1), blockt #2732
- [x] [#3422](https://github.com/moto-nrw/project-phoenix/issues/3422) `modules/studentpresence/legacy` aufgelöst — 13.969 LOC, Budget-Eintrag gelöscht (Legacy-Summe 51.310 → 49.904), 79 `student-presence`-`adapter`/`domain`-Regeln weg statt konvertiert, 0 Keys (Ratchet unverändert), Komplexitäts-Ratchet −35 Einträge, Policy-Epoche 20 → 21; Ausnahme für die 57 Ersatzregeln per ADR 0036; offen: Passthrough-Budget `modules/studentpresence` 64 → 76
- [ ] [#3413](https://github.com/moto-nrw/project-phoenix/issues/3413) `modules/workforce/legacy/timetracking` — 13.511 LOC
- [x] [#3418](https://github.com/moto-nrw/project-phoenix/issues/3418) `modules/workforce/legacy/shiftplanning` aufgelöst — 5.336 LOC (Legacy-Summe 49.904 → 44.517), 44 Kompatibilitätsregeln weg statt konvertiert, 5 verwaiste Regeln gelöscht (195 → 150), 0 Keys (Ratchet unverändert), Policy-Epoche 21 → 22; Dienstplan-Kommandos in `modules/workforce/internal/planning`, Tagesinformationen bei Timetable, Krankheits-Kaskade und Terminvertretung als Workflow `workflows/shiftplansync`; Ausnahme für die Ersatzregeln per ADR 0037; offen: `models/schedule`-Zeilen der Dienstplan-Reads gehen mit #3424
- [x] [#3427](https://github.com/moto-nrw/project-phoenix/issues/3427) `modules/careplan/legacy` aufgelöst (`carelifecycle` und `careexitview`) — 5.278 LOC, 46 Kompatibilitätsregeln weg, 1 Key weniger (625 → 624); Ausnahme für Operator-Setting und verschobene Suiten per ADR 0035
- [x] [#3420](https://github.com/moto-nrw/project-phoenix/issues/3420) `workflows/parentportal/legacy` aufgelöst — 5.881 LOC, 46 Kompatibilitätsregeln weg, Schreibpfade als Owner-Commands (Care Plan, People Directory, Audit Platform); offen: Announcement-Quittungen in `messaging` (Communication)
- [x] [#3410](https://github.com/moto-nrw/project-phoenix/issues/3410) `modules/careplan/legacy` Wurzelpaket aufgelöst — 874 LOC, 15 Keys weniger (644 → 629)

## Schuld sichtbar machen · 96 Regeln, 29 Keys

- [ ] [#3421](https://github.com/moto-nrw/project-phoenix/issues/3421) `inbound-parent.*` zu exaktem Debt konvertiert — 27 Regeln weg, 29 Keys unter #3421 (568 → 597); offen: Keys abbauen, dann schließen
- [ ] [#3416](https://github.com/moto-nrw/project-phoenix/issues/3416) `issue`-Feld an Policy-Regeln und `rules.stale`
- [ ] [#3423](https://github.com/moto-nrw/project-phoenix/issues/3423) Die acht neuen Quality-Ratchets auf null fahren

## api/students · 61 Keys

- [x] [#3351](https://github.com/moto-nrw/project-phoenix/issues/3351) `modules/careplan/legacy/careschedule` auflösen — 9.561 LOC (nativ in Care Plan, ADR 0030)
- [x] [#3352](https://github.com/moto-nrw/project-phoenix/issues/3352) Status-Tage und Präsenz-Reads → `modules/studentpresence` — die 16 Produktionsdateien hängen seit #3422 am öffentlichen Vertrag (`StatusDays`, `StatusDayOverviews`, `StudentHistory`); `api/students` bindet die Präsenz jetzt über den eigenen Port `StudentPresence` statt der ganzen `Presence`-Komposition, 0 Keys, keine Regel geändert
- [ ] [#3353](https://github.com/moto-nrw/project-phoenix/issues/3353) Offering-Change- und Pickup-Entscheidungen → Owner-Commands
- [ ] [#3354](https://github.com/moto-nrw/project-phoenix/issues/3354) Stammdaten- und Elternantrags-Reviews → Owner-Module
- [ ] [#3356](https://github.com/moto-nrw/project-phoenix/issues/3356) Settings-, Listenexport-, Messaging-, IoT-, Aktivitäts- und Schulstruktur-Kanten
- [ ] [#2731](https://github.com/moto-nrw/project-phoenix/issues/2731) Carrier — 61

## Identity · 4 Keys

- [ ] [#3446](https://github.com/moto-nrw/project-phoenix/issues/3446) Fremde Implementierungs-Imports aus den Identity-Behavior-Suiten entfernen — 4

- [ ] [#3487](https://github.com/moto-nrw/project-phoenix/issues/3487) `legacy/jwt` aus dem `legacy/`-Pfad umbenennen — entschieden in ADR 0031: das Paket bleibt der Session-Adapter, der Umzug läuft zuletzt nach den Carriern und blockt nichts
- [x] [#3230](https://github.com/moto-nrw/project-phoenix/issues/3230) `api/auth` → `modules/identityaccess/inbound/auth` — Relocation (Epoche 19), 3 Keys weniger (684 → 681), 26 bleiben unter dem neuen Pfad bei #2736
- [x] [#3231](https://github.com/moto-nrw/project-phoenix/issues/3231) Identity-Handler aus `api/operator` → `modules/identityaccess/inbound/operator`, 1 Key weniger (681 → 680). Router, Fremd-Komposition und die Handler-Tests mit `jwt` oder `models/platform` bleiben in `api/operator`: nur `inbound-operator` darf die Owner-Routen mounten, PR-Modus lässt keine neue Erlaubnis zu. Paketlöschung bei #2736 nach #2725
- [x] [#2736](https://github.com/moto-nrw/project-phoenix/issues/2736) Carrier geschlossen — alle 37 Keys und alle Regeln mit #2736 weg (624 → 586): Account-Routen nach `modules/identityaccess/inbound/account` (`identity-access`/`http`), Settings Platform mit Public-Vertrag `modules/settings` und `modules/settings/compose`, `api/operator` nur noch Router; offen: Weitergabe des Recovery-Proof-Headers der Operator-Refresh-Route ohne Test
- [x] [#3501](https://github.com/moto-nrw/project-phoenix/issues/3501) `modules/identityaccess/legacy/usercontext` in den Caller-Context von Identity & Access aufgelöst — alle 55 Keys von #2725 weg (680 → 625), `/api/me` in `modules/identityaccess/inbound/me`; offen: Timetable-Read der geplanten Aufsichten, Personen-Read und -Schreiben über People Directory

## Storage

- [ ] [#2759](https://github.com/moto-nrw/project-phoenix/issues/2759) Cutover `users.students` — DB-Hälfte durch PR #3384, offen ist der Caller-Switch
- [ ] [#3432](https://github.com/moto-nrw/project-phoenix/issues/3432) Drei Projection-Grants von `users.students` umhängen — blockt #2760
- [ ] [#2760](https://github.com/moto-nrw/project-phoenix/issues/2760) Contract `users.students` — blockiert durch #2759, #3387, #3432
- [x] [#2755](https://github.com/moto-nrw/project-phoenix/issues/2755) Backfill `users.students_guardians` — Migration 1.15.413 und `backfill guardian-owner`; Checkpoint und Verifikation (Counts, Checksummen, Account-Bindung, RLS) liegen für #2756 bereit, `users.students_guardians` bleibt autoritativ
- [x] [#2756](https://github.com/moto-nrw/project-phoenix/issues/2756) Cutover `users.students_guardians` — Migration 1.15.417 und Caller-Switch in einem Release: People Directory schreibt die Beziehung und führt die Arbeitseinheit, Care Plan die Abholberechtigung, Identity & Access den Portalzugang; gelesen wird über die Projektion `guardian-link-view`. Die alte Tabelle bleibt als trigger-gepflegter Rollback-Spiegel mit Zähler für #2757
- [ ] [#2757](https://github.com/moto-nrw/project-phoenix/issues/2757) Contract `users.students_guardians`
- [x] [#2753](https://github.com/moto-nrw/project-phoenix/issues/2753) Cutover `users.staff` — Migration 1.15.409 und Caller-Switch in einem Release: School Membership schreibt die Mitgliedschaft, Workforce das Beschäftigungsprofil (`StaffEmployments`); Kompatibilitäts-View, Archiv und Zähler bleiben für #2754
- [ ] [#2754](https://github.com/moto-nrw/project-phoenix/issues/2754) Contract `users.staff`
- [x] [#2762](https://github.com/moto-nrw/project-phoenix/issues/2762) Cutover Activity Instances und Participants — Migration 1.15.415 und Caller-Switch in einem Release: Timetable plant Blöcke und Teilnehmer, Student Presence führt sie aus und erfasst die Anwesenheit (`active.activity_sessions`, `active.activity_session_attendance`); die alten Spalten bleiben als trigger-gepflegter Rollback-Spiegel mit Zähler für #2763
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

## Endkette · 161 Keys

- [ ] [#2750](https://github.com/moto-nrw/project-phoenix/issues/2750) root api, cmd, main composition — 67
- [ ] [#2748](https://github.com/moto-nrw/project-phoenix/issues/2748) shared test und E2E composition — 41 (Schnitt 1: `internal/testdb` ohne `crypto`, E2E ohne `jwt`, `tenant`, `auth/device`, `models/users`, `models/audit`; Epoche 24 gewährt dem Fixture-Owner per ADR 0039 eigenes Werkzeug, `tenant-runtime/public`, `legacy-shared/domain` und `security-runtime/contract`; die restlichen Keys zeigen auf Pakete, die andere Carrier auflösen: `models/*` #2729/#2742/#2733, `services/users` #2728, `database/repositories/*` #2727, Settings, Root-Composition #2747/#2750, Stundenplan-Zeilen #3424 — keine Regel dafür)
- [ ] [#2743](https://github.com/moto-nrw/project-phoenix/issues/2743) repository Factory — 17
- [ ] [#2747](https://github.com/moto-nrw/project-phoenix/issues/2747) service Factory — 32
- [ ] [#2751](https://github.com/moto-nrw/project-phoenix/issues/2751) Legacy-Composition löschen, leeren Ratchet beweisen — 4
- [ ] [#2745](https://github.com/moto-nrw/project-phoenix/issues/2745) API-Aggregat — 0
- [ ] [#2749](https://github.com/moto-nrw/project-phoenix/issues/2749) Scheduler-Setter, breite Test-Composition — 0
