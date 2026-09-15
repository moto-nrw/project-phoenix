# Anwesenheitsrechte und Elternmeldungen einrichten

Gilt für die Umsetzung von [#3180](https://github.com/moto-nrw/project-phoenix/issues/3180).
Diese Anleitung schaltet keine Schule um. Alle Beispiele sind synthetisch.

## Am Berg nach der Auslieferung

Eine berechtigte Person richtet unter **Einstellungen → Betrieb** Folgendes ein:

1. **Sehen und bearbeiten** öffnen. Bei **Welche Gruppen und Blöcke sieht das Team?**
   die Auswahl **Alle Gruppen und Blöcke** setzen.
2. Bei **Wo darf das Team Kinder an- und abmelden?** **Überall** wählen.
   Falls moto den Sichtwechsel anbietet, beide Änderungen ausdrücklich bestätigen.
3. Bei **Wer darf Kinder krank oder entschuldigt melden?** **Das ganze Team** wählen.
4. **Elternmeldungen** öffnen. **Krankmeldungen durch Eltern** auf
   **Sofort übernehmen** setzen. **Entschuldigungen durch Eltern** auf
   **Erst bestätigen** setzen.
5. **Wer bearbeitet diese Elternanfragen?** entsprechend der Entscheidung der
   Schule wählen. Schulweite Anwesenheitsbearbeitung verlangt keine Teamfreigabe
   für Elternanfragen. Die bisherige Freigabe kann bestehen bleiben.

**Anwesenheit erfassen** bleibt Sache des moto-Teams. Web und NFC dürfen
gleichzeitig eingeschaltet sein. Die neue Gliederung ändert keine
Konfigurationsrechte und führt keine neue IoT-Schreibsperre ein.

Mit einem berechtigten, nicht eingeplanten OGS-Mitarbeiterkonto an einem
synthetischen Kind prüfen: Ein laufender fremder Block erlaubt An- und Abmelden.
Starten und Beenden des Blocks sowie Aufsichtsübernahme erhalten dadurch keine
zusätzliche Freigabe. Eine Elternkrankmeldung setzt keinen tatsächlichen
Ankunftszeitpunkt und legt keinen Raumaufenthalt an.

## Was die Bereiche voneinander trennt

| Bereich | Wirkung | Keine Wirkung auf |
|---|---|---|
| Sichtbereich | Gruppen- und Blockübersicht | Allgemeine Kinderdatensicht, Schreibrechte |
| Anwesenheitsbearbeitung | Zuständigkeitsbereich tatsächlicher An-/Abmeldungen und Wechsel | Aufsicht, Vertretung, Betreuungsplan, historische Sonderrechte |
| Direkte Tagesmeldungen | Krank/entschuldigt erstellen, ändern und zurücknehmen | Andere Statusarten, Freigabe von Elternanfragen |
| Elternmeldungen | Meldung aus, sofort wirksam oder zunächst Anfrage | Tatsächliche Ankunft, Eltern-Kind-Beziehungsrechte |
| Elternfreigabe | Krankmeldungs- und Entschuldigungsanfragen prüfen | Andere Elternanfragen, Enrollment, Personal-Abwesenheitsanträge |

`own` bewahrt die bisherigen Zuständigkeiten und Sonderzugänge, einschließlich
gültiger Vertretungen und vorhandener Zugänge zu offenen Räumen.
`all_staff` ersetzt weder das erforderliche Aktionsrecht noch die Prüfung auf
OGS-Mitarbeit und dieselbe Schule. Schulweite Bearbeitung benötigt schulweite
Sicht. Beim Einschränken zuerst die Bearbeitung, dann die Sicht ändern.

## Bestehende Schulen und Standardwerte

Es gibt keinen pauschalen Daten-Backfill und keine tenant-spezifische Ausnahme.
Fehlende neue Werte werden über die Registry aufgelöst:

| Schlüssel | Standard | Bedeutung |
|---|---|---|
| `operations.attendance_edit_scope` | `own` | Kein neuer schulweiter Bypass |
| `operations.student_absence_edit_scope` | `all_staff` | Bisherige rollenbasierte Tagesmeldungen bleiben möglich |
| `operations.parent_sick_reports_enabled` | `true` | Bestehenden Hauptschalter nicht zusätzlich einschränken |
| `operations.parent_excused_reports_enabled` | `true` | Bestehenden Hauptschalter nicht zusätzlich einschränken |
| `operations.parent_absence_review_scope` | `inherit` | Bisherige Gruppenleitungsfreigabe übernehmen |

Die beiden Meldearten verwenden weiterhin
`operations.parent_sick_note_enabled` als gemeinsamen Hauptschalter und
`operations.parent_sick_requires_approval` beziehungsweise
`operations.parent_excused_requires_approval` für die Bestätigung.
Die Settings-API projiziert daraus die drei sichtbaren Auswahlwerte.
Sie speichert keinen zweiten, konkurrierenden Modus.

**Aus** setzt nur das betreffende neue Abschaltflag. Die bisherige
Bestätigungswahl bleibt gespeichert. Wird bei ausgeschaltetem Hauptschalter
eine Art eingeschaltet, bleibt die andere aus: Ihre Abschaltung und das
Einschalten des Hauptschalters erfolgen in derselben Transaktion.
Gleichzeitige direkte und zusammengesetzte Schreibzugriffe teilen eine Sperre.

`inherit` wird als effektive Auswahl **Admins** oder
**Admins und zuständige Gruppenleitungen** angezeigt. Lesen materialisiert
keinen Override. Eine ausdrückliche Auswahl gilt nur für Abwesenheitsanfragen.
Der bisherige Gruppenleitungs-Schalter steuert weiterhin die anderen
Anfragearten und die noch geerbte Abwesenheitsfreigabe.

## Zurücksetzen

| Auswahl | Wirkung von „Zurücksetzen“ |
|---|---|
| Sicht | Registry-Standard `all_staff` |
| Anwesenheitsbearbeitung | `own`, ohne Entzug bisheriger Sonderzugänge |
| Direkte Tagesmeldungen | `all_staff` |
| Eine Eltern-Meldeart | Entfernt deren Abschalt- und Bestätigungs-Override; bei eingeschaltetem Hauptschalter gilt „Erst bestätigen“, sonst „Aus“ |
| Elternfreigabe | Entfernt den Override und übernimmt wieder den bisherigen Gruppenleitungs-Schalter |

Reset ist kein „letzte Änderung rückgängig“. Er verändert weder die andere
Meldeart noch deren Bestätigungswahl. Explizite Overrides, auch mit dem
Standardwert, behalten bei den projizierten Auswahlfeldern ihre
Zurücksetzen-Funktion. Der ältere Hauptschalter wird durch Reset nicht geändert.

## Rücknahme einer Auslieferung

Vor einer Versionsrücknahme effektive Werte **und** vorhandene Overrides
festhalten. Audit-Einträge nicht löschen. Ein abgebrochener Settings-Schreibvorgang
muss alle beteiligten Werte gemeinsam zurückrollen.

Die alte Version kennt weder unabhängige Abschaltungen noch den neuen
Freigabebereich. Unterschiedlich eingeschaltete Meldearten oder eine ausdrückliche
neue Freigabe lassen sich daher nicht allgemein verlustfrei auf alte Schlüssel
reduzieren. In solchen Fällen eine kompatible Version beibehalten oder vorwärts
korrigieren; neue Overrides nicht einfach löschen. Auch ein zuvor gewähltes
`admins` für direkte Tagesmeldungen würde eine alte Version nicht beachten.

## Verifikation im Repository

Die vorhandenen synthetischen Seed-Profile bilden zwei Gegenbeispiele ab:

| Profil | Web / NFC | Sicht / Bearbeitung | Direkte Tagesmeldung | Eltern krank / entschuldigt | Elternfreigabe |
|---|---|---|---|---|---|
| Vollbetrieb | beide an | beide schulweit | berechtigtes Team | sofort / Bestätigung | zuständige Gruppenleitungen |
| Manuell | Web an, NFC aus | eigene Zuständigkeiten | Admins | Bestätigung / sofort | Admins |

Die Profilkonfiguration nutzt die öffentlichen Settings-Endpunkte. Ihre
Reihenfolge funktioniert auch beim erneuten Setzen aus dem jeweils anderen
gültigen Sicht-/Bearbeitungszustand. Der vollständige Seed-Lauf bleibt eine
separate Integrationsprüfung, zusätzlich zu den HTTP-Vertragstests.

Die Settings-Tests unter `backend/services/config/` prüfen die alten
Haupt-/Bestätigungskombinationen, unabhängige Änderungen, Reset, konkurrierende
Zugriffe und Transaktionsabbruch. Die Usercontext-Tests prüfen geerbte und
ausdrückliche Freigaben sowie aktuelle und abgelaufene Vertretungen.
Parent-Tests prüfen unabhängige Schreibsperren, Beziehungsrechte und
das Ausbleiben tatsächlicher Anwesenheit. Maßgeblich bleiben die tatsächlich
ausgeführten Prüfungen und Screenshots im Änderungsnachweis.

## Sichtprüfung am 12.09.2026

Lokales Profil **Vollbetrieb**, synthetisches Schul-Admin-Konto. Die Suche
grenzt die aufgenommenen Einstellungsbereiche ein. Desktop: 1440 × 1000 Pixel;
Handy: 390 × 844 Pixel, bei Bedarf bis zum unteren Feld gescrollt.

| Ansicht | Desktop | Handy |
|---|---|---|
| Anwesenheit erfassen (Operator) | [Screenshot](screenshots/3180/settings-operator-desktop.png) | [Übersicht](screenshots/3180/settings-operator-mobile.png), [untere Felder](screenshots/3180/settings-operator-mobile-detail.png) |
| Sehen und bearbeiten | [Screenshot](screenshots/3180/settings-team-desktop.png) | [Übersicht](screenshots/3180/settings-team-mobile.png), [untere Felder](screenshots/3180/settings-team-mobile-detail.png) |
| Elternmeldungen | [Screenshot](screenshots/3180/settings-parents-desktop.png) | [Felder](screenshots/3180/settings-parents-mobile.png), [geöffnete Auswahl](screenshots/3180/settings-parents-mobile-options.png) |

Die Beschreibung trennt Überblick, Anwesenheit, Tagesmeldungen und Elternfreigabe.
Die Voraussetzung für „Überall“ steht direkt am Feld. Die lange Freigabe-Auswahl
wird auf dem Handy gekürzt; beim Öffnen steht die vollständige Auswahl bereit.
Der Zurücksetzen-Knopf bleibt durch Zeilenumbruch erreichbar. Die beiden
Desktopbilder sind auch in den passenden Hilfeschritten eingebunden.

Die Operator-Aufnahmen stammen aus derselben lokalen Schule nach regulärer
Anmeldung mit E-Mail-Code. Web und NFC sind gleichzeitig eingeschaltet.
Der mobile Seitencontainer wurde auf die verfügbare Breite begrenzt:
382 Pixel Seitenbreite bei 382 Pixel verfügbarer Breite, ohne horizontalen
Überlauf. Alle drei Felder und die Rücksetz-Aktion sind lesbar und erreichbar.
Serverseitige Aktionsrechte belegen die Tests, nicht diese Aufnahmen.

Prüfergebnisse dieses Sichtprüfungs-Schritts:

- 60 Tests in `settings-category`, `settings-field`, `select-field` und
  `settings-filter` bestanden; `pnpm run check` ohne Warnungen bestanden.
- Der Produktionsbuild für `generate:guides` war erfolgreich. Der Standardlauf
  scheiterte anschließend am belegten Port 3000. Mit einer vorübergehenden,
  danach entfernten lokalen Konfiguration auf Port 3181 liefen alle vier
  PDF-Erzeugungstests erfolgreich durch.
- Die Seiten 138 bis 141 der erzeugten `features.pdf` wurden als Bilder geprüft:
  Beide Schrittlisten und Hinweise bleiben vollständig auf ihrer Seite;
  der jeweilige Screenshot folgt auf der nächsten Seite. Keine abgeschnittenen
  Texte oder überlappenden Elemente in diesen vier Seiten.

## API-Nachweis der Elternfreigabe

`TestParentAbsenceReviewScopeKeepsReadsAndDecisionsConsistent` verwendet
die produktiven Studenten-Routen mit echten, isolierten Datenbank-Fixtures.
Er prüft folgende Übergänge mit unveränderten Claims auf folgenden Requests:

- `admins` sperrt Mitarbeitende; der bisherige Admin-Zugang bleibt erhalten.
- `group_leaders` sperrt zunächst, gibt nach vorhandener Gruppenzuordnung frei
  und verliert die Freigabe beim Wechsel zurück zu `admins`.
- `all_staff` erlaubt einer berechtigten Person ohne Gruppenzuständigkeit
  Liste, Zähler, vertrauliche Notiz sowie Ablehnung und Bestätigung.
- Die Freigabe erweitert weder Stammdatenanfragen noch deren Zähler.
  Eine gemischte Sammelauswahl wird mit `bulk_approval_ineligible` abgewiesen;
  die Abwesenheitsanfrage bleibt offen und kein Tagesstatus wird geschrieben.
- Bestätigung bleibt bei `student_absence_edit_scope = admins` möglich.
  Elternherkunft, Elternkonto und tatsächlicher Prüfer bleiben gespeichert.
  Weder Ankunft noch Raumaufenthalt entstehen.

Nach dem letzten Testausbau bestanden der gezielte API-Test,
`TestHermeticTestPatterns` und der Linter für `api/students` (0 Meldungen).
Der Architekturcheck blieb bei 793 Composition-Feldern/-Settern und
1624 Legacy-Verstößen. Der unten dokumentierte vollständige Backend-Lauf
enthält auch den ergänzten gemischten Sammelfall.

## Review und Aktionsaufnahmen

Die getrennte Standards-Review fand keine zusätzlichen Codeverstöße.
Die Spec-Review fand eine zu enge Bindung der Aktion **Rest des Tages** an
die Blockzuständigkeit. Diese wurde korrigiert: Krank-/Entschuldigt-Meldungen
und ihre Rücknahme verwenden ihre eigene Freigabe; andere Statusarten und
Lebenszyklusaktionen behalten die bisherige Blockberechtigung.
Die fokussierte Nachprüfung fand keine weitere Abweichung in dieser Korrektur.

Der öffentliche Kalender sperrt am Aufnahmetag (Samstag) neue Betreuungstermine.
Für reproduzierbare Aktionsbilder dienen daher ausdrücklich **Storybook-Fixtures**,
nicht ein vorgetäuschter laufender Schulbetrieb. Die Story
`TimetableRosterRights/SchoolWideAttendance` rendert die echte Komponente mit
`canOperate: false`, `canEditAttendance: true`, `canReportAbsence: true`.
Die Berechtigungen werden getrennt durch Service-/API-Tests nachgewiesen.

Für den Vergleich wurde dieselbe Story im isolierten Ausgangsstand
`8d0a410c96717ac6f01874f2540a90488963aa51` und im geänderten Arbeitsbaum
gerendert. Der Arbeitsbaum wurde dabei nicht umgeschaltet. Die Bilder zeigen
links den Ausgangsstand und rechts die Änderung, mit identischen Daten und
Viewport-Breiten (Desktop 1440, Handy 390 Pixel):

- [Aktionsvergleich Desktop](screenshots/3180/pair-actions-desktop.png)
- [Aktionsvergleich Handy](screenshots/3180/pair-actions-mobile.png)
- [Rest des Tages Desktop](screenshots/3180/rest-of-day-desktop.png)
- [Rest des Tages Handy](screenshots/3180/rest-of-day-mobile.png)

Die Aktionsbeschriftungen und der Dialog sind lesbar, die mobilen Aktionen
brechen ohne Überlauf um. Die Auswahl erklärt sichtbar „Nur dieser Block“
gegenüber „Ab 13:00 Uhr alle Blöcke heute“. Beenden und allgemeines Abwesend
bleiben im dargestellten, nicht zuständigen Mitarbeiterzustand verborgen.

## Umfassende Prüfungen nach der Review-Korrektur

- Backend: `CGO_ENABLED=0 scripts/run-go-toolchain.sh scripts/test-backend.sh`
  erfolgreich, 26.072 Tests und zwei vorgesehene Skips. Der vorherige Fehler
  des Composition-Inventars betraf neun verschobene Zeilenverweise; es wurden
  keine Konstruktoren oder erlaubten Abhängigkeiten hinzugefügt.
- Frontend: vollständiger Lauf mit 1.156 Testdateien und 15.586 Tests erfolgreich.
  Danach bestanden die von der Review-Korrektur betroffenen drei Roster-Dateien
  mit 25 Tests und erneut `pnpm run check`, auch nach Ergänzung der Story.
- Seed: frischer lokaler Reset, vollständiger Seed aller vier Profile und
  `simulate full-day --profile vollbetrieb` erfolgreich. Die Simulation
  erzeugte acht Sitzungen, 90 Anwesenheitseinträge und 84 Check-ins.
- Der separate `TestSeedCoverageRatchet` gegen diese Datenbank bestand
  (149 von 226 Tabellen gefüllt, 77 bestehende Klassifikationen).
  Der Seed erzeugt neben einer sofortigen Krankmeldung nun auch eine
  bestätigungspflichtige Entschuldigung. Die Allowlist wurde nicht erweitert.
- Architekturcheck: unverändert 793 Composition-Felder/-Setter und
  1624 Legacy-Verstöße. Der gezielte Linter für Schedule und Seed meldete
  keine Probleme.
- Abschluss: `go vet ./...` und der vollständige `golangci-lint run --timeout 10m`
  bestanden, letzterer mit 0 Meldungen. Die beiden erweiterten Active-Service-Mocks
  in Scheduler und Usercontext erfüllen das Interface; beide Pakete bestanden erneut.
- Nach der mobilen Operator-Korrektur bestanden alle 15 Tests dieser Seite
  und `pnpm run check` ohne Warnungen. Die fokussierte Nachprüfung fand
  keinen weiteren Fehler.
- `CGO_ENABLED=0 scripts/run-go-toolchain.sh scripts/test-changed.sh origin/development`
  bestand ohne `--fast`, erneut nach der letzten Codeänderung: 144 betroffene
  Go-Pakete sowie 183 Frontend-Testdateien mit 3.780 Tests. Auch der abschließende
  Architekturcheck bestand mit den unveränderten Werten oben.

## Verständlichkeit und Abnahmezuordnung

- Zweck und Grenze stehen direkt an den Feldern: Sicht ist kein Schreibrecht,
  Anwesenheit ändert keine Aufsicht, Elternmeldungen sind kein Check-in.
- Die Voraussetzung für „Überall“ wird erklärt und ausdrücklich bestätigt.
  Die UI schaltet den Sichtbereich nicht still um.
- Tagesmeldungen und Elternfreigabe haben getrennte Beschriftungen und Hinweise.
  Die Eltern-Auswahl nennt Aus, sofortige Übernahme und Bestätigung ausdrücklich.
- Desktop und Handy wurden für alle drei Einstellungsbereiche sowie die
  betroffenen Roster-Aktionen geprüft. Vollständige Auswahltexte sind im
  geöffneten Menü sichtbar; Rücksetzen bleibt erreichbar.

| Abnahmekriterium aus #3180 | Nachweis |
|---|---|
| Drei Bereiche, unveränderte Konfigurationsrechte | Registry-/Schema-Tests, Tenant- und Operator-Aufnahmen oben |
| Rechte-Matrix und getrennte Lebenszyklusrechte | `TestTimetableSchoolWideAttendanceDoesNotGrantLifecycleRights`, `TestTimetableSchoolWideAttendancePreservesAccessBoundaries`, Roster-UI-Tests |
| Admin, bestehende Zuständigkeiten, Portale, Tenant und Aktionsrechte | Timetable-Can-Operate-, Active-Transit- und Usercontext-Policy-Tests; Spec-Review der bestehenden Tagesrouten |
| An-/Abmelden, Wechsel, Rücknahmen und Sammelatomarität | Active-/Tracking-/Transit-API-Tests, `TestActiveService_SchoolWideAttendanceMove`, Timetable-Tests |
| Tagesmeldungen ohne Einplanung und alternative Schreibpfade | `TestDirectAbsenceScope`, `TestDirectAbsenceScopeRestrictsExistingReportsOnNextRequest`, `TestTimetableAbsenceWithoutBlockAssignment`, Status-Day-Bulk-Tests |
| Offene Räume und keine fiktive Zuständigkeit | Bestehende Open-Room-/Transit-Tests und explizite leere Aufsichts-/Einplanungsassertions im Timetable-Test |
| Unabhängige Eltern-Meldearten und Freigaben | `TestParentReportModeLegacyProfilesAndIndependentWrites`, Parent-Service-Tests |
| Beziehung, Liste, Zähler, Notizen, Zuständigkeit und Entscheidung | `TestParentAbsenceReviewScopeKeepsReadsAndDecisionsConsistent`, Guardian-/Parent- und Usercontext-Tests |
| Web-Sperre und unveränderter NFC-Vertrag | Bestehende Web-Guard-/IoT-Tests, WebDisabled-Roster-Story und Seed-Profile; kein Kiosk-Vertrag geändert |
| Gleichzeitige Settings-Änderungen und laufende Clients | `TestAttendanceScopeConcurrentWritersRejectStalePair`, Settings-Cache-Bridge-Tests, Next-Request-Tests |
| Altkombinationen, Reset, Defaults, Fehler und Transaktionen | Config-Tests für Attendance-Scope, Parent-Report-Modes und Parent-Absence-Scope |
| Synthetische Demo-Profile | Vollständiger Seed, Full-Day-Simulation und Seed-Coverage-Ratchet oben |
| Hilfe, verständliche Texte und Screenshots | Hilfeschritte, PDF-Sichtprüfung und Aufnahmen oben |
| Prüfungen und Architektur | Vollständige Testläufe, Lint, Typecheck, Architektur und Changed-Code-Check im Abschlussnachweis |
