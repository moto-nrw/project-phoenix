"use client";

import { useState, useRef, useEffect, useCallback, useMemo } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import { DayPicker } from "react-day-picker";
import { format, addMonths, subMonths, type Locale } from "date-fns";
import { de } from "date-fns/locale";
import "react-day-picker/style.css";
import { isValidISODate, parseISODate, toISODate } from "~/lib/date-helpers";
import type { DatePickerLabels } from "~/lib/date-picker-labels";
import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
import {
  clampCalendarWidth,
  computeCalendarPanelPosition,
  type PanelGeometry,
} from "~/components/ui/calendar-panel-position";

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
 * Trigger height. The picker replaced native inputs that sat next to kit
 * `Input`s, so it has to be able to match their two sizes — a 38px trigger
 * beside a 48px input reads as a broken row.
 *
 * - "sm"  (default) the picker's original compact trigger, used by the filter
 *         bars and dropdowns that adopted it first
 * - "md"  matches `Input controlSize="compact"` (h-10)
 * - "lg"  matches the default `Input` (px-4 py-3 text-base)
 */
type ControlSize = "sm" | "md" | "lg";

// "lg" is py-[11px] rather than py-3: the kit Input draws its outline with
// border-0 plus an inset ring, which costs no layout height, while this trigger
// uses a real 1px border. Matching py-3 would leave it 2px taller than the
// input next to it.
const TRIGGER_SIZE_CLASS: Record<ControlSize, string> = {
  sm: "px-3 py-2 text-sm",
  md: "h-10 px-3 py-2 text-sm",
  lg: "px-4 py-[11px] text-base",
};

// Labels default to German here because the staff and operator portals are
// German-only; the parent portal and public enrollment form pass translated
// strings alongside a matching `locale`. The DatePickerLabels contract itself
// lives in ~/lib/date-picker-labels so the hook that builds it stays inside
// src/lib.
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

// Geometry lives in calendar-panel-position.ts so this picker and the range
// picker cannot drift apart again.
type PopoverPosition = PanelGeometry;

function computePopoverPosition(rect: DOMRect): PopoverPosition {
  const viewport = { width: window.innerWidth, height: window.innerHeight };
  return computeCalendarPanelPosition(
    rect,
    clampCalendarWidth(rect.width, viewport.width),
    viewport,
  );
}

type DatePickerProps =
  | {
      readonly mode?: "single";
      readonly value?: Date | null;
      readonly onChange: (date: Date | null) => void;
      readonly placeholder?: string;
      readonly className?: string;
      /**
       * @deprecated No longer read. The panel measures the viewport and flips
       * up or down on its own; callers cannot know which side fits. Kept so the
       * existing call sites keep compiling.
       */
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
      /** Keeps a selected day from being deselected in the calendar. */
      readonly required?: boolean;
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
      /** Trigger height. Match the kit Input next to it — see ControlSize. */
      readonly controlSize?: ControlSize;
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
      /** @deprecated No longer read — see the single-mode prop. */
      readonly dropdownPlacement?: "up" | "down";
      readonly calendarLayout?: CalendarLayout;
      readonly disabledDates?: Date[];
    };

interface MultipleDatePickerCalendarProps {
  readonly values: Date[];
  readonly onChangeDates: (dates: Date[]) => void;
  readonly disabledDates?: Date[];
  readonly compact: boolean;
  readonly calendarLayout: CalendarLayout;
}

const EMPTY_DISABLED_DATES: Date[] = [];

