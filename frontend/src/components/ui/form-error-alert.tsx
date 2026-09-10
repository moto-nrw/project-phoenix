"use client";

import { useEffect, useRef } from "react";

import { Alert } from "./alert";

interface FormErrorAlertProps {
  /** The form's current error. Nothing renders while it is empty. */
  readonly message: string | null | undefined;
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
 * the message changes, so the person reads the reason instead of wondering
 * why nothing happened. A toast cannot do that job: it fades before anyone
 * has found the field it names.
 */
export function FormErrorAlert({ message, className }: FormErrorAlertProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!message) return;
    // happy-dom and older WebKit builds ship no scrollIntoView on divs.
    ref.current?.scrollIntoView?.({ behavior: "smooth", block: "nearest" });
  }, [message]);

  if (!message) return null;

  return (
    <div ref={ref} className={className}>
      <Alert type="error" message={message} />
    </div>
  );
}
