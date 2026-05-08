"use client";

import { useTenantSafe } from "~/components/tenant/tenant-provider";

/**
 * Hook returning whether the tenant has the student-photo feature enabled.
 *
 * Pure selector over TenantContext. The provider owns the cross-tab
 * subscription so a single settings-broadcast fans out via React context to
 * every consumer rather than triggering one /auth/tenant/resolve fetch per
 * mounted card — see TenantProvider for details. Every authenticated user
 * (including non-admin Betreuer without config:read) sees the same value
 * because the flag travels through /api/auth/tenant/resolve, not the
 * permission-gated settings schema endpoint.
 */
export function useStudentPhotosEnabled(): {
  enabled: boolean;
  isLoading: boolean;
} {
  // useTenantSafe never throws, so React's hook-order invariant stays intact
  // even when this hook is rendered outside a TenantProvider (HMR, tests).
  const ctx = useTenantSafe();
  return {
    enabled: ctx?.tenant?.studentPhotosEnabled === true,
    isLoading: ctx !== null && !ctx.tenant,
  };
}
