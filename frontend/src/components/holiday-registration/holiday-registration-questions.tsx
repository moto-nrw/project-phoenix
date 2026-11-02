"use client";

// Die Fragen des Trägers in der Ferienanmeldung (#5198): je Familie nach den
// Angaben der Eltern, je Kind nach den Gesundheitsangaben. Eine Frage mit
// Bedingung erscheint erst, wenn die Bedingung erfüllt ist.

import dynamic from "next/dynamic";
import { useTranslations } from "next-intl";
import {
  CustomFieldInput,
  InfoBlock,
} from "~/components/enrollment/custom-field-input";
import {
  type ConditionContext,
  isFieldVisible,
} from "~/lib/enrollment-field-visibility";
import type { HolidayFormField } from "~/lib/holiday-form-field";
import type { FieldError } from "./holiday-registration-model";

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
  const tr = useTranslations("enrollmentForm");
  const visible = fields.filter((field) => isFieldVisible(field, context));
  if (visible.length === 0) return null;
  return (
    <div className="space-y-4">
      {visible.map((field) => {
        if (field.type === "information") {
          return <InfoBlock key={field.key} field={field} />;
        }
        const name = `${prefix}custom_data.${field.key}`;
        return (
          <CustomFieldInput
            key={field.key}
            field={field}
            inputName={name}
            value={answers[field.key]}
            onChange={(value) => onChange(field.key, value)}
            error={fieldError(name)}
            tr={tr}
            renderStructured={() =>
              field.type === "date" ? (
                <CustomDateFieldInput
                  inputName={name}
                  field={field}
                  value={answers[field.key]}
                  onChange={(value: unknown) => onChange(field.key, value)}
                  error={fieldError(name)}
                  controlId={name.replaceAll(".", "-")}
                />
              ) : null
            }
          />
        );
      })}
    </div>
  );
}
