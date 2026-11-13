"use client";

import { useEffect, useId, useImperativeHandle, useRef } from "react";

import { cn } from "~/lib/utils";

import { FieldLabel, isInvalidAria, isRequiredField } from "./field-label";

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  /** For a caller that sets the caret, such as inserting a placeholder. */
  readonly ref?: React.Ref<HTMLTextAreaElement>;
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
  /** Grow with the controlled `value`, then scroll inside the field. */
  readonly autoGrow?: boolean;
}

const AUTO_GROW_MAX_HEIGHT = 160;

// The cap is 160px, but at most a third of the visible viewport: with a phone's
// soft keyboard open the page is only a few hundred pixels tall, and a 160px
// field would push the line being typed out of view (#3664).
function fitToContent(field: HTMLTextAreaElement) {
  const visibleHeight = window.visualViewport?.height ?? window.innerHeight;
  const maxHeight = Math.min(
    AUTO_GROW_MAX_HEIGHT,
    Math.round(visibleHeight / 3),
  );
  field.style.height = "auto";
  field.style.height = `${Math.min(field.scrollHeight, maxHeight)}px`;
  field.style.overflowY = field.scrollHeight > maxHeight ? "auto" : "hidden";
}

function useAutoGrow(
  ref: React.RefObject<HTMLTextAreaElement | null>,
  enabled: boolean,
  value: unknown,
) {
  useEffect(() => {
    if (enabled && ref.current) fitToContent(ref.current);
  }, [ref, enabled, value]);
  // The keyboard opens after the field gains focus, without a value change.
  useEffect(() => {
    const field = ref.current;
    if (!enabled || !field) return;
    const onResize = () => fitToContent(field);
    const viewport = window.visualViewport;
    viewport?.addEventListener("resize", onResize);
    return () => viewport?.removeEventListener("resize", onResize);
  }, [ref, enabled]);
}

// Multiline counterpart to ui/Input — same ring-based chrome and focus
// treatment. Added for the absence decision/resubmit notes (#1419); reuse it
// for every new multiline field instead of hand-rolling a <textarea>.
export function Textarea({
  ref,
  autoGrow = false,
  label,
  name,
  id,
  error,
  hint,
  errorLive = false,
  className = "",
  "aria-describedby": describedBy,
  "aria-invalid": ariaInvalid,
  ...props
}: TextareaProps) {
  const generatedId = useId();
  const inputId = id ?? name ?? generatedId;
  const fieldRef = useRef<HTMLTextAreaElement>(null);
  useImperativeHandle(ref, () => fieldRef.current as HTMLTextAreaElement, []);
  useAutoGrow(fieldRef, autoGrow, props.value);
  const errorId = error ? `${inputId}-error` : undefined;
  const hintId = hint ? `${inputId}-hint` : undefined;

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
      <textarea
        ref={fieldRef}
        id={inputId}
        name={name}
        {...props}
        aria-invalid={error ? true : ariaInvalid}
        aria-describedby={
          [describedBy, hintId, errorId].filter(Boolean).join(" ") || undefined
        }
        className={cn(
          "aria-[invalid=true]:ring-moto-red block w-full resize-none rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 read-only:bg-gray-50 read-only:text-gray-500 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-600 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200 pointer-coarse:text-base",
          autoGrow && "max-h-40 overflow-hidden",
          className,
        )}
      />
      {hint && (
        <p id={hintId} className="mt-1 text-sm text-gray-600">
          {hint}
        </p>
      )}
      {error && (
        <p
          id={errorId}
          role={errorLive ? "alert" : undefined}
          className="text-moto-red-strong mt-1 text-sm"
        >
          {error}
        </p>
      )}
    </div>
  );
}
