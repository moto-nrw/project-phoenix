"use client";

import { useContext } from "react";
import { TenantContext } from "~/components/tenant/tenant-provider";
import type { PresenceMode } from "~/lib/tenant-api";

/**
 * Returns the current tenant's presence mode ("detailed" | "binary").
 *
 * Falls back to "detailed" when called outside a TenantProvider or before
 * the tenant metadata has resolved — matches the backend's safe default so
 * first-paint shows the richer view even if the tenant turns out to be
 * binary (the switch happens transparently on the next render).
 *
 * Safe to call from any component. Consumers:
 *  - StudentPresenceBadge (picks between PresenceBadge and LocationBadge)
 *  - Sidebar (hides room/activity nav items in binary mode)
 *  - Page guards (404 on /rooms, /active-supervisions in binary mode)
 */
export function usePresenceMode(): PresenceMode {
  const ctx = useContext(TenantContext);
  return ctx?.tenant?.presenceMode ?? "detailed";
}
