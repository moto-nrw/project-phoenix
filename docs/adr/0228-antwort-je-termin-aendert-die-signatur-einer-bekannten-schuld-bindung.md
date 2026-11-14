---
status: proposed
---

# Antwort je Termin ändert die Signatur einer bekannten Schuld-Bindung

Entscheidung vom 2026-11-13 für
[#6987](https://github.com/moto-ogs/project-phoenix/issues/6987). Sie genehmigt
drei geänderte Command-Effect-Bindungen. Sie fügt keinen Owner, keine
Policy-Regel, keine Sperre und keine Laufzeitausnahme hinzu und ändert weder
Wirkung noch Einstufung.

## Kontext

Eine Zu- oder Absage kann seit #6987 für einen einzelnen Termin einer Reihe
gelten. Appointments speichert sie an der Empfängerzeile. Der Befehl
`appointments.Capability.UpdateAppointmentRecipientResponse` nimmt dafür statt
`(recipientID, status)` jetzt `RecipientResponse{RecipientID, OccurrenceDate,
Status}`. Die Zahl der Methoden der Capability bleibt gleich.

School Calendar ruft den Befehl aus dem Portal auf. Diese Bindung ist seit
[ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md)
als bekannte Schuld (`debt`) verzeichnet: Das Portal koordiniert Lebenszyklus,
Empfänger und Benachrichtigungen, statt das einem Workflow zu überlassen. Eine
geänderte Signatur einer Schuld-Bindung verlangt eine ADR, die in derselben
Änderung hinzukommt; eine ADR aus der Basis kann sie nicht genehmigen.

| Port | Ziel | Digest neu |
|---|---|---|
| `appointments.Capability.UpdateAppointmentRecipientResponse` | `appointments.Capability.UpdateAppointmentRecipientResponse` | `ee2cc1582836b6b54efb57a268adbe46` |
| `appointments.Capability.UpdateAppointmentRecipientResponse` | `appointments.Owner.UpdateAppointmentRecipientResponse` | `10d3b511b96c0301937e2db962d7d8b9` |
| `appointments.Capability.UpdateAppointmentRecipientResponse` (Bindung `Owner`) | `appointments.Owner.UpdateAppointmentRecipientResponse` | `10d3b511b96c0301937e2db962d7d8b9` |

## Entscheidung

Die drei Einträge nennen diese ADR als `approval_adr`. Wirkung (`mutation`),
Einstufung (`debt`) und Ziel-Owner (`appointments`) bleiben. Der Befehl
schreibt weiter nur die eigene Empfängerzeile in einer Einheit; neu ist ein
optionales Datum, das die Antwort auf einen Termin der Reihe begrenzt.

Ein zweiter Befehl für die Antwort je Termin hätte die Signatur unverändert
gelassen, aber die Capability und ihr Method-Set verbreitert
(`TestModuleFacadeMethodSetRatchet`) und eine neue Schuld-Bindung derselben Art
angelegt. Die Schuld selbst, die Koordination im Portal, bleibt offen; ihr
Abbau gehört zum Workflow-Umbau, den ADR 0083 nennt, nicht zu #6987.

Liegt diese ADR in der Basis, kann sie keine weitere Änderung genehmigen.
