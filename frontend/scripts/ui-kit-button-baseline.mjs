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
    file: "src/components/guardians/guardian-picker-panel.tsx",
    lines: [326],
    reason: "Trefferzeile der Erziehungsberechtigten-Suche: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/messaging/new-message-modal.tsx",
    lines: [381],
    reason: "Elternteil in der Empfängerliste wählen: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/messaging/new-team-message-modal.tsx",
    lines: [161],
    reason: "Person in der Empfängerliste wählen: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/announcements/parent-target-picker.tsx",
    lines: [46],
    reason: "Zielgruppe ankreuzen (Häkchen-Zeile): Auswahlzeile",
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
  {
    rule: "no-hand-rolled-button",
    file: "src/components/active-supervisions/spontaneous-activity-start.tsx",
    lines: [376],
    reason: "Startkachel mit Symbol, Titel und Beschreibung: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/calendar/personal-calendar.tsx",
    lines: [1059],
    reason: "Termin-Block im Kalenderraster: Kalenderzelle",
  },
  {
    rule: "no-button-override",
    file: "src/components/class-day/class-day-overview.tsx",
    lines: [172],
    reason:
      "Klassenkarte: Kachel. Bleibt ButtonLink, weil TileCard den Seitenwechsel nicht an die Portalhülle meldet",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/dashboard/mobile-bottom-nav.tsx",
    lines: [1302, 1503, 1538],
    reason: "Navigationszeilen der Seitenleiste und der unteren Leiste",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/home/day-flow-block.tsx",
    lines: [140],
    reason: "Listenzeile der Startseite, verlinkt weiter",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/home/home-block-content.tsx",
    lines: [202, 274],
    reason: "Listenzeilen der Startseite, verlinken weiter",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/home/messages-block.tsx",
    lines: [82],
    reason: "Listenzeile der Startseite, verlinkt weiter",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/home/my-group-block.tsx",
    lines: [253, 284, 316],
    reason: "Listenzeilen der Startseite, verlinken weiter",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/home/open-requests-block.tsx",
    lines: [101],
    reason: "Listenzeile der Startseite, verlinkt weiter",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/home/reminders-block.tsx",
    lines: [134],
    reason: "Listenzeile der Startseite, verlinkt weiter",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/care-plan-view.tsx",
    lines: [507],
    reason: "Mobile Tagesauswahl mit Auswahlzustand: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/care-schedule-manager.tsx",
    lines: [1312],
    reason: "Tagesauswahl mit Auswahlzustand: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/care-schedule-manager.tsx",
    lines: [1512, 1555],
    reason:
      "Abwesenheits- und Wochentagskarte, öffnen Dialog bzw. Editor: Kacheln",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/companion-picker.tsx",
    lines: [410],
    reason: "Suchtreffer zum Auswählen: Auswahlzeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/parent-messages-card.tsx",
    lines: [138],
    reason: "Nachrichtenzeile, öffnet den Verlauf: Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/planned-status-days-modal.tsx",
    lines: [947],
    reason: "Kalendertag zum Umschalten: Kalender-/Auswahlzelle",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/students/student-detail-components.tsx",
    lines: [1176],
    reason: "Verlaufskachel mit Symbol, Titel und Beschreibung: Kachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/time-tracking/betreuungsplan-heute-card.tsx",
    lines: [386],
    reason:
      "Zeile „Betreuungsplan heute“, verlinkt auf Heute: Navigationszeile",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/event-form/step-termin.tsx",
    lines: [140],
    reason: "Typauswahl im Terminformular: Auswahlkachel",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/components/timetable/period-switcher-dropdown.tsx",
    lines: [277],
    reason: "Zeitraum-Option im Auswahlmenü: Menüeintrag",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/announcements/page.tsx",
    lines: [678],
    reason: "Typ der Ankündigung: Wertauswahl, kein Knopf",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/announcements/page.tsx",
    lines: [709],
    reason: "Auslöser des Dringlichkeits-Felds, sieht aus wie ein Select",
  },
  {
    rule: "no-hand-rolled-button",
    file: "src/app/operator/announcements/page.tsx",
    lines: [819, 961, 1050],
    reason: "Auswahlzeilen mit Häkchen (Rollen, Träger, Schulen), kein Knopf",
  },
];
