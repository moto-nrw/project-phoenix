"use client";

import { useEffect, useLayoutEffect, useRef } from "react";

import { createLogger } from "~/lib/logger";

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

interface SwrLoadResource {
  readonly data?: unknown;
  readonly error?: unknown;
  /** Partial failure payloads are not confirmed initial data. */
  readonly hasConfirmedData?: boolean;
}

const logger = createLogger({ component: "SwrLoadError" });

/**
 * A failed revalidation must not unmount confirmed content or open dialogs.
 * Only failures without confirmed data block rendering. Scope/access discards
 * still remove the data at the resource boundary, not in this presenter.
 */
export function useSwrResourceLoadError(
  resources: readonly SwrLoadResource[],
  object: string,
  retry: () => unknown,
) {
  const hasData = (resource: SwrLoadResource) =>
    resource.hasConfirmedData ?? resource.data !== undefined;
  const failure = resources.find((item) => item.error && !hasData(item))?.error;
  const backgroundError = resources.find(
    (item) => item.error && hasData(item),
  )?.error;
  useEffect(() => {
    if (backgroundError)
      logger.warn("swr_background_refresh_failed", {
        object,
        error:
          backgroundError instanceof Error
            ? backgroundError.message
            : String(backgroundError),
      });
  }, [backgroundError, object]);
  const error = useSwrLoadError(failure, object, retry);
  // Hide a previous presentation synchronously, even before its effect clears.
  return { error: failure ? error : null, failure };
}
