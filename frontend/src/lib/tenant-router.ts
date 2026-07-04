"use client";

// eslint-disable-next-line no-restricted-imports -- this IS the tenant-aware wrapper
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import {
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";
import { attemptNavigation } from "~/lib/hooks/navigation-guard-store";

/**
 * Tenant-aware router that handles both subdomain and path-based routing.
 *
 * - **Subdomain mode** (school-a.localhost:3000): pushes bare paths (`/dashboard`),
 *   the proxy rewrites them internally to `/school-a/dashboard`.
 * - **Path mode** (localhost:3000/school-a): prefixes paths with the tenant slug.
 *
 * Returns a memoized object so it can safely appear in useEffect dependency arrays
 * without causing infinite re-render loops.
 *
 * Example:
 *   const router = useTenantRouter();
 *   router.push("/dashboard");
 *   // subdomain mode → /dashboard  (browser URL stays clean)
 *   // path mode      → /school-a/dashboard
 */
export function useTenantRouter() {
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  const router = useRouter();

  return useMemo(() => {
    const prefix =
      !tenantSlug || routingMode === "subdomain" ? "" : `/${tenantSlug}`;
    return {
      // push/replace consult the navigation-guard registry first: if a page has
      // an active unsaved-changes guard, the navigation is deferred to its
      // confirmation instead of running immediately. With no guard registered
      // attemptNavigation returns false and we navigate as normal. back/forward
      // are already covered by the guard's popstate handling, so they stay bare.
      push: (path: string) => {
        const full = `${prefix}${path}`;
        if (!attemptNavigation(() => router.push(full), full)) {
          router.push(full);
        }
      },
      replace: (path: string) => {
        const full = `${prefix}${path}`;
        if (!attemptNavigation(() => router.replace(full), full)) {
          router.replace(full);
        }
      },
      back: () => router.back(),
      forward: () => router.forward(),
      refresh: () => router.refresh(),
      prefetch: (path: string) => router.prefetch(`${prefix}${path}`),
    };
  }, [tenantSlug, router, routingMode]);
}
