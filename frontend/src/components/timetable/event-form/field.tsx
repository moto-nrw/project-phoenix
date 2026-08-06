import { timetableRequiredMark } from "../timetable-style";

export function Field({
  label,
  htmlFor,
  required = false,
  error,
  action,
  children,
}: {
  label: string;
  htmlFor: string;
  required?: boolean;
  error?: string;
  /** Optional control rendered opposite the label, e.g. a "Verwalten" link. */
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  const labelEl = (
    <label htmlFor={htmlFor} className="text-xs font-semibold text-gray-700">
      {label}
      {required && <span className={timetableRequiredMark}>*</span>}
    </label>
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
      {error && (
        <p
          id={`${htmlFor}_error`}
          role="alert"
          className="text-moto-red mt-1 text-xs"
        >
          {error}
        </p>
      )}
    </div>
  );
}
