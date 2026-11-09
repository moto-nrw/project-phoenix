"use client";
import { Z_LAYER } from "./overlay-styles";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { parseISODate } from "~/lib/date-helpers";
import { cn } from "~/lib/utils";

import {
  Suspense,
  useCallback,
  useEffect,
  useRef,
  useState,
  useId,
} from "react";
import { createPortal } from "react-dom";
import type { DateRange, Matcher } from "react-day-picker";
import { addDays } from "date-fns/addDays";
import { addMonths } from "date-fns/addMonths";
import { format } from "date-fns/format";
import { subMonths } from "date-fns/subMonths";
import { de } from "date-fns/locale/de";
import "react-day-picker/style.css";
import {
  CALENDAR_PANEL_MARGIN,
  computeCalendarPanelPosition,
  type PanelGeometry,
} from "~/components/ui/calendar-panel-position";
import { selectTriggerClassName } from "~/components/ui/select-trigger-styles";
import {
  DAY_PICKER_CLASSNAMES,
  DayPickerSkeleton,
  LazyDayPicker,
  preloadDayPicker,
} from "~/components/ui/lazy-day-picker";

interface Preset {
  readonly label: string;
  readonly range: () => DateRange;
}

interface DateRangePickerProps {
  readonly value: DateRange | undefined;
  readonly onChange: (range: DateRange | undefined) => void;
  readonly presets?: ReadonlyArray<Preset>;
  readonly className?: string;
  readonly fromMin?: Date;
  readonly toMax?: Date;
  /** Extra classes on the trigger chip, e.g. `w-full sm:w-auto` for a
   * mobile filter row that should fill the line. */
  readonly triggerClassName?: string;
  /** Between the arrows of a framed group: frame and radius come from the group,
   * the trigger brings only content and height. */
  readonly embedded?: boolean;
}

// Beside a second month the grid cannot follow the panel's width, so from `sm`
// on each month has a fixed one; the single month on a phone fills the panel.
// The cells touch: the range band runs through them without a gap.
const RANGE_MONTHS_CLASS = "flex flex-col gap-4 sm:flex-row";
const RANGE_MONTH_CLASS = "w-full sm:w-56";
const RANGE_CLASSNAMES = {
  ...DAY_PICKER_CLASSNAMES,
  months: RANGE_MONTHS_CLASS,
  month: RANGE_MONTH_CLASS,
  day: "flex-1 min-w-0 h-9 p-0 text-center text-sm relative",
};

// The look of `DAY_PICKER_CLASSNAMES.selected`. Important because the day marks
// a caller adds (`modifiersClassNames`) are too.
const RANGE_PILL_CLASS =
  "[&>button]:!bg-gray-900 [&>button]:!text-white [&>button:hover]:!bg-gray-800";

/**
 * Compact date-range picker for chart card headers. Trigger is a small chip,
 * popover shows preset shortcuts on the left and a two-month range calendar
 * on the right. Matches the styling of the existing single-date picker
 * (`date-picker.tsx`) so the look stays consistent.
 */
