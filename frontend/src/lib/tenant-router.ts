"use client";

// eslint-disable-next-line no-restricted-imports -- this IS the tenant-aware wrapper
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import {
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

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
      push: (path: string) => router.push(`${prefix}${path}`),
      replace: (path: string) => router.replace(`${prefix}${path}`),
      back: () => router.back(),
      forward: () => router.forward(),
      refresh: () => router.refresh(),
      prefetch: (path: string) => router.prefetch(`${prefix}${path}`),
    };
  }, [tenantSlug, router, routingMode]);
}
