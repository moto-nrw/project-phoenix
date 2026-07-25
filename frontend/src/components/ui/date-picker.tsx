"use client";

import { useState, useRef, useEffect, useCallback, useMemo } from "react";
import { createPortal } from "react-dom";
import { DayPicker } from "react-day-picker";
import { format, addMonths, subMonths, type Locale } from "date-fns";
import { de } from "date-fns/locale";
import "react-day-picker/style.css";
import { isValidISODate, parseISODate, toISODate } from "~/lib/date-helpers";

// How the calendar is placed relative to the trigger:
// - "overlay": absolutely positioned inside the picker's own stacking context.
//   Cheapest option, but any ancestor with overflow hidden/auto clips it.
// - "inline": rendered in normal flow below the trigger. Never clipped, but it
//   pushes the surrounding content down while open.
// - "popover": rendered into document.body via a portal and positioned from the
//   trigger's viewport rect. Escapes clipping ancestors AND leaves the layout
//   untouched — the right choice inside scrollable panels/modals.
type CalendarLayout = "overlay" | "inline" | "popover";

/**
 * Control labels. The defaults are German because the staff and operator
 * portals are German-only; the parent portal and public enrollment form pass
 * translated strings alongside a matching `locale`.
 */
export interface DatePickerLabels {
  readonly clear?: string;
  readonly previousMonth?: string;
  readonly nextMonth?: string;
  readonly month?: string;
  readonly year?: string;
  /** Receives the count, e.g. (3) => "3 Tage ausgewählt". */
  readonly selectedDays?: (count: number) => string;
}

const DEFAULT_LABELS = {
  clear: "Datum löschen",
  previousMonth: "Vorheriger Monat",
  nextMonth: "Nächster Monat",
  month: "Monat",
  year: "Jahr",
  selectedDays: (count: number) => `${count} Tage ausgewählt`,
} as const;

function resolveLabels(labels?: DatePickerLabels) {
  return { ...DEFAULT_LABELS, ...labels };
}

// Viewport gap and calendar footprint used to flip/clamp the popover. The
// calendar is a fixed 7x6 grid of 32px days plus header and padding, so a
// constant is accurate enough and avoids a measure-then-reposition flash.
const POPOVER_MARGIN = 8;
const POPOVER_WIDTH = 268;
const POPOVER_HEIGHT = 340;

interface PopoverPosition {
  top: number;
  left: number;
}

// Places the calendar below the trigger, flipping above when the viewport
// bottom would cut it off, and clamps both axes into the viewport so the
// calendar stays fully reachable on small screens.
function computePopoverPosition(rect: DOMRect): PopoverPosition {
  let top = rect.bottom + 4;
  if (top + POPOVER_HEIGHT > window.innerHeight - POPOVER_MARGIN) {
    const above = rect.top - 4 - POPOVER_HEIGHT;
    top =
      above >= POPOVER_MARGIN
        ? above
        : Math.max(
            POPOVER_MARGIN,
            window.innerHeight - POPOVER_MARGIN - POPOVER_HEIGHT,
          );
  }
  const maxLeft = window.innerWidth - POPOVER_MARGIN - POPOVER_WIDTH;
  const left = Math.max(POPOVER_MARGIN, Math.min(rect.left, maxLeft));
  return { top, left };
}

type DatePickerProps =
  | {
      readonly mode?: "single";
      readonly value?: Date | null;
      readonly onChange: (date: Date | null) => void;
      readonly placeholder?: string;
      readonly className?: string;
      readonly dropdownPlacement?: "up" | "down";
      readonly calendarLayout?: CalendarLayout;
      // Earliest selectable day (inclusive). Days before it are disabled — used
      // to forbid past-date selection while keeping today choosable.
      readonly minDate?: Date;
      // Latest selectable day (inclusive). Days after it are disabled — used to
      // cap selection at a planning horizon while keeping that day choosable.
      readonly maxDate?: Date;
      /** Hide the inline clear ("X") control. Use when the value is required. */
      readonly hideClearButton?: boolean;
      // Swaps the "Juni 2026" caption for month + year dropdowns. Month arrows
      // alone need ~80 clicks to reach a birth year, so any field where the
      // target is years away from today should turn this on.
      readonly monthYearNavigation?: boolean;
      /** Blocks opening the calendar and greys the trigger. */
      readonly disabled?: boolean;
      // The trigger is a button, not an input: `id` lets an external <label
      // htmlFor> point at it, `invalid` paints the error border a native input
      // got from the browser, and ariaDescribedBy links the caller's error text.
      readonly id?: string;
      readonly ariaLabel?: string;
      readonly ariaDescribedBy?: string;
      readonly invalid?: boolean;
      // Calendar language. Defaults to German — the staff and operator portals
      // are German-only; the parent-facing surfaces pass their resolved locale.
      readonly locale?: Locale;
      // Overrides for the built-in German control labels, for those same
      // parent-facing surfaces.
      readonly labels?: DatePickerLabels;
    }
  | {
      readonly mode: "multiple";
      readonly values: Date[];
      readonly onChangeDates: (dates: Date[]) => void;
      readonly placeholder?: string;
      readonly className?: string;
      readonly dropdownPlacement?: "up" | "down";
      readonly calendarLayout?: CalendarLayout;
      readonly disabledDates?: Date[];
    };