export function DateRangePicker({
  value,
  onChange,
  presets,
  className = "",
  fromMin,
  toMax,
  triggerClassName,
  embedded = false,
}: DateRangePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const panelId = useId();
  // Closing from inside the panel (Escape, a finished range, a preset) hands
  // focus back to the trigger; the panel would otherwise take it along.
  const closeToTrigger = useCallback(() => {
    setIsOpen(false);
    triggerRef.current?.focus();
  }, []);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      const target = event.target as Node;
      // The panel now lives in a portal outside containerRef, so it has to be
      // treated as "inside" explicitly — otherwise clicking a day would close
      // the panel before the selection lands.
      if (panelRef.current?.contains(target)) {
        return;
      }
      if (containerRef.current && !containerRef.current.contains(target)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      <button
        ref={triggerRef}
        type="button"
        aria-haspopup="dialog"
        aria-expanded={isOpen}
        aria-controls={isOpen ? panelId : undefined}
        onClick={() => setIsOpen((v) => !v)}
        // Fetch the calendar module before the click, so opening does not
        // wait for it.
        onPointerEnter={preloadDayPicker}
        onFocus={preloadDayPicker}
        // Derselbe Auslöser wie jedes Auswahlfeld (`CustomSelect`).
        className={
          embedded
            ? cn(
                "flex h-9 w-full items-center justify-center gap-2 px-3 text-sm transition-colors",
                isOpen ? "bg-gray-50" : "hover:bg-gray-50",
              )
            : cn(
                selectTriggerClassName({ className: triggerClassName }),
                // `moto-content-surface` supplies the white background outside
                // the Tailwind layer, so the open state must take precedence.
                isOpen && "bg-gray-50!",
              )
        }
      >
        <span
          className={cn(
            "whitespace-nowrap tabular-nums",
            value?.from ? "text-gray-900" : "text-gray-500",
          )}
        >
          {formatRangeLabel(value)}
        </span>
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

      {isOpen && (
        <RangeCalendar
          triggerRef={triggerRef}
          panelRef={panelRef}
          panelId={panelId}
          onEscape={closeToTrigger}
          value={value}
          presets={presets}
          fromMin={fromMin}
          toMax={toMax}
          onChange={(range) => {
            onChange(range);
            // Close only when both bounds are picked (or when a preset fires).
            if (range?.from && range.to) {
              closeToTrigger();
            }
          }}
        />
      )}
    </div>
  );
}

function isSameCalendarDay(left: Date, right: Date): boolean {
  return (
    left.getFullYear() === right.getFullYear() &&
    left.getMonth() === right.getMonth() &&
    left.getDate() === right.getDate()
  );
}

function formatRangeLabel(range: DateRange | undefined): string {
  if (!range?.from) return "Zeitraum wählen";
  // Ein einzelner Tag zeigt nur ein Datum, nicht „6.–6. Okt. 2026".
  if (!range.to || isSameCalendarDay(range.from, range.to)) {
    return format(range.from, "d. MMM yyyy", { locale: de });
  }
  const sameYear = range.from.getFullYear() === range.to.getFullYear();
  const sameMonth = sameYear && range.from.getMonth() === range.to.getMonth();
  // Je kürzer, desto eher passt das Label in einen Chip neben den Pfeilen,
  // ohne dass auf dem Telefon das Jahr in eine zweite Zeile rutscht:
  // „1.–30. Aug. 2026" statt „1. Aug. – 30. Aug. 2026".
  if (sameMonth) {
    return `${format(range.from, "d.", { locale: de })}–${format(range.to, "d. MMM yyyy", { locale: de })}`;
  }
  if (sameYear) {
    return `${format(range.from, "d. MMM", { locale: de })} – ${format(range.to, "d. MMM yyyy", { locale: de })}`;
  }
  return `${format(range.from, "d. MMM yyyy", { locale: de })} – ${format(range.to, "d. MMM yyyy", { locale: de })}`;
}

