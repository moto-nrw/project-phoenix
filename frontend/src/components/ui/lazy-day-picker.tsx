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
 * one bar per week, so the panel keeps its size when the grid arrives. The
 * class names mirror the caller's DayPicker `classNames`.
 */
export function DayPickerSkeleton({
  month,
  numberOfMonths = 1,
  monthsClassName,
  monthClassName,
  weekdaysClassName,
  weekClassName,
}: {
  readonly month: Date;
  readonly numberOfMonths?: number;
  readonly monthsClassName?: string;
  readonly monthClassName?: string;
  readonly weekdaysClassName: string;
  readonly weekClassName: string;
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
          <Skeleton className={weekdaysClassName} />
          {Array.from({ length: mondayWeekRows(first) }, (_, week) => (
            <Skeleton key={week} className={weekClassName} />
          ))}
        </div>
      ))}
    </div>
  );
}
