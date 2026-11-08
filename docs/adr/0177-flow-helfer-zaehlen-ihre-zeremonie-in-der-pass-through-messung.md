---
status: accepted
---

# Flow-Helfer zählen ihre Zeremonie in der Pass-through-Messung

Entscheidung vom 08.11.2026 für Punkt 1 von
[#6178](https://github.com/moto-nrw/project-phoenix/issues/6178). Sie ergänzt
Bedingung 3 von `TestModulePassthroughRatchet` und das gemeinsame Prädikat
`internal/passthrough.IsDelegation` ([ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md),
[ADR 0090](0090-exact-method-evidence-supplements-pass-through-floors.md)). Kein Budget,
keine Untergrenze, keine Transaktion, Fehlerübersetzung oder Berechtigung
ändert sich.

## Kontext

Die öffentlichen Flows in `modules/identityaccess/internal/application`
schrieben dieselbe Zeremonie um ihre Arbeit: `if s == nil { return …,
errXUnavailable }`, `defer func() { err = authenticationError(err) }()` (oder
die Übersetzung im `return`) und `ctx = bindRuntime(s.attach, ctx)`. Auf
development standen 136 `bindRuntime`-Aufrufe im Paket, 56 davon als
`bindRuntime(s.attach, ctx)`.

Ein generischer Helfer kürzt jeden dieser Flows um drei Zeilen. Bedingung 3
zählt aber nur Methoden mit höchstens 14 Rumpfzeilen. 22 Flows mit genau einem
Feldaufruf lagen nur wegen der Zeremonie bei 15 bis 22 Zeilen; gekürzt fielen
mehrere unter die Grenze und würden neu gezählt. Das Budget von
`modules/identityaccess` dürfte dafür steigen, und das ist ausgeschlossen
(ADR 0057 Punkt 5, ADR 0075 Punkt 5).

Zwei Wege standen zur Wahl:

- **(a) Eine neue Untergrenzen-Form „Unit-of-Work-Wrapper über einen
  generischen Helfer“.** Die Untergrenze von ADR 0075 ist ein Protokoll
  gezählter Treffer, kein Freibetrag: Budget gleich Messwert. Eine neue Form
  würde die neu sichtbaren Flows weiter zählen und das Budget heben.
- **(b) Der Scanner sieht durch den Helfer.** Er rechnet die Zeilen zurück,
  die der Helfer übernimmt. Eine Kürzung bleibt messneutral.

## Entscheidung

1. **Variante (b).** (a) erfüllt die Bedingung nicht, dass das Budget nicht
   steigt; (b) misst denselben Rumpf wie vorher und ist einfacher.
2. **Die Helfer.** `runFlow`, `runFlowErr` und `runFlow2`
   (`modules/identityaccess/internal/application/flow_unit.go`) führen eine
   Funktion unter der komponierten Unit of Work aus und übersetzen ihren
   Fehler. Jeder Empfänger baut seine `flowUnit` in `flow()`: Bindung,
   Übersetzer und den Sentinel eines nil-Empfängers. Ein nil-Empfänger gibt
   wie bisher Nullwerte und den unübersetzten Sentinel zurück, ohne die
   Funktion aufzurufen. Die drei Namen sind für diese Helfer reserviert; im
   gescannten Baum gibt es keine andere freie Funktion dieses Namens.
3. **Die Messregel.** Ruft der Rumpf einer Methode einen der drei Helfer als
   freie Funktion auf, rechnet Bedingung 3 einmal `FlowHelperLines = 3` auf
   die Rumpfspanne. Der Helfer übernimmt die nil-Prüfung (drei Zeilen), die
   Bindung und die aufgeschobene Übersetzung (je eine Zeile); sein
   Funktionsliteral kostet zwei Rahmenzeilen. Bedingungen 1, 2 und 4 bleiben
   unverändert: Der Helferaufruf ist kein Feldaufruf, die Feldaufrufe im
   Literal zählen wie bisher. Ein Methodenaufruf `s.runFlow(…)` ist kein
   Flow-Helfer.
4. **Messneutralität ist Pflicht.** Für Flows mit aufgeschobener Übersetzung
   ist die Regel zeilengenau. Flows, die nur im `return` übersetzten, sparen
   eine oder zwei Zeilen weniger; dort kann die Regel einen Treffer
   verdecken. Eine Kürzung darf deshalb den Zählstatus keiner Methode ändern.
   Zwei Methoden bleiben ausgeschrieben, weil sie sonst aus der Zählung
   fielen: `OperatorProvisioning.ListPendingOperatorInvitations` und
   `OperatorProvisioning.RevokeOperatorInvitationByActor`.
5. **Was nicht gekürzt wird.** Flows, die erst innerhalb einer Transaktion
   binden oder innerhalb des Literals übersetzen (Rollen-, Berechtigungs- und
   Gastzugriffs-Flows, die Gast-Einladungen von `AccountLifecycle`), behalten
   ihre Reihenfolge. Neun lange Flows, die selbst eine Transaktion öffnen
   (`RegisterSchoolAccount`, `LinkSchoolAccount`, `AcceptOperatorInvitation`,
   `HasSchoolPortalAccess`, `ValidateSessionTokens`, `AssignPermissionToRole`,
   `AssignRoleToAccount`, `DeleteRole`, `RemoveRoleFromAccount`), bleiben
   ausgeschrieben: Im Literal überschreiten sie `gocognit` 15, und ein
   ausgelagerter Rumpf änderte ihren Schlüssel in
   `transaction_boundaries.json`, das keine Umbenennung zulässt. Ein langer
   Flow ohne eigene Transaktion (`LoginTraeger`) legt seinen Rumpf in die
   gleichnamige kleine Methode. Die fünf Widerrufs-Flows von
   `session_revocation.go` (`DeleteAccountSessionsWithAudit`,
   `ScheduleAccountWideRevoke`, `MarkAccountWideWipeCompleted`,
   `RevokeAllTokensWithReason`, `RevokeTokensByTenantID`) bleiben
   ausgeschrieben, weil andere Flows sie innerhalb einer gespeicherten
   Transaktionsfunktion (`mintUnit`, `ledgerUnit`) aufrufen. Dort fasst die
   Command-Effect-Inventur ([ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md))
   jedes Argument eines eigenen Funktionsparameters zusammen, statt es beim
   Aufrufer zu lesen; über den Parameter `fn` der Helfer bekäme jeder dieser
   Ports die Ziele aller anderen Flows (236 falsche Einträge für
   `ValidateSessionTokens`, `RefreshTokenWithAudit` und
   `AcceptTraegerInvitation`). Flows, die einen rohen Sentinel ohne Übersetzung
   zurückgeben (`LogoutOperator`, `VerifyAccessSession`, `CheckPasswordLogin`,
   `VerifyAccountTenantMembership`), bleiben ebenfalls stehen, weil der Helfer
   jeden Fehler übersetzt.

## Messung

Gemessen auf development `bf6c0a14ca` und dem Stand dieser Änderung:

- `TestModulePassthroughRatchet`: `modules/identityaccess` 208 → 208,
  Gesamt 1403 → 1403.
- `passthrough.Scan` über das ganze Backend: 3674 Methoden vorher und
  nachher, Liste der IDs identisch. Keine Methode kommt hinzu oder fällt weg.
- 77 Flows laufen über die Helfer (47 `runFlow`, 21 `runFlowErr`,
  9 `runFlow2`). `bindRuntime(s.attach, ctx)`: 56 → 18; alle
  `bindRuntime`-Aufrufe im Paket: 136 → 61, drei davon in den Helfern.

Das Register `architecture/passthrough.json` bekommt einen neuen Scope, weil
sich die Quellen des Scanners ändern, und neue Evidenz für acht gezählte
Methoden, deren Rumpf jetzt über einen Helfer läuft. Der Prädikat-Test
`TestPassthroughPredicateFixtures` deckt die Regel ab: elf Zeilen über den
Helfer zählen, zwölf nicht, eine andere freie Funktion und ein Methodenaufruf
gleichen Namens bekommen keinen Aufschlag, und zwei Helferaufrufe zählen
einmal.

## Konsequenzen

- Ein neuer Flow im Paket schreibt `return runFlow(ctx, s.flow(), …)` statt
  der drei Zeilen Zeremonie.
- Wer einen weiteren Helfer dieser Art in einem anderen Owner einführt, braucht
  eine eigene Entscheidung: Die Regel gilt nur für die drei Namen hier.
- Ein Flow, den andere Flows innerhalb einer gespeicherten
  Transaktionsfunktion aufrufen, bleibt ausgeschrieben, bis die
  Command-Effect-Inventur eigene Argumente auch dort beim Aufrufer liest.
  Die Inventur zeigt eine Verletzung als neue Ziele an Ports, die sie nie
  erreichen; solche Einträge werden nicht gelabelt, sondern die Kürzung wird
  zurückgenommen.
- Eine Kürzung, die den Zählstatus einer Methode ändert, ist kein Refactoring
  im Sinn dieser ADR. Der Vergleich der Scan-IDs vor und nach der Änderung
  gehört in den PR.