// The panel keeps its content width — a two-month grid with a preset column
// cannot shrink to a chip-sized trigger — but it is placed by the same shared
// geometry as the single-date panel, so an edge is flush with the trigger
// instead of always hugging the right via `sm:right-0`.
function RangeCalendar({
  triggerRef,
  panelRef,
  panelId,
  onEscape,
  ...props
}: RangeCalendarProps & {
  readonly triggerRef: React.RefObject<HTMLElement | null>;
  readonly panelRef: React.RefObject<HTMLDivElement | null>;
  readonly panelId: string;
  readonly onEscape: () => void;
}) {
  const [geometry, setGeometry] = useState<PanelGeometry | null>(null);
  const [isNarrowViewport, setIsNarrowViewport] = useState<boolean | null>(
    null,
  );
  // Portals only exist client-side — `document` must not be touched while the
  // tree renders on the server.
  const [mounted, setMounted] = useState(false);

  // Content width is only known after the first paint, so measure the panel and
  // place it from that — using a guessed width is what previously left the
  // single-date card 22px off its own trigger.
  const sync = useCallback(() => {
    const trigger = triggerRef.current;
    const panel = panelRef.current;
    if (!mounted || !trigger || !panel) return;
    if (window.innerWidth < 640) {
      setIsNarrowViewport(true);
      return;
    }
    setIsNarrowViewport(false);
    const rect = trigger.getBoundingClientRect();
    setGeometry(
      computeCalendarPanelPosition(
        rect,
        panel.getBoundingClientRect().width,
        { width: window.innerWidth, height: window.innerHeight },
        panel.getBoundingClientRect().height,
      ),
    );
    // `mounted` gates the portal, so the panel only exists to measure once it
    // is true — the effect below has to re-run at that point.
  }, [triggerRef, panelRef, mounted]);

  useEffect(() => {
    setMounted(true);
  }, []);

  // Opening moves focus into the panel: onto the chosen start day when there
  // is one, else onto the first control. The panel is portaled after the
  // page, so keyboard users would otherwise not reach it.
  useEffect(() => {
    if (!mounted) return;
    const frame = requestAnimationFrame(() => {
      const panel = panelRef.current;
      if (!panel || panel.contains(document.activeElement)) return;
      const target =
        panel.querySelector<HTMLElement>('[aria-selected="true"] button') ??
        panel.querySelector<HTMLElement>(
          "button:not([disabled]), [tabindex='0']",
        );
      target?.focus({ preventScroll: true });
    });
    return () => cancelAnimationFrame(frame);
  }, [mounted, panelRef]);

  useEffect(() => {
    sync();
    window.addEventListener("resize", sync);
    window.addEventListener("scroll", sync, true);
    return () => {
      window.removeEventListener("resize", sync);
      window.removeEventListener("scroll", sync, true);
    };
  }, [sync]);

  // Below the sm breakpoint the panel is too wide for any alignment to help, so
  // it spans the viewport with a margin, as before.
  const narrowStyle = {
    left: CALENDAR_PANEL_MARGIN,
    right: CALENDAR_PANEL_MARGIN,
    top: 80,
  } as const;

  if (!mounted) return null;

  return createPortal(
    <div
      ref={panelRef}
      id={panelId}
      role="dialog"
      aria-label="Zeitraum wählen"
      data-range-panel
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          event.stopPropagation();
          onEscape();
        }
      }}
      className={`fixed ${Z_LAYER.popover} moto-popover-surface max-h-[calc(100vh-6rem)] w-max max-w-[calc(100vw-1rem)] overflow-y-auto rounded-xl border sm:max-h-none sm:overflow-visible`}
      style={
        isNarrowViewport || !geometry
          ? {
              ...narrowStyle,
              visibility: isNarrowViewport === null ? "hidden" : undefined,
            }
          : { top: geometry.top, left: geometry.left }
      }
    >
      <RangeCalendarInline {...props} />
    </div>,
    document.body,
  );
}

function useIsSingleMonthCalendar(): boolean {
  const [isSingleMonth, setIsSingleMonth] = useState(false);

  useEffect(() => {
    const query = window.matchMedia("(max-width: 639px)");
    const update = () => setIsSingleMonth(query.matches);
    update();
    query.addEventListener("change", update);
    return () => query.removeEventListener("change", update);
  }, []);

  return isSingleMonth;
}

function formatDraftRangeLabel(
  draftFrom: Date | undefined,
  draftTo: Date | undefined,
): string {
  if (!draftFrom) return "Startdatum wählen";
  if (!draftTo) return "Enddatum wählen";
  if (isSameCalendarDay(draftFrom, draftTo)) {
    return format(draftFrom, "d. MMM yyyy", { locale: de });
  }
  return `${format(draftFrom, "d. MMM yyyy", { locale: de })} – ${format(
    draftTo,
    "d. MMM yyyy",
    { locale: de },
  )}`;
}

