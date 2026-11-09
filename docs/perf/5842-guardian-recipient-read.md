# #5842: Vertrag des Empfänger-Reads

Vertrag vor der Umsetzung, geprüft gegen `development` @ `2ab7182111` und
nach statischer Freigabe umgesetzt.
Nur der wiederholte Link-SELECT in `guardian_recipients_batch.go` entfällt.
Nur Guardian-Empfänger mit gesetzter `GuardianProfileID` übernehmen die
gelieferten IDs. Das Slice wird nur gelesen, nicht verändert.
`FindAppointmentRecipientStudents` bleibt für `appointment_responses.go:86`.

## Aufrufer und Messung

Real-DB-Capture mit identischen Fixtures vor/nach der Produktionsänderung.
Gezählt werden SELECTs im bestehenden Tenant-Rahmen, einschließlich der
unveränderten Aufrufer-Reads, ohne Fixture-Aufbau. N bezeichnet Termine.

| Aufrufer | Szenario | N3 alt → neu | N8 alt → neu |
|---|---|---|---|
| `notifications.go:90`, `notifyGuardians` → `reachableGuardianRecipients` (`notifications.go:247`) | Mail-only, Storno | 36 → 33 | 96 → 88 |
| `notifications.go:555`, `queueGuardianDevices` → `reachableGuardianRecipients` (`notifications.go:247`) | Push-only, Storno | 36 → 33 | 96 → 88 |
| Beide obigen Aufrufer | Mail + Push, Storno | 51 → 45 | 136 → 120 |
| `GuardianNotificationAudiences`, exportiert über `compose/new.go:22` | Ein Batch-Aufruf | 4 → 3 | 4 → 3 |
| Exportierter Aufrufer über Reminder-Command | Bestehendes exaktes Read-Budget | 11 → 10 | 11 → 10 |

Damit entfällt bei allen drei Aufrufern genau ein SELECT je Auflösung,
bei Mail + Push zwei je benachrichtigtem Termin. Kein Latenzversprechen.

## Invarianten

| Fall | Alt | Geplant |
|---|---|---|
| Guardian mit Konto und erlaubten Kindern | Link-SELECT nach Empfänger- und Tenant-ID; Kinder aufsteigend | Dieselben IDs aus `AppointmentRecipient.StudentIDs`; korrelierte Projektion nach Empfänger und Tenant, ebenfalls aufsteigend |
| Mehrere Kinder / Empfänger | Kinder je Empfänger aufsteigend; Empfängerreihenfolge und Duplikate bleiben | Identische Reihenfolge und Duplikatbehandlung, keine neue Deduplizierung |
| Keine Empfänger / nur Staff / Guardian ohne Profil-ID | Kein Kontakt- oder Link-Read, leere Zielmenge | Unverändert |
| Guardian ohne gültiges Konto / ohne Portalzugriff | Keine Zustellung | Unverändert: Kontakt- und Beziehungsprüfung bleiben nach dem Empfänger-Read |
| Fremder Tenant | Expliziter Tenantfilter und RLS schließen fremde Links aus | Korrelierter Tenantfilter, äußerer Tenantfilter und RLS schließen dieselben Links aus |
| Gelöschter Terminlink | Physisch entfernte Zeile fehlt | Fehlt auch in der Projektion; Linktabelle hat weder Soft-Delete- noch Aktivflag |
| Entfernte Beziehung / entzogene Berechtigung | Kontaktprojektion und `guardianPermissionSet` schließen das Kind aus | Dieselben Prüfungen am selben Zeitpunkt; keine Wiederverwendung älterer Berechtigungen |
| Gleichzeitige Linkänderung zwischen den beiden Reads | Zweiter READ-COMMITTED-Snapshot kann neue Linkmenge sehen | E2: gemeinsame Linkmenge aus dem ersten Empfänger-SELECT; beide Reads lagen bereits nach etwaigen Terminsperren |

Der alte Reader sortiert global nach `(recipient_id, student_id)`; nach der
Gruppierung bleibt nur die aufsteigende Kindreihenfolge je Empfänger relevant.
Die Projektion liefert genau diese Reihenfolge. Der eindeutige Schlüssel
`(tenant_id, recipient_id, student_id)` verhindert doppelte Links.

