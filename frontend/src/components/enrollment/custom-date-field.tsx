"use client";

// Eine Datumsfrage eines Formulars: Beschriftung, Hilfetext, Fehler am Feld.
// Das Bedienelement ist die Datumsauswahl des Kits; der Aufrufer bringt sie
// mit, übersetzt (CustomDateFieldInput) oder erst bei Bedarf geladen
// (Bearbeitung durch das Träger-Büro, #5629).

import type { ReactNode } from "react";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { type CustomFieldInputProps, HelpText } from "./custom-field-input";

/** What the frame hands its date control. */
export interface DateControlProps {
  readonly id: string;
  readonly name: string;
  /** "YYYY-MM-DD", or "" when unset. */
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly invalid: boolean;
  readonly ariaDescribedBy: string | undefined;
}

export function CustomDateField({
  field,
  inputName,
  value,
  onChange,
  error,
  controlId,
  renderControl,
}: Pick<
  CustomFieldInputProps,
  "field" | "inputName" | "value" | "onChange" | "error" | "controlId"
> &
  Readonly<{ renderControl: (control: DateControlProps) => ReactNode }>) {
  const dateId = controlId ?? `enrollment-${inputName}`;
  const errorId = `${dateId}-error`;
  return (
    <div>
      <FieldLabel
        htmlFor={dateId}
        required={field.required}
        invalid={Boolean(error)}
      >
        {field.label}
      </FieldLabel>
      <HelpText text={field.help_text} />
      {renderControl({
        id: dateId,
        name: inputName,
        value: typeof value === "string" ? value : "",
        onChange,
        invalid: Boolean(error),
        ariaDescribedBy: error ? errorId : undefined,
      })}
      <FieldErrorHint id={errorId} message={error} />
    </div>
  );
}
