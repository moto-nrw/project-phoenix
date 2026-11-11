"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

import { useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "RefreshAfterWrite" });

/** Shown data stays while a reload fails; this says it may be old (#6636). */
export const STALE_DATA_SUFFIX =
  "Die angezeigten Angaben sind vielleicht nicht mehr aktuell.";

/** Says that the write held, whatever the failed reload reported (#6636). */
export const SAVED_BUT_STALE_SUFFIX =
  "Ihre Änderung ist gespeichert. Nur die Anzeige ist noch nicht aktuell.";

export interface RefreshAfterWriteResult {
  readonly error: unknown;
}

/**
 * Reloads the shown data after a confirmed write (#6636). The write has
 * already succeeded, so a failed reload is a load error, never an error of
 * the action: it goes to `useApiLoadError` (render `error` in
 * `LoadErrorAlert`), says that the change is saved, and its retry only
 * reloads. `refresh()` never throws; it resolves `false` when the reload
 * failed, so a caller cannot route the failure into its action's catch.
 * SWR revalidation resolves even on fetch failure: reload callbacks must
 * return its current error via the reader's protected `readCurrent()`.
 */
export function useRefreshAfterWrite(
  object: string,
  reload: () => Promise<RefreshAfterWriteResult>,
) {
  const { error, show, clear } = useApiLoadError();
  // The retry runs the latest reload, not the one of the failed attempt.
  const reloadRef = useRef(reload);
  useLayoutEffect(() => {
    reloadRef.current = reload;
  });
  const refreshRef = useRef<() => Promise<boolean>>(() =>
    Promise.resolve(true),
  );

  const refresh = useCallback(async () => {
    clear();
    try {
      const result = await reloadRef.current();
      if (result.error) throw result.error;
      return true;
    } catch (err) {
      logger.warn("refresh_after_write_failed", {
        object,
        error: err instanceof Error ? err.message : String(err),
      });
      void show(err, {
        object,
        messageSuffix: SAVED_BUT_STALE_SUFFIX,
        retry: () => void refreshRef.current(),
      });
      return false;
    }
  }, [clear, object, show]);
  useLayoutEffect(() => {
    refreshRef.current = refresh;
  });

  return { error, refresh, clear };
}
