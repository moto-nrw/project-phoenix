"use client";

import { useEffect, useRef } from "react";

import { Alert } from "./alert";
import { Button } from "./button";
import {
  formErrorAttempt,
  formErrorDetail,
  formErrorMessage,
  type FormErrorInput,
} from "./form-error";
import { RequestIdButton } from "./request-id-button";

interface FormErrorAlertProps {
  /** The form's current error. Nothing renders while it is empty. A value
   *  from `useFormError` carries the attempt counter; a plain string works
   *  for a one-shot message. */
  readonly message: FormErrorInput;
  readonly className?: string;
}

/**
 * The one place a form reports what went wrong (BAUARTEN-SPEC, Bauart 2
 * Regel 5): an error `Alert` at the top of the edit area. `FormModal` and
 * `SlideOverBody` render it from their `error` prop; a form that lives on a
 * page or card uses it directly.
 *
 * Why not just `<Alert>`: a long form is usually scrolled to its Speichern
 * button when the error arrives. The alert scrolls itself into view whenever
 * the message changes or a new attempt fails with the same message, so the
 * person reads the reason instead of wondering why nothing happened. A toast
 * cannot do that job: it fades before anyone has found the field it names.
 */
export function FormErrorAlert({ message, className }: FormErrorAlertProps) {
  const ref = useRef<HTMLDivElement>(null);
  const text = formErrorMessage(message);
  const attempt = formErrorAttempt(message);

  useEffect(() => {
    if (!text) return;
    // happy-dom and older WebKit builds ship no scrollIntoView on divs.
    ref.current?.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
  }, [text, attempt]);

  if (!text) return null;

  // A server or unavailable error from the shared API error path carries a
  // retry and the request ID to copy (#2511).
  const detail = formErrorDetail(message);
  const requestId = detail?.requestId;
  const retry = detail?.retry;
  const action =
    requestId || retry ? (
      <span className="flex flex-wrap items-center justify-end gap-1">
        {retry ? (
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={retry.onClick}
            className="shrink-0 self-center text-current underline underline-offset-2 hover:bg-black/5 hover:text-current"
          >
            {retry.label}
          </Button>
        ) : null}
        {requestId ? (
          <RequestIdButton
            requestId={requestId.value}
            label={requestId.label}
            copyLabel={requestId.copyLabel}
            copiedLabel={requestId.copiedLabel}
            copyFailedLabel={requestId.copyFailedLabel}
          />
        ) : null}
      </span>
    ) : undefined;

  return (
    <div ref={ref} className={className}>
      <Alert type="error" message={text} action={action} />
    </div>
  );
}