interface MultipleDatePickerCalendarProps {
  readonly values: Date[];
  readonly onChangeDates: (dates: Date[]) => void;
  readonly disabledDates?: Date[];
  readonly dropdownPlacement: "up" | "down";
  readonly calendarLayout: CalendarLayout;
}

const EMPTY_DISABLED_DATES: Date[] = [];

export function DatePicker({
  placeholder = "Datum auswählen",
  className = "",
  dropdownPlacement = "up",
  calendarLayout = "overlay",
  ...props
}: DatePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);
  const [mounted, setMounted] = useState(false);
  // null until the trigger rect has been measured. The portal renders only once
  // a position exists, so the calendar never paints at the top-left corner
  // before an effect moves it into place.
  const [popoverPosition, setPopoverPosition] =
    useState<PopoverPosition | null>(null);
  const isPopover = calendarLayout === "popover";
  const isMultiple = props.mode === "multiple";
  const isDisabled = !isMultiple && props.disabled === true;
  const locale = isMultiple ? de : (props.locale ?? de);
  const labels = resolveLabels(isMultiple ? undefined : props.labels);
  const displayValue = isMultiple
    ? formatMultipleDateLabel(props.values, labels.selectedDays)
    : props.value
      ? format(props.value, "dd.MM.yyyy", { locale })
      : null;

  // Portals only exist client-side; render nothing on the server pass.
  useEffect(() => {
    setMounted(true);
  }, []);

  const syncPopoverPosition = useCallback(() => {
    if (!containerRef.current) return;
    setPopoverPosition(
      computePopoverPosition(containerRef.current.getBoundingClientRect()),
    );
  }, []);

  // Opening measures the trigger and shows the calendar in the SAME event, so
  // React commits both together and the portal's first paint is already in
  // place. Positioning from an effect instead would paint one frame at the
  // viewport origin first, which reads as the calendar flashing top-left.
  const toggleOpen = () => {
    if (isOpen) {
      setIsOpen(false);
      return;
    }
    if (isPopover) {
      syncPopoverPosition();
    }
    setIsOpen(true);
  };

  // Keep the position correct while the viewport changes. Any scroll (including
  // inside a filter panel or modal) closes the calendar: the trigger would
  // otherwise slide away from a portal that lives outside that scroll container
  // — the same trade-off the operator status dropdown makes.
  useEffect(() => {
    if (!isPopover || !isOpen) return;
    const close = () => setIsOpen(false);
    window.addEventListener("resize", syncPopoverPosition);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("resize", syncPopoverPosition);
      window.removeEventListener("scroll", close, true);
    };
  }, [isPopover, isOpen, syncPopoverPosition]);

  useEffect(() => {
    function handleOutsideInteraction(event: Event) {
      const target = event.target as Node;
      // The portalled calendar is NOT inside containerRef, so it has to be
      // treated as "inside" explicitly. Without this the pointerdown handler
      // unmounts the calendar before the day button's click lands and picking a
      // date silently does nothing.
      if (popoverRef.current?.contains(target)) {
        return;
      }
      if (containerRef.current && !containerRef.current.contains(target)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("pointerdown", handleOutsideInteraction, true);
    document.addEventListener("mousedown", handleOutsideInteraction, true);
    return () => {
      document.removeEventListener(
        "pointerdown",
        handleOutsideInteraction,
        true,
      );
      document.removeEventListener("mousedown", handleOutsideInteraction, true);
    };
  }, []);

  const calendar = isMultiple ? (
    <MultipleDatePickerCalendar
      values={props.values}
      onChangeDates={props.onChangeDates}
      disabledDates={props.disabledDates}
      dropdownPlacement={dropdownPlacement}
      calendarLayout={calendarLayout}
    />
  ) : (
    <DatePickerCalendar
      value={props.value}
      minDate={props.minDate}
      maxDate={props.maxDate}
      monthYearNavigation={props.monthYearNavigation}
      locale={locale}
      labels={labels}
      dropdownPlacement={dropdownPlacement}
      calendarLayout={calendarLayout}
      onChange={(date) => {
        props.onChange(date);
        setIsOpen(false);
      }}
    />
  );

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      <button
        type="button"
        id={isMultiple ? undefined : props.id}
        aria-label={isMultiple ? undefined : props.ariaLabel}
        // The trigger is a plain button, and ARIA does not allow aria-invalid /
        // aria-required on that role (oxlint jsx-a11y enforces it). The invalid
        // state is therefore carried visually plus by the caller's own error
        // text, and `aria-describedby` links the two when the caller passes it.
        aria-describedby={isMultiple ? undefined : props.ariaDescribedBy}
        disabled={isDisabled}
        onClick={toggleOpen}
        className={`flex w-full items-center justify-between rounded-lg border bg-white px-3 py-2 text-sm transition-all ${
          !isMultiple && props.invalid ? "border-[#FF3130]" : "border-gray-200"
        } ${
          isDisabled
            ? "cursor-not-allowed bg-gray-50 text-gray-400"
            : isOpen
              ? "border-gray-300 bg-gray-50"
              : "hover:bg-gray-50"
        }`}
      >
        <span className={displayValue ? "text-gray-900" : "text-gray-500"}>
          {displayValue ?? placeholder}
        </span>
        <div className="flex items-center gap-1">
          {displayValue &&
            !isDisabled &&
            !(!isMultiple && props.hideClearButton) && (
              <span
                role="button"
                tabIndex={0}
                onClick={(e) => {
                  e.stopPropagation();
                  if (isMultiple) {
                    props.onChangeDates([]);
                  } else {
                    props.onChange(null);
                  }
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.stopPropagation();
                    if (isMultiple) {
                      props.onChangeDates([]);
                    } else {
                      props.onChange(null);
                    }
                  }
                }}
                className="rounded p-0.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600"
                aria-label={labels.clear}
              >
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M6 18L18 6M6 6l12 12"
                  />
                </svg>
              </span>
            )}
          <svg
            className="h-4 w-4 text-gray-400"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"
            />
          </svg>
        </div>
      </button>

      {/* The overlay/inline layouts render in place; the popover layout renders
          the calendar into document.body at a viewport-fixed position. Portals
          only exist client-side, hence the `mounted` guard on that branch, and
          the position guard keeps the first paint from landing at the origin. */}
      {isOpen && !isPopover && calendar}
      {isOpen && isPopover && mounted && popoverPosition
        ? createPortal(
            <div
              ref={popoverRef}
              className="fixed z-[10001]"
              style={{ top: popoverPosition.top, left: popoverPosition.left }}
            >
              {calendar}
            </div>,
            document.body,
          )
        : null}
    </div>
  );
}

