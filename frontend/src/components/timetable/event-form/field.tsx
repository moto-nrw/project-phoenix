import { timetableRequiredMark } from "../timetable-style";

export function Field({
  label,
  htmlFor,
  required = false,
  error,
  children,
}: {
  label: string;
  htmlFor: string;
  required?: boolean;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <label htmlFor={htmlFor} className="text-xs font-semibold text-gray-700">
        {label}
        {required && <span className={timetableRequiredMark}>*</span>}
      </label>
      {children}
      {error && (
        <p
          id={`${htmlFor}_error`}
          role="alert"
          className="mt-1 text-xs text-[#FF3130]"
        >
          {error}
        </p>
      )}
    </div>
  );
}
