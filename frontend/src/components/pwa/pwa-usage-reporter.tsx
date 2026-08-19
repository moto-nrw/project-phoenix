"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import type { PushPortal } from "~/lib/push-api";
import { reportStandaloneUsage } from "~/lib/pwa-usage-api";

/**
 * Reports PWA standalone usage once per authenticated session (#2189).
 * Mounted next to PushSubscriptionSync in the portal providers; renders
 * nothing and never blocks anything — the report itself swallows all errors.
 */
export function PwaUsageReporter({ portal }: { readonly portal: PushPortal }) {
  const { data: session, status } = useSession();

  useEffect(() => {
    if (status !== "authenticated" || !session?.user.id) return;
    void reportStandaloneUsage(portal, session.user.id, session.user.tenantId);
  }, [portal, session?.user.id, session?.user.tenantId, status]);

  return null;
}
