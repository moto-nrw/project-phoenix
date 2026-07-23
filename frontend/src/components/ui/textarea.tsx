"use client";

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  readonly label?: string;
  readonly error?: string;
}

// Multiline counterpart to ui/Input — same ring-based chrome and focus
// treatment. Added for the absence decision/resubmit notes (#1419); reuse it
// for every new multiline field instead of hand-rolling a <textarea>.
export function Textarea({
  label,
  name,
  error,
  className = "",
  ...props
}: TextareaProps) {
  return (
    <div>
      {label && (
        <label
          htmlFor={name}
          className="mb-2 block text-sm font-medium text-gray-700"
        >
          {label}
        </label>
      )}
      <textarea
        id={name}
        name={name}
        aria-invalid={error ? true : undefined}
        className={`block w-full resize-none rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200 ${className}`}
        {...props}
      />
      {error && (
        <p role="alert" className="mt-1 text-xs text-red-600">
          {error}
        </p>
      )}
    </div>
  );
}
