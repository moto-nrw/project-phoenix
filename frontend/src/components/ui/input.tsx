"use client";

import { Eye, EyeOff } from "lucide-react";
import { forwardRef, useState } from "react";

import { getDefaultMaxLength } from "~/lib/constants/input-limits";

import { FieldLabel, isInvalidAria, isRequiredField } from "./field-label";

interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  readonly label?: string;
  readonly error?: string;
  /** A line under the field that stays visible, read out with the field. */
  readonly hint?: string;
  /**
   * Read the hint out the moment it appears (`role="alert"`), for a check
   * that runs while the person types. A hint after Speichern stays quiet:
   * the form's `FormErrorAlert` announces the failure once.
   */
  readonly errorLive?: boolean;
  /**
   * `compact`: 40 px, like a `field` button. `touch`: 40 px from `lg` on,
   * 44 px with 16-px text below (finger height, no zoom on iOS) — for a field
   * that stands alone in a phone layout, such as the help search.
   */
  readonly controlSize?: "default" | "compact" | "touch";
  /**
   * With `type="password"`: an eye button in the field shows and hides the
   * value. The field keeps the visibility itself unless `revealed` is passed.
   */
  readonly revealable?: boolean;
  /** Who owns the visibility when two fields share one eye button. */
  readonly revealed?: boolean;
  readonly onRevealedChange?: (revealed: boolean) => void;
  // Plain strings instead of useTranslations: staff and operator logins render
  // without a NextIntlClientProvider. The parents portal passes resolved text.
  readonly showPasswordLabel?: string;
  readonly hidePasswordLabel?: string;
}

const CONTROL_SIZE_CLASS = {
  default: "px-4 py-3 text-base",
  compact: "h-10 px-3 py-2 text-sm",
  touch: "h-11 px-3 py-2 text-base lg:h-10 lg:text-sm",
} as const;

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  {
    label,
    name,
    id,
    error,
    hint,
    errorLive = false,
    className = "",
    controlSize = "default",
    maxLength,
    type,
    revealable = false,
    revealed,
    onRevealedChange,
    showPasswordLabel = "Passwort anzeigen",
    hidePasswordLabel = "Passwort verbergen",
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
  const [ownRevealed, setOwnRevealed] = useState(false);
  const isRevealed = revealed ?? ownRevealed;
  const errorId = error && inputId ? `${inputId}-error` : undefined;
  const hintId = hint && inputId ? `${inputId}-hint` : undefined;

  const controlClassName = `block w-full rounded-lg border-0 bg-white text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200 read-only:bg-gray-50 read-only:text-gray-500 aria-[invalid=true]:ring-moto-red ${CONTROL_SIZE_CLASS[controlSize]}`;

  const revealPadding = controlSize === "default" ? "pr-12" : "pr-10";
  const control = (
    <input
      ref={ref}
      id={inputId}
      name={name}
      type={revealable && isRevealed ? "text" : type}
      maxLength={maxLength ?? getDefaultMaxLength(type ?? "text")}
      {...props}
      aria-invalid={error ? true : ariaInvalid}
      aria-describedby={
        [describedBy, hintId, errorId].filter(Boolean).join(" ") || undefined
      }
      className={`${controlClassName} ${revealable ? revealPadding : ""} ${className}`}
    />
  );

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
      {revealable ? (
        <div className="relative">
          {control}
          <button
            type="button"
            onClick={() => {
              setOwnRevealed(!isRevealed);
              onRevealedChange?.(!isRevealed);
            }}
            className="absolute top-1/2 right-2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none pointer-coarse:right-1 pointer-coarse:h-11 pointer-coarse:w-11"
            aria-label={isRevealed ? hidePasswordLabel : showPasswordLabel}
          >
            {isRevealed ? (
              <EyeOff className="h-5 w-5" aria-hidden="true" />
            ) : (
              <Eye className="h-5 w-5" aria-hidden="true" />
            )}
          </button>
        </div>
      ) : (
        control
      )}
      {hint && (
        <p id={hintId} className="mt-1 text-xs text-gray-600">
          {hint}
        </p>
      )}
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
