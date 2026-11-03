"use client";

// Die Felder der Fragen des Trägers (#5198): je Familie oder je Kind, eine
// Frage mit Bedingung erst, wenn die Bedingung erfüllt ist. Das öffentliche
// Formular und die Bearbeitung durch das Träger-Büro (#5629) nutzen dieselben
// Felder; Sprache und Datumsfeld bringt der Aufrufer mit. Das Modul lädt
// darum weder Übersetzungen noch einen Kalender.

import type { ReactNode } from "react";
import {
  CustomFieldInput,
  type CustomFieldInputProps,
  InfoBlock,
} from "~/components/enrollment/custom-field-input";
import {
  type ConditionContext,
  isFieldVisible,
} from "~/lib/enrollment-field-visibility";
import type { HolidayFormField } from "~/lib/holiday-form-field";

/** One date question as the caller renders it. */
export interface HolidayDateQuestion {
  readonly name: string;
  readonly field: HolidayFormField;
  readonly value: unknown;
  readonly error: string | undefined;
  readonly onChange: (value: unknown) => void;
}

/**
 * The visible questions of one scope. `prefix` is the request path of the
 * scope ("" for the family, "children.0." for a child), so a refused answer
 * from the server lands at its control.
 */
export function HolidayQuestionFields({
  fields,
  answers,
  context,
  prefix,
  fieldError,
  onChange,
  tr,
  renderDate,
}: Readonly<{
  fields: readonly HolidayFormField[];
  answers: Readonly<Record<string, unknown>>;
  context: ConditionContext;
  prefix: string;
  fieldError: (name: string) => string | undefined;
  onChange: (key: string, value: unknown) => void;
  /** The texts of the field controls, such as „Ja“ and „Bitte wählen“. */
  tr: CustomFieldInputProps["tr"];
  renderDate: (question: HolidayDateQuestion) => ReactNode;
}>) {
  const visible = fields.filter((field) => isFieldVisible(field, context));
  if (visible.length === 0) return null;
  return (
    <div className="space-y-4">
      {visible.map((field) => {
        if (field.type === "information") {
          return <InfoBlock key={field.key} field={field} />;
        }
        const name = `${prefix}custom_data.${field.key}`;
        const change = (value: unknown) => onChange(field.key, value);
        return (
          <CustomFieldInput
            key={field.key}
            field={field}
            inputName={name}
            value={answers[field.key]}
            onChange={change}
            error={fieldError(name)}
            tr={tr}
            renderStructured={() =>
              field.type === "date"
                ? renderDate({
                    name,
                    field,
                    value: answers[field.key],
                    error: fieldError(name),
                    onChange: change,
                  })
                : null
            }
          />
        );
      })}
    </div>
  );
}
