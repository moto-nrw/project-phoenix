"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { hasPermission, isAdmin } from "~/lib/auth-utils";
import { useTenantRouter } from "~/lib/tenant-router";

interface UseRequirePermissionReturn {
  /** True once the session is loaded AND the user holds the permission (or is admin). Render gated content only when this is true. */
  readonly isReady: boolean;
  /** True while the session is still loading. Useful for showing a spinner instead of a flash of empty page. */
  readonly isLoading: boolean;
}

/**
 * Gate a client page on a single tenant permission (admins always pass via the
 * `admin:*` / `*:*` wildcard). Callers lacking it are redirected to /dashboard;
 * unauthenticated users are redirected to "/" by NextAuth's `required: true`.
 * The permission mirror of useRequireAdmin — use it where a page is open to more
 * than admins (e.g. the Änderungsanfragen queue, gated on users:update and
 * scoped per child in the backend).
 *
 * Usage:
 *   const { isReady } = useRequirePermission("users:update");
 *   if (!isReady) return <Loading fullPage={false} />;
 */
export function useRequirePermission(
  permission: string,
): UseRequirePermissionReturn {
  const { data: session, status } = useSession({ required: true });
  const router = useTenantRouter();
  const allowed = isAdmin(session) || hasPermission(session, permission);

  useEffect(() => {
    if (status !== "authenticated") return;
    if (!allowed) router.replace("/dashboard");
  }, [status, allowed, router]);

  return {
    isReady: status === "authenticated" && allowed,
    isLoading: status === "loading",
  };
}
