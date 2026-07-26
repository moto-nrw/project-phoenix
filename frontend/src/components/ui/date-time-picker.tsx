"use client";

import { ISODatePicker } from "~/components/ui/date-picker";

// Wire shape of `<input type="datetime-local">`: "YYYY-MM-DDTHH:mm" in local
// time, "" when unset. Keeping it means the callers' existing conversion
// helpers (toLocalInputValue / fromLocalInputValue) work unchanged.
const DATE_LENGTH = 10;

// Mirrors the date half's trigger heights so the two never look stepped.
const TIME_SIZE_CLASS: Record<"sm" | "md" | "lg", string> = {
  sm: "px-3 py-2 text-sm",
  md: "h-10 px-3 py-2 text-sm",
  lg: "px-4 py-3 text-base",
};

function splitLocal(value: string): { day: string; time: string } {
  if (!value) return { day: "", time: "" };
  return {
    day: value.slice(0, DATE_LENGTH),
    time: value.slice(DATE_LENGTH + 1, DATE_LENGTH + 6),
  };
}

/**
 * Kit replacement for `<input type="datetime-local">`: the kit calendar for the
 * day plus a time field for the clock.
 *
 * The time half stays a native `<input type="time">` — that is the app-wide
 * convention (every shift, activity and pickup form uses it) and the kit has no
 * time control, so inventing one here would create a second pattern.
 */
export function DateTimePicker({
  value,
  onChange,
  min,
  disabled = false,
  id,
  className = "",
  dateAriaLabel = "Datum",
  timeAriaLabel = "Uhrzeit",
  invalid,
  // Clock used when a day is picked while the time half is still empty.
  // Deadlines usually want the end of the day, openings the start, so the
  // caller decides rather than silently getting midnight.
  defaultTime = "00:00",
  calendarLayout = "popover",
  hideClearButton = false,
  required = false,
  controlSize = "sm",
}: {
  /** "YYYY-MM-DDTHH:mm" in local time, or "" when unset. */
  readonly value: string;
  /** Emits the same shape, or "" when the day is cleared. */
  readonly onChange: (value: string) => void;
  /** Earliest allowed value as "YYYY-MM-DDTHH:mm". */
  readonly min?: string;
  readonly disabled?: boolean;
  readonly id?: string;
  readonly className?: string;
  readonly dateAriaLabel?: string;
  readonly timeAriaLabel?: string;
  readonly invalid?: boolean;
  readonly defaultTime?: string;
  readonly calendarLayout?: "overlay" | "inline" | "popover";
  /** Hide the clear control. Use when the value is required. */
  readonly hideClearButton?: boolean;
  /** Prevents clearing an already selected date from the calendar. */
  readonly required?: boolean;
  /** Height of both halves. Match the kit Input next to it. */
  readonly controlSize?: "sm" | "md" | "lg";
}) {
  const { day, time } = splitLocal(value);
  const minParts = splitLocal(min ?? "");

  return (
    <div className={`flex items-center gap-2 ${className}`}>
      <ISODatePicker
        id={id}
        value={day}
        min={minParts.day || undefined}
        onChange={(nextDay) => {
          if (!nextDay) {
            onChange("");
            return;
          }
          onChange(`${nextDay}T${time || defaultTime}`);
        }}
        disabled={disabled}
        hideClearButton={hideClearButton}
        required={required}
        controlSize={controlSize}
        calendarLayout={calendarLayout}
        ariaLabel={dateAriaLabel}
        invalid={invalid}
        className="min-w-0 flex-1"
        placeholder="Datum wählen"
      />
      <input
        type="time"
        aria-label={timeAriaLabel}
        value={time}
        // A lower bound only constrains the clock on the boundary day itself;
        // any later day may start at 00:00.
        min={day && day === minParts.day ? minParts.time : undefined}
        disabled={disabled || !day}
        onChange={(event) => {
          const nextTime = event.target.value;
          if (!day) return;
          onChange(nextTime ? `${day}T${nextTime}` : "");
        }}
        className={`w-[7.5rem] shrink-0 rounded-lg border border-gray-200 bg-white text-gray-900 ${TIME_SIZE_CLASS[controlSize]} transition-colors hover:bg-gray-50 focus:border-gray-300 focus:ring-2 focus:ring-gray-200 focus:outline-none disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400`}
      />
    </div>
  );
}
