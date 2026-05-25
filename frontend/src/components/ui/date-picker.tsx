"use client";

import { useState, useRef, useEffect } from "react";
import { DayPicker } from "react-day-picker";
import { format, addMonths, subMonths } from "date-fns";
import { de } from "date-fns/locale";
import "react-day-picker/style.css";

type DatePickerProps =
  | {
      readonly mode?: "single";
      readonly value?: Date | null;
      readonly onChange: (date: Date | null) => void;
      readonly placeholder?: string;
      readonly className?: string;
      readonly dropdownPlacement?: "up" | "down";
      readonly calendarLayout?: "overlay" | "inline";
    }
  | {
      readonly mode: "multiple";
      readonly values: Date[];
      readonly onChangeDates: (dates: Date[]) => void;
      readonly placeholder?: string;
      readonly className?: string;
      readonly dropdownPlacement?: "up" | "down";
      readonly calendarLayout?: "overlay" | "inline";
      readonly disabledDates?: Date[];
    };

interface MultipleDatePickerCalendarProps {
  readonly values: Date[];
  readonly onChangeDates: (dates: Date[]) => void;
  readonly disabledDates?: Date[];
  readonly dropdownPlacement: "up" | "down";
  readonly calendarLayout: "overlay" | "inline";
}

export function DatePicker({
  placeholder = "Datum auswählen",
  className = "",
  dropdownPlacement = "up",
  calendarLayout = "overlay",
  ...props
}: DatePickerProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);
  const isMultiple = props.mode === "multiple";
  const displayValue = isMultiple
    ? formatMultipleDateLabel(props.values)
    : props.value
      ? format(props.value, "dd.MM.yyyy", { locale: de })
      : null;

  useEffect(() => {
    function handleOutsideInteraction(event: Event) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
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

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        className={`flex w-full items-center justify-between rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm transition-all ${
          isOpen ? "border-gray-300 bg-gray-50" : "hover:bg-gray-50"
        }`}
      >
        <span className={displayValue ? "text-gray-900" : "text-gray-500"}>
          {displayValue ?? placeholder}
        </span>
        <div className="flex items-center gap-1">
          {displayValue && (
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
              aria-label="Datum löschen"
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

      {isOpen && (
        <>
          {isMultiple ? (
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
              dropdownPlacement={dropdownPlacement}
              calendarLayout={calendarLayout}
              onChange={(date) => {
                props.onChange(date);
                setIsOpen(false);
              }}
            />
          )}
        </>
      )}
    </div>
  );
}

function formatMultipleDateLabel(values: Date[]): string | null {
  if (values.length === 0) {
    return null;
  }
  if (values.length === 1) {
    return format(values[0]!, "dd.MM.yyyy", { locale: de });
  }
  return `${values.length} Tage ausgewählt`;
}

function DatePickerCalendar({
  value,
  dropdownPlacement,
  calendarLayout,
  onChange,
}: {
  readonly value?: Date | null;
  readonly dropdownPlacement: "up" | "down";
  readonly calendarLayout: "overlay" | "inline";
  readonly onChange: (date: Date | null) => void;
}) {
  const [month, setMonth] = useState(value ?? new Date());

  return (
    <div
      className={getCalendarContainerClass(dropdownPlacement, calendarLayout)}
    >
      {/* Custom header with navigation */}
      <div className="mb-3 flex items-center justify-between">
        <button
          type="button"
          onClick={() => setMonth(subMonths(month, 1))}
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
        <span className="text-sm font-medium text-gray-900">
          {format(month, "MMMM yyyy", { locale: de })}
        </span>
        <button
          type="button"
          onClick={() => setMonth(addMonths(month, 1))}
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
      <DayPicker
        mode="single"
        selected={value ?? undefined}
        month={month}
        onMonthChange={setMonth}
        onSelect={(date) => onChange(date ?? null)}
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

function MultipleDatePickerCalendar({
  values,
  onChangeDates,
  disabledDates = [],
  dropdownPlacement,
  calendarLayout,
}: MultipleDatePickerCalendarProps) {
  const [month, setMonth] = useState(values[0] ?? new Date());

  return (
    <div
      className={getCalendarContainerClass(dropdownPlacement, calendarLayout)}
    >
      <div className="mb-3 flex items-center justify-between">
        <button
          type="button"
          onClick={() => setMonth(subMonths(month, 1))}
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
        <span className="text-sm font-medium text-gray-900">
          {format(month, "MMMM yyyy", { locale: de })}
        </span>
        <button
          type="button"
          onClick={() => setMonth(addMonths(month, 1))}
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

function getCalendarContainerClass(
  dropdownPlacement: "up" | "down",
  calendarLayout: "overlay" | "inline",
): string {
  const base = "rounded-xl border border-gray-200 bg-white p-3 shadow-lg";
  if (calendarLayout === "inline") {
    return `${base} mt-2 w-fit max-w-full`;
  }
  const placement =
    dropdownPlacement === "down" ? "top-full mt-1" : "bottom-full mb-1";
  return `${base} absolute left-0 z-[10001] ${placement}`;
}
