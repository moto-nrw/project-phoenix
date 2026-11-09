import baseline from "./ui-kit-button-baseline.json" with { type: "json" };

// Button-Ratchet (#5531, scripts/oxlint-plugin-ui-kit.mjs).
//
// Baseline: seit #6182 für alle Regeln leer, die Regeln sind hart null. Jeder
// Fund außerhalb der Allowlist schlägt im Lint fehl; `pnpm run
// ui-kit:button-baseline` weigert sich, die Baseline wieder anzuheben.
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
    lines: [143, 182, 199],
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
    file: "src/components/ui/switch.tsx",
    lines: [39],
    reason: "Switch: der Kit-Baustein für An/Aus (role=switch), kein Knopf",
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
    lines: [457],
    reason: "Auslöser des Datumsfelds (Eingabefeld, sieht aus wie ein Select)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/planning/plan-block.tsx",
    lines: [213],
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
    lines: [323],
    reason: "Trefferzeile der Erziehungsberechtigten-Suche: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/messaging/new-message-modal.tsx",
    lines: [380],
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
    file: "src/components/files/files-page.tsx",
    lines: [158],
    reason: "Ordner-Navigation (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/help/help-view.tsx",
    lines: [1052],
    reason: "Artikel-Navigation der Hilfe (aria-current), Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/auth/auth-shell.tsx",
    lines: [349],
    reason: "Auswahlpunkte des Zitat-Karussells (Seitenauswahl), kein Knopf",
  },
  {
    rule: "no-button-override",
    file: "src/app/[tenant]/(protected)/time-tracking/page.tsx",
    lines: [829, 862, 962, 1143],
    reason:
      "Pausen-Stepper −/+ und Stempelknöpfe Abwesenheit/Ausstempeln der Zeiterfassung (#5529: nicht angleichen)",
  },
  {
    rule: "no-button-override",
    file: "src/components/parent/calendar/parent-calendar-page.tsx",
    lines: [524, 776],
    reason:
      "Termin-Zeilen und Monats-Chips im Elternkalender, optisch Listeneinträge (#5529: nicht angleichen)",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/active-supervisions/spontaneous-activity-start.tsx",
    lines: [329],
    reason: "Startkachel mit Symbol, Titel und Beschreibung: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/calendar/personal-calendar.tsx",
    lines: [1066],
    reason: "Termin-Block im Kalenderraster: Kalenderzelle",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/care-schedule-manager.tsx",
    lines: [1544],
    reason: "Wochentagskarte, öffnet den Editor: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/companion-picker.tsx",
    lines: [403],
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
    lines: [995],
    reason: "Kalendertag zum Umschalten: Kalender-/Auswahlzelle",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/period-switcher-dropdown.tsx",
    lines: [306],
    reason: "Zeitraum-Option im Auswahlmenü: Menüeintrag",
  },
  // #5608: Seit die Regel `moto-content-surface`, Farben per `style` und
  // `label`/`span` mit `cursor-pointer` erkennt, sieht sie diese Kacheln und
  // Felder. Sie sind keine Knöpfe im Sinn von #5529.
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/tile-card.tsx",
    lines: [94, 149],
    reason: "TileCard selbst: der Kit-Baustein für klickbare Kacheln",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/ui/color-swatch-input.tsx",
    lines: [21],
    reason: "Farbfeld: zeigt die gewählte Farbe und öffnet die Farbauswahl",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/compact-student-card.tsx",
    lines: [88],
    reason: "Kinderkarte im Raster, öffnet das Profil: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/parent/child/child-switcher.tsx",
    lines: [55],
    reason:
      "Kinderkarte im Elternportal (Ausnahme des Epics #5576, nicht anfassen): Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/care-schedule-manager.tsx",
    lines: [1501],
    reason: "Statustag-Karte, öffnet das Entfernen: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/calendar/personal-calendar.tsx",
    lines: [1015],
    reason: "Termin-Block in der Wochenansicht: Kalenderzelle",
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
];