function formatMultipleDateLabel(
  values: Date[],
  selectedDays: (count: number) => string,
): string | null {
  if (values.length === 0) {
    return null;
  }
  if (values.length === 1) {
    return format(values[0]!, "dd.MM.yyyy", { locale: de });
  }
  return selectedDays(values.length);
}

function DatePickerCalendar({
  value,
  minDate,
  maxDate,
  monthYearNavigation,
  locale,
  labels,
  dropdownPlacement,
  calendarLayout,
  onChange,
}: {
  readonly value?: Date | null;
  readonly minDate?: Date;
  readonly maxDate?: Date;
  readonly monthYearNavigation?: boolean;
  readonly locale: Locale;
  readonly labels: Required<DatePickerLabels>;
  readonly dropdownPlacement: "up" | "down";
  readonly calendarLayout: CalendarLayout;
  readonly onChange: (date: Date | null) => void;
}) {
  const [month, setMonth] = useState(value ?? new Date());

  return (
    <div
      className={getCalendarContainerClass(dropdownPlacement, calendarLayout)}
    >
      <CalendarNavHeader
        month={month}
        onMonthChange={setMonth}
        monthYearNavigation={monthYearNavigation}
        minDate={minDate}
        maxDate={maxDate}
        locale={locale}
        labels={labels}
      />
      <DayPicker
        mode="single"
        selected={value ?? undefined}
        disabled={buildSingleDisabledMatchers(minDate, maxDate)}
        month={month}
        onMonthChange={setMonth}
        onSelect={(date) => onChange(date ?? null)}
        locale={locale}
        weekStartsOn={1}
        showOutsideDays
        hideNavigation
        classNames={{
          root: "text-sm",
          months: "flex flex-col",
          month: "",
          month_caption: "hidden",
          month_grid: "w-full border-collapse",
          weekdays: "flex",
          weekday: "text-gray-500 w-8 font-normal text-xs text-center",
          week: "flex w-full mt-1",
          day: "w-8 h-8 text-center text-sm p-0 relative",
          day_button:
            "w-8 h-8 rounded-lg hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-200 transition-colors",
          selected: "bg-gray-900 text-white hover:bg-gray-800 rounded-lg",
          today: "font-bold text-blue-600",
          outside: "text-gray-300",
          disabled: "text-gray-300 cursor-not-allowed",
        }}
      />
    </div>
  );
}

