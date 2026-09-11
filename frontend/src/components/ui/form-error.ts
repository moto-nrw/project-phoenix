"use client";

import { useReducer } from "react";

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
  // useReducer: the dispatch is identity-stable for the component's life, so
  // a handler may list `setError` in its dependency array without re-creating
  // itself on every render. The lint cannot see that through a custom hook,
  // so callers still list it; that is correct and cheap.
  const [state, setError] = useReducer(nextFormErrorState, EMPTY_STATE);
  return [state.error, setError] as const;
}

interface FormErrorState {
  readonly error: FormError | null;
  /** Attempts seen so far, kept across clears: a handler that clears the
   *  error and sets it again in the same tick (React batches both) must
   *  still land on a NEW attempt number, or the alert would not re-scroll. */
  readonly attempts: number;
}

const EMPTY_STATE: FormErrorState = { error: null, attempts: 0 };

function nextFormErrorState(
  previous: FormErrorState,
  message: string | null | undefined,
): FormErrorState {
  if (!message) {
    return previous.error === null
      ? previous
      : { error: null, attempts: previous.attempts };
  }
  const attempts = previous.attempts + 1;
  return { error: { message, attempt: attempts }, attempts };
}
