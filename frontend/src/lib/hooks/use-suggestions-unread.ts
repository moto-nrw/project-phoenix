"use client";

import { useSession } from "next-auth/react";
import { fetchUnreadCount } from "~/lib/suggestions-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useUnreadCount } from "./use-unread-count";

export function useSuggestionsUnread() {
  const { status } = useSession();
  // Per-tenant cache key: the suggestions unread count is tenant-scoped metadata
  // (this sidebar lives in the tenant-staff shell), so a tenant switch must not
  // surface the previous school's count from localStorage. Mirrors
  // useMessagesUnread; changing the key also re-runs the fetch (useUnreadCount's
  // refresh depends on cacheKey), so the badge refreshes on the tenant change.
  const tenantSlug = useTenantSlugSafe();
  return useUnreadCount({
    enabled: status === "authenticated",
    fetcher: fetchUnreadCount,
    cacheKey: `suggestions_unread_count:${tenantSlug ?? ""}`,
    eventNames: ["suggestions-unread-refresh"],
  });
}
