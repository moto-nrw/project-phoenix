/**
 * Control labels for the kit date pickers. The defaults inside the picker are
 * German because the staff and operator portals are German-only; the parent
 * portal and public enrollment form pass translated strings alongside a
 * matching `locale` (see `~/lib/hooks/use-localized-date-picker`).
 *
 * The contract lives here rather than on the component so the hook that builds
 * it does not have to reach from `src/lib/` into `src/components/`.
 */
export interface DatePickerLabels {
  readonly clear?: string;
  /** What the clear button does (#6744), shown on hover and focus. */
  readonly clearHint?: string;
  readonly previousMonth?: string;
  readonly previousMonthHint?: string;
  readonly nextMonth?: string;
  readonly nextMonthHint?: string;
  readonly month?: string;
  readonly year?: string;
  /** Receives the count, e.g. (3) => "3 Tage ausgewählt". */
  readonly selectedDays?: (count: number) => string;
}
