"use client";

import { useEffect, useLayoutEffect, useRef } from "react";

import { useApiLoadError } from "~/contexts/ToastContext";

/**
 * Shows the error of a failed SWR load where the data is missing (#2514):
 * catalog text, retry and request ID through `useApiLoadError`. Render the
 * returned value in `LoadErrorAlert` or TenantPage `error`. A successful
 * reload clears it.
 */
export function useSwrLoadError(
  error: unknown,
  object: string,
  retry: () => unknown,
) {
  const { error: shown, show, clear } = useApiLoadError();
  // The retry closes over the latest mutate, not the one of the failed load.
  const retryRef = useRef(retry);
  useLayoutEffect(() => {
    retryRef.current = retry;
  });

  // Clears only a shown error: a needless clear would cost every page a
  // render.
  const shownRef = useRef(false);

  useEffect(() => {
    if (error) {
      shownRef.current = true;
      void show(error, { object, retry: () => void retryRef.current() });
    } else if (shownRef.current) {
      shownRef.current = false;
      clear();
    }
  }, [error, object, show, clear]);

  return shown;
}
