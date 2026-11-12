"use client";

import { useCallback, useLayoutEffect, useRef } from "react";

import { useApiLoadError } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import { reloadDocument } from "~/lib/reload-document";
import { isAccessRefusal, shownDataRefusalCount } from "~/lib/access-refusal";

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
 * A refused reload (401/403, #6801) is no stale display: the account may no
 * longer see the shown data. The hook then loads the whole page again
 * (`reloadDocument`), so the data, its actions and open dialogs go and the
 * page's first load shows the refusal. Before navigating it sets a load
 * error in case the navigation guard cancels the document reload; there is
 * no "saved" text and no retry. It sees the refusal either way: `reload`
 * rejects with it, or an SWR revalidation it ran was refused while its key
 * still held data (SWR keeps the data and never rejects; the middleware in
 * `~/lib/swr/hooks` counts it). There is no option to turn this off, so no
 * caller needs a 401/403 branch of its own for the reload after a write. A
 * `reload` must let a refusal reject (or run through SWR) and await what it
 * starts; one that catches a refusal hides it from the hook.
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
      const refusalsBefore = shownDataRefusalCount();
      let failure: unknown;
      let failed = false;
      try {
        await reloadRef.current(...args);
      } catch (err) {
        failure = err;
        failed = true;
      }
      if (
        (failed && isAccessRefusal(failure)) ||
        shownDataRefusalCount() !== refusalsBefore
      ) {
        logger.warn("refresh_after_write_refused", {
          object,
          status: isAccessRefusal(failure) ? failure.status : undefined,
        });
        void logger.flush();
        // Keep an error if beforeunload cancels navigation. Use the shared
        // permission code without an HTTP status: showing the original 401
        // would start a second navigation through useApiLoadError.
        await show(
          new ApiError("Access to refreshed data was refused", undefined, {
            code: "general.permission",
          }),
          { object },
        );
        reloadDocument();
        return false;
      }
      if (!failed) return true;
      logger.warn("refresh_after_write_failed", {
        object,
        error: failure instanceof Error ? failure.message : String(failure),
      });
      void show(failure, {
        object,
        messageSuffix: savedSuffix,
        retry: () => void refreshRef.current(...args),
      });
      return false;
    },
    [clear, object, savedSuffix, show],
  );
  useLayoutEffect(() => {
    refreshRef.current = refresh;
  });

  return { error, refresh, clear };
}