| Fehlerpunkt / Sperrposition | Status, Code und Persistenz alt → geplant |
|---|---|
| Zugriff, Validierung, fehlender Termin, belegter Slot | Vorhandene 4xx samt Codes und Reihenfolge unverändert |
| Empfänger-SELECT scheitert | Vorhandener 500-Pfad unverändert; kein weiterer Kontakt-/Link-Read |
| Kontakt-/Berechtigungsprojektion scheitert | Vorhandener 500-Pfad unverändert; keine Zustellung |
| Nur der bisherige zweite Link-SELECT scheitert | Alt: Abbruch/500, bei Schreibaufrufen Rollback. Neu: Statement existiert nicht; E1 erlaubt den Wegfall dieses zusätzlichen Fehlerpunkts. Fehler im weiterhin nötigen Link-Subquery brechen den ersten SELECT weiterhin ab |
| Terminsperre bei Buchung k | Unverändert: Konferenz-/Buchungsprotokoll, dann einzelnes `FindAppointmentForUpdate`, dann vollständige Verarbeitung von k; keine frühere Sperre auf k+1 |
| Storno, Notice-Storno, Zustellung, endgültiges Löschen scheitern | Unveränderte Fehlerweitergabe und Transaktionsgrenzen; derselbe Rollback, keine neue Teilpersistenz, kein Retry/Replay |
| Termin fehlt / ist storniert oder gelöscht | Unverändertes No-op beim Storno bzw. bisherige Wiederanlage beim Verschieben |
| Storno meldet keinen Übergang | Weiterhin kein Notice-Storno und keine Lifecycle-Benachrichtigung |

Uhrzeit, Kalendertag, Settings, Config, Payloads und Idempotenzschlüssel bleiben
an ihren bisherigen Berechnungsstellen. Keine Änderung an Writes, Savepoints,
Commit, Sperrreihenfolge oder Timeouts. Die Lock-/Storno-/Notice-Bündelung
bleibt außerhalb dieses Pakets, Refs #5842.

## Nachweise

1. `TestGuardianNotificationAudiencesMatchLegacyLinks`: Real-DB-Vergleich beider Linkprojektionen und der vollständigen Audience:
   mehrere Kinder, Reihenfolge, leere/staff-only Empfänger, getrennte Tenants,
   entfernte Links und Beziehungen, entzogenes Portalrecht und ungültiges Konto.
2. `TestGuardianRecipientReadFailuresRollBackCancellation`: echte SELECT-
   Berechtigungsfehler in Empfänger- und Kontakt-Read, SQLSTATE `42501`,
   unveränderte Abbrüche, keine Zustellung und zurückgerollter Storno.
   Auf altem Produktionscode ebenfalls grün.
3. `TestGuardianRecipientReadNeedsOnlyProjectedLinkColumns`: Spaltengrants
   isolieren den alten zweiten SELECT. Er scheitert weiterhin mit `42501`,
   während die eingebettete Projektion lesbar bleibt. Audience, Mail und Push
   gelingen nach der Änderung; Gegenprobe auf altem Produktionscode rot (E1).
4. `TestGuardianNotificationAudiencesShareRecipientLinkSnapshot`: ein expliziter
   Callback löscht Links nach dem Kontakt-Read. Die aktuelle Auflösung behält
   den Empfänger-Snapshot, die nächste sieht die Löschung (E2).
5. `TestGuardianNotificationAudiencesIgnoreNonGuardianStudentProjection`:
   Staff-Zeilen mit Profil-ID liefern keine Kinder; die übergebenen Slices
   bleiben unverändert. `TestGuardianRecipientReadStatements` schützt alle
   drei Aufrufer vor einem erneuten eigenständigen Link-SELECT.

Die vorhandenen Konferenz-Storno-/Verschiebe- und Benachrichtigungstests
bleiben erhalten. Das exakte Reminder-Read-Budget sinkt ausschließlich
von 11 auf 10; kein neues oder erhöhtes Budget.
