# Sperrinventar: Stundenplan, Personal, Mitgliedschaft (#6702)

Stand: Branch `fix/6702-sperrinventar-orderlock`. Dieses Dokument ordnet jede
Sperre der Bereiche `modules/timetable`, `modules/workforce`,
`modules/schoolmembership`, `workflows/shiftplansync`,
`workflows/staffoffboarding` und `workflows/staffpersonmerge` sowie die
gelisteten Fremdstellen einer Klasse von `tenant.OrderLock` zu. Die
maschinenlesbare, geprüfte Fassung ist
[`backend/test/row_lock_admission_test.go`](../../backend/test/row_lock_admission_test.go):
Der Guard findet jede `FOR UPDATE`/`FOR NO KEY UPDATE`/`FOR SHARE`/`FOR KEY SHARE`-,
`LOCK TABLE`- und Advisory-Stelle (auch `tenant.AcquireLock`) und verlangt ein
geprüftes `tenant.OrderLock` davor.

## Reihenfolge

| Klasse | Sperre | Schlüssel |
|---|---|---|
| `LockAccount` | Konto beim Austritt (Identity & Access) | Konto-ID, aufsteigend |
| `LockStudentClassWrites` | Klassen-Schreibgate, nur exklusiv (Jahrgangswechsel, erste Sperre) | Schule |
| `LockPayrollRules` | Lohnregeln der Schule | Schule |
| `LockStaffWorkTime` | Saldo- und Abwesenheits-Gate je Person | Person, Saldo vor Abwesenheit |
| `LockStaffShifts` | Schicht-Gate je Person | Person, aufsteigend |
| `LockRecurrence`, `LockGradeTransitions` | Wiederholungs-, Jahrgangs-Gate | Schule |
| `LockReference` | Kategorien, Planungsspuren, Vorlagen-/Aktivitätszeile, Spontanstart-Gates | ungeordnet |
| `LockScheduleDay` | Tagessperre | Datum, aufsteigend |
| `LockStaffPersonMerge` | Merge-Gate der Schule | Schule |
| `LockStaffRows` | Staff-Zeile `FOR NO KEY UPDATE` | Person, **aufsteigend über die ganze Transaktion** |
| `LockLeaf` | jede andere Zeile, Tabelle, Blatt-Advisory, `SKIP LOCKED` | ungeordnet |

Implizite FK-Key-Shares auf `users.staff_school_memberships` (Zuordnungen,
Schichten, Abwesenheiten, Akteurspalten, Tombstone-Trigger) warten nicht mehr:
Staff-Zeilen werden mit `FOR NO KEY UPDATE` gesperrt, und Staff-Zeilen werden
nur mit der ganzen Schule gelöscht. Eine selbst eingefügte Staff-Zeile gilt
als gehalten (`tenant.HoldInserted`).

## Stellen und Behandlung

### Timetable
| Stelle | Klasse | Aufrufer / Transaktion |
|---|---|---|
| `store.go` `FindCategory`, `FindCategoryByName` | Reference | Vorlage anlegen/ändern/teilen, Aktivität anlegen, Spontanstart; Kategoriepflege |
| `planning_tracks.go` `FindPlanningTrack`, `ReorderPlanningTracks` | Reference | Vorlagenschreiber; Spurpflege |
| `groups.go` `FindGroup` | Reference | `CanModifyActivity` (Aktivitätsrouten), Vorlage ändern (Vorlagenzeile vor Personen) |
| `timetable_data_spontaneous.go` Spontanstart-Gates | Reference | Spontanstart |
| `activity_instances.go` `findActivityInstance`, `lockedActivityInstanceIDs`, `DeleteActivityInstancesBefore` | Leaf | Terminschreiber, Neuplanen/Teilen/Ende, Aufbewahrung |
| `instance_students.go` (2) | Leaf | Absage/Abschluss/Korrektur, Betreuungsende |
| `offboarding.go` Termine/Zuordnungen, Vorlagenbetreuer | Leaf | Austritt nach der Staff-Zeile |
| `pickup_extensions.go`, `week_protocols.go`, `class_arrival_plan_commands.go`, `supervisors.go` (LOCK TABLE), `store.go` `LockStudentEnrollmentsForCareExit` | Leaf | jeweilige Schreiber |
| `store.go` `AcquireTransactionLock` | Primitive | Wiederholungs-/Jahrgangs-Gate, `lockScheduleDays` |

Flüsse mit Vorab-Sperre aller Personen (aufsteigend, vor der ersten Zeile):
Vorlage anlegen (inkl. Personen des Materialisierungsfensters, Materialisierung
im Sicherungspunkt), Vorlage ändern (Vorlagenzeile zuerst), Teilen, Umwandeln
in Serie, Termin anlegen/ändern, Absage und Sammelabsage, Spontanstart,
Kiosk-Sitzungsspiegel, Aktivität anlegen und Betreuerrouten, Materialisierung.

