import type { ComponentProps } from "react";

import type { ISODatePicker } from "~/components/ui/date-picker";

type ISODatePickerProps = ComponentProps<typeof ISODatePicker>;

/**
 * Stand-in for the kit `ISODatePicker` that renders the native date input it
 * replaced, so a test can still set a date with `fireEvent.change` instead of
 * opening a calendar overlay and clicking a day.
 *
 * Use it when the test is about the surrounding business rule (which days get
 * submitted, whether an invalid range blocks the save) rather than about the
 * picker itself. Tests that assert on the picker's own behaviour should drive
 * the real component — see `shift-edit-modal.series.test.tsx`.
 *
 *   vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
 *     ...(await importOriginal<object>()),
 *     ...isoDatePickerMock(),
 *   }));
 *
 * Spreading the original keeps `DatePicker` real, so calendar-driven tests in
 * the same file keep exercising the actual overlay.
 */
export function isoDatePickerMock() {
  return {
    ISODatePicker: ({
      value,
      onChange,
      min,
      max,
      id,
      ariaLabel,
      disabled,
      label,
      error,
    }: ISODatePickerProps) => (
      // The real component renders its own label and error text, so the stub has
      // to as well — otherwise getByLabelText stops finding the field.
      <div>
        {label && <label htmlFor={id}>{label}</label>}
        <input
          type="date"
          id={id}
          aria-label={ariaLabel}
          value={value}
          min={min}
          max={max}
          disabled={disabled}
          onChange={(event) => onChange(event.target.value)}
        />
        {error && <p role="alert">{error}</p>}
      </div>
    ),
  };
}
