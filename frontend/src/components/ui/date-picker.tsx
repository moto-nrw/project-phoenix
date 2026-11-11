"use client";

import { FieldLabel } from "./field-label";
import { Z_LAYER } from "./overlay-styles";
import { useState, useRef, useEffect, useCallback, useId } from "react";
import { createPortal } from "react-dom";
import { FocusScope } from "@radix-ui/react-focus-scope";
import type { Matcher } from "react-day-picker";
import type { Locale } from "date-fns";
import { format } from "date-fns/format";
import { de } from "date-fns/locale/de";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import {
  formatDate,
  isValidISODate,
  parseGermanDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import type { DatePickerLabels } from "~/lib/date-picker-labels";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { cn } from "~/lib/utils";
import {
  MODAL_SCOPE_SELECTOR,
  OverlayOwnerProvider,
  SCROLL_AREA_PROPS,
  SHEET_SCOPE_SELECTOR,
} from "./overlay-stack";
import { useAnchoredPanel } from "./anchored-panel";
import { PANEL_GAP, PANEL_VIEWPORT_MARGIN } from "./panel-geometry";
import { preloadDayPicker } from "~/components/ui/lazy-day-picker";
import {
  CalendarMonthHeader,
  CalendarShell,
  resolveCalendarLabels,
  type CalendarLabels,
} from "./calendar-shell";
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
// - "popover-below": like "popover", but stays below the trigger while at
//   least a usable, internally scrollable calendar height remains.
type CalendarLayout = "overlay" | "inline" | "popover" | "popover-below";

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

// Geometry lives in calendar-panel-position.ts so this picker and the range
// picker cannot drift apart again.
type PopoverPosition = PanelGeometry & { readonly maxHeight?: number };
const MIN_USABLE_CALENDAR_HEIGHT = 240;

function computePopoverPosition(
  rect: DOMRect,
  prefersBelow: boolean,
): PopoverPosition {
  const viewport = { width: window.innerWidth, height: window.innerHeight };
  const width = clampCalendarWidth(rect.width, viewport.width);
  // A 240px panel keeps the header and several full week rows usable. Below
  // that floor, the normal flip-up rule is safer than a tiny scroll slit.
  const availableBelow =
    viewport.height - PANEL_VIEWPORT_MARGIN - (rect.bottom + PANEL_GAP);
  const canStayBelow =
    prefersBelow && availableBelow >= MIN_USABLE_CALENDAR_HEIGHT;
  const position = canStayBelow
    ? computeCalendarPanelPosition(
        rect,
        width,
        viewport,
        MIN_USABLE_CALENDAR_HEIGHT,
      )
    : computeCalendarPanelPosition(rect, width, viewport);
  const opensBelow = position.top >= rect.bottom;
  return canStayBelow && opensBelow
    ? {
        ...position,
        maxHeight: Math.max(
          0,
          viewport.height - position.top - PANEL_VIEWPORT_MARGIN,
        ),
      }
    : position;
}

type DatePickerProps =
  | {
      readonly mode?: "single";
      readonly value?: Date | null;
      readonly onChange: (date: Date | null) => void;
      readonly placeholder?: string;
      readonly className?: string;
      /** Optional visual treatment for compact filter toolbars. */
      readonly triggerClassName?: string;
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
      /**
       * Month the calendar opens on while no value is set, e.g. the start
       * date for an end-date field. Without it the calendar opens on today.
       */
      readonly defaultMonth?: Date;
      /** Additional days that cannot be selected. */
      readonly disabledDay?: Matcher;
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
      /**
       * Field name a server field error (`errors[].field`) refers to. Renders
       * a hidden input with the value, so the shared error path can find the
       * field and focus its trigger (#2516).
       */
      readonly name?: string;
      readonly ariaLabel?: string;
      readonly ariaDescribedBy?: string;
      /**
       * A line under the field that stays visible, read out with the trigger.
       * Same look and place as the `hint` of the kit `Input`.
       */
      readonly hint?: string;
      readonly invalid?: boolean;
      /** Trigger height. Match the kit Input next to it — see ControlSize. */
      readonly controlSize?: ControlSize;
      // Calendar language. Defaults to German — the staff and operator portals
      // are German-only; the parent-facing surfaces pass their resolved locale.
      readonly locale?: Locale;
      // Overrides for the built-in German control labels, for those same
      // parent-facing surfaces.
      readonly labels?: DatePickerLabels;
      readonly iconOnly?: boolean;
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
  const fallbackId = useId();
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
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
  const prefersBelow = calendarLayout === "popover-below";
  const isMultiple = props.mode === "multiple";
  const isDisabled = !isMultiple && props.disabled === true;
  // Known only after the trigger is measured; until then assume the roomy
  // variant, which is what every field wider than a filter chip gets.
  const isCompactPanel =
    calendarLayout === "inline" ||
    (popoverPosition !== null &&
      popoverPosition.width < COMPACT_CARD_MAX_WIDTH);
  const locale = isMultiple ? de : (props.locale ?? de);
  const labels = resolveCalendarLabels(isMultiple ? undefined : props.labels);
  const displayValue = isMultiple
    ? formatMultipleDateLabel(props.values, labels.selectedDays)
    : props.value
      ? // Deliberately a fixed German pattern, not the locale's own: the
        // calendar body is translated, but the trigger text stays dd.MM.yyyy
        // everywhere. Switching it to "P" is a separate, test-pinned decision.
        format(props.value, "dd.MM.yyyy", { locale })
      : null;
  const iconOnly = !isMultiple && props.iconOnly === true;
  const hint = isMultiple ? undefined : props.hint;
  const hintId =
    !isMultiple && hint ? `${props.id ?? fallbackId}-hint` : undefined;
  const triggerDescribedBy = isMultiple
    ? undefined
    : [props.ariaDescribedBy, hintId].filter(Boolean).join(" ") || undefined;
  const canClear =
    Boolean(displayValue) &&
    !iconOnly &&
    !isDisabled &&
    !(!isMultiple && (props.hideClearButton || props.required));

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
      computePopoverPosition(
        containerRef.current.getBoundingClientRect(),
        prefersBelow,
      ),
    );
  }, [prefersBelow]);

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

  // The engine closes the calendar on a press outside and on Escape. It takes
  // its place in the overlay stack (#6732): over the overlay it opened from,
  // under one that opens later. It closes when that overlay closes or rests,
  // or when a dialog, side panel or tour opens above it, as the click that
  // usually opens them would have. Any scroll outside the calendar (page,
  // filter panel, modal) closes the floating calendar: the trigger would
  // otherwise slide away from a portal that lives outside that scroll
  // container. Its own scroll on a short viewport and the scroll of its
  // month and year lists do not count.
  const closeCovered = useCallback(() => setIsOpen(false), []);
  const { owner } = useAnchoredPanel({
    open: isOpen && !isDisabled,
    onClose: (reason) => {
      setIsOpen(false);
      if (reason === "escape") triggerRef.current?.focus();
    },
    anchorRef: containerRef,
    panelRef: popoverRef,
    onScroll: isPopover ? "close" : "follow",
    reposition: isPopover ? syncPopoverPosition : undefined,
    onCovered: closeCovered,
  });

  const calendarContent = isMultiple ? (
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
      defaultMonth={props.defaultMonth}
      minDate={props.minDate}
      maxDate={props.maxDate}
      disabledDay={props.disabledDay}
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
  // Selects in the calendar head belong to the calendar.
  const calendar = (
    <OverlayOwnerProvider owner={owner}>{calendarContent}</OverlayOwnerProvider>
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
      : (containerRef.current?.closest(MODAL_SCOPE_SELECTOR) ?? document.body);
  const needsPortalledFocusScope =
    containerRef.current?.closest(SHEET_SCOPE_SELECTOR) !== null;

  const floatingPanel = popoverPosition ? (
    <div
      ref={popoverRef}
      className={`fixed ${Z_LAYER.popover} max-h-[calc(100dvh-1rem)] overflow-x-hidden overflow-y-auto overscroll-contain`}
      style={{
        top: popoverPosition.top,
        left: popoverPosition.left,
        width: popoverPosition.width,
        maxHeight: popoverPosition.maxHeight,
        pointerEvents: "auto",
      }}
    >
      {calendar}
    </div>
  ) : null;

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      {!isMultiple && props.name ? (
        <input
          type="hidden"
          name={props.name}
          value={props.value ? toISODate(props.value) : ""}
        />
      ) : null}
      <div className="flex items-center gap-1" data-date-picker-controls>
        <button
          ref={triggerRef}
          type="button"
          id={isMultiple ? undefined : props.id}
          aria-label={
            isMultiple
              ? undefined
              : (props.ariaLabel ?? (iconOnly ? "Kalender öffnen" : undefined))
          }
          aria-expanded={isOpen}
          aria-describedby={triggerDescribedBy}
          data-form-invalid={!isMultiple && props.invalid ? true : undefined}
          disabled={isDisabled}
          onClick={toggleOpen}
          // Fetch the calendar module before the click, so opening does not
          // wait for it.
          onPointerEnter={preloadDayPicker}
          onFocus={preloadDayPicker}
          className={cn(
            "flex items-center rounded-lg border transition-all",
            iconOnly
              ? "h-10 w-10 shrink-0 justify-center"
              : `min-w-0 flex-1 justify-between ${
                  TRIGGER_SIZE_CLASS[
                    (isMultiple ? undefined : props.controlSize) ?? "sm"
                  ]
                }`,
            !isMultiple && props.invalid
              ? "border-moto-red"
              : "border-gray-200",
            isDisabled
              ? "cursor-not-allowed bg-gray-50 text-gray-400"
              : isOpen
                ? "border-gray-300 bg-gray-50"
                : "bg-white hover:bg-gray-50",
            !isMultiple ? props.triggerClassName : undefined,
          )}
        >
          {!iconOnly && (
            <span className={displayValue ? "text-gray-900" : "text-gray-500"}>
              {displayValue ?? placeholder}
            </span>
          )}
          <svg
            className="h-4 w-4 shrink-0 text-gray-400"
            aria-hidden="true"
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
        </button>
        {canClear && (
          <Button
            type="button"
            variant="surface"
            size="icon"
            // Neben dem großen Feld (44 px) so hoch wie das Feld (#5608).
            touchTarget={!isMultiple && props.controlSize === "lg"}
            onClick={() => {
              if (isMultiple) {
                props.onChangeDates([]);
              } else {
                props.onChange(null);
              }
            }}
            className="shrink-0"
            aria-label={labels.clear}
          >
            <svg
              className="h-4 w-4"
              aria-hidden="true"
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
          </Button>
        )}
      </div>
      {hint && (
        <p id={hintId} className="mt-1 text-xs text-gray-600">
          {hint}
        </p>
      )}

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
                {floatingPanel}
              </FocusScope>
            ) : (
              floatingPanel
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
  defaultMonth,
  minDate,
  maxDate,
  disabledDay,
  monthYearNavigation,
  locale,
  labels,
  compact,
  calendarLayout,
  required,
  onChange,
}: {
  readonly value?: Date | null;
  readonly defaultMonth?: Date;
  readonly minDate?: Date;
  readonly maxDate?: Date;
  readonly disabledDay?: Matcher;
  readonly monthYearNavigation?: boolean;
  readonly locale: Locale;
  readonly labels: CalendarLabels;
  readonly compact: boolean;
  readonly calendarLayout: CalendarLayout;
  readonly required?: boolean;
  readonly onChange: (date: Date | null) => void;
}) {
  const today = useBerlinToday();
  const [month, setMonth] = useState(
    value ?? defaultMonth ?? parseISODate(today),
  );
  const controlledMonthTime = value?.getTime();

  useEffect(() => {
    if (controlledMonthTime === undefined) return;
    setMonth(new Date(controlledMonthTime));
  }, [controlledMonthTime]);

  return (
    <div
      data-date-picker-panel
      {...SCROLL_AREA_PROPS}
      className={getCalendarContainerClass(calendarLayout, compact)}
    >
      <CalendarMonthHeader
        compact={compact}
        month={month}
        onMonthChange={setMonth}
        monthYearNavigation={monthYearNavigation}
        minDate={minDate}
        maxDate={maxDate}
        locale={locale}
        labels={labels}
      />
      <CalendarShell
        mode="single"
        selected={value ?? undefined}
        required={required}
        disabled={buildSingleDisabledMatchers(minDate, maxDate, disabledDay)}
        month={month}
        onMonthChange={setMonth}
        onSelect={(date: Date | undefined) => onChange(date ?? null)}
        locale={locale}
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
  const today = useBerlinToday();
  const [month, setMonth] = useState(values[0] ?? parseISODate(today));

  return (
    <div
      data-date-picker-panel
      {...SCROLL_AREA_PROPS}
      className={getCalendarContainerClass(calendarLayout, compact)}
    >
      <CalendarMonthHeader month={month} onMonthChange={setMonth} />
      <CalendarShell
        mode="multiple"
        selected={values}
        disabled={disabledDates}
        month={month}
        onMonthChange={setMonth}
        onSelect={(dates: Date[] | undefined) => onChangeDates(dates ?? [])}
        locale={de}
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
  defaultMonth,
  disabledDay,
  label,
  error,
  hint,
  ...rest
}: {
  // Renders the same label, hint and error markup as the kit `Input`, so a
  // field that used <Input label="Startdatum" type="date"> keeps its layout.
  readonly label?: string;
  readonly error?: string;
  /** See `DatePicker` `hint`: stays visible under the field, above `error`. */
  readonly hint?: string;
  /** "YYYY-MM-DD", or "" when unset. */
  readonly value: string;
  /** Emits "YYYY-MM-DD", or "" when cleared. */
  readonly onChange: (value: string) => void;
  /** Earliest selectable day as "YYYY-MM-DD". */
  readonly min?: string;
  /** Latest selectable day as "YYYY-MM-DD". */
  readonly max?: string;
  /**
   * "YYYY-MM-DD" whose month the calendar opens on while `value` is empty,
   * e.g. the start date for an end-date field. "" or unset opens on today.
   */
  readonly defaultMonth?: string;
  /** Additional days that cannot be selected. */
  readonly disabledDay?: Matcher;
  readonly placeholder?: string;
  readonly className?: string;
  readonly triggerClassName?: string;
  readonly dropdownPlacement?: "up" | "down";
  readonly calendarLayout?: CalendarLayout;
  readonly monthYearNavigation?: boolean;
  readonly hideClearButton?: boolean;
  /** Prevents clearing an already selected date from the calendar. */
  readonly required?: boolean;
  readonly disabled?: boolean;
  readonly id?: string;
  /** See `DatePicker` `name`: lets a server field error focus this field. */
  readonly name?: string;
  readonly ariaLabel?: string;
  readonly ariaDescribedBy?: string;
  readonly invalid?: boolean;
  readonly controlSize?: ControlSize;
  readonly locale?: Locale;
  readonly labels?: DatePickerLabels;
}) {
  const fallbackId = useId();
  const errorId = error && rest.id ? `${rest.id}-error` : undefined;
  // Rendered here rather than by DatePicker, so it reads before the error as
  // in the kit `Input`.
  const hintId = hint ? `${rest.id ?? fallbackId}-hint` : undefined;
  const describedBy =
    [rest.ariaDescribedBy, hintId, errorId].filter(Boolean).join(" ") ||
    undefined;
  const picker = (
    <DatePicker
      {...rest}
      invalid={rest.invalid ?? Boolean(error)}
      ariaDescribedBy={describedBy}
      value={toDateOrNull(value)}
      defaultMonth={toDateOrNull(defaultMonth) ?? undefined}
      minDate={toDateOrNull(min) ?? undefined}
      maxDate={toDateOrNull(max) ?? undefined}
      disabledDay={disabledDay}
      required={rest.required}
      onChange={(date) => onChange(date ? toISODate(date) : "")}
    />
  );

  if (!label && !error && !hint) {
    return picker;
  }

  return (
    <div>
      {label && (
        <FieldLabel
          htmlFor={rest.id}
          required={rest.required}
          invalid={Boolean(error)}
        >
          {label}
        </FieldLabel>
      )}
      {picker}
      {hint && (
        <p id={hintId} className="mt-1 text-xs text-gray-600">
          {hint}
        </p>
      )}
      {error && (
        <p id={errorId} role="alert" className="text-moto-red mt-1 text-xs">
          {error}
        </p>
      )}
    </div>
  );
}

interface ISODateInputProps {
  readonly id: string;
  readonly label: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onValidityChange?: (valid: boolean) => void;
  readonly min?: string;
  readonly max?: string;
  readonly invalidDateError?: string;
  readonly minDateError?: string;
  readonly maxDateError?: string;
  readonly disabled?: boolean;
  /**
   * A check of the surrounding form on the stored value (missing date, date
   * range, overlap). It is shown and wired like the field's own check, which
   * wins while the typed text is no valid date.
   */
  readonly error?: string | null;
}

export function ISODateInput({
  id,
  label,
  value,
  onChange,
  onValidityChange,
  min,
  max,
  invalidDateError = "Bitte geben Sie ein gültiges Datum im Format TT.MM.JJJJ ein.",
  minDateError = "Das Datum liegt vor dem zulässigen Zeitraum.",
  maxDateError = "Das Datum liegt nach dem zulässigen Zeitraum.",
  disabled,
  error,
}: ISODateInputProps) {
  const [inputValue, setInputValue] = useState(() =>
    formatISODateInputValue(value),
  );
  const [inputError, setInputError] = useState<string | null>(() =>
    validateStoredISODate(
      value,
      min,
      max,
      invalidDateError,
      minDateError,
      maxDateError,
    ),
  );
  const errorId = `${id}-error`;
  const shownError = inputError ?? error ?? null;

  useEffect(() => {
    setInputValue(formatISODateInputValue(value));
    const nextError = validateStoredISODate(
      value,
      min,
      max,
      invalidDateError,
      minDateError,
      maxDateError,
    );
    setInputError(nextError);
    onValidityChange?.(nextError === null);
  }, [
    value,
    min,
    max,
    invalidDateError,
    minDateError,
    maxDateError,
    onValidityChange,
  ]);

  const validate = (nextValue: string, showIncompleteError: boolean) => {
    const trimmed = nextValue.trim();
    if (trimmed === "") {
      setInputError(null);
      onValidityChange?.(true);
      onChange("");
      return;
    }

    const isoDate = parseGermanDate(trimmed);
    let nextError: string | null = null;
    if (!isoDate) {
      if (showIncompleteError || trimmed.length >= 10) {
        nextError = invalidDateError;
      }
    } else if (min && isoDate < min) {
      nextError = minDateError;
    } else if (max && isoDate > max) {
      nextError = maxDateError;
    }

    setInputError(nextError);
    const valid = Boolean(isoDate) && nextError === null;
    onValidityChange?.(valid);
    if (valid && isoDate) onChange(isoDate);
  };

  return (
    <div>
      <FieldLabel htmlFor={id}>{label}</FieldLabel>
      <div className="flex items-start gap-2">
        <div className="min-w-0 flex-1">
          <Input
            id={id}
            name={id}
            type="text"
            controlSize="compact"
            inputMode="numeric"
            autoComplete="bday"
            placeholder="TT.MM.JJJJ"
            maxLength={10}
            value={inputValue}
            disabled={disabled}
            aria-invalid={shownError ? true : undefined}
            aria-describedby={shownError ? errorId : undefined}
            onChange={(event) => {
              const nextValue = event.target.value;
              setInputValue(nextValue);
              validate(nextValue, false);
            }}
            onBlur={() => validate(inputValue, true)}
            className={shownError ? "ring-moto-red" : ""}
          />
        </div>
        <DatePicker
          value={toDateOrNull(value)}
          minDate={toDateOrNull(min) ?? undefined}
          maxDate={toDateOrNull(max) ?? undefined}
          onChange={(date) => {
            const isoDate = date ? toISODate(date) : "";
            setInputValue(formatISODateInputValue(isoDate));
            setInputError(null);
            onValidityChange?.(true);
            onChange(isoDate);
          }}
          iconOnly
          hideClearButton
          monthYearNavigation
          disabled={disabled}
          ariaLabel={`${label} im Kalender auswählen`}
          invalid={Boolean(shownError)}
        />
      </div>
      {inputError ? (
        <p id={errorId} role="alert" className="text-moto-red mt-1 text-xs">
          {inputError}
        </p>
      ) : error ? (
        <p id={errorId} className="text-moto-red mt-1 text-xs">
          {error}
        </p>
      ) : null}
    </div>
  );
}

function validateStoredISODate(
  value: string,
  min: string | undefined,
  max: string | undefined,
  invalidDateError: string,
  minDateError: string,
  maxDateError: string,
): string | null {
  if (value === "") return null;
  const isoDate = value.slice(0, 10);
  if (!isValidISODate(isoDate)) return invalidDateError;
  if (min && isoDate < min) return minDateError;
  if (max && isoDate > max) return maxDateError;
  return null;
}

function formatISODateInputValue(value: string): string {
  const isoDate = value.slice(0, 10);
  return isValidISODate(isoDate) ? formatDate(isoDate) : value;
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

// Combines the optional lower/upper bounds into react-day-picker matchers.
// An empty matcher list disables nothing, so an unbounded picker keeps every
// day selectable.
function buildSingleDisabledMatchers(
  minDate?: Date,
  maxDate?: Date,
  disabledDay?: Matcher,
): Matcher[] {
  const matchers: Matcher[] = [];
  if (minDate) matchers.push({ before: minDate });
  if (maxDate) matchers.push({ after: maxDate });
  if (disabledDay) matchers.push(disabledDay);
  return matchers;
}

// A panel narrower than this cannot afford the roomier padding: at a very small
// minimum, p-4 would shave the day buttons below their 20px floor and truncate
// the "Juli 2026" caption. Below the threshold the card falls back to p-3 and
// spends the space on the grid instead.
const COMPACT_CARD_MAX_WIDTH = 240;

function getCalendarContainerClass(
  calendarLayout: CalendarLayout,
  compact: boolean,
): string {
  const base = `moto-popover-surface rounded-xl border ${
    compact ? "p-3" : "p-4"
  }`;
  // The floating layouts get their width from the portal wrapper. Inline
  // calendars belong to their container, but retain the minimum width needed
  // for 20px day buttons with compact card padding.
  if (calendarLayout === "inline") {
    return `${base} mt-2 w-full min-w-[222px] max-w-full`;
  }
  return `${base} w-full`;
}
