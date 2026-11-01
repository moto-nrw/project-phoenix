"use client";

import { useEffect, useLayoutEffect, useRef } from "react";

import { useApiLoadError } from "~/contexts/ToastContext";

/**
 * Shows the error of a failed SWR load where the data is missing (#2514):
 * catalog text, retry and request ID through `useApiLoadError`. Render the
 * returned value in `LoadErrorAlert` or TenantPage `error`. A successful
 * reload clears it. With data, background failures are logged by the shared
 * SWR reader only: content and open dialogs must stay mounted.
 */
export function useSwrLoadError(
  error: unknown,
  object: string,
  retry: () => unknown,
  data?: unknown,
) {
  const { error: shown, show, clear } = useApiLoadError();
  const loadError = data === undefined ? error : undefined;
  // The retry closes over the latest mutate, not the one of the failed load.
  const retryRef = useRef(retry);
  useLayoutEffect(() => {
    retryRef.current = retry;
  });

  // Clears only a shown error: a needless clear would cost every page a
  // render.
  const shownRef = useRef(false);

  useEffect(() => {
    if (loadError) {
      shownRef.current = true;
      void show(loadError, { object, retry: () => void retryRef.current() });
    } else if (shownRef.current) {
      shownRef.current = false;
      clear();
    }
  }, [loadError, object, show, clear]);

  return loadError ? shown : null;
}
