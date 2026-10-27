"use client";

import { useSession } from "next-auth/react";
import { hasPermission } from "~/lib/auth-utils";
import { useTenantParentNewsEnabled } from "~/lib/tenant-context";

/**
 * Ob die Person Elternmitteilungen schreiben kann. Dieselbe Bedingung wie der
 * Eintrag „Mitteilungen" in der Seitenleiste: jede Route unter
 * /api/parent-announcements verlangt admin:*, und die Schule muss
 * `operations.parent_news_enabled` eingeschaltet haben (vom Tenant-Resolve,
 * #4996). So führt kein Knopf auf eine Seite, die der Person fehlt.
 */
export function useParentAnnouncementsAvailable(): boolean {
  const { data: session } = useSession();
  const canAnnounce = hasPermission(session, "admin:*");
  const parentNewsEnabled = useTenantParentNewsEnabled();
  return canAnnounce && parentNewsEnabled;
}
