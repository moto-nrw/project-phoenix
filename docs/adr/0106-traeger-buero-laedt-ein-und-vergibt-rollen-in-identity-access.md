---
status: accepted
---

# Träger-Büro lädt ein und vergibt Rollen in Identity & Access

Entscheidung vom 25.10.2026 zu [#4650](https://github.com/moto-nrw/project-phoenix/issues/4650),
zweiter Schnitt von [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 4. ADR 0084 legt fest: Mitgliedschaft bei Organisation & Tenancy,
Rollen und Rechte bei Identity & Access, Einladung „über das bestehende
Einladungssystem“, weitere Personen lädt ein, wer „Konten verwalten“ hat.
Diese ADR hält fest, wie das umgesetzt ist und was es in den Registern
ändert (ADR 0057 Punkt 5, ADR 0070).

## Entscheidung

1. **Eigene Einladungstabelle im Einladungssystem.** Die Einladung ins
   Träger-Büro liegt in `auth.traeger_invitations` (Identity & Access), nicht
   in `auth.invitation_tokens`. Die Schul-Einladung ist an eine Schule
   gebunden: `tenant_id` ist Pflicht, RLS gilt, `role_id` zeigt auf eine
   Schulrolle, die Annahme legt Person und Personalakte an. Ein Träger ist
   kein Tenant und bekommt keine RLS-Familie (ADR 0084). Die Tabelle folgt
   dem bestehenden System so weit es trägt: nur der SHA-256 des Link-Tokens
   wird gespeichert (#4902), die Lebensdauer ist
   `invitation_token_expiry_hours`, die Mail läuft durch die Outbox von
   Delivery im selben Schreibvorgang, die Fehlercodes von abgelaufenen,
   verbrauchten und unbekannten Links sind die der Schul-Einladung. Je Träger
   und Adresse ist höchstens eine Einladung offen; eine neue verbraucht die
   alte.
2. **Annahme ohne Schulkonto.** Eine Adresse ohne Konto wählt ein Passwort
   und bekommt ein Konto ohne Person: Ein Träger-Konto braucht keine
   Personalakte einer Schule. Eine Adresse mit Konto bestätigt mit dessen
   Passwort; die Einladung gibt Zugang, nie fremde Zugangsdaten. Die
   Annahme sperrt das Konto über die Adresse (wie die Schul-Einladung gegen
   die Löschung unbenutzter Konten), setzt die Rollen, aktiviert die
   Mitgliedschaft und verbraucht den Link in einer Einheit.
3. **Identity & Access führt die Einheit.** Einladen, Annehmen, Rollen
   ändern und Entfernen sind Befehle von Identity & Access. Die Mitgliedschaft
   von Organisation & Tenancy nimmt als teilnehmender Befehl teil
   (ADR 0067, Command Participation): `ActivateTraegerMembership` und das
   neue `DeactivateTraegerMembership` laufen über `RunAdmin` in der
   umgebenden administrativen Transaktion. Der Port `TraegerBueroMemberships`
   ist konsumenteneigen und an der Root gebunden; es entsteht keine
   Policy-Regel und kein Import zwischen den Ownern. Die Einheit ist die
   vorhandene globale Einheit `newCommandTransactions().RunAdmin` aus
   `compose.New`; es entsteht keine neue Stelle im Transaktionsregister.
4. **Prüfung gegen die Datenbank.** Die Routen prüfen das Recht
   `traeger_accounts:manage` im Token. Jeder Befehl prüft zusätzlich in der
   Einheit, dass der Träger lebt, die handelnde Person aktives Mitglied mit
   eingeschaltetem Konto ist und ihre aktuellen Rollen „Konten verwalten“
   geben. Der Träger kommt
   immer aus der Sitzung.
5. **Letzte Verwaltung.** Ein Befehl, der der letzten aktiven Person die
   Rolle Verwaltung nimmt (Rolle ändern oder entfernen), wird mit
   `traeger.last_administrator` abgelehnt. Jeder ändernde Befehl sperrt
   zuerst die Verwaltungs-Rollenzeilen des Trägers (`FOR UPDATE`); zwei
   gleichzeitige Entfernungen warten aufeinander und zählen danach neu.
6. **Wirkung auf Sitzungen.** Geänderte Rollen erreichen eine Sitzung bei
   ihrer nächsten Erneuerung; der Refresh liest die Rollen ohnehin neu.
   Eine Entfernung beendet die Mitgliedschaft, entzieht die Rollen und
   löscht die Träger-Sitzungen der Person sofort: Kein Refresh gelingt
   mehr. Ein schon ausgestelltes Zugangstoken gilt wie in allen Portalen bis
   zu seinem Ablauf (`AUTH_JWT_EXPIRY`); die Kontenverwaltung selbst prüft
   die Mitgliedschaft bei jedem Befehl in der Datenbank.
7. **Protokoll.** Jeder Schritt und jedes Lesen der Personenliste landet in
   `audit.traeger_account_changes` (Audit Platform, ohne Tenant wie das
   Operator-Protokoll). Der Eintrag wird in derselben Einheit geschrieben;
   scheitert er, rollt die Änderung zurück.

## Folgen in den Registern

- `moduleComposeBudgets["modules/identityaccess/compose"]` steigt von 1302
  auf 1326 Zeilen, `moduleComposeBudgetTotal` um genau dieselben 24 Zeilen:
  das Abhängigkeitsfeld `Traeger` in `compose.Dependencies` und
  `newPortalAccess`, das die Kontenverwaltung über die vorhandene globale
  Einheit baut. Kein anderer Eintrag wächst.
- `command-effects.json` erhält die Bindungen der Root: Mitgliedschaft
  (zwei Leaf-Befehle, zwei Lesungen), Protokoll (Plattform, ADR 0098) und
  die Einladungsmail (Plattform, ADR 0098). Die Mail nutzt den konkreten
  `OutboxEnqueuer`, damit keine ungeklärte dynamische Bindung entsteht.
- `passthrough.json` erhält die genaue Evidenz der neuen Root-Bindungen;
  kein Pass-through-Budget steigt.
- Neue Datenobjekte: `auth.traeger_invitations` (identity-access) und
  `audit.traeger_account_changes` (audit-platform), beide mit Träger-Bezug
  im Lebenszyklus-Katalog. Gelöscht wird im stündlichen Cleanup: abgelaufene
  Einladungen zusammen mit den abgelaufenen Schul-Einladungen, Protokoll-
  einträge mit der Frist des Operator-Protokolls (vor dem 1. Januar des
  Vorjahres). Beide Tabellen gehen außerdem mit dem Träger.

## Nicht entschieden

- Die erste Person lädt das moto-Team weiterhin über „Person zuordnen“ im
  Operator-Portal zu (#4649); eine Operator-Einladung per Mail folgt bei
  Bedarf.
- Zwei-Faktor-Pflicht für Träger-Konten kommt mit #4651; die Annahme stellt
  keine Sitzung aus, die Anmeldung danach läuft über den normalen Login.
- Die Ansicht des Protokolls im Träger-Portal gehört zu #4655.

## Nachtrag 2026-10-27: Einladung durch das moto-Team (#5144)

Der offene Punkt „Operator-Einladung per Mail“ ist entschieden (Spec #4611,
User Story 2). Das moto-Team lädt im Operator-Portal ein, auch die erste
Person, ohne dass sie ein Schulkonto braucht.

- **Dieselbe Einladung.** `POST /operator/organizations/{id}/traeger-members/invitations`
  legt die Einladung in `auth.traeger_invitations` an, mit derselben Mail und
  derselben Annahmeseite wie aus dem Träger-Portal. `created_by` bleibt leer:
  Das moto-Team ist kein Konto des Trägers. Das Träger-Büro sieht die
  Einladung unter den offenen Einladungen und kann sie zurücknehmen.
- **Einheit und Protokoll.** Organisation & Tenancy besitzt die Einheit: Sie
  prüft den Träger, ruft den Leaf-Befehl
  `identityaccess.TraegerOperatorInvitation.InviteTraegerAccountForOperator`
  (ADR 0067, Command Participation) und schreibt das Operator-Protokoll
  (`create`/`traeger_invitation`, mit Adresse und Rollen) in derselben
  Einheit. Der Leaf-Befehl verweigert ohne umgebende Admin-Transaktion und
  nimmt zuerst die Verwaltungs-Sperre aus Punkt 5, wie jeder Befehl des
  Träger-Büros. `audit.traeger_account_changes` erhält für das Anlegen keinen
  Eintrag, weil der Handelnde dort ein Konto sein muss; die Annahme schreibt
  wie bisher `invitation_accepted`.
- **Mehrere Rollen.** „Person zuordnen“ nimmt jetzt eine Rollenmenge
  (`roles`) statt einer Rolle; Organisation & Tenancy vergibt sie in einer
  Einheit über den vorhandenen Leaf-Befehl je Rolle.
- **Register.** `command-effects.json` erhält die Bindung
  `ProvisioningIdentity.InviteTraegerAccount` als Leaf-Mutation. Kein Budget
  steigt.
