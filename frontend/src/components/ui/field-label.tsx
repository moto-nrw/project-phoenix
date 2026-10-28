import type { ReactNode } from "react";

import { cn } from "~/lib/utils";

/**
 * The star after the label of a field that must be filled in (#5217).
 *
 * The one way a form marks a required field: `Input`, `Textarea`,
 * `TimeField`, `FieldLabel` and `ConsentCheckbox` render it from their
 * `required` prop, a `<legend>` of a required group renders it after its
 * text. Never write the star into a translation or a label string. It is
 * hidden from screen readers, which hear "required" from the control's
 * `required` / `aria-required` instead.
 */
export function RequiredMark() {
  return (
    <span aria-hidden="true" className="text-moto-red-strong">
      {" *"}
    </span>
  );
}

/**
 * Whether a field asks for the star: native `required`, or `aria-required`
 * on a form that runs its own check instead of the browser's.
 */
export function isRequiredField(props: {
  readonly required?: boolean;
  readonly "aria-required"?: boolean | "true" | "false";
}): boolean {
  return (
    props.required === true ||
    props["aria-required"] === true ||
    props["aria-required"] === "true"
  );
}

/**
 * The visible label of one form field, for a control that has no `label`
 * prop of its own (`CustomSelect`, `DatePicker`, a picker of its own). Point
 * it at the control with `htmlFor`, or give it an `id` the control names in
 * `aria-labelledby`. `Input` and `Textarea` render the same label themselves.
 */
export function FieldLabel({
  children,
  required = false,
  invalid = false,
  htmlFor,
  id,
  className,
}: Readonly<{
  children: ReactNode;
  required?: boolean;
  /** The field failed a check: the label turns red like its hint. */
  invalid?: boolean;
  htmlFor?: string;
  id?: string;
  className?: string;
}>) {
  // The star sits next to the label element, not inside it: the label's
  // text stays the field's name ("Vorname", not "Vorname *") for screen
  // readers and for everything that finds a field by its label.
  const text = htmlFor ? (
    <label id={id} htmlFor={htmlFor}>
      {children}
    </label>
  ) : (
    <span id={id}>{children}</span>
  );
  return (
    <span
      className={cn(
        "mb-2 block text-sm font-medium",
        invalid ? "text-moto-red-strong" : "text-gray-700",
        className,
      )}
    >
      {text}
      {required && <RequiredMark />}
    </span>
  );
}
