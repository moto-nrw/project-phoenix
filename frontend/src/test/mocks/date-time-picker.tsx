import type { ComponentProps } from "react";

import type { DateTimePicker } from "~/components/ui/date-time-picker";

type DateTimePickerProps = ComponentProps<typeof DateTimePicker>;

/**
 * Stand-in for the kit `DateTimePicker` that renders the native
 * `<input type="datetime-local">` it replaced, so a test can set a timestamp
 * with a single `fireEvent.change` instead of driving a calendar plus a
 * separate time field.
 *
 *   vi.mock("~/components/ui/date-time-picker", async (importOriginal) => ({
 *     ...(await importOriginal<object>()),
 *     ...dateTimePickerMock(),
 *   }));
 */
export function dateTimePickerMock() {
  return {
    DateTimePicker: ({
      value,
      onChange,
      min,
      id,
      disabled,
      dateAriaLabel,
    }: DateTimePickerProps) => (
      <input
        type="datetime-local"
        id={id}
        // Mirrors the name the replaced native input carried, so tests that
        // query by name keep working.
        name={id}
        aria-label={dateAriaLabel}
        value={value}
        min={min}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      />
    ),
  };
}