function MultipleDatePickerCalendar({
  values,
  onChangeDates,
  disabledDates = EMPTY_DISABLED_DATES,
  dropdownPlacement,
  calendarLayout,
}: MultipleDatePickerCalendarProps) {
  const [month, setMonth] = useState(values[0] ?? new Date());

  return (
    <div
      className={getCalendarContainerClass(dropdownPlacement, calendarLayout)}
    >
      <CalendarNavHeader month={month} onMonthChange={setMonth} />
      <DayPicker
        mode="multiple"
        selected={values}
        disabled={disabledDates}
        month={month}
        onMonthChange={setMonth}
        onSelect={(dates) => onChangeDates(dates ?? [])}
        locale={de}
        weekStartsOn={1}
        showOutsideDays
        hideNavigation
        classNames={{
          root: "text-sm",
          months: "flex flex-col",
          month: "",
          month_caption: "hidden",
          month_grid: "w-full border-collapse",
          weekdays: "flex",
          weekday: "text-gray-500 w-8 font-normal text-xs text-center",
          week: "flex w-full mt-1",
          day: "w-8 h-8 text-center text-sm p-0 relative",
          day_button:
            "w-8 h-8 rounded-lg hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-200 transition-colors",
          selected: "bg-gray-900 text-white hover:bg-gray-800 rounded-lg",
          today: "font-bold text-blue-600",
          outside: "text-gray-300",
          disabled: "text-gray-300 cursor-not-allowed",
        }}
      />
    </div>
  );
}

/**
 * DatePicker for the "YYYY-MM-DD" string shape the API speaks.
 *
 * Every form in the app stores calendar dates as ISO day strings, which is what
 * the replaced native `<input type="date">` round-tripped. Converting per call
 * site would mean 30+ copies of the same parse/format pair — and every one of
 * them a chance to reach for `.toISOString()` and land a day early in Berlin's
 * pre-02:00 window. This wrapper owns the conversion once, via the audited
 * helpers in `~/lib/date-helpers`.
 */
export function ISODatePicker({
  value,
  onChange,
  min,
  max,
  label,
  error,
  ...rest
}: {
  // Renders the same label and error markup as the kit `Input`, so a field that
  // used <Input label="Startdatum" type="date"> keeps its exact layout.
  readonly label?: string;
  readonly error?: string;
  /** "YYYY-MM-DD", or "" when unset. */
  readonly value: string;
  /** Emits "YYYY-MM-DD", or "" when cleared. */
  readonly onChange: (value: string) => void;
  /** Earliest selectable day as "YYYY-MM-DD". */
  readonly min?: string;
  /** Latest selectable day as "YYYY-MM-DD". */
  readonly max?: string;
  readonly placeholder?: string;
  readonly className?: string;
  readonly dropdownPlacement?: "up" | "down";
  readonly calendarLayout?: CalendarLayout;
  readonly monthYearNavigation?: boolean;
  readonly hideClearButton?: boolean;
  readonly disabled?: boolean;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly ariaDescribedBy?: string;
  readonly invalid?: boolean;
  readonly locale?: Locale;
  readonly labels?: DatePickerLabels;
}) {
  const errorId = error && rest.id ? `${rest.id}-error` : undefined;
  const picker = (
    <DatePicker
      {...rest}
      invalid={rest.invalid ?? Boolean(error)}
      ariaDescribedBy={rest.ariaDescribedBy ?? errorId}
      value={toDateOrNull(value)}
      minDate={toDateOrNull(min) ?? undefined}
      maxDate={toDateOrNull(max) ?? undefined}
      onChange={(date) => onChange(date ? toISODate(date) : "")}
    />
  );

  if (!label && !error) {
    return picker;
  }

  return (
    <div>
      {label && (
        <label
          htmlFor={rest.id}
          className="mb-2 block text-sm font-medium text-gray-700"
        >
          {label}
        </label>
      )}
      {picker}
      {error && (
        <p id={errorId} role="alert" className="mt-1 text-xs text-red-600">
          {error}
        </p>
      )}
    </div>
  );
}