interface RangeCalendarProps {
  readonly value: DateRange | undefined;
  readonly presets?: ReadonlyArray<Preset>;
  readonly onChange: (range: DateRange | undefined) => void;
  readonly fromMin?: Date;
  readonly toMax?: Date;
  readonly modifiers?: Record<string, Matcher | Matcher[]>;
  readonly modifiersClassNames?: Record<string, string>;
}

export function RangeCalendarInline({
  value,
  presets,
  onChange,
  fromMin,
  toMax,
  modifiers: extraModifiers,
  modifiersClassNames: extraModifiersClassNames,
}: RangeCalendarProps) {
  const today = useBerlinToday();
  const [month, setMonth] = useState(value?.from ?? parseISODate(today));
  const isSingleMonth = useIsSingleMonthCalendar();
  // Manual draft state. We use `mode="single"` on the underlying DayPicker
  // and manage the two-click range logic ourselves, react-day-picker v10's
  // mode="range" calls onSelect with surprising payloads (sometimes a complete
  // {from, to} pair on first click), which would cause the picker to close
  // prematurely. With manual control we know exactly what happens per click.
  const [draftFrom, setDraftFrom] = useState<Date | undefined>(value?.from);
  const [draftTo, setDraftTo] = useState<Date | undefined>(value?.to);

  const handleDayClick = (day: Date | undefined) => {
    if (!day) return;
    // First click of a new selection (no `from` yet, or already had both bounds).
    if (!draftFrom || draftTo) {
      setDraftFrom(day);
      setDraftTo(undefined);
      return;
    }
    // Second click: complete the range. Swap if the user picked the end first.
    const finalRange =
      day < draftFrom
        ? { from: day, to: draftFrom }
        : { from: draftFrom, to: day };
    setDraftFrom(finalRange.from);
    setDraftTo(finalRange.to);
    onChange(finalRange);
  };

  const handlePreset = (preset: Preset) => {
    const range = preset.range();
    setDraftFrom(range.from);
    setDraftTo(range.to);
    onChange(range);
  };

  const hasPresets = presets && presets.length > 0;
  const draftLabel = formatDraftRangeLabel(draftFrom, draftTo);

  return (
    <div
      className={`flex flex-col sm:flex-row ${hasPresets ? "" : "justify-center"}`}
    >
      {hasPresets && (
        <div className="flex max-w-[calc(100vw-2rem)] gap-1 overflow-x-auto border-b border-gray-100 p-2 text-sm sm:max-w-none sm:flex-col sm:overflow-visible sm:border-r sm:border-b-0 sm:p-3">
          {presets.map((preset) => (
            <button
              key={preset.label}
              type="button"
              onClick={() => handlePreset(preset)}
              className="shrink-0 rounded-md px-3 py-2 text-left text-gray-700 hover:bg-gray-100"
            >
              {preset.label}
            </button>
          ))}
        </div>
      )}
      <div className="p-3 sm:p-4">
        <div className="mb-3 text-center text-sm font-medium text-gray-700">
          {draftLabel}
        </div>
        <div className="mb-3 flex items-center justify-between">
          <button
            type="button"
            onClick={() => setMonth(subMonths(month, 1))}
            aria-label="Vorheriger Monat"
            className="rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
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
                d="M15 19l-7-7 7-7"
              />
            </svg>
          </button>
          <span className="text-sm font-medium text-gray-900">
            {format(month, "MMMM yyyy", { locale: de })}
            {/* A phone shows one month, so the heading names only that one. */}
            {isSingleMonth
              ? null
              : ` – ${format(addMonths(month, 1), "MMMM yyyy", { locale: de })}`}
          </span>
          <button
            type="button"
            onClick={() => setMonth(addMonths(month, 1))}
            aria-label="Nächster Monat"
            className="rounded-lg p-1.5 text-gray-600 hover:bg-gray-100"
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
                d="M9 5l7 7-7 7"
              />
            </svg>
          </button>
        </div>
        {/* The skeleton keeps the panel's measured size: RangeCalendar places
            the panel from its width on mount. */}
        <Suspense
          fallback={
            <DayPickerSkeleton
              month={month}
              numberOfMonths={isSingleMonth ? 1 : 2}
              monthsClassName={RANGE_MONTHS_CLASS}
              monthClassName={RANGE_MONTH_CLASS}
            />
          }
        >
          <LazyDayPicker
            today={parseISODate(today)}
            mode="single"
            selected={undefined}
            onDayClick={handleDayClick}
            month={month}
            onMonthChange={setMonth}
            numberOfMonths={isSingleMonth ? 1 : 2}
            locale={de}
            weekStartsOn={1}
            showOutsideDays={false}
            hideNavigation
            disabled={[
              ...(fromMin ? [{ before: fromMin }] : []),
              ...(toMax ? [{ after: toMax }] : []),
            ]}
            modifiers={{
              ...extraModifiers,
              // Range start/end only fire when there's an actual span (from !==
              // to). A single-day or in-progress selection is handled by
              // `singlePick` so it gets a standalone rounded pill instead of a
              // half-band that points nowhere.
              rangeStart:
                draftFrom &&
                draftTo &&
                draftFrom.getTime() !== draftTo.getTime()
                  ? [draftFrom]
                  : [],
              rangeEnd:
                draftTo &&
                draftFrom &&
                draftFrom.getTime() !== draftTo.getTime()
                  ? [draftTo]
                  : [],
              rangeMiddle:
                draftFrom && draftTo
                  ? { after: draftFrom, before: draftTo }
                  : [],
              singlePick:
                draftFrom &&
                (!draftTo || draftFrom.getTime() === draftTo.getTime())
                  ? [draftFrom]
                  : [],
            }}
            modifiersClassNames={{
              ...extraModifiersClassNames,
              // Half-cell gradient on the cell (light band only on the inner
              // half) + dark rounded pill on the button. This keeps the band
              // continuous from start to end without "tails" outside.
              rangeStart: `!bg-[linear-gradient(to_right,transparent_50%,var(--color-gray-100)_50%)] ${RANGE_PILL_CLASS}`,
              rangeEnd: `!bg-[linear-gradient(to_right,var(--color-gray-100)_50%,transparent_50%)] ${RANGE_PILL_CLASS}`,
              // Normal weight also for today; its blue stays readable here.
              rangeMiddle:
                "!bg-gray-100 [&>button]:font-normal [&>button:hover]:bg-gray-200",
              singlePick: RANGE_PILL_CLASS,
            }}
            classNames={RANGE_CLASSNAMES}
          />
        </Suspense>
      </div>
    </div>
  );
}

// Helper to build common preset lists. Callers pass an `anchor` (the earliest
// possible date for "Gesamt", typically the staff member's schedule
// validFrom or the start of the year).
export function buildDefaultPresets(anchor: Date, today: Date): Preset[] {
  return [
    {
      label: "Letzte 7 Tage",
      range: () => ({ from: addDays(today, -6), to: today }),
    },
    {
      label: "Letzte 30 Tage",
      range: () => ({ from: addDays(today, -29), to: today }),
    },
    {
      label: "Letzte 60 Tage",
      range: () => ({ from: addDays(today, -59), to: today }),
    },
    {
      label: "Letzte 90 Tage",
      range: () => ({ from: addDays(today, -89), to: today }),
    },
    {
      label: "Dieses Jahr",
      range: () => ({
        from: new Date(today.getFullYear(), 0, 1),
        to: today,
      }),
    },
    {
      label: "Gesamt",
      range: () => ({ from: anchor, to: today }),
    },
  ];
}
