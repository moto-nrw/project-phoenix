import baseline from "./ui-kit-button-baseline.json" with { type: "json" };

// Button-Ratchet (#5531, scripts/oxlint-plugin-ui-kit.mjs).
//
// Baseline: Funde je Datei und Regel zum Stand von #5531, erzeugt mit
// `pnpm run ui-kit:button-baseline`. Shrink-only: eine Datei
// mit mehr Funden schlägt im Lint fehl; eine mit weniger lässt den Test
// „button ratchet baseline“ fehlschlagen, bis die Zahl gesenkt ist.
/** @type {Record<string, Record<string, number>>} */
export const buttonRatchetBaseline = baseline;

// Bewusste Ausnahmen aus #5529 („nicht angleichen“ und „keine Buttons im
// Sinne dieses Auftrags“). Sie gehören nicht in die Baseline: sie sollen nie
// verschwinden. Jede Zeile nennt Datei, Regel, Fundzeile und Grund.
/** @type {ReadonlyArray<{ rule: string; file: string; lines: readonly number[]; reason: string }>} */
export const buttonRatchetAllowlist = [
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/segmented-control.tsx",
    lines: [134, 173, 190],
    reason: "SegmentedControl: Wertauswahl, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/toggle-chip.tsx",
    lines: [38],
    reason: "ToggleChip: Schalter (aria-pressed), kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/FilterPanel.tsx",
    lines: [217],
    reason: "Auswahloptionen eines Filters, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/NavigationTabs.tsx",
    lines: [196],
    reason: "Reiter: Auslöser des mobilen Reiter-Menüs",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/DesktopFilters.tsx",
    lines: [170],
    reason: "Auslöser eines Filter-Auswahlfelds, sieht aus wie ein Select",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/date-picker.tsx",
    lines: [440],
    reason: "Auslöser des Datumsfelds (Eingabefeld, sieht aus wie ein Select)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/date-range-picker.tsx",
    lines: [81],
    reason: "Auslöser des Zeitraumfelds (Eingabefeld)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/plan-block.tsx",
    lines: [208],
    reason: "Planungsblock im Raster: Kachel, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/portal-bottom-nav.tsx",
    lines: [133],
    reason: "Navigationszeile der unteren Leiste",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/parent/shell/parent-bottom-nav.tsx",
    lines: [177, 215, 262],
    reason: "Navigationszeilen der unteren Leiste im Elternportal",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/files/files-page.tsx",
    lines: [148],
    reason: "Ordner-Navigation (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/help/help-view.tsx",
    lines: [1076],
    reason: "Artikel-Navigation der Hilfe (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/auth/auth-shell.tsx",
    lines: [360],
    reason: "Auswahlpunkte des Zitat-Karussells (Seitenauswahl), kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/settings/fields/boolean-field.tsx",
    lines: [17],
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/enrollment/enrollment-form-editor.tsx",
    lines: [3150],
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/provisioning/provisioning-shared.tsx",
    lines: [209],
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/weekly-calendar-grid.tsx",
    lines: [388],
    reason: "Tagesauswahl (aria-pressed), Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/staff/dienstplan-resource-grid.tsx",
    lines: [718],
    reason: "Tagesauswahl (aria-pressed), Auswahlzeile",
  },
  {
    rule: "no-button-override",
    file: "src/app/[tenant]/(protected)/time-tracking/page.tsx",
    lines: [854, 887, 987, 1156],
    reason:
      "Pausen-Stepper −/+ und Stempelknöpfe Abwesenheit/Ausstempeln der Zeiterfassung (#5529: nicht angleichen)",
  },
  {
    rule: "no-button-override",
    file: "src/components/parent/calendar/parent-calendar-page.tsx",
    lines: [548, 797],
    reason:
      "Termin-Zeilen und Monats-Chips im Elternkalender, optisch Listeneinträge (#5529: nicht angleichen)",
  },
];
