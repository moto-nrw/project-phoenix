"use client";

import { useCallback, useState } from "react";

/**
 * A form's current error together with the number of the attempt that
 * produced it. The counter is what lets the alert scroll into view on EVERY
 * failed save, including a repeated click that yields the same sentence:
 * React skips a re-render for an unchanged string, so a plain string state
 * cannot tell the second attempt from the first.
 */
export interface FormError {
  readonly message: string;
  readonly attempt: number;
}

/** What the error slots accept: a plain string still works for one-shot
 *  messages; `useFormError` yields the counted form. */
export type FormErrorInput = string | FormError | null | undefined;

export function formErrorMessage(error: FormErrorInput): string | null {
  if (!error) return null;
  const message = typeof error === "string" ? error : error.message;
  return message ? message : null;
}

export function formErrorAttempt(error: FormErrorInput): number {
  return typeof error === "object" && error !== null ? error.attempt : 0;
}

/**
 * State for a form's validation or save error.
 *
 * `setError(message)` records a new attempt every time it is called with a
 * message, even the same one; `setError(null)` (or an empty string) clears
 * it. Pass the value to `FormModal` / `SlideOverBody` `error` or to
 * `FormErrorAlert` `message`; read the text via `error?.message`.
 */
export function useFormError(): readonly [
  FormError | null,
  (message: string | null | undefined) => void,
] {
  const [error, setState] = useState<FormError | null>(null);
  const setError = useCallback((message: string | null | undefined) => {
    setState((previous) =>
      message ? { message, attempt: (previous?.attempt ?? 0) + 1 } : null,
    );
  }, []);
  return [error, setError] as const;
}
