import baseline from "./ui-kit-button-baseline.json" with { type: "json" };

// Button-Ratchet (#5531, scripts/oxlint-plugin-ui-kit.mjs).
//
// Baseline: Funde je Datei und Regel zum Stand von #5531, erzeugt mit
// `node scripts/ui-kit-button-baseline-update.mjs`. Shrink-only: eine Datei
// mit mehr Funden schlägt im Lint fehl; eine mit weniger lässt den Test
// „button ratchet baseline“ fehlschlagen, bis die Zahl gesenkt ist.
/** @type {Record<string, Record<string, number>>} */
export const buttonRatchetBaseline = baseline;

// Bewusste Ausnahmen aus #5529 („nicht angleichen“ und „keine Buttons im
// Sinne dieses Auftrags“). Sie gehören nicht in die Baseline: sie sollen nie
// verschwinden. Jede Zeile nennt Datei, Regel, Zahl und Grund.
/** @type {ReadonlyArray<{ rule: string; file: string; count: number; reason: string }>} */
export const buttonRatchetAllowlist = [
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/segmented-control.tsx",
    count: 3,
    reason: "SegmentedControl: Wertauswahl, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/toggle-chip.tsx",
    count: 1,
    reason: "ToggleChip: Schalter (aria-pressed), kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/FilterPanel.tsx",
    count: 1,
    reason: "Auswahloptionen eines Filters, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/NavigationTabs.tsx",
    count: 1,
    reason: "Reiter: Auslöser des mobilen Reiter-Menüs",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/DesktopFilters.tsx",
    count: 1,
    reason: "Auslöser eines Filter-Auswahlfelds, sieht aus wie ein Select",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/date-picker.tsx",
    count: 2,
    reason: "Auslöser des Datumsfelds und Leeren-X im Eingabefeld",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/date-range-picker.tsx",
    count: 1,
    reason: "Auslöser des Zeitraumfelds (Eingabefeld)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/database/database-form.tsx",
    count: 1,
    reason: "Entfernen-× in einem Auswahl-Chip im Eingabefeld",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/plan-block.tsx",
    count: 1,
    reason: "Planungsblock im Raster: Kachel, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/portal-bottom-nav.tsx",
    count: 1,
    reason: "Navigationszeile der unteren Leiste",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/parent/shell/parent-bottom-nav.tsx",
    count: 3,
    reason: "Navigationszeilen der unteren Leiste im Elternportal",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/files/files-page.tsx",
    count: 1,
    reason: "Ordner-Navigation (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/help/help-view.tsx",
    count: 1,
    reason: "Artikel-Navigation der Hilfe (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/settings/fields/boolean-field.tsx",
    count: 1,
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/enrollment/enrollment-form-editor.tsx",
    count: 1,
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/provisioning/provisioning-shared.tsx",
    count: 1,
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/weekly-calendar-grid.tsx",
    count: 1,
    reason: "Tagesauswahl (aria-pressed), Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/staff/dienstplan-resource-grid.tsx",
    count: 1,
    reason: "Tagesauswahl (aria-pressed), Auswahlzeile",
  },
  {
    rule: "no-button-override",
    file: "src/app/[tenant]/(protected)/time-tracking/page.tsx",
    count: 4,
    reason:
      "Pausen-Stepper −/+ und Stempelknöpfe Abwesenheit/Ausstempeln der Zeiterfassung (#5529: nicht angleichen)",
  },
  {
    rule: "no-button-override",
    file: "src/components/parent/calendar/parent-calendar-page.tsx",
    count: 2,
    reason:
      "Termin-Zeilen und Monats-Chips im Elternkalender, optisch Listeneinträge (#5529: nicht angleichen)",
  },
];