export function DatePicker({
  placeholder = "Datum auswählen",
  className = "",
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
  // "overlay" used to position itself with CSS alone (absolute left-0), which
  // meant no viewport awareness and a second alignment rule in the kit. Both
  // floating layouts now go through the measured portal path, so every calendar
  // panel in the app obeys the same geometry; "inline" stays in normal flow.
  const isPopover = calendarLayout !== "inline";
  const isMultiple = props.mode === "multiple";
  const isDisabled = !isMultiple && props.disabled === true;
  // Known only after the trigger is measured; until then assume the roomy
  // variant, which is what every field wider than a filter chip gets.
  const isCompactPanel =
    popoverPosition !== null && popoverPosition.width < COMPACT_CARD_MAX_WIDTH;
  const locale = isMultiple ? de : (props.locale ?? de);
  const labels = resolveLabels(isMultiple ? undefined : props.labels);
  const displayValue = isMultiple
    ? formatMultipleDateLabel(props.values, labels.selectedDays)
    : props.value
      ? // Deliberately a fixed German pattern, not the locale's own: the
        // calendar body is translated, but the trigger text stays dd.MM.yyyy
        // everywhere. Switching it to "P" is a separate, test-pinned decision.
        format(props.value, "dd.MM.yyyy", { locale })
      : null;

  // Portals only exist client-side; render nothing on the server pass.
  useEffect(() => {
    setMounted(true);
  }, []);

  // A caller can disable the field while an async submission is in flight.
  // The portal lives outside the disabled trigger, so close it explicitly and
  // keep it out of the render tree in the same commit.
  useEffect(() => {
    if (isDisabled) setIsOpen(false);
  }, [isDisabled]);

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
    const close = (event: Event) => {
      // Scrolling the year list (101 entries) must not count: that menu is a
      // portal of our own header, and closing on its scroll would make the
      // dropdown unusable.
      const target = event.target;
      if (
        target instanceof Element &&
        target.closest('[role="listbox"]') !== null
      ) {
        return;
      }
      setIsOpen(false);
    };
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
      // The month/year dropdowns portal their menus to body too, so an option
      // click lands outside both refs. Without this the calendar would close on
      // pointerdown and the month change would never apply.
      if (
        target instanceof Element &&
        target.closest('[role="listbox"]') !== null
      ) {
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
      compact={isCompactPanel}
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
      compact={isCompactPanel}
      calendarLayout={calendarLayout}
      required={props.required}
      onChange={(date) => {
        props.onChange(date);
        setIsOpen(false);
      }}
    />
  );

  // A portaled panel must remain inside a modal's FocusScope: portaling to
  // document.body puts its day and month controls outside the trap, which
  // redirects keyboard focus back to the trigger. Modal wrappers have no
  // transform or overflow clipping, so they can host the fixed panel directly.
  // Vaul slide-overs do transform their content, so those keep a body portal
  // and receive their own FocusScope below instead.
  const portalContainer =
    typeof document === "undefined"
      ? null
      : (containerRef.current?.closest("[data-modal-focus-scope]") ??
        document.body);
  const needsPortalledFocusScope =
    containerRef.current?.closest("[data-date-picker-focus-trap]") !== null;

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
        // The background is picked in exactly one place: a base `bg-white` plus
        // a conditional `bg-gray-50` are two utilities of equal specificity, and
        // which one wins depends on Tailwind's output order — the disabled state
        // silently rendered white.
        className={`flex w-full items-center justify-between rounded-lg border transition-all ${
          TRIGGER_SIZE_CLASS[
            (isMultiple ? undefined : props.controlSize) ?? "sm"
          ]
        } ${
          !isMultiple && props.invalid ? "border-[#FF3130]" : "border-gray-200"
        } ${
          isDisabled
            ? "cursor-not-allowed bg-gray-50 text-gray-400"
            : isOpen
              ? "border-gray-300 bg-gray-50"
              : "bg-white hover:bg-gray-50"
        }`}
      >
        <span className={displayValue ? "text-gray-900" : "text-gray-500"}>
          {displayValue ?? placeholder}
        </span>
        <div className="flex items-center gap-1">
          {displayValue &&
            !isDisabled &&
            !(!isMultiple && (props.hideClearButton || props.required)) && (
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
      {isOpen && !isDisabled && !isPopover && calendar}
      {isOpen &&
      !isDisabled &&
      isPopover &&
      mounted &&
      popoverPosition &&
      portalContainer
        ? createPortal(
            needsPortalledFocusScope ? (
              <FocusScope asChild loop trapped>
                <div
                  ref={popoverRef}
                  className="fixed z-[10001]"
                  style={{
                    top: popoverPosition.top,
                    left: popoverPosition.left,
                    width: popoverPosition.width,
                    pointerEvents: "auto",
                  }}
                >
                  {calendar}
                </div>
              </FocusScope>
            ) : (
              <div
                ref={popoverRef}
                className="fixed z-[10001]"
                style={{
                  top: popoverPosition.top,
                  left: popoverPosition.left,
                  width: popoverPosition.width,
                  pointerEvents: "auto",
                }}
              >
                {calendar}
              </div>
            ),
            portalContainer,
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
  compact,
  calendarLayout,
  required,
  onChange,
}: {
  readonly value?: Date | null;
  readonly minDate?: Date;
  readonly maxDate?: Date;
  readonly monthYearNavigation?: boolean;
  readonly locale: Locale;
  readonly labels: Required<DatePickerLabels>;
  readonly compact: boolean;
  readonly calendarLayout: CalendarLayout;
  readonly required?: boolean;
  readonly onChange: (date: Date | null) => void;
}) {
  const [month, setMonth] = useState(value ?? new Date());

  return (
    <div
      data-date-picker-panel
      className={getCalendarContainerClass(calendarLayout, compact)}
    >
      <CalendarNavHeader
        compact={compact}
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
        required={required}
        disabled={buildSingleDisabledMatchers(minDate, maxDate)}
        month={month}
        onMonthChange={setMonth}
        onSelect={(date: Date | undefined) => onChange(date ?? null)}
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
          // flex-1 + min-w-0 lets the seven columns take exactly the width the
          // card has, in both directions: a wide field gets a wide grid, a
          // narrow one gets narrow cells rather than forcing the card wider
          // than the field it belongs to.
          weekday:
            "text-gray-500 flex-1 min-w-0 font-normal text-xs text-center pb-1",
          week: "flex w-full mt-1.5",
          // px-1 on the cell keeps a 8px gutter between two columns, and the
          // selection is painted on the inner button rather than the cell —
          // otherwise the cell background bleeds into its own padding and two
          // neighbouring selected days fuse into one continuous dark bar.
          day: "flex-1 min-w-0 h-9 px-1 text-center text-sm py-0 relative",
          day_button:
            "w-full h-9 rounded-lg hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-200 transition-colors",
          selected:
            "text-white [&>button]:bg-gray-900 [&>button:hover]:bg-gray-800",
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
  compact,
  calendarLayout,
}: MultipleDatePickerCalendarProps) {
  const [month, setMonth] = useState(values[0] ?? new Date());

  return (
    <div className={getCalendarContainerClass(calendarLayout, compact)}>
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
          // flex-1 + min-w-0 lets the seven columns take exactly the width the
          // card has, in both directions: a wide field gets a wide grid, a
          // narrow one gets narrow cells rather than forcing the card wider
          // than the field it belongs to.
          weekday:
            "text-gray-500 flex-1 min-w-0 font-normal text-xs text-center pb-1",
          week: "flex w-full mt-1.5",
          // px-1 on the cell keeps a 8px gutter between two columns, and the
          // selection is painted on the inner button rather than the cell —
          // otherwise the cell background bleeds into its own padding and two
          // neighbouring selected days fuse into one continuous dark bar.
          day: "flex-1 min-w-0 h-9 px-1 text-center text-sm py-0 relative",
          day_button:
            "w-full h-9 rounded-lg hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-200 transition-colors",
          selected:
            "text-white [&>button]:bg-gray-900 [&>button:hover]:bg-gray-800",
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
  /** Prevents clearing an already selected date from the calendar. */
  readonly required?: boolean;
  readonly disabled?: boolean;
  readonly id?: string;
  readonly ariaLabel?: string;
  readonly ariaDescribedBy?: string;
  readonly invalid?: boolean;
  readonly controlSize?: ControlSize;
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
      required={rest.required}
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
        <p id={errorId} role="alert" className="mt-1 text-xs text-[#FF3130]">
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

// One above the calendar's own portal (z-[10001]), so the month/year menu opens
// on top of the calendar surface instead of behind it.
const NAV_MENU_Z_INDEX = 10002;

// The menu surface is the caller's job in this kit component — without these it
// renders as unstyled text floating over the calendar. min-w keeps a 46px-wide
// year trigger from producing an unclickably narrow list.
const NAV_MENU_CLASS =
  "scrollbar-thin min-w-24 overflow-y-auto rounded-xl border border-gray-200 bg-white py-1 shadow-lg";
const NAV_OPTION_CLASS =
  "flex w-full cursor-pointer items-center gap-2 px-4 py-2 text-left text-sm text-gray-700 transition-colors hover:bg-gray-50";
const NAV_OPTION_ACTIVE_CLASS =
  "flex w-full cursor-pointer items-center gap-2 bg-gray-50 px-4 py-2 text-left text-sm font-medium text-gray-900 transition-colors";

const NAV_SELECT_CLASS =
  "inline-flex h-9 min-w-0 cursor-pointer items-center gap-2 rounded-lg border border-gray-200 bg-white px-2.5 text-sm leading-5 font-medium text-gray-900 shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";

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
  compact = false,
  month,
  onMonthChange,
  monthYearNavigation = false,
  minDate,
  maxDate,
  locale = de,
  labels = DEFAULT_LABELS,
}: {
  readonly compact?: boolean;
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
    <div
      className={`flex items-center justify-between ${
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
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M15 19l-7-7 7-7"
          />
        </svg>
      </button>
      {monthYearNavigation ? (
        // Kit dropdowns, not native <select>: a native select opens an
        // OS-level popup, which cannot be styled to match the kit and does not
        // exist in the DOM (so it never appears in a screenshot or a test).
        // The menu z-index has to clear the calendar's own portal.
        <div className="flex min-w-0 items-center gap-1">
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
            className={NAV_SELECT_CLASS}
            menuClassName={NAV_MENU_CLASS}
            optionClassName={NAV_OPTION_CLASS}
            activeOptionClassName={NAV_OPTION_ACTIVE_CLASS}
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
            className={NAV_SELECT_CLASS}
            menuClassName={NAV_MENU_CLASS}
            optionClassName={NAV_OPTION_CLASS}
            activeOptionClassName={NAV_OPTION_ACTIVE_CLASS}
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

// A panel narrower than this cannot afford the roomier padding: at 176px, p-4
// leaves 20px day cells and truncates the "Juli 2026" caption. Below the
// threshold the card falls back to p-3 and spends the space on the grid instead.
const COMPACT_CARD_MAX_WIDTH = 240;

function getCalendarContainerClass(
  calendarLayout: CalendarLayout,
  compact: boolean,
): string {
  const base = `rounded-xl border border-gray-200 bg-white shadow-lg ${
    compact ? "p-3" : "p-4"
  }`;
  // The floating layouts get their width from the portal wrapper, which is
  // sized to the trigger; inline takes its parent's width and only needs the
  // legible-minimum floor.
  if (calendarLayout === "inline") {
    return `${base} mt-2 w-full min-w-[176px]`;
  }
  return `${base} w-full`;
}
