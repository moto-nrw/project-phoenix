"use client";

import { useEffect, useRef, type ReactNode } from "react";

import { Alert } from "./alert";
import { Button } from "./button";
import { ModuleNotBookedAlert } from "./module-not-booked-state";
import {
  formErrorAttempt,
  formErrorDetail,
  formErrorMessage,
  type FormErrorDetail,
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
  const detail = formErrorDetail(message);

  useEffect(() => {
    if (!text) return;
    // happy-dom and older WebKit builds ship no scrollIntoView on divs.
    ref.current?.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
  }, [text, attempt]);

  if (!text) return null;

  if (detail?.notBookedModule) {
    return (
      <div ref={ref} className={className}>
        <ModuleNotBookedAlert module={detail.notBookedModule} />
      </div>
    );
  }

  const action = errorAlertActions(detail);

  return (
    <div ref={ref} className={className}>
      <Alert
        type="error"
        message={text}
        action={action}
        actionLayout="stacked"
      />
    </div>
  );
}

/**
 * Retry and the request ID to copy, which a server or unavailable error from
 * the shared API error path carries (#2511). Nothing for other errors.
 */
export function errorAlertActions(
  detail: FormErrorDetail | null,
): ReactNode | undefined {
  const requestId = detail?.requestId;
  const retry = detail?.retry;
  if (!requestId && !retry) return undefined;
  return (
    // -ml-2.5 takes back the ghost button's own padding, so the first
    // action starts exactly under the message text.
    <span className="-ml-2.5 flex flex-wrap items-center gap-x-1">
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
  );
}

/**
 * A failed load shown where the data is missing (#2513), from
 * `useApiLoadError`. Unlike `FormErrorAlert` it does not scroll: nobody
 * pressed a button, so the page must not jump.
 */
export function LoadErrorAlert({
  error,
  className,
}: {
  readonly error: FormErrorInput;
  readonly className?: string;
}) {
  const text = formErrorMessage(error);
  const detail = formErrorDetail(error);
  if (!text) return null;
  // A module the school did not book is a state, not a failure (#4894): the
  // role-specific explanation names the next step, and there is nothing to retry.
  if (detail?.notBookedModule) {
    return (
      <ModuleNotBookedAlert
        module={detail.notBookedModule}
        className={className}
      />
    );
  }
  return (
    <Alert
      type="error"
      message={text}
      action={errorAlertActions(detail)}
      actionLayout="stacked"
      className={className}
    />
  );
}
