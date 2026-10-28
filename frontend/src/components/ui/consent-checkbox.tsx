"use client";

import type { ReactNode } from "react";

import { Checkbox } from "./checkbox";
import { ChoiceTile } from "./choice-tile";
import { RequiredMark } from "./field-label";
import { FieldErrorHint } from "./field-error-hint";

/**
 * One consent or confirmation a parent ticks in a form: the box, its text,
 * the star when it is required and the hint when the check refused it
 * (#5217). The OGS-Anmeldung and the Ferienanmeldung both build their
 * Einwilligungen from it, so they look and behave the same.
 *
 * `action` sits at the end of the row, outside the label, so a button there
 * (the legal text to read) never ticks the box.
 */
export function ConsentCheckbox({
  name,
  label,
  checked,
  onChange,
  required = false,
  error,
  action,
  id,
}: Readonly<{
  /** Form name; the error path finds the field by it. */
  name: string;
  label: ReactNode;
  checked: boolean;
  onChange: (checked: boolean) => void;
  required?: boolean;
  /** The hint under the row. Nothing renders while it is empty. */
  error?: string;
  action?: ReactNode;
  /** Defaults to `name`. */
  id?: string;
}>) {
  const inputId = id ?? name;
  const errorId = `${inputId}-error`;
  return (
    <div>
      <ChoiceTile
        as="div"
        selected={checked}
        tone="green"
        className={`items-start p-3 ${error && !checked ? "border-moto-red" : ""}`}
      >
        <label
          htmlFor={inputId}
          className="flex min-w-0 flex-1 cursor-pointer items-start gap-3"
        >
          <Checkbox
            id={inputId}
            name={name}
            checked={checked}
            required={required}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? errorId : undefined}
            onChange={(event) => onChange(event.target.checked)}
            className="mt-0.5"
          />
          <span className="min-w-0 flex-1 leading-6 text-gray-900">
            {label}
            {required && <RequiredMark />}
          </span>
        </label>
        {action}
      </ChoiceTile>
      <FieldErrorHint id={errorId} message={error} />
    </div>
  );
}
