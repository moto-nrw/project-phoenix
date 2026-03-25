"use client";

// eslint-disable-next-line no-restricted-imports -- this IS the tenant-aware wrapper
import { useRouter } from "next/navigation";
import { useMemo } from "react";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";

/**
 * Detect whether the browser is running in subdomain-based tenant routing.
 * When a subdomain matches the tenant slug (e.g. school-a.localhost),
 * the proxy already rewrites paths internally, so the router must
 * NOT add the slug to the path — otherwise URLs become redundant:
 *   school-a.localhost:3000/school-a/dashboard  (wrong, double slug)
 *   school-a.localhost:3000/dashboard            (correct, proxy handles it)
 */
function useIsSubdomainMode(tenantSlug: string): boolean {
  // Safe for SSR: window check prevents server-side errors.
  // No hydration mismatch because the result is only used in callbacks, not rendered.
  if (typeof window === "undefined") return false;
  return window.location.hostname.startsWith(`${tenantSlug}.`);
}

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
  const router = useRouter();
  const isSubdomain = useIsSubdomainMode(tenantSlug ?? "");

  return useMemo(() => {
    const prefix = !tenantSlug || isSubdomain ? "" : `/${tenantSlug}`;
    return {
      push: (path: string) => router.push(`${prefix}${path}`),
      replace: (path: string) => router.replace(`${prefix}${path}`),
      back: () => router.back(),
      forward: () => router.forward(),
      refresh: () => router.refresh(),
      prefetch: (path: string) => router.prefetch(`${prefix}${path}`),
    };
  }, [tenantSlug, router, isSubdomain]);
}
