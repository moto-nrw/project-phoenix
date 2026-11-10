---
status: accepted
---

# Raumbesuchs-Aufbewahrung löscht und protokolliert je Stapel

Entscheidung vom 10.11.2026 zu
[#6455](https://github.com/moto-nrw/project-phoenix/issues/6455). Sie ändert,
wie die Aufbewahrung abgelaufene Raumbesuche löscht, und genehmigt die
umbenannte Bindung von Student Presence. Kein neuer Owner, kein neues
Datenobjekt, keine Policy-Regel, kein neuer Port zwischen Ownern.

## Kontext

`CleanupExpiredVisits` löschte die abgelaufenen Besuche Kind für Kind und
schrieb je Kind eine Zeile in `audit.data_deletions`, jeweils in einer eigenen
Transaktion. Bei 250 Kindern waren das 1256 Anweisungen. Die Frist hängt vom
Kind ab (kürzeste angenommene Einwilligung, sonst die Schuleinstellung); bei
ausgeschaltetem Anwesenheitsprotokoll gilt die Berliner Tagesgrenze. Ein
fehlgeschlagenes Kind durfte die anderen nicht aufhalten, und das Ergebnis
nannte genau die fehlgeschlagenen Kinder.

## Entscheidung

1. **Mengenbasierte Löschung im Owner.** Student Presence ersetzt
   `DeleteCompletedVisitsBefore` (ein Kind) durch `DeleteExpiredVisits`
   (viele Kinder). Die Anwendung bildet je Kind die Grenze: Tagesgrenze bei
   ausgeschaltetem Protokoll (`exit_time` davor), sonst `created_at` älter als
   die Frist des Kindes. Ein `DELETE … USING unnest(…) RETURNING` im
   Postgres-Adapter löscht für alle Kinder auf einmal, mandantengefiltert,
   nur abgeschlossene Besuche, und zählt je Kind die tatsächlich gelöschten
   Zeilen. Doppelte Kinder, fehlende Fristen und eine fehlende Uhr lehnt die
   Anwendung vor jeder Anweisung ab.
2. **Ein Audit-Append je Stapel.** Audit Platform nimmt `DataDeletion` in
   `AppendAll` auf (Leaf nach dem Nachtrag vom 08.11.2026 zu
   [ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)).
   `DataDeletionRepository.CreateAll` schreibt alle Zeilen in einer
   Anweisung, alles oder nichts; ohne `AppendAll` am Command einzeln. Student
   Presence erreicht ihn nur über den eigenen Port `DeletionAudit.CreateAll`,
   den die Wurzel an den Audit-Owner bindet. Die Zeile eines Kindes behält
   Kind, gelöschte Anzahl, Frist oder Tagesgrenze, Grund und Mandant.
3. **Fehlergrenze je Kind bleibt.** Ein Stapel (100 Kinder) läuft in einer
   Transaktion. Scheitert sie, rollt sie ganz zurück, und derselbe Stapel
   läuft Kind für Kind mit je eigener Transaktion erneut. Zählung, Fehlerliste
   und Löschstand sind danach dieselben wie vorher. Ein Stapel aus einem Kind
   läuft nur einmal. Das Abschalten des Protokolls
   (`ApplyAttendanceLogSetting`) bleibt in der Transaktion der Einstellung:
   Jeder Fehler bricht die Änderung ab, wie bisher. Die Transaktionsgrenze
   heißt jetzt `expireVisitsInTx` statt `processStudent`; das Register der
   Transaktionsgrenzen lässt genau diesen Eintrag mit seinem Grund zu
   (`backend/test/visit_retention_transaction.go`).
4. **Bindung.** Die drei unaufgelösten Einträge von `DeleteExpiredVisits` auf
   `Service.observe` sind dieselbe Beobachtung wie bei allen anderen
   Schreibvorgängen von Student Presence (`runWrite`); diese ADR genehmigt
   sie anstelle der Einträge von `DeleteCompletedVisitsBefore`.

## Folgen

- Ein Stapel braucht zwei Anweisungen statt zwei je Kind. Gemessen über den
  ganzen Lauf: 3 Kinder 21 → 11, 12 Kinder 66 → 11, 250 Kinder 1256 → 21.
  Die Budgets `modules.studentpresence.visit_retention.students_3` und
  `…_12` halten 11 exakt.
- Ein dauerhaft fehlerhaftes Kind kostet seinen Stapel einen zweiten Lauf.
- Ein vorübergehender Fehler, der beim Einzellauf verschwindet, erscheint
  nicht im Ergebnis; die Löschung ist dann trotzdem vollständig protokolliert.
