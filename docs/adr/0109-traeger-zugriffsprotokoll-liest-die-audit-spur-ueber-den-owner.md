---
status: accepted
---

# Träger-Zugriffsprotokoll liest die Audit-Spur über ihren Owner

Entscheidung vom 25.10.2026 zu [#4655](https://github.com/moto-nrw/project-phoenix/issues/4655),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611)
(User Story 10). Sie setzt
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 7 um und hält fest, was das Zugriffsprotokoll in den Registern
ändert, wie es [ADR 0104](0104-traeger-projektion-bekommt-ihre-komposition-und-ihre-lesegrenze.md)
für jede weitere Fläche verlangt. Sie führt keinen neuen Owner ein.

## Kontext

Das Träger-Büro soll sehen, wer wann welche personenbezogenen Daten an einer
Schule des Trägers gelesen oder geändert hat, gefiltert nach Person, Schule,
Zeitraum und Art, und das als CSV exportieren. Die Lesevorgänge stehen in
`audit.data_access_log`, die Änderungen in den Ledgern der Audit Platform
(`audit.student_field_edits`, `audit.guardian_changes`,
`audit.guardian_financial_change_ledger`, `audit.student_consent_changes`,
`audit.staff_master_data_changes`, `audit.personnel_number_changes`,
`audit.attendance_corrections`, `audit.student_deletions`). Alle gehören der
Audit Platform.

ADR 0084 sieht für jede Fläche einen eigenen `read_projections`-Eintrag in
`traeger-view` vor. Der Architektur-Check lässt einen neuen
`tenant_safe`-Grant im Pull-Request-Modus aber nur für einen neu angelegten
Projektions-Owner zu (`readProjectionLoosenings` in
`backend/internal/architecture/policy_compare.go`); `traeger-view` gibt es
seit #4649. Ein Grant auf die Audit-Tabellen ist deshalb ohne neuen Owner
nicht möglich. Der zweite erlaubte Weg für fremde Daten ist die Abfrage beim
Owner. ADR 0084 nutzt ihn schon für die Schulmenge (Organisation & Tenancy)
und für die Stichtagszahlen (#2791).

## Entscheidung

1. **Lesen über den Owner.** Die Audit Platform bietet die öffentliche
   Fähigkeit `auditlog.AccessTrailQuery`
   (`modules/auditlog/access_trail.go`). Ihre Abfragen liegen im
   Postgres-Paket der Audit Platform und lesen nur deren eigene Tabellen
   (`modules/auditlog/internal/adapters/auditpostgres/access_trail.go`). Sie
   öffnet keine Transaktion, sondern läuft in der Admin-Einheit der
   Projektion. Jeder Zweig jeder Abfrage filtert mit `tenant_id = ANY(?0)`
   auf die Schulmenge, die die Projektion übergibt. Der Ratchet
   `TestAuditAccessTrailQueriesFilterOnTheSchoolSet`
   (`backend/test/traeger_view_scope_ratchet_test.go`) prüft das für die
   Datei, auch für jeden `UNION ALL`-Zweig.
2. **Namen über ihre Owner.** Die Namen der Personen kommen von People
   Directory (`ListPersonsByAccount`, `ListPersonsByID`), und zwar nur für
   das Paar aus Konto und Schule des Eintrags. Zwei Ledger speichern statt
   des Kontos die Staff-ID der handelnden Person: `audit.personnel_number_changes`
   immer, `audit.staff_master_data_changes` außer in den Bereichen
   `bank-steuer`, `dokumente` und `school-classes`. Die Audit-Abfrage liefert
   deshalb Konto-ID oder Staff-ID getrennt; eine gespeicherte 0 (System)
   wird NULL. Staff-IDs löst School Membership (`ListStaff`, auch frühere
   Mitarbeitende) zur Person und damit zum Konto auf; der Filter nach einer
   Person sucht zusätzlich nach den Staff-IDs ihres Kontos an den Schulen
   des Trägers. Für Personen des Träger-Büros ohne Person an der Schule
   kommt die E-Mail-Adresse von Identity & Access (`ListTraegerAccounts`),
   nur für Konten mit einer Träger-Rolle bei diesem Träger. Alles läuft in
   derselben Admin-Einheit.
3. **Ports gehören der Projektion.** `traegerview` deklariert die Ports
   `AccessTrail`, `PersonDirectory`, `StaffDirectory`, `BureauAddresses` und `AccessRecorder` in
   seinem öffentlichen Paket. Die Root bindet sie
   (`backend/api/serve_mounts_traeger.go`). `traeger-view` bekommt keine
   neue Regel in `policy.json` und keinen neuen `read_projections`-Eintrag.
4. **Das Lesen wird selbst protokolliert** (ADR 0084 Abschnitt 7). Nach der
   Admin-Einheit schreibt die Projektion über `AccessRecorder` für jede
   Schule, die die Antwort zeigt, einen Eintrag in `audit.data_access_log`
   (`traeger_access_log` oder `traeger_access_log_export`, Rolle `traeger`,
   Kanal `traeger_portal`, Träger und Zahl der Zeilen in `metadata`). Die
   Root öffnet dafür je Schule eine Tenant-Transaktion mit RLS. Schlägt ein
   Eintrag fehl, gibt es keine Antwort.
5. **Register.**
   - Das Transaktionsregister erhält genau eine administrative Stelle:
     `*Overview.read` in
     `modules/traegerview/internal/application/overview.go`. Sie steht mit
     festem Grund in `backend/test/traeger_view_transaction.go`.
     `Overview.Schools` bleibt unverändert.
   - Das Query-Budget `api.traeger.access_log` (10, exact) steht in
     `backend/test/query_budgets.go`.
   - Fünf Root-Übersetzungen kommen mit genauer Evidenz in
     `backend/architecture/passthrough.json`;
     `traegerportal.Resource.RouterWithAuthRateLimiter` fällt dort heraus,
     weil es jetzt zwei Routengruppen einhängt.
   - Das Compose-Budget von `modules/traegerview/compose` sinkt von 22 auf
     20 Zeilen, die Summe um 2.

## Verworfene Optionen

- **Neuer Projektions-Owner für das Zugriffsprotokoll:** Der Check ließe den
  Grant zu. ADR 0084 macht `traeger-view` aber zum einzigen Ort, der für das
  Träger-Portal über Schulen liest. Ein zweiter Owner würde diese Grenze
  teilen, nur um eine Regel des Prüfers zu umgehen.
- **Lesen je Schule in Tenant-Transaktionen:** RLS wäre die Grenze. Die
  Liste bräuchte aber eine Abfrage je Schule und müsste Seiten über alle
  Schulen selbst zusammensetzen. Das Query-Budget wüchse mit der Zahl der
  Schulen.
- **Nur `audit.data_access_log`:** Das Issue verlangt auch Änderungen.

## Folgen

- Eine neue Art von Lese- oder Änderungseintrag erscheint im Protokoll, wenn
  die Audit Platform sie in `accessTrailEntries` aufnimmt. Eine unbekannte
  Ressourcenart zeigt das Portal als „Sonstige personenbezogene Daten“.
- `audit.data_access_log` hat keinen Index auf `(tenant_id, accessed_at)`.
  Bei großen Trägern kann das Protokoll langsam werden; ein Index ist dann
  eine eigene Migration.
