"use client";

// eslint-disable-next-line no-restricted-imports -- this IS the tenant-aware wrapper
import { useRouter } from "next/navigation";
import { useTenant } from "~/components/tenant/tenant-provider";

/**
 * Tenant-aware router that automatically prefixes paths with the tenant slug.
 * Use this instead of `useRouter()` for all in-app navigation within tenant-scoped routes.
 *
 * Example:
 *   const router = useTenantRouter();
 *   router.push("/dashboard");  // navigates to /school-a/dashboard
 */
export function useTenantRouter() {
  const { tenantSlug } = useTenant();
  const router = useRouter();

  return {
    push: (path: string) => router.push(`/${tenantSlug}${path}`),
    replace: (path: string) => router.replace(`/${tenantSlug}${path}`),
    back: () => router.back(),
    forward: () => router.forward(),
    refresh: () => router.refresh(),
    prefetch: (path: string) => router.prefetch(`/${tenantSlug}${path}`),
  };
}
