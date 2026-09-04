"use client";

import { useSession } from "next-auth/react";

import { fetchCareWithdrawals } from "~/lib/care-exit-api";
import { canReviewCareWithdrawals } from "~/lib/change-request-access";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useShellSeed } from "~/lib/shell-seed";
import { useUnreadCount } from "./use-unread-count";

export function useCareWithdrawalsPending() {
  const { data: session, status } = useSession();
  const { mode } = useShellAuth();
  const tenantSlug = useTenantSlugSafe();
  const accountId = session?.user?.id ?? "";
  return useUnreadCount({
    enabled:
      status === "authenticated" &&
      mode === "teacher" &&
      canReviewCareWithdrawals(session),
    fetcher: async () => (await fetchCareWithdrawals({ pageSize: 1 })).total,
    cacheKey: `care_withdrawals_pending_count:${tenantSlug ?? ""}:${accountId}`,
    eventNames: ["change-requests-refresh"],
    eventDebounceMs: 500,
    refetchOnFocus: true,
    initialCount: useShellSeed()?.counts.careWithdrawalsPending,
  });
}
