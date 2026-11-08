"use client";

// Die Fragen des Trägers in der Ferienanmeldung (#5198): je Familie nach den
// Angaben der Eltern, je Kind nach den Gesundheitsangaben. Eine Frage mit
// Bedingung erscheint erst, wenn die Bedingung erfüllt ist. Die Felder kommen
// aus HolidayQuestionFields; hier kommen Übersetzung und Kalender dazu.

import dynamic from "next/dynamic";

import type { ConditionContext } from "~/lib/enrollment-field-visibility";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import { HolidayQuestionFields } from "./holiday-question-fields";
import type { FieldError } from "./holiday-registration-model";
import { useCareTermTranslations } from "~/lib/care-facility-term";

// The calendar is large; only a form with a date question loads it.
const CustomDateFieldInput = dynamic(
  () =>
    import("~/components/enrollment/custom-date-field-input").then(
      (module) => module.CustomDateFieldInput,
    ),
  { ssr: false },
);

/**
 * The visible questions of one scope. `prefix` is the request path of the
 * scope ("" for the family, "children.0." for a child), so a refused answer
 * from the server lands at its control.
 */
export function HolidayQuestions({
  fields,
  answers,
  context,
  prefix,
  fieldError,
  onChange,
}: Readonly<{
  fields: readonly HolidayFormField[];
  answers: Readonly<Record<string, unknown>>;
  context: ConditionContext;
  prefix: string;
  fieldError: FieldError;
  onChange: (key: string, value: unknown) => void;
}>) {
  const tr = useCareTermTranslations("enrollmentForm");
  return (
    <HolidayQuestionFields
      fields={fields}
      answers={answers}
      context={context}
      prefix={prefix}
      fieldError={fieldError}
      onChange={onChange}
      tr={tr}
      renderDate={({ name, field, value, error, onChange: change }) => (
        <CustomDateFieldInput
          inputName={name}
          field={field}
          value={value}
          onChange={change}
          error={error}
          controlId={name.replaceAll(".", "-")}
        />
      )}
    />
  );
}
