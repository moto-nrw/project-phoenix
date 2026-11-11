"use client";

import { Suspense, useMemo, type ComponentProps } from "react";
import type { Locale } from "date-fns";
import { addMonths } from "date-fns/addMonths";
import { format } from "date-fns/format";
import { subMonths } from "date-fns/subMonths";
import { de } from "date-fns/locale/de";
import "react-day-picker/style.css";

import { parseISODate } from "~/lib/date-helpers";
import type { DatePickerLabels } from "~/lib/date-picker-labels";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { ListboxDropdown } from "./listbox-dropdown";
import {
  DAY_PICKER_CLASSNAMES,
  DayPickerSkeleton,
  LazyDayPicker,
} from "./lazy-day-picker";
import { Z_INDEX } from "./overlay-styles";

/**
 * The calendar shell of the kit. `DatePicker` and `DateRangePicker` render
 * their months through it, so the settings every kit calendar shares live
 * here once: weeks start on Monday, today is the school's day (Berlin), the
 * grid shows no navigation of its own, and a skeleton holds the grid's place
 * until the calendar module has loaded. `CalendarMonthHeader` is the head
 * above the grid, with its labels.
 */

// Labels default to German here because the staff and operator portals are
// German-only; the parent portal and public enrollment form pass translated
// strings alongside a matching `locale`. The DatePickerLabels contract itself
// lives in ~/lib/date-picker-labels so the hook that builds it stays inside
// src/lib.
const CALENDAR_LABELS = {
  clear: "Datum löschen",
  previousMonth: "Vorheriger Monat",
  nextMonth: "Nächster Monat",
  month: "Monat",
  year: "Jahr",
  selectedDays: (count: number) => `${count} Tage ausgewählt`,
} as const;

export type CalendarLabels = Required<DatePickerLabels>;

export function resolveCalendarLabels(labels?: DatePickerLabels) {
  return { ...CALENDAR_LABELS, ...labels };
}

type DayPickerProps = ComponentProps<typeof LazyDayPicker>;
type DistributiveOmit<T, K extends PropertyKey> = T extends unknown
  ? Omit<T, K>
  : never;

/** What a calendar decides itself; the shell sets the rest. */
type CalendarShellProps = DistributiveOmit<
  DayPickerProps,
  | "today"
  | "weekStartsOn"
  | "hideNavigation"
  | "month"
  | "onMonthChange"
  | "classNames"
> & {
  readonly month: Date;
  readonly onMonthChange: (month: Date) => void;
  /**
   * Layout classes of the grid. The look stays `DAY_PICKER_CLASSNAMES`; a
   * range calendar replaces `months`, `month` and `day`, and the skeleton
   * takes the same `months` and `month`.
   */
  readonly classNames?: Record<keyof typeof DAY_PICKER_CLASSNAMES, string>;
  /** Classes of the skeleton's month wrappers, for a multi-month grid. */
  readonly skeletonClassNames?: {
    readonly months?: string;
    readonly month?: string;
  };
};

export function CalendarShell({
  month,
  onMonthChange,
  classNames = DAY_PICKER_CLASSNAMES,
  skeletonClassNames,
  showOutsideDays = true,
  ...dayPicker
}: CalendarShellProps) {
  const today = useBerlinToday();
  return (
    <Suspense
      fallback={
        <DayPickerSkeleton
          month={month}
          numberOfMonths={dayPicker.numberOfMonths}
          monthsClassName={skeletonClassNames?.months}
          monthClassName={skeletonClassNames?.month}
        />
      }
    >
      <LazyDayPicker
        {...(dayPicker as DayPickerProps)}
        today={parseISODate(today)}
        month={month}
        onMonthChange={onMonthChange}
        weekStartsOn={1}
        showOutsideDays={showOutsideDays}
        hideNavigation
        classNames={classNames}
      />
    </Suspense>
  );
}

// Month names come from the active date-fns locale, never a hardcoded list, so
// the dropdown matches the calendar grid in every language.
function buildMonthLabels(locale: Locale): string[] {
  return Array.from({ length: 12 }, (_, index) =>
    format(new Date(2000, index, 1), "MMMM", { locale }),
  );
}

// Year span offered by the dropdowns when the caller sets no bounds. Back far
// enough for any birthday, forward far enough for a planning horizon.
const DEFAULT_YEARS_BACK = 100;
const DEFAULT_YEARS_AHEAD = 5;

// One above the calendar's own portal (`Z_LAYER.popover`), so the month/year
// menu opens on top of the calendar surface instead of behind it.
const NAV_MENU_Z_INDEX = Z_INDEX.popoverMenu;

// min-w keeps a 46px-wide year trigger from producing an unclickably narrow
// list. Surface and options are the kit listbox's own.
const NAV_MENU_CLASS = "min-w-24";

const NAV_SELECT_CLASS =
  "w-full min-w-0 cursor-pointer gap-1.5 overflow-hidden px-2.5 leading-5 font-medium";

