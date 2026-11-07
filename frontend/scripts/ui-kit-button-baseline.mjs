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
    lines: [224],
    reason: "Auswahloptionen eines Filters, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/page-header/NavigationTabs.tsx",
    lines: [197],
    reason: "Reiter: Auslöser des mobilen Reiter-Menüs",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/date-picker.tsx",
    lines: [443],
    reason: "Auslöser des Datumsfelds (Eingabefeld, sieht aus wie ein Select)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/planning/plan-block.tsx",
    lines: [208],
    reason: "Planungsblock im Raster: Kachel, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/portal-bottom-nav.tsx",
    lines: [393, 432],
    reason: "Navigationszeilen im „Mehr“-Blatt der unteren Leiste",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/guardians/guardian-picker-panel.tsx",
    lines: [329],
    reason: "Trefferzeile der Erziehungsberechtigten-Suche: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/messaging/new-message-modal.tsx",
    lines: [379],
    reason: "Elternteil in der Empfängerliste wählen: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/messaging/new-team-message-modal.tsx",
    lines: [159],
    reason: "Person in der Empfängerliste wählen: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/announcements/parent-target-picker.tsx",
    lines: [48],
    reason: "Zielgruppe ankreuzen (Häkchen-Zeile): Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/files/files-page.tsx",
    lines: [157],
    reason: "Ordner-Navigation (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/help/help-view.tsx",
    lines: [1055],
    reason: "Artikel-Navigation der Hilfe (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/auth/auth-shell.tsx",
    lines: [349],
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
    lines: [3132],
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/provisioning/provisioning-shared.tsx",
    lines: [218],
    reason: "Schalter (role=switch)",
  },
  {
    rule: "no-button-override",
    file: "src/app/[tenant]/(protected)/time-tracking/page.tsx",
    lines: [828, 861, 961, 1142],
    reason:
      "Pausen-Stepper −/+ und Stempelknöpfe Abwesenheit/Ausstempeln der Zeiterfassung (#5529: nicht angleichen)",
  },
  {
    rule: "no-button-override",
    file: "src/components/parent/calendar/parent-calendar-page.tsx",
    lines: [521, 771],
    reason:
      "Termin-Zeilen und Monats-Chips im Elternkalender, optisch Listeneinträge (#5529: nicht angleichen)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/active-supervisions/spontaneous-activity-start.tsx",
    lines: [378],
    reason: "Startkachel mit Symbol, Titel und Beschreibung: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/calendar/personal-calendar.tsx",
    lines: [1068],
    reason: "Termin-Block im Kalenderraster: Kalenderzelle",
  },
  {
    rule: "no-button-override",
    file: "src/components/class-day/class-day-overview.tsx",
    lines: [179],
    reason:
      "Klassenkarte: Kachel. Bleibt ButtonLink, weil TileCard den Seitenwechsel nicht an die Portalhülle meldet",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/care-schedule-manager.tsx",
    lines: [1545],
    reason: "Wochentagskarte, öffnet den Editor: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/companion-picker.tsx",
    lines: [397],
    reason: "Suchtreffer zum Auswählen: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/parent-messages-card.tsx",
    lines: [144],
    reason: "Nachrichtenzeile, öffnet den Verlauf: Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/planned-status-days-modal.tsx",
    lines: [946],
    reason: "Kalendertag zum Umschalten: Kalender-/Auswahlzelle",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/event-form/step-termin.tsx",
    lines: [149],
    reason: "Typauswahl im Terminformular: Auswahlkachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/period-switcher-dropdown.tsx",
    lines: [306],
    reason: "Zeitraum-Option im Auswahlmenü: Menüeintrag",
  },
  // #5574: hellgraue Fläche als gewählter Zustand einer Auswahl oder
  // Navigation, nicht als Ruhezustand eines Knopfs.
  {
    rule: "no-grey-button-fill",
    file: "src/components/timetable/period-switcher-dropdown.tsx",
    lines: [306],
    reason:
      "Auswahlzeile im Zeitraum-Menü: grau markiert den gewählten Zeitraum",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/announcements/page.tsx",
    lines: [702, 850, 943],
    reason: "Auswahlzeilen mit Häkchen (Rollen, Träger, Schulen), kein Knopf",
  },
];
