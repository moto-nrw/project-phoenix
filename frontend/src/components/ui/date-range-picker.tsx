"use client";

import { useEffect, useRef, useState } from "react";
import { DayPicker, type DateRange } from "react-day-picker";
import { addDays, addMonths, format, subMonths } from "date-fns";
import { de } from "date-fns/locale";
import "react-day-picker/style.css";

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
}: DateRangePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      <button
        type="button"
        onClick={() => setIsOpen((v) => !v)}
        className={`inline-flex h-8 items-center gap-2 rounded-full border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 transition-colors ${
          isOpen ? "bg-gray-50" : "hover:bg-gray-50"
        }`}
      >
        <svg
          className="h-3.5 w-3.5 text-gray-400"
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
        <span>{formatRangeLabel(value)}</span>
      </button>

      {isOpen && (
        <RangeCalendar
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
  if (sameYear) {
    return `${format(range.from, "d. MMM", { locale: de })} - ${format(range.to, "d. MMM yyyy", { locale: de })}`;
  }
  return `${format(range.from, "d. MMM yyyy", { locale: de })} - ${format(range.to, "d. MMM yyyy", { locale: de })}`;
}

function RangeCalendar(props: RangeCalendarProps) {
  return (
    <div className="fixed inset-x-4 top-20 z-[10001] max-h-[calc(100vh-6rem)] overflow-y-auto rounded-xl border border-gray-200 bg-white shadow-[0_8px_30px_rgb(0,0,0,0.12)] sm:absolute sm:top-full sm:right-0 sm:left-auto sm:mt-2 sm:max-h-none sm:overflow-visible">
      <RangeCalendarInline {...props} />
    </div>
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
}

export function RangeCalendarInline({
  value,
  presets,
  onChange,
  fromMin,
  toMax,
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
        <div className="flex max-w-[calc(100vw-2rem)] gap-1 overflow-x-auto border-b border-gray-100 p-3 text-xs sm:max-w-none sm:flex-col sm:overflow-visible sm:border-r sm:border-b-0">
          {presets.map((preset) => (
            <button
              key={preset.label}
              type="button"
              onClick={() => handlePreset(preset)}
              className="shrink-0 rounded-md px-3 py-1.5 text-left text-gray-700 hover:bg-gray-100"
            >
              {preset.label}
            </button>
          ))}
        </div>
      )}
      <div className="p-3">
        <div className="mb-2 px-1 text-center text-xs text-gray-500">
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
            // Half-cell gradient on the cell (light-green band only on the
            // inner half) + dark rounded pill on the button. This keeps the
            // band continuous from start to end without "tails" outside.
            rangeStart:
              "!bg-[linear-gradient(to_right,transparent_50%,#83CD2D26_50%)] [&>button]:!bg-[#83CD2D] [&>button]:!text-white [&>button]:!rounded-lg",
            rangeEnd:
              "!bg-[linear-gradient(to_right,#83CD2D26_50%,transparent_50%)] [&>button]:!bg-[#83CD2D] [&>button]:!text-white [&>button]:!rounded-lg",
            rangeMiddle: "!bg-[#83CD2D]/15 [&>button]:!text-[#4a7a15]",
            singlePick:
              "[&>button]:!bg-[#83CD2D] [&>button]:!text-white [&>button]:!rounded-lg",
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
