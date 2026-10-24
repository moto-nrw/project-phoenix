---
status: accepted
---

# Unterschrift ist ein Baustein der Audit Platform

Entscheidung vom 24.10.2026 für #4785. Einverständnisse (#4785), die
Anmeldung (#4786) und die Abholerlaubnis (#4780) brauchen dieselbe einfache
elektronische Signatur mit gezeichneter Unterschrift: ein Bild, wer es wann
gegeben hat, die Prüfsumme dessen, was unterschrieben wurde, und eine
Prüfsumme über den ganzen Eintrag, unveränderlich und an die Schule gebunden.

## Entscheidung

1. Die Unterschriften liegen in `audit.signatures` und gehören der Audit
   Platform. Sie ist bereits Eigentümerin der nur anhängenden Nachweis-
   Ledger (`audit.guardian_changes`, `audit.student_consent_changes` und
   weitere) und eine Plattform, von der Communication, Enrollment und der
   Parent-Portal-Workflow abhängen dürfen, ohne dass zwischen zwei Domänen
   eine neue Kante entsteht. Kein neuer Owner, keine neue Policy-Regel außer
   dem Datenobjekt.
2. Der Baustein entscheidet keine Berechtigung. Wer unterschreiben darf,
   prüft der nutzende Ablauf vor seiner Transaktion, wie bisher
   (`ResolvePermittedChild` mit der passenden Berechtigung). Gespeichert und
   gelesen wird nur in der Mandanten-Transaktion des Ablaufs; Lesen liefert
   nur Einträge dieser Schule.
3. Die SHA-256-Identität kommt aus Security Runtime und wird an der Root
   gebunden; die Audit Platform bekommt keine Krypto-Freigabe.
4. Jede Unterschrift hat genau einen Bezug, der ihre Löschung bestimmt:
   das Kind (`(tenant_id, student_id)`) oder die Anmeldung
   (`(tenant_id, enrollment_request_id)`), jeweils mit `ON DELETE CASCADE`.
   Das Konto darf fehlen (öffentliche Anmeldung).
5. Die Übersetzung auf die Werte der Konsumenten liegt in der Root
   (`roots/owners/signatures.go`), nicht in deren Compose-Paketen.

## Folgen für die Ratchets

`modules/auditlog/compose` braucht einen Konstruktor, der den internen
Postgres-Adapter über die umgebende Mandanten-Transaktion baut. Das ist nur
dort möglich: Die Root darf den internen Adapter nicht importieren, und das
öffentliche Paket würde einen Importzyklus schließen. Der Konstruktor steht
neben dem gleichartigen der Sorgeberechtigten-Änderungen und erhöht das
Compose-Budget der Audit Platform und die Summe um genau 6 Zeilen
(183 → 189; Summe +6). Er enthält keine Logik außer der Konstruktion.
Alle übrigen Budgets bleiben unverändert oder sinken: Die Zeile im
Parent-Portal-Compose und in der Communication-Composition sind ausgeglichen,
und `seed_extras.go` verlässt das Dateigrößen-Register.
