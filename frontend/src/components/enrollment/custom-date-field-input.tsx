"use client";
import { ISODatePicker } from "~/components/ui/date-picker";
import { useLocalizedDatePicker } from "~/lib/hooks/use-localized-date-picker";
import { CustomDateField } from "./custom-date-field";
import type { CustomFieldInputProps } from "./custom-field-input";

/** A date question with the calendar of the date picker. */
export function CustomDateFieldInput(
  props: Pick<
    CustomFieldInputProps,
    "field" | "inputName" | "value" | "onChange" | "error" | "controlId"
  >,
) {
  const datePicker = useLocalizedDatePicker();
  return (
    <CustomDateField
      {...props}
      renderControl={(control) => (
        <ISODatePicker
          {...datePicker}
          id={control.id}
          name={control.name}
          controlSize="md"
          value={control.value}
          onChange={control.onChange}
          invalid={control.invalid}
          ariaDescribedBy={control.ariaDescribedBy}
          calendarLayout="popover"
          // A school-defined date field can be anything from a birthday to a
          // future appointment, so the year dropdown stays available and no
          // bound is imposed.
          monthYearNavigation
        />
      )}
    />
  );
}
