---
status: accepted
---

# „Gastprofil“ heißt externe Betreuungskraft

Entscheidung zu [#4573](https://github.com/moto-nrw/project-phoenix/issues/4573),
aufbauend auf [#3823](https://github.com/moto-nrw/project-phoenix/issues/3823).
Der Begriff „Gastprofil“ entfällt. Er stand nur im Code und meinte ein
internes Merkmal; die Nutzer sehen „Extern“. Neben der Schulrolle „Extern“ und
der alten Systemrolle `guest` („Gast“) war er der dritte ähnliche Begriff.
Übrig bleiben zwei: die Rolle „Extern“ und das Merkmal, das jetzt so heißt,
wie es ist.

Das Merkmal ist der **Datensatz einer externen Betreuungskraft**
(`users.external_caregivers`). Er gehört zu einer Person des Personals und
trägt die Organisation und einen optionalen Zeitraum. Ist er im Zeitraum
aktiv, ist die Person extern: Zeiterfassung und Dienstplan führen sie nicht,
sie übernimmt keine Vertretung und keine Gruppe, und ein Aufsichtsstart
stempelt keine Arbeitszeit. Ein Konto ändert daran nichts.

Davon getrennt bleiben die Rechte:

- Die **Schulrolle „Extern“** regelt nur, was die Person in moto darf. Die
  Leitung kann sie ändern. Sie macht niemanden extern.
- Die **Systemrolle `guest`** („Gast“) bleibt unverändert. Über sie entscheidet
  eine eigene ADR. In Produktion haben sie 8 Konten in 3 Schulen, und sie
  öffnet `users:read` samt Elternnachrichten. Empfehlung: mit den Schulen
  klären, die Konten nach „Extern“ oder „Betreuer“ überführen, dann die Rolle
  entfernen.

Die Tabelle `users.guests` heißt ab Migration 1.15.475
`users.external_caregivers`; Sequenz, Indizes, Constraints, RLS-Policy und
Trigger folgen dem Namen. Zeilen, Row-Level-Security, Rechte der
Datenbankrollen und Fremdschlüssel bleiben, weil PostgreSQL sie an die Tabelle
bindet. In Produktion hatte die Tabelle beim Lesen am 23.10.2026 keine Zeilen.

Im Code tragen die beiden Bedeutungen des Merkmals eigene Namen:
`HasExternalCaregiverRecord` ist der rohe Befund (aktiver Datensatz),
`IsExternalCaregiver` die Regel `schoolmembership.ExternalCaregiver`
(aktiver Datensatz und bekannte Person). Bisher stand beides im selben Feld
`IsGuest`. Das Verhalten bleibt gleich.

Die Architekturprüfung erkennt eine Umbenennung: Eine neue Migration mit
`ALTER TABLE schema.alt RENAME TO neu` darf `schema.neu` in `policy.json`
eintragen, wenn die Basis `schema.alt` besitzt, der Kandidat `schema.alt`
streicht und der Besitzer gleich bleibt (School Membership). Sonst bliebe
eine Umbenennung eine neue Zuordnung. Shrink-only-Register ändern sich nicht.