// The offered years always include the month currently on screen, so a value
// outside the caller's bounds (legacy data) still shows its own year instead of
// silently snapping to the nearest allowed one.
function buildYearOptions(
  month: Date,
  minDate?: Date,
  maxDate?: Date,
): number[] {
  const thisYear = new Date().getFullYear();
  const first = Math.min(
    minDate?.getFullYear() ?? thisYear - DEFAULT_YEARS_BACK,
    month.getFullYear(),
  );
  const last = Math.max(
    maxDate?.getFullYear() ?? thisYear + DEFAULT_YEARS_AHEAD,
    month.getFullYear(),
  );
  return Array.from({ length: last - first + 1 }, (_, index) => first + index);
}

function ChevronPath({ direction }: { readonly direction: "back" | "next" }) {
  return (
    <path
      strokeLinecap="round"
      strokeLinejoin="round"
      strokeWidth={2}
      d={direction === "back" ? "M15 19l-7-7 7-7" : "M9 5l7 7-7 7"}
    />
  );
}

/**
 * Head above the calendar grid: month arrows and the caption. For one month
 * (`DatePicker`) the caption is the month, or month and year dropdowns when
 * the target date can be years away (birthdays). A range calendar names both
 * months it shows, between smaller arrows. Day-level bounds stay with the
 * DayPicker matchers, so navigating to a partly out-of-range month simply
 * shows disabled days.
 */
export function CalendarMonthHeader({
  variant = "single",
  compact = false,
  month,
  onMonthChange,
  monthYearNavigation = false,
  minDate,
  maxDate,
  locale = de,
  labels = CALENDAR_LABELS,
}: {
  /** `range`: two months side by side (`DateRangePicker`). */
  readonly variant?: "single" | "range";
  readonly compact?: boolean;
  readonly month: Date;
  readonly onMonthChange: (month: Date) => void;
  readonly monthYearNavigation?: boolean;
  readonly minDate?: Date;
  readonly maxDate?: Date;
  readonly locale?: Locale;
  readonly labels?: CalendarLabels;
}) {
  const monthLabels = useMemo(() => buildMonthLabels(locale), [locale]);

  if (variant === "range") {
    return (
      <div className="mb-3 flex items-center justify-between">
        <button
          type="button"
          onClick={() => onMonthChange(subMonths(month, 1))}
          aria-label={labels.previousMonth}
          className="rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
        >
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <ChevronPath direction="back" />
          </svg>
        </button>
        <span className="text-sm font-medium text-gray-900">
          {format(month, "MMMM yyyy", { locale })}
          {" - "}
          {format(addMonths(month, 1), "MMMM yyyy", { locale })}
        </span>
        <button
          type="button"
          onClick={() => onMonthChange(addMonths(month, 1))}
          aria-label={labels.nextMonth}
          className="rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
        >
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <ChevronPath direction="next" />
          </svg>
        </button>
      </div>
    );
  }

  return (
    <div
      className={`grid grid-cols-[2rem_minmax(0,1fr)_2rem] items-center ${
        compact ? "mb-3 gap-1" : "mb-4 gap-2"
      }`}
    >
      <button
        type="button"
        aria-label={labels.previousMonth}
        onClick={() => onMonthChange(subMonths(month, 1))}
        className="shrink-0 rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
      >
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <ChevronPath direction="back" />
        </svg>
      </button>
      {monthYearNavigation ? (
        // Kit dropdowns, not native <select>: a native select opens an
        // OS-level popup, which cannot be styled to match the kit and does not
        // exist in the DOM (so it never appears in a screenshot or a test).
        // The menu z-index has to clear the calendar's own portal.
        <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_5rem] items-center gap-1">
          <ListboxDropdown
            ariaLabel={labels.month}
            value={String(month.getMonth())}
            options={monthLabels.map((label, index) => ({
              value: String(index),
              label,
            }))}
            onChange={(next) =>
              onMonthChange(new Date(month.getFullYear(), Number(next), 1))
            }
            containerClassName="relative min-w-0"
            size="compact"
            className={NAV_SELECT_CLASS}
            menuClassName={NAV_MENU_CLASS}
            menuZIndex={NAV_MENU_Z_INDEX}
            portalScopeSelector="[data-date-picker-panel]"
            triggerRole="combobox"
          />
          <ListboxDropdown
            ariaLabel={labels.year}
            value={String(month.getFullYear())}
            options={buildYearOptions(month, minDate, maxDate).map((year) => ({
              value: String(year),
              label: String(year),
            }))}
            onChange={(next) =>
              onMonthChange(new Date(Number(next), month.getMonth(), 1))
            }
            containerClassName="relative min-w-0"
            size="compact"
            className={NAV_SELECT_CLASS}
            menuClassName={NAV_MENU_CLASS}
            menuZIndex={NAV_MENU_Z_INDEX}
            portalScopeSelector="[data-date-picker-panel]"
            triggerRole="combobox"
          />
        </div>
      ) : (
        <span className="truncate text-sm font-medium text-gray-900">
          {format(month, "MMMM yyyy", { locale })}
        </span>
      )}
      <button
        type="button"
        aria-label={labels.nextMonth}
        onClick={() => onMonthChange(addMonths(month, 1))}
        className="shrink-0 rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
      >
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <ChevronPath direction="next" />
        </svg>
      </button>
    </div>
  );
}
