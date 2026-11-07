import { useId, type ComponentProps, type ReactNode } from "react";

import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { SectionLabel } from "~/components/ui/section-label";
import { cn } from "~/lib/utils";

/**
 * FormFieldset: a framed group of related fields, with its name on the frame.
 *
 * `required` adds the star to the legend, `hint` a line above the fields.
 * `error` turns the frame red, shows the message under the fields and links it
 * to the group for screen readers. Everything else goes to the native
 * `fieldset`, for example `disabled`, `id` or `ref`.
 */
export function FormFieldset({
  legend,
  required = false,
  hint,
  error,
  errorId,
  className,
  children,
  "aria-describedby": describedBy,
  ...rest
}: Readonly<
  Omit<ComponentProps<"fieldset">, "aria-invalid"> & {
    legend: ReactNode;
    required?: boolean;
    hint?: ReactNode;
    error?: string | null;
    /** The error line's id, for a form that finds or names it from outside. */
    errorId?: string;
  }
>) {
  const baseId = useId();
  const hintId = `${baseId}-hint`;
  const resolvedErrorId = errorId ?? `${baseId}-error`;
  const describedByIds =
    [describedBy, hint ? hintId : null, error ? resolvedErrorId : null]
      .filter(Boolean)
      .join(" ") || undefined;
  return (
    <fieldset
      aria-invalid={error ? true : undefined}
      aria-describedby={describedByIds}
      className={cn(
        "rounded-xl border p-4",
        error ? "border-moto-red" : "border-gray-200",
        className,
      )}
      {...rest}
    >
      <SectionLabel as="legend" required={required} className="px-1">
        {legend}
      </SectionLabel>
      {hint ? (
        <p id={hintId} className="mb-2 text-xs leading-5 text-gray-500">
          {hint}
        </p>
      ) : null}
      {children}
      <FieldErrorHint id={resolvedErrorId} message={error} />
    </fieldset>
  );
}
