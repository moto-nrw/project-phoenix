"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { DayPicker, type DateRange, type Matcher } from "react-day-picker";
import { addDays, addMonths, format, subMonths } from "date-fns";
import { de } from "date-fns/locale";
import "react-day-picker/style.css";
import {
  CALENDAR_PANEL_MARGIN,
  computeCalendarPanelPosition,
  type PanelGeometry,
} from "~/components/ui/calendar-panel-position";

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
}

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
  triggerClassName = "",
}: DateRangePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);

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
        onClick={() => setIsOpen((v) => !v)}
        // Gleiche Geometrie wie jedes andere Bedienelement im Seitenkopf:
        // 36 px hoch, 12 px Radius, 14 px Schrift. Die frühere Pille (32 px,
        // rounded-full, 12 px Schrift) stand neben 36-px-Knöpfen und las sich
        // als anderes Bauteil.
        className={`inline-flex h-9 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 transition-colors ${
          isOpen ? "bg-gray-50" : "hover:bg-gray-50"
        } ${triggerClassName}`}
      >
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
        <span className="whitespace-nowrap tabular-nums">
          {formatRangeLabel(value)}
        </span>
      </button>

      {isOpen && (
        <RangeCalendar
          triggerRef={triggerRef}
          panelRef={panelRef}
          value={value}
          presets={presets}
          fromMin={fromMin}
          toMax={toMax}
          onChange={(range) => {
            onChange(range);
            // Close only when both bounds are picked (or when a preset fires).
            if (range?.from && range.to) {
              setIsOpen(false);
            }
          }}
        />
      )}
    </div>
  );
}

function formatRangeLabel(range: DateRange | undefined): string {
  if (!range?.from) return "Zeitraum wählen";
  if (!range.to) {
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
  ...props
}: RangeCalendarProps & {
  readonly triggerRef: React.RefObject<HTMLElement | null>;
  readonly panelRef: React.RefObject<HTMLDivElement | null>;
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
      data-range-panel
      className="fixed z-[10001] max-h-[calc(100vh-6rem)] w-max max-w-[calc(100vw-1rem)] overflow-y-auto rounded-xl border border-gray-200 bg-white shadow-[0_8px_30px_rgb(0,0,0,0.12)] sm:max-h-none sm:overflow-visible"
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
  if (!draftFrom) return "Klicke ein Startdatum";
  if (!draftTo) return "Klicke ein Enddatum";
  return `${format(draftFrom, "d. MMM yyyy", { locale: de })} - ${format(
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
  const [month, setMonth] = useState(value?.from ?? new Date());
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
            {" - "}
            {format(addMonths(month, 1), "MMMM yyyy", { locale: de })}
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
        <DayPicker
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
              draftFrom && draftTo && draftFrom.getTime() !== draftTo.getTime()
                ? [draftFrom]
                : [],
            rangeEnd:
              draftTo && draftFrom && draftFrom.getTime() !== draftTo.getTime()
                ? [draftTo]
                : [],
            rangeMiddle:
              draftFrom && draftTo ? { after: draftFrom, before: draftTo } : [],
            singlePick:
              draftFrom &&
              (!draftTo || draftFrom.getTime() === draftTo.getTime())
                ? [draftFrom]
                : [],
          }}
          modifiersClassNames={{
            ...extraModifiersClassNames,
            // Half-cell gradient on the cell (light-green band only on the
            // inner half) + dark rounded pill on the button. This keeps the
            // band continuous from start to end without "tails" outside.
            rangeStart:
              "!bg-[linear-gradient(to_right,transparent_50%,var(--color-moto-green-soft)_50%)] [&>button]:!bg-moto-green [&>button]:!text-gray-950 [&>button]:!rounded-lg",
            rangeEnd:
              "!bg-[linear-gradient(to_right,var(--color-moto-green-soft)_50%,transparent_50%)] [&>button]:!bg-moto-green [&>button]:!text-gray-950 [&>button]:!rounded-lg",
            rangeMiddle: "!bg-moto-green/15 [&>button]:!text-moto-green-strong",
            singlePick:
              "[&>button]:!bg-moto-green [&>button]:!text-gray-950 [&>button]:!rounded-lg",
          }}
          classNames={{
            root: "text-sm",
            months: "flex flex-col gap-4 sm:flex-row",
            month: "",
            month_caption: "hidden",
            month_grid: "border-collapse",
            weekdays: "flex",
            weekday: "text-gray-500 w-8 font-normal text-xs text-center",
            week: "flex w-full mt-1",
            day: "w-8 h-8 text-center text-sm p-0 relative",
            day_button:
              "w-8 h-8 rounded-lg hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-gray-200 transition-colors",
            today: "font-bold text-[#70b525]",
            outside: "text-gray-300",
            disabled: "text-gray-200 cursor-not-allowed",
          }}
        />
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
