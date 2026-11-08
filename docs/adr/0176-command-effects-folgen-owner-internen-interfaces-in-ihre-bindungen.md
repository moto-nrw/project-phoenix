---
status: accepted
---

# Command Effects folgen owner-internen Interfaces in ihre Bindungen

Entscheidung vom 08.11.2026 für
[#6125](https://github.com/moto-nrw/project-phoenix/issues/6125). Sie ergänzt
[ADR 0098](0098-command-effect-binding-flow-and-platform-protocols.md)
Abschnitt 2 („Own interfaces remain separate ports“). Kein Runtime-Owner,
Befehl, keine Transaktion, Berechtigung, Policy-Regel und kein Budget ändert
sich. Nur `internal/architecture` und das reviewte Register ändern sich.

## Kontext

Der Scanner fasste konkrete Typen des Quell-Owners als dessen eigene
Implementierung zusammen und folgte darin nur Funktionswerten. Zwei Fälle
blieben so unsichtbar:

- Ein Adapter im `compose`-Paket des Owners, etwa
  `identityaccess/compose.tenantRuntime`, bindet `tenant.WithinAdmin`,
  `tenant.WithTenantTx` oder `tenant.WithinTenantRetry` an einen Port des
  Owners. Sein Rumpf galt als eigener Code, nicht als Bindung. Der Port
  `ports.Runtime` hatte deshalb keinen einzigen Eintrag.
- Ein Methodenaufruf auf einem eigenen Interface (`s.runtime.WithAdminTx`,
  `s.tx.RunPlatform`) galt als separater Port und wurde nicht verfolgt. Die
  Einstiegsports (Tenant-MFA-Austausch, `SwitchSchool`, Einladungen,
  Provisioning) zeigten die Transaktion nicht, die sie öffnen.

## Entscheidung

1. **Adapter der Owner-Komposition sind Bindungen.** Ein konkreter Typ aus einem
   Paket des Quell-Owners mit der Rolle `compose` wird wie ein Root-Adapter
   gelesen: Sein Rumpf ist Bindungsevidenz, seine Plattform- und Fremdaufrufe
   sind Ziele. Owner komponieren sich selbst (ADR 0068); was die Root für
   fremde Ports tut, tut `compose` für die eigenen.
2. **Owner-interne Interfaces werden aufgelöst.** Ruft der eigene Code eine
   Methode auf einem benannten Interface des Owners auf, und ist jede Bindung
   dieses Interfaces ein konkreter Typ des Owners selbst, folgt der Scanner
   dem Aufruf in diese Bindungen. Ein `compose`-Adapter wird nach Punkt 1
   gelesen, eine andere eigene Implementierung (etwa die Unit of Work
   `application.CommandTransactions`) weiter nur über ihre Funktionswerte.
3. **Die offene Frage aus #6125:** Ein eigenes Interface mit mindestens einer
   Bindung außerhalb des Owners (Root, anderer Owner) oder einer Bindung durch
   ein weiteres Interface bleibt ein separater Port. Dort bindet jemand anderes
   die Implementierung, und der Port trägt deren Einträge schon selbst. Die
   Bedingung liest die typisierten Bindungen, keinen Namen wie „Runtime“.

Die Präzisionsregeln aus ADR 0098 bleiben: Eigene Argumente an eigene
Parameter werden beim Aufrufer zusammengefasst, eigene Rümpfe ändern keinen
Fingerabdruck. Der Fixture-Test
`TestCommandEffectsFollowOwnInterfacesBoundInsideTheOwner` deckt beide Fälle
und das fremd gebundene Gegenbeispiel ab. Der Test zur Präzision des eigenen
Runners bleibt unverändert grün.

## Register-Delta und Freigabe

Gegen development `95be8866a6` wächst das Register von 11.194 auf 19.703
Einträge: 8.511 neu, 2 entfernt, 194 Fingerabdrücke geändert. Kein Port wächst
um mehr als 36 Einträge.

Entfernt sind zwei falsche Ziele von
`inbound/account.UnitOfWork.WithinAdmin`: `TransactionRunner.RunInTx` und
`tenant.WithinCurrentTenant`. `compose.TenantUnitOfWork.WithinAdmin` ruft nur
`tenant.WithinAdmin`, und dieser Eintrag bleibt. Die 194 geänderten Einträge
(110 scope, 52 query, 32 lock leaf) behalten ihr Label. Ihre Begründung nennt
die neue Evidenz.

Die neuen Einträge wurden nach den Regeln von ADR 0098 Abschnitt 5 eingeordnet;
die Generierung vergibt keine Labels:

| Effekt | Klassifikation | Einträge |
|---|---|---|
| scope | platform | 5.307 |
| query | query | 2.081 |
| mutation / lock | leaf | 274 / 236 |
| unresolved | unresolved | 447 |
| mutation / lock | debt | 60 / 63 |
| mutation / lock / durable | platform | 17 / 18 / 8 |

Neue Plattformziele sind `tenant.WithinTenantRetry`, `WithinAdminRetry`,
`WithAdminTx` und `RequireTransaction`. Alle nehmen einen Callback entgegen
oder prüfen die laufende Unit; sie sind `scope` nach der Tabelle in ADR 0098.

Die 123 neuen Schulden sind sichtbare Koordinationsbefunde, keine Freigaben.
Jedes Ziel war in der Basis schon als Schuld oder als benannte Ausnahme
an einem anderen Port erfasst; neu ist nur der Einstiegsport:

- Enrollment nimmt Care Plans Wochen-Abholsperre und schreibt deren
  Abholänderungen und Entschuldigungen nach (42), dazu
  `LockRecurrenceWrites` (8).
- Identity & Access stellt School-Membership-Personal wieder her (24).
- People Directory nimmt School Membership `StudentClassWriteGate` (35) und
  `LockGroupAssignments` (2), Timetables `LockPlannedSupervisors` (2) und
  Workforces `LockGroupSubstitutions` (2).
- Workforce schreibt School Memberships `UpdateEmploymentDetails` (8).

Die 447 `unresolved`-Einträge betreffen bekannte Grenzen aus ADR 0098:
Beobachter- und Uhr-Callables, die nur `_test.go` oder `testhooks` füttern
(Workforce 203, Student Presence 182, Enrollment 34, übrige 28).

570 Einträge (447 unresolved, 123 debt) nennen `approval_adr: "0176"`. Sobald
diese ADR in der Basis liegt, gibt sie nichts weiter frei.

## Grenzen

- Closures im `compose`-Paket, die als Funktionswert übergeben werden, gelten
  weiter als eigene Implementierung. Nur Adapter*typen* sind Bindungen.
- Die Inventur bleibt eine statische May-Bind-Aussage. Sie beweist weder
  Transaktionsteilnahme noch Sperrreihenfolge; dafür bleiben die Verhaltens-
  und Vertragstests aus ADR 0067 zuständig.
