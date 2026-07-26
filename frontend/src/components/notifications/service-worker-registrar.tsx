"use client";

import { useEffect } from "react";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ServiceWorkerRegistrar" });

/**
 * Registers /sw.js once on mount (#2003). Registration is independent of the
 * push opt-in: an updated worker must roll out to devices that already
 * subscribed, and registering is side-effect-free for everyone else (no
 * permission prompt — that only ever happens from the explicit opt-in button).
 */
export function ServiceWorkerRegistrar() {
  useEffect(() => {
    if (!("serviceWorker" in navigator)) return;
    navigator.serviceWorker.register("/sw.js").catch((err: unknown) => {
      logger.warn("service_worker_registration_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    });
  }, []);

  return null;
}
