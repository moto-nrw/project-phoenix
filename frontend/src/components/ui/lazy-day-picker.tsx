"use client";

import { lazy } from "react";
import { Skeleton } from "~/components/ui/skeleton";
import { cn } from "~/lib/utils";

// react-day-picker, together with the date-fns barrel it imports, is only
// needed once a calendar is open. Loading it on demand keeps it out of the
// initial bundle of every page with a date field, and out of every Vitest file
// that renders a form without opening its calendar (about 1 s of module work
// per file). Triggers call `preloadDayPicker()` on hover and focus, so the
// module is usually loaded before the click; until then the caller's
// <Suspense> shows `DayPickerSkeleton` in the grid's place.
//
// Only the kit pickers import react-day-picker at runtime; everything else
// imports its types. The oxlint `no-restricted-imports` entry keeps it so.

// oxlint-disable-next-line no-restricted-imports -- the one runtime entry point
const loadDayPicker = () => import("react-day-picker");

export const LazyDayPicker = lazy(() =>
  loadDayPicker().then((module) => ({ default: module.DayPicker })),
);

/**
 * Starts loading the calendar module, e.g. when the trigger gets focus.
 * Tests that open a real calendar await it in `beforeAll`: a cold import can
 * take longer than Testing Library's one-second `findBy` timeout on a busy
 * machine.
 */
export function preloadDayPicker(): Promise<void> {
  // A failed preload is harmless: rendering LazyDayPicker imports the module
  // again and surfaces its own error.
  return loadDayPicker().then(
    () => undefined,
    () => undefined,
  );
}

/**
 * The one calendar look (#5585, epic #5576 decision 7): selection gray-900,
 * today blue, cells 36 px high. Every kit calendar spreads this set; a range
 * calendar replaces only the layout of `months`, `month` and `day`, not the look.
 */
export const DAY_PICKER_CLASSNAMES = {
  root: "text-sm",
  months: "flex flex-col",
  month: "",
  month_caption: "hidden",
  month_grid: "w-full border-collapse",
  weekdays: "flex",
  // flex-1 + min-w-0 lets the seven columns take exactly the width the card
  // has, in both directions: a wide field gets a wide grid, a narrow one gets
  // narrow cells rather than forcing the card wider than the field it belongs
  // to.
  weekday: "text-gray-500 flex-1 min-w-0 font-normal text-xs text-center pb-1",
  week: "flex w-full mt-1.5",
  // px-1 on the cell keeps a 8px gutter between two columns, and the selection
  // is painted on the inner button rather than the cell: otherwise the cell
  // background bleeds into its own padding and two neighbouring selected days
  // fuse into one continuous dark bar.
  // By finger a day is 44 px high and the gutter narrower, so the button
  // itself reaches the touch size; the focus ring is the kit's gray-600.
  day: "flex-1 min-w-0 h-9 px-1 text-center text-sm py-0 relative pointer-coarse:h-11 pointer-coarse:px-0.5",
  day_button:
    "w-full h-9 rounded-lg hover:bg-gray-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-gray-600 transition-colors pointer-coarse:h-11",
  // On the button, not the cell: the text colour of `today` sits on the cell.
  selected:
    "[&>button]:bg-gray-900 [&>button]:text-white [&>button:hover]:bg-gray-800",
  today: "font-bold text-moto-blue-strong",
  outside: "text-gray-300",
  disabled: "text-gray-300 cursor-not-allowed",
} as const;

/**
 * Week rows the calendar renders for `month` with weeks starting on Monday,
 * the setting of both kit pickers.
 */
export function mondayWeekRows(month: Date): number {
  const year = month.getFullYear();
  const monthIndex = month.getMonth();
  const leadingDays = (new Date(year, monthIndex, 1).getDay() + 6) % 7;
  const daysInMonth = new Date(year, monthIndex + 1, 0).getDate();
  return Math.ceil((leadingDays + daysInMonth) / 7);
}

/**
 * Placeholder with the calendar grid's footprint: per month a weekday row and
 * one bar per week, so the panel keeps its size when the grid arrives. The bars
 * mirror `DAY_PICKER_CLASSNAMES`, the outer classes the caller's `months` and
 * `month`.
 */
export function DayPickerSkeleton({
  month,
  numberOfMonths = 1,
  monthsClassName,
  monthClassName,
}: {
  readonly month: Date;
  readonly numberOfMonths?: number;
  readonly monthsClassName?: string;
  readonly monthClassName?: string;
}) {
  const months = Array.from(
    { length: numberOfMonths },
    (_, offset) => new Date(month.getFullYear(), month.getMonth() + offset, 1),
  );
  return (
    <div
      aria-hidden="true"
      data-day-picker-skeleton
      className={monthsClassName}
    >
      {months.map((first) => (
        // A flex column keeps the bars' vertical margins from collapsing into
        // each other; the grid's table rows add them up.
        <div
          key={first.getTime()}
          className={cn("flex flex-col", monthClassName)}
        >
          <Skeleton className="mb-1 h-4" />
          {Array.from({ length: mondayWeekRows(first) }, (_, week) => (
            <Skeleton key={week} shape="control" className="mt-1.5 h-9" />
          ))}
        </div>
      ))}
    </div>
  );
}
