"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { isAdmin } from "~/lib/auth-utils";
import { useTenantRouter } from "~/lib/tenant-router";

interface UseRequireAdminReturn {
  /** True once the session is loaded AND the user is an admin. Render gated content only when this is true. */
  readonly isReady: boolean;
  /** True while the session is still loading. Useful for showing a spinner instead of a flash of empty page. */
  readonly isLoading: boolean;
}

/**
 * Gate a client page on admin role. Non-admins are redirected to /dashboard;
 * unauthenticated users are redirected to "/" by NextAuth's `required: true`.
 *
 * Usage:
 *   const { isReady } = useRequireAdmin();
 *   if (!isReady) return <Loading fullPage={false} />;
 */
export function useRequireAdmin(): UseRequireAdminReturn {
  const { data: session, status } = useSession({ required: true });
  const router = useTenantRouter();
  const userIsAdmin = isAdmin(session);

  useEffect(() => {
    if (status !== "authenticated") return;
    if (!userIsAdmin) router.replace("/dashboard");
  }, [status, userIsAdmin, router]);

  return {
    isReady: status === "authenticated" && userIsAdmin,
    isLoading: status === "loading",
  };
}
