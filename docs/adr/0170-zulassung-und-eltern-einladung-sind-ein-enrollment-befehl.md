---
status: accepted
---

# Zulassung und Eltern-Einladung sind ein Enrollment-Befehl

Status: Angenommen. Entschieden am 08.11.2026 im umsetzenden PR zu
[#6093](https://github.com/moto-nrw/project-phoenix/issues/6093); Review und
Merge dieses PRs sind die Freigabe. Die Entscheidung wendet
[ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
(Addendum #4538, Ausnahme E1) an. Sie führt keinen Owner, keine Policy-Regel
und keinen Workflow ein.

## Kontext

Lässt die Schule ein Kind zu, dessen Elternteil noch kein Konto hat, schuldet
die Zulassung eine Einladung von Identity & Access. Bisher baute der
HTTP-Handler diese Einheit zusammen:

- `decideChildWithRetry` öffnete die Mandanten-Transaktion, rief
  `Decisions.Decide` und danach über eine Handler-Runtime
  `CreateGuardianInvitation`. Er wiederholte die Transaktion einmal nach einem
  vorübergehenden Datenbankfehler, solange der Entscheidungsteil nicht
  durchgelaufen war.
- Die manuell angelegte, direkt zugelassene Anmeldung reihte dieselbe
  Einladung im Handler noch einmal von Hand ein.

Das Inventar (`docs/architecture/4538-cross-owner-write-inventory.md`, Zeile
E1) führt diese Einladung schon als Teil der Entscheidungseinheit („handler-
created invitation enqueue … in the outer unit"). Ein weiterer Einstieg (CLI,
Job) hätte Einheit, Einladung und Wiederholung erneut nachbauen müssen.

## Entscheidung

1. **Ein teilnehmender Enrollment-Befehl, kein Workflow.**
   `enrollment.DecisionCommand.DecideAndInvite`
   (`modules/enrollment/internal/application/decide_and_invite.go`) führt
   Entscheidung und Einladung in einer Einheit aus. Er tritt der Transaktion
   des Aufrufers bei; ohne eine öffnet er eine eigene über den
   Transaktions-Port der Owner-Runtime.
2. **Die Wiederholung gehört dem, der die Einheit öffnet.** Nur ohne
   umgebende Transaktion wiederholt der Befehl einmal nach einem
   vorübergehenden Fehler, und nur, solange `Decide` nicht durchgelaufen ist.
   Ein Fehler danach, auch beim Commit, kommt zurück, damit keine Zulassung
   doppelt wirkt. Ein beendeter Aufrufer-Kontext beendet die Wiederholung.
   Den Fehlerklassifikator bindet die Wurzel
   (`transactionruntime.IsTransientDatabaseError`).
3. **Identity & Access über einen Consumer-Port.**
   `DecisionGuardianInvitations` ist ein Port der Enrollment-Anwendung; die
   Wurzel bindet ihn an `identityaccess.Owner`. Wie der bestehende
   `DecisionGuardianAccess` nimmt er die Werte des öffentlichen
   Identity-&-Access-Vertrags (`GuardianInvitationRequest`) statt eigener
   Kopien. Die Einladung und ihre Mail treten der
   Einheit bei; scheitert eine, rollt die Zulassung zurück. Die manuelle
   Zulassung ruft denselben Befehl innerhalb ihrer Einreichungs-Transaktion
   und besitzt damit keine Wiederholung.
4. **HTTP ruft nur noch den Befehl.** Die Handler-Runtime
   `GuardianInvitationRuntime`, `decideChildWithRetry` und
   `enqueueDecisionInvite` entfallen. Die Transaktionsgrenze im Register
   `backend/test/transaction_boundaries.json` wandert vom Handler in den
   Befehl (genau geprüfte Ausnahme in
   `backend/test/enrollment_decision_command_transaction.go`).

## Begründung

- Die Bedingungen für Befehlsteilnahme aus ADR 0067 sind erfüllt: Die
  Einladung tritt der Einheit bei, Identity & Access behält Prüfung und
  Schreiben, und es entsteht kein neues Protokoll (keine Sperrreihenfolge,
  kein Nachlesen fremder Daten unter Sperre, keine gemeinsame
  Nach-Commit-Steuerung).
- Ein Workflow, der nur `Decide` und die Einladung nacheinander ruft, wäre
  genau der Wrapper um einen unveränderten Domain-Koordinator, den ADR 0067
  ausschließt. Der eigentliche Schnitt von E1 (Freigabe, bestehendes Kind,
  Abgleich; laut Inventar 1.800–3.000 Zeilen) bleibt ein eigener PR.
- Ziel-Owner und Reihenfolge ändern sich nicht: Identity & Access ist schon
  Ziel von E1 (`GrantGuardianTenantAccess`), und die Einladung folgt wie
  bisher auf `Decide` in derselben Einheit. E1 wird nicht auf einen weiteren
  Owner ausgedehnt.

## Folgen

- Die Command-Effect-Einträge der neuen Bindungen
  (`DecisionCommand.DecideAndInvite`, `ManualEnrollmentDecider.DecideAndInvite`,
  `DecisionService.DecideAndInvite`, `DecisionGuardianInvitations`) übernehmen
  die Einstufung der bestehenden `Decide`-Einträge beziehungsweise E1 und
  nennen diese ADR als `approval_adr`.
- `Decide` bleibt für die Wiederanmeldung (`RolloverDecider`) und die Tests
  unverändert. Wer es direkt ruft und die Einladung will, legt sein
  `PendingInvite` in derselben Transaktion an. Die automatische Zulassung
  der Wiederanmeldung (`rollover_review.go`) legt wie bisher keine an; diese
  ADR ändert das nicht. `DecideAndInvite` liefert kein offenes
  `PendingInvite`.
- Die Wiederholungsregel ist mit Unit-Tests am Befehl belegt
  (`decide_and_invite_internal_test.go`), die Atomarität beider Einstiege mit
  `TestParentInvitationOutboxIsAtomicWithApproval`.