// Accepts both a bare "YYYY-MM-DD" and a full backend timestamp
// ("2015-03-04T00:00:00Z"), because several master-data endpoints return the
// latter for what is semantically a calendar day. Taking the leading day
// substring is safe here — unlike `.toISOString()`, it never shifts the day,
// since the string is already the local calendar date the server sent.
function toDateOrNull(value: string | undefined): Date | null {
  if (!value) return null;
  const day = value.slice(0, 10);
  return isValidISODate(day) ? parseISODate(day) : null;
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

const NAV_SELECT_CLASS =
  "rounded-md border border-gray-200 bg-white px-1.5 py-1 text-sm font-medium text-gray-900 hover:bg-gray-50 focus:border-gray-300 focus:ring-2 focus:ring-gray-200 focus:outline-none";

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

// Shared caption for both calendars: month arrows plus, when the target date
// can be years away (birthdays), month and year dropdowns in place of the
// static caption. Day-level bounds stay with the DayPicker matchers, so
// navigating to a partly out-of-range month simply shows disabled days.
function CalendarNavHeader({
  month,
  onMonthChange,
  monthYearNavigation = false,
  minDate,
  maxDate,
  locale = de,
  labels = DEFAULT_LABELS,
}: {
  readonly month: Date;
  readonly onMonthChange: (month: Date) => void;
  readonly monthYearNavigation?: boolean;
  readonly minDate?: Date;
  readonly maxDate?: Date;
  readonly locale?: Locale;
  readonly labels?: Required<DatePickerLabels>;
}) {
  const monthLabels = useMemo(() => buildMonthLabels(locale), [locale]);

  return (
    <div className="mb-3 flex items-center justify-between gap-1">
      <button
        type="button"
        aria-label={labels.previousMonth}
        onClick={() => onMonthChange(subMonths(month, 1))}
        className="rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
      >
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M15 19l-7-7 7-7"
          />
        </svg>
      </button>
      {monthYearNavigation ? (
        <div className="flex items-center gap-1">
          <select
            aria-label={labels.month}
            value={month.getMonth()}
            onChange={(event) =>
              onMonthChange(
                new Date(month.getFullYear(), Number(event.target.value), 1),
              )
            }
            className={NAV_SELECT_CLASS}
          >
            {monthLabels.map((label, index) => (
              <option key={label} value={index}>
                {label}
              </option>
            ))}
          </select>
          <select
            aria-label={labels.year}
            value={month.getFullYear()}
            onChange={(event) =>
              onMonthChange(
                new Date(Number(event.target.value), month.getMonth(), 1),
              )
            }
            className={NAV_SELECT_CLASS}
          >
            {buildYearOptions(month, minDate, maxDate).map((year) => (
              <option key={year} value={year}>
                {year}
              </option>
            ))}
          </select>
        </div>
      ) : (
        <span className="text-sm font-medium text-gray-900">
          {format(month, "MMMM yyyy", { locale })}
        </span>
      )}
      <button
        type="button"
        aria-label={labels.nextMonth}
        onClick={() => onMonthChange(addMonths(month, 1))}
        className="rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
      >
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 5l7 7-7 7"
          />
        </svg>
      </button>
    </div>
  );
}

// Combines the optional lower/upper bounds into react-day-picker matchers.
// An empty matcher list disables nothing, so an unbounded picker keeps every
// day selectable.
function buildSingleDisabledMatchers(
  minDate?: Date,
  maxDate?: Date,
): ({ before: Date } | { after: Date })[] {
  const matchers: ({ before: Date } | { after: Date })[] = [];
  if (minDate) matchers.push({ before: minDate });
  if (maxDate) matchers.push({ after: maxDate });
  return matchers;
}

function getCalendarContainerClass(
  dropdownPlacement: "up" | "down",
  calendarLayout: CalendarLayout,
): string {
  const base = "rounded-xl border border-gray-200 bg-white p-3 shadow-lg";
  if (calendarLayout === "inline") {
    return `${base} mt-2 w-fit max-w-full`;
  }
  // The portal wrapper owns placement; the card only needs its own surface.
  if (calendarLayout === "popover") {
    return `${base} w-fit`;
  }
  const placement =
    dropdownPlacement === "down" ? "top-full mt-1" : "bottom-full mb-1";
  return `${base} absolute left-0 z-[10001] ${placement}`;
}
