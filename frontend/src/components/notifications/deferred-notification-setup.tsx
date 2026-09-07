"use client";

import { useEffect, useMemo, useState, type ComponentType } from "react";
import {
  isPushSupported,
  isStandaloneApp,
  needsIOSInstall,
  type PushPortal,
} from "~/lib/push-api";
import { createLogger } from "~/lib/logger";
import { isAndroidDevice, isSamsungInternet } from "~/lib/pwa-install-prompt";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  setupStorageKey,
  shouldStartNotificationSetup,
} from "./notification-setup-decision";

const logger = createLogger({ component: "DeferredNotificationSetup" });

type NotificationSetupDialog = ComponentType<{
  portal: PushPortal;
  accountId: string;
}>;

function canRequireNotificationSetup(): boolean {
  return (
    needsIOSInstall() ||
    isPushSupported() ||
    (isAndroidDevice(window.navigator) &&
      !isSamsungInternet(window.navigator) &&
      !isStandaloneApp())
  );
}

/**
 * Loads the guided dialog only for a device that can use it and has not
 * already completed or postponed its setup. This wrapper stays in the shell;
 * the heavy dialog code does not.
 */
export function DeferredNotificationSetup({
  portal,
  accountId,
}: Readonly<{
  portal: PushPortal;
  accountId: string;
}>) {
  const tenantSlug = useTenantSlugSafe();
  const storageKey = useMemo(
    () => setupStorageKey(portal, accountId, tenantSlug),
    [accountId, portal, tenantSlug],
  );
  const [Dialog, setDialog] = useState<NotificationSetupDialog | null>(null);

  useEffect(() => {
    if (
      !canRequireNotificationSetup() ||
      !shouldStartNotificationSetup(storageKey)
    ) {
      setDialog(null);
      return;
    }

    let active = true;
    void import("./notification-setup-dialog")
      .then(({ NotificationSetupDialog }) => {
        if (active) setDialog(() => NotificationSetupDialog);
      })
      .catch((error: unknown) => {
        logger.error("notification_setup_load_failed", {
          error: error instanceof Error ? error.message : String(error),
        });
      });
    return () => {
      active = false;
    };
  }, [storageKey]);

  return Dialog ? <Dialog portal={portal} accountId={accountId} /> : null;
}
