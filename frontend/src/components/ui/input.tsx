"use client";

import { getDefaultMaxLength } from "~/lib/constants/input-limits";

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  readonly label?: string;
  readonly error?: string;
  readonly controlSize?: "default" | "compact";
}

export function Input({
  label,
  name,
  id,
  error,
  className = "",
  controlSize = "default",
  maxLength,
  type,
  ...props
}: InputProps) {
  // Mirrors ui/Textarea: a caller may identify the field with `id`, `name`, or
  // both. Deriving the association from `name` alone left `htmlFor` undefined
  // whenever only `id` was passed.
  const inputId = id ?? name;

  const controlClassName =
    controlSize === "compact"
      ? "block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200"
      : "block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200";

  return (
    <div>
      {label && (
        <label
          htmlFor={inputId}
          className="mb-2 block text-sm font-medium text-gray-700"
        >
          {label}
        </label>
      )}
      <input
        id={inputId}
        name={name}
        type={type}
        maxLength={maxLength ?? getDefaultMaxLength(type ?? "text")}
        aria-invalid={error ? true : undefined}
        className={`${controlClassName} ${className}`}
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
