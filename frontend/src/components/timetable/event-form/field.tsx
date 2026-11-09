import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";

export function Field({
  label,
  htmlFor,
  required = false,
  error,
  hint,
  action,
  children,
}: {
  label: string;
  htmlFor: string;
  required?: boolean;
  error?: string;
  /** A line under the field that stays visible. Point the control's `aria-describedby` at `${htmlFor}_hint`. */
  hint?: string;
  /** Optional control rendered opposite the label, e.g. a "Verwalten" link. */
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  const labelEl = (
    <FieldLabel
      htmlFor={htmlFor}
      required={required}
      invalid={Boolean(error)}
      className="mb-0 text-xs font-semibold"
    >
      {label}
    </FieldLabel>
  );

  return (
    <div className="flex flex-col gap-1">
      {action ? (
        <div className="flex items-baseline justify-between gap-2">
          {labelEl}
          {action}
        </div>
      ) : (
        labelEl
      )}
      {children}
      {hint && (
        <p id={`${htmlFor}_hint`} className="text-xs text-gray-600">
          {hint}
        </p>
      )}
      <FieldErrorHint id={`${htmlFor}_error`} message={error} live />
    </div>
  );
}
