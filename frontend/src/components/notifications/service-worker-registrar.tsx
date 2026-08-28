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
import { reportStandaloneUsage } from "~/lib/pwa-usage-api";

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
  const { data: session, status } = useSession();

  // Re-runs whenever the authenticated identity changes, not just on the
  // status flip: a second tab can swap the account or the school underneath a
  // mounted session, and a subscription still bound to the previous identity
  // would keep delivering that account's notifications to this device (#2208).
  const accountID = session?.user.id;
  const tenantID = session?.user.tenantId;

  useEffect(() => {
    if (status !== "authenticated" || !isPushSupported()) return;
    syncExistingPushSubscription(portal).catch((err: unknown) => {
      if (isPushConfigurationMissing(err)) return;
      logger.warn("push_subscription_sync_failed", {
        portal,
        error: err instanceof Error ? err.message : String(err),
      });
    });
  }, [portal, status, accountID, tenantID]);

  useEffect(() => {
    if (status !== "authenticated" || !accountID) return;
    void reportStandaloneUsage(portal, accountID, tenantID);
  }, [portal, accountID, tenantID, status]);

  return null;
}
