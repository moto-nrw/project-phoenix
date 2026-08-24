"use client";

import { redirect } from "next/navigation";
import { useSession } from "next-auth/react";
import { hasEffectiveAdminScope } from "~/lib/auth-utils";
import { useTenantAwarePath } from "~/lib/tenant-path";

interface UseRequireAdminReturn {
  /** True once the session is loaded AND the user is an admin. Render gated content only when this is true. */
  readonly isReady: boolean;
  /** True while the session is still loading. Useful for showing a spinner instead of a flash of empty page. */
  readonly isLoading: boolean;
}

/**
 * Gate a client page on the effective admin scope. Non-admins are redirected
 * to /dashboard; unauthenticated users are redirected to "/" by NextAuth's
 * `required: true`.
 *
 * Usage:
 *   const { isReady } = useRequireAdmin();
 *   if (!isReady) return <Loading fullPage={false} />;
 */
export function useRequireAdmin(): UseRequireAdminReturn {
  const { data: session, status } = useSession({ required: true });
  const tenantPath = useTenantAwarePath();
  const userIsAdmin = hasEffectiveAdminScope(session);

  if (status === "authenticated" && !userIsAdmin) {
    redirect(tenantPath("/dashboard"));
  }

  return {
    isReady: status === "authenticated" && userIsAdmin,
    isLoading: status === "loading",
  };
}
