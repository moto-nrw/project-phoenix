"use client";

import { useState } from "react";

import { ISODatePicker } from "~/components/ui/date-picker";
import { TimeField } from "~/components/ui/time-field";

// Wire shape of `<input type="datetime-local">`: "YYYY-MM-DDTHH:mm" in local
// time, "" when unset. Keeping it means the callers' existing conversion
// helpers (toLocalInputValue / fromLocalInputValue) work unchanged.
const DATE_LENGTH = 10;

// `TimeField` hands up every keystroke ("1", "15", "15:3"); only a real clock
// time may reach the value, because the callers convert it to an instant.
const COMPLETE_TIME = /^(?:[01]\d|2[0-3]):[0-5]\d$/;

function splitLocal(value: string): { day: string; time: string } {
  if (!value) return { day: "", time: "" };
  return {
    day: value.slice(0, DATE_LENGTH),
    time: value.slice(DATE_LENGTH + 1, DATE_LENGTH + 6),
  };
}

/**
 * On the boundary day the clock may sit below the minimum time — whether it was
 * typed into the time half, carried over from an earlier selection or supplied
 * by `defaultTime`. Both halves clamp through here so neither can emit a value
 * that undercuts `min`. Later days are unconstrained and may start at 00:00.
 * "HH:mm" compares correctly as a string.
 */
export function clampTimeToMin(
  day: string,
  time: string,
  minParts: { day: string; time: string },
): string {
  if (day !== minParts.day || !minParts.time) return time;
  return time < minParts.time ? minParts.time : time;
}

/**
 * Kit replacement for `<input type="datetime-local">`: the kit calendar for the
 * day plus the kit `TimeField` for the clock.
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
}) {
  const { day: valueDay, time } = splitLocal(value);
  const minParts = splitLocal(min ?? "");
  // What the time half shows while its text is not a time the value may take.
  const [typedTime, setTypedTime] = useState<string | null>(null);
  // Erasing the time clears the value, but whoever erases it to type a new one
  // must not lose the day (and the field they are typing in) on the way. The
  // day is held until the field is left.
  const [heldDay, setHeldDay] = useState("");
  const day = valueDay || heldDay;

  // Text the value could not take yet: a complete time is raised to the
  // minimum, anything else falls back to the saved time.
  const settleTypedTime = () => {
    if (typedTime && COMPLETE_TIME.test(typedTime)) {
      onChange(`${day}T${clampTimeToMin(day, typedTime, minParts)}`);
    }
    setTypedTime(null);
  };

  return (
    <div className={`flex items-center gap-2 ${className}`}>
      <ISODatePicker
        id={id}
        value={day}
        min={minParts.day || undefined}
        onChange={(nextDay) => {
          // A calendar change commits the saved time. Incomplete or invalid
          // text is only a temporary edit and must not look selected for the
          // new day.
          setTypedTime(null);
          setHeldDay("");
          if (!nextDay) {
            onChange("");
            return;
          }
          const nextTime = clampTimeToMin(
            nextDay,
            time || defaultTime,
            minParts,
          );
          onChange(`${nextDay}T${nextTime}`);
        }}
        disabled={disabled}
        hideClearButton={hideClearButton}
        required={required}
        controlSize="md"
        calendarLayout={calendarLayout}
        ariaLabel={dateAriaLabel}
        invalid={invalid}
        className="min-w-0 flex-1"
        placeholder="Datum wählen"
      />
      <div className="w-24 shrink-0">
        <TimeField
          inline
          label={timeAriaLabel}
          hint="Uhrzeit im Format 15:30"
          placeholder="Uhrzeit"
          value={typedTime ?? time}
          required={required}
          invalid={invalid}
          disabled={disabled || !day}
          onChange={(nextTime) => {
            if (!nextTime) {
              // An optional field has to stay emptiable.
              setTypedTime(null);
              setHeldDay(day);
              onChange("");
              return;
            }
            if (!COMPLETE_TIME.test(nextTime)) {
              setTypedTime(nextTime);
              return;
            }
            // "153" reads as 01:53 on its way to 15:30, so a time below the
            // minimum is only raised once the field is left.
            if (clampTimeToMin(day, nextTime, minParts) !== nextTime) {
              setTypedTime(nextTime);
              return;
            }
            setTypedTime(null);
            onChange(`${day}T${nextTime}`);
          }}
          // Enter submits the form without leaving the field.
          onKeyDown={(event) => {
            if (event.key === "Enter") settleTypedTime();
          }}
          onBlur={() => {
            settleTypedTime();
            setHeldDay("");
          }}
        />
      </div>
    </div>
  );
}
