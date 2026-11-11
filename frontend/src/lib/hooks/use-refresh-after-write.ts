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

/**
 * The `refresh` of `useRefreshAfterWrite`, handed to a dialog that closes
 * after its write (#6652). It never rejects, so the dialog may call it after
 * its write; the owner of the shown data renders the load error, which
 * outlives the closed dialog.
 */
export type RefreshAfterWrite<A extends unknown[] = []> = (
  ...args: A
) => Promise<boolean>;

/**
 * Reloads the shown data after a confirmed write (#6636, #6652). The write
 * has already succeeded, so a failed reload is a load error, never an error
 * of the action: it goes to `useApiLoadError` (render `error` in
 * `LoadErrorAlert`), says that the change is saved, and its retry only
 * reloads, with the arguments of the failed attempt. `refresh()` never
 * throws; it resolves `false` when the reload failed, so a caller cannot
 * route the failure into its action's catch.
 *
 * This is the one place that separates a write's error from its reload's
 * error; `error-path/no-reload-in-write-catch` rejects a reload in the
 * write's catch.
 */
export function useRefreshAfterWrite<A extends unknown[] = []>(
  object: string,
  reload: (...args: A) => Promise<unknown> | unknown,
  options: {
    /** The translated `SAVED_BUT_STALE_SUFFIX` on a portal with languages. */
    readonly savedSuffix?: string;
  } = {},
) {
  const savedSuffix = options.savedSuffix ?? SAVED_BUT_STALE_SUFFIX;
  const { error, show, clear } = useApiLoadError();
  // The retry runs the latest reload, not the one of the failed attempt.
  const reloadRef = useRef(reload);
  useLayoutEffect(() => {
    reloadRef.current = reload;
  });
  const refreshRef = useRef<RefreshAfterWrite<A>>(() => Promise.resolve(true));

  const refresh = useCallback<RefreshAfterWrite<A>>(
    async (...args: A) => {
      clear();
      try {
        await reloadRef.current(...args);
        return true;
      } catch (err) {
        logger.warn("refresh_after_write_failed", {
          object,
          error: err instanceof Error ? err.message : String(err),
        });
        void show(err, {
          object,
          messageSuffix: savedSuffix,
          retry: () => void refreshRef.current(...args),
        });
        return false;
      }
    },
    [clear, object, savedSuffix, show],
  );
  useLayoutEffect(() => {
    refreshRef.current = refresh;
  });

  return { error, refresh, clear };
}
