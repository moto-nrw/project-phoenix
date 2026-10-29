"use client";
import { ISODatePicker } from "~/components/ui/date-picker";
import { FieldLabel } from "~/components/ui/field-label";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import type { CustomFieldInputProps } from "./custom-field-input";
export function CustomDateFieldInput({
  field,
  inputName,
  value,
  onChange,
  error,
  controlId,
}: Pick<
  CustomFieldInputProps,
  "field" | "inputName" | "value" | "onChange" | "error" | "controlId"
>) {
  const dateId = controlId ?? `enrollment-${inputName}`;
  const errorId = `${dateId}-error`;
  const datePicker = useLocalizedDatePicker();
  const valueStr = typeof value === "string" ? value : "";
  return (
    <div>
      <FieldLabel
        htmlFor={dateId}
        required={field.required}
        invalid={Boolean(error)}
      >
        {field.label}
      </FieldLabel>
      {field.help_text ? (
        <span className="-mt-1 mb-2 block text-xs leading-5 text-gray-500">
          {field.help_text}
        </span>
      ) : null}
      <ISODatePicker
        {...datePicker}
        id={dateId}
        name={inputName}
        controlSize="md"
        value={valueStr}
        onChange={onChange}
        invalid={Boolean(error)}
        ariaDescribedBy={error ? errorId : undefined}
        calendarLayout="popover"
        // A school-defined date field can be anything from a birthday to a
        // future appointment, so the year dropdown stays available and no
        // bound is imposed.
        monthYearNavigation
      />
      <FieldErrorHint id={errorId} message={error} />
    </div>
  );
}
