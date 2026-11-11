---
status: accepted
---

# Holiday-Allocation-Workflow braucht Compose und Sperrbefehl im Budget

**Status: Angenommen.** Freigegeben von Yannick Wenger am 11.11.2026.
Vorschlag vom 11.11.2026 zu
[#6555](https://github.com/moto-nrw/project-phoenix/issues/6555). Beleg nach
[ADR 0206](0206-ferienplatz-aenderungen-und-neueinfrieren-haben-einen-workflow.md),
Abschnitt „Architekturprüfung bei der späteren Umsetzung“: Diese ADR gibt
keine Budgets pauschal frei; benötigte Erhöhungen brauchen konkrete,
gesondert freizugebende Belege. Diese ADR ist dieser Beleg und gilt nur für
die zwei unten gemessenen Stellen.

## Kontext

ADR 0206 verlangt zwei Formen, die zwei Ratchets zählen:

1. Das Paket `workflows/holidayallocation/compose` baut den Workflow und
   bindet den Träger-Transaktionsläufer (ADR 0068). Jedes Compose-Paket hat
   einen Eintrag im Compose-LOC-Register von
   `backend/test/module_legacy_budget_ratchet_test.go`. Gemessen: 13 LOC.
2. Parent Billing bietet `HoldHolidayAllocationInvoices`: Akteur prüfen,
   ausschließlich die Abrechnungssperre B des Zeitraums nehmen, den Fehler
   benennen. ADR 0206 verbietet, dabei schon Gruppen oder Rechnungen zu
   lesen, weil P noch nicht gehalten ist. Der Befehl hat damit die Form der
   Kantenübersetzung nach
   [ADR 0075](0075-pass-throughs-keep-a-per-owner-floor.md) und zählt im
   Pass-through-Register von `modules/parentbilling` als ein Treffer.

Gegengerechnet ist, was der Umbau entfernt: der HTTP-Decorator
`enrollment/http.NewBilledHolidayAllocation` (40 Zeilen) und die drei
Koordinatoren in Parent Billing (`RebookHolidayChild`,
`SetHolidayWeekCapacity`, `AllocateHolidayPlaces`). Keiner davon zählte in
einem der beiden Register; es gibt dort nichts abzuziehen.

## Entscheidung

- `moduleComposeBudgets["workflows/holidayallocation/compose"] = 13`,
  `moduleComposeBudgetTotal` + 13.
- `modulePassthroughOwnerBudgets["modules/parentbilling"]` + 1,
  `modulePassthroughOwnerBudgetTotal` + 1.

Weitere Erhöhungen, auch für spätere Teilnehmer des Workflows, brauchen
einen eigenen Beleg.

## Verworfene Alternativen

- **Ohne Compose-Paket, Root bindet `tenant.WithinTraeger` selbst:**
  widerspricht ADR 0206 und ADR 0068; die Root hielte dann den
  Transaktionsläufer des Workflows.
- **Sperre B mit den bisherigen Lesezugriffen von `lockSiblings`:** liest
  veränderliche Grundlagen vor P und widerspricht der Sperrreihenfolge von
  ADR 0206.
- **Sperre B im Neueinfrieren:** käme nach P und öffnete den Rückweg
  P → B gegen die Ferienlauf-Freigabe.
