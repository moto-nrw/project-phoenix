"use client";

import { useTenantSafe } from "~/lib/tenant-context";

// Pure selector over TenantContext — provider owns the cross-tab subscription.
// useTenantSafe never throws so the hook-order invariant survives HMR/tests.
export function useStudentPhotosEnabled(): {
  enabled: boolean;
  isLoading: boolean;
} {
  const ctx = useTenantSafe();
  return {
    enabled: ctx?.tenant?.studentPhotosEnabled === true,
    isLoading: ctx !== null && !ctx.tenant,
  };
}