### Workforce, Schichtplan-Sync
| Stelle | Klasse | Aufrufer / Transaktion |
|---|---|---|
| Saldo-, Abwesenheits-Gate | StaffWorkTime | Abwesenheiten, Krankmeldung, Austritt, Monatsabschluss, Arbeitszeit, Presence, Settings |
| Qualifikations-Gate | Leaf | Personalakte nach der Staff-Zeile |
| `payroll_rules.go` | PayrollRules | Regelpflege, Monatsabschluss, Export |
| `payroll_dispatch.go` (3), `documentcleanup.go`, `staffrecord_store.go` (SKIP LOCKED) | Leaf | Versand, Download, Aufräumjobs |
| `absence_store.go` Abwesenheitsart | Leaf | jetzt nach Gates und Staff-Zeile: Krankmeldung, Typwechsel, Umbuchen |
| `offboarding.go` (4), `staffemployment.go`, `staff_bank_data_rekey.go`, `substitution_store.go` (2), `worksession_store.go` (2) | Leaf | Austritt, Personalakte, Rekey-CLI, Vertretung, Arbeitszeit |
| `absence_store.go` `AcquireXactLock`, `AcquireXactLock` | Primitive | die Gate-Funktionen oben |

### School Membership und Fremdstellen
| Stelle | Klasse | Aufrufer / Transaktion |
|---|---|---|
| `store.go` `FindStaff` | StaffRows | `findStaffRow` |
| `staff_person_merge.go` `StaffMergeMemberships` | StaffRows (alle, aufsteigend, `application.StaffPersonMerge`) | Merge, jetzt vor den Personen |
| `store.go` `CreateStaff` (Merge-Gate geteilt) | StaffPersonMerge | Staff anlegen; Identität nimmt das Gate vor Person/Staff |
| `student_enrollment.go` Klassen-Gate exklusiv | StudentClassWrites | Jahrgangswechsel (erste Sperre) |
| `student_enrollment.go` Klassen-Gate geteilt, CTE | — (Leser) | Schülerschreiber, Anwesenheit, Wiedereröffnen, People Directory: Leser des Gates, im Guard als dokumentierte Ausnahme |
| `retired_staff.go` (SKIP LOCKED), `FindTeacher`, `FindExternalCaregiverRecord`, `FindClassListEntry`, Klassen-/Gruppenzuordnungen, `LockGroupAssignments`, `LockChildQuota`, `LockStudentWindow` | Leaf | Application-Methoden lassen jede Sperre direkt vor dem Store-Aufruf zu |
| `peoplepostgres/person.go` `FindByIDForUpdate` | Leaf | Staff anlegen (nach der übernommenen Staff-Zeile) |
| `postgres/staff_person_merge.go` `StaffMergePeople` | Leaf | Merge nach den Staff-Zeilen |
| Presence `LockStaffSupervision` | Leaf | Austritt |
| Presence `lockStaffForSupervision` | StaffWorkTime | Betreuerschreiber: alle Saldo-Gates vor allen Staff-Zeilen |
| `workflows/staffpersonmerge` Gate | StaffPersonMerge | Merge (exklusiv) |
| Austritt `PreviewAccess` | Account | Austritt; geplante Austritte sperren Konten vor ihren Gates; Zugang schließen: Konto vor Person |
| Personal-Import | StaffPersonMerge | Upsert nimmt das Merge-Gate vor der ersten Aktualisierung |
| Personal-Import Aktualisieren/Upsert | StaffRows | jeder Chunk sperrt die Staff-Zeilen aller Treffer aufsteigend vor seiner ersten Zeile (`BatchWriteConfig`) |
| Identitätskette | StaffPersonMerge | Merge-Gate vor Person und Staff-Zeile |

Nächtliche Aufbewahrung: Die Jahrgangswechsel-Aufbewahrung (Jahrgangs-Gate)
läuft nach der Kinder- und vor der Personal-Löschung.

## Bekannte Grenzen

- Geteilte Halter des Klassen-Schreibgates nehmen es an verschiedenen Stellen
  ihres Protokolls (nach Tag oder Aktivitätszeile, vor dem Wiederholungs-Gate)
  und stehen daher nicht im Sperrbuch. Ob ein Jahrgangswechsel unter dem
  exklusiven Gate Tagessperren nimmt (dann Zyklus mit Wiedereröffnen), ist
  offen und als Issue-Kandidat gemeldet.

- Zwei Root-Bindungen unter ADR-0067-Ausnahme (Personalakte `LockStaff`,
  Stammdaten `UpdatePerson`) sperren die Person ohne Zulassung. Beide halten
  die Reihenfolge (Person nach der Staff-Zeile bzw. ohne weitere geordnete
  Sperre); eine Änderung braucht eine neue Freigabe-ADR.
- Sperren anderer Module (Care Plan, Kalender, Presence-Zeilen) liegen außerhalb
  dieses Inventars; ihre FK-Key-Shares auf Staff-Zeilen warten nicht mehr.
