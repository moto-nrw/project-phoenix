"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { createLogger } from "~/lib/logger";
import {
  isPushConfigurationMissing,
  isPushSupported,
  syncExistingPushSubscription,
  type PushPortal,
} from "~/lib/push-api";

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

/**
 * Rebinds an already opted-in device whenever an authenticated portal session
 * starts. This fixes shared-device and failed-logout cleanup cases without
 * prompting users who have never enabled notifications.
 */
export function PushSubscriptionSync({
  portal,
}: {
  readonly portal: PushPortal;
}) {
  const { status } = useSession();

  useEffect(() => {
    if (status !== "authenticated" || !isPushSupported()) return;
    syncExistingPushSubscription(portal).catch((err: unknown) => {
      if (isPushConfigurationMissing(err)) return;
      logger.warn("push_subscription_sync_failed", {
        portal,
        error: err instanceof Error ? err.message : String(err),
      });
    });
  }, [portal, status]);

  return null;
}
