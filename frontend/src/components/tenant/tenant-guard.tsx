"use client";

import { useEffect, useRef } from "react";
import { signIn, useSession } from "next-auth/react";
import { mutate } from "~/lib/swr";
import { clearSessionCache } from "~/lib/session-cache";
import { switchTenant } from "~/lib/tenant-api";
import { useTenant } from "~/components/tenant/tenant-provider";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TenantGuard" });

/**
 * Client component that detects when the session's tenant differs from the
 * URL tenant and auto-switches the session to match the URL.
 *
 * Two-tab scenario:
 * 1. Tab A (school-a): session=school-a → no mismatch → renders normally
 * 2. Tab B (school-b): session=school-a, URL=school-b → mismatch → auto-switch
 * 3. Return to Tab A: SessionProvider refetches, session=school-b, URL=school-a → auto-switch back
 *
 * RLS provides defense-in-depth during any brief mismatch window.
 */
export function TenantGuard({ children }: { children: React.ReactNode }) {
  const { data: session, status, update } = useSession();
  const { tenant } = useTenant();
  const switchAttempted = useRef(false);

  const sessionTenantId = session?.user?.tenantId;
  const urlTenantId = tenant?.tenantId;
  const urlSlug = tenant?.slug;

  useEffect(() => {
    // Only check when authenticated and tenant context is resolved
    if (status !== "authenticated" || !tenant) return;

    // Skip when session has no tenantId (e.g. platform admins)
    if (sessionTenantId === undefined) return;

    // No mismatch — reset guard for future switches
    if (sessionTenantId === urlTenantId) {
      switchAttempted.current = false;
      return;
    }

    // Mismatch detected — auto-switch (but only once per mismatch)
    if (switchAttempted.current) return;
    switchAttempted.current = true;

    logger.info("tenant_mismatch_detected", {
      session_tenant_id: sessionTenantId,
      url_tenant_id: urlTenantId,
      url_slug: urlSlug,
    });

    void (async () => {
      try {
        // 1. Get new tokens for the URL tenant
        const tokens = await switchTenant(urlSlug!);

        // 2. Update NextAuth session with the new tokens
        await signIn("credentials", {
          redirect: false,
          internalRefresh: true,
          token: tokens.access_token,
          refreshToken: tokens.refresh_token,
        });

        // 3. Clear SWR cache to prevent stale cross-tenant data
        await mutate(() => true, undefined, { revalidate: false });

        // 4. Clear session cache for fresh token resolution
        clearSessionCache();

        // 5. Refetch session to trigger re-render with new tenant
        await update();

        logger.info("tenant_auto_switched", {
          from_tenant_id: sessionTenantId,
          to_slug: urlSlug,
        });
      } catch (err) {
        logger.error("tenant_auto_switch_failed", {
          error: err instanceof Error ? err.message : String(err),
          target_slug: urlSlug,
        });
        // User lacks access to this tenant — redirect to root
        window.location.href = "/";
      }
    })();
  }, [status, tenant, sessionTenantId, urlTenantId, urlSlug, update]);

  // Show loading state while session is loading
  if (status === "loading") {
    return (
      <div className="flex min-h-[200px] items-center justify-center">
        <div className="text-sm text-gray-500">Sitzung wird geladen...</div>
      </div>
    );
  }

  // Show switching state during mismatch auto-switch
  if (
    status === "authenticated" &&
    tenant &&
    sessionTenantId !== undefined &&
    sessionTenantId !== urlTenantId
  ) {
    return (
      <div className="flex min-h-[200px] items-center justify-center">
        <div className="text-sm text-gray-500">Mandant wird gewechselt...</div>
      </div>
    );
  }

  return <>{children}</>;
}
