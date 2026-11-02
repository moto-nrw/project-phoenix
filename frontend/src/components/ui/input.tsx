"use client";

import { forwardRef } from "react";

import { getDefaultMaxLength } from "~/lib/constants/input-limits";

import { FieldLabel, isInvalidAria, isRequiredField } from "./field-label";

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  readonly label?: string;
  readonly error?: string;
  /**
   * Read the hint out the moment it appears (`role="alert"`), for a check
   * that runs while the person types. A hint after Speichern stays quiet:
   * the form's `FormErrorAlert` announces the failure once.
   */
  readonly errorLive?: boolean;
  readonly controlSize?: "default" | "compact";
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  {
    label,
    name,
    id,
    error,
    errorLive = false,
    className = "",
    controlSize = "default",
    maxLength,
    type,
    "aria-describedby": describedBy,
    "aria-invalid": ariaInvalid,
    ...props
  },
  ref,
) {
  // Mirrors ui/Textarea: a caller may identify the field with `id`, `name`, or
  // both. Deriving the association from `name` alone left `htmlFor` undefined
  // whenever only `id` was passed.
  const inputId = id ?? name;
  const errorId = error && inputId ? `${inputId}-error` : undefined;

  const controlClassName =
    controlSize === "compact"
      ? "block h-10 w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200 read-only:bg-gray-50 read-only:text-gray-500 aria-[invalid=true]:ring-moto-red"
      : "block w-full rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200 read-only:bg-gray-50 read-only:text-gray-500 aria-[invalid=true]:ring-moto-red";

  return (
    <div>
      {label && (
        <FieldLabel
          htmlFor={inputId}
          required={isRequiredField(props)}
          invalid={Boolean(error) || isInvalidAria(ariaInvalid)}
        >
          {label}
        </FieldLabel>
      )}
      <input
        ref={ref}
        id={inputId}
        name={name}
        type={type}
        maxLength={maxLength ?? getDefaultMaxLength(type ?? "text")}
        {...props}
        aria-invalid={error ? true : ariaInvalid}
        aria-describedby={
          [describedBy, errorId].filter(Boolean).join(" ") || undefined
        }
        className={`${controlClassName} ${className}`}
      />
      {error && (
        <p
          id={errorId}
          role={errorLive ? "alert" : undefined}
          className="text-moto-red-strong mt-1 text-xs"
        >
          {error}
        </p>
      )}
    </div>
  );
});
