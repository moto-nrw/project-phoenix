import { useCallback, useEffect, useRef, useState } from "react";

/**
 * Scroll-to-first-error for long, multi-field forms.
 *
 * Attach `formRef` to the <form> and `errorRef` to the top-level error banner,
 * mark every invalid input/select/checkbox with `aria-invalid="true"`, then
 * call `scrollToError()` after errors are set — both on client-side validation
 * failures and on late-arriving server errors. Each call scrolls the first
 * `[aria-invalid="true"]` field into view so the user lands on the actual
 * problem and its inline message, falling back to the banner for server-level
 * errors that mark no field.
 *
 * It targets the offending field and is keyed on a per-attempt counter, so a
 * repeated submit with the same unchanged error still scrolls back to it.
 */
export function useScrollToFirstError() {
  const formRef = useRef<HTMLFormElement>(null);
  const errorRef = useRef<HTMLDivElement>(null);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (attempt === 0) return;
    const firstInvalid = formRef.current?.querySelector<HTMLElement>(
      '[aria-invalid="true"]',
    );
    (firstInvalid ?? errorRef.current)?.scrollIntoView({
      behavior: "smooth",
      block: "center",
    });
  }, [attempt]);

  const scrollToError = useCallback(() => setAttempt((n) => n + 1), []);

  return { formRef, errorRef, scrollToError };
}
