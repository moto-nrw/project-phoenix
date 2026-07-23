"use client";

import { useCallback } from "react";
import {
  type TenantRoutingMode,
  useTenantRoutingModeSafe,
  useTenantSlugSafe,
} from "~/lib/tenant-context";

export function tenantAwarePath(
  path: string,
  tenantSlug: string | null | undefined,
  routingMode: TenantRoutingMode = "path",
): string {
  const normalizedPath = path.startsWith("/") ? path : `/${path}`;
  if (!tenantSlug) return normalizedPath;
  if (routingMode === "subdomain") return normalizedPath;

  return `/${tenantSlug}${normalizedPath}`;
}

/**
 * Removes the external tenant segment from a pathname only in path-routing
 * mode. In subdomain mode the slug may equal a real route segment, so stripping
 * it there would corrupt active-link checks.
 */
export function normalizeTenantPathname(
  pathname: string,
  tenantSlug: string | null | undefined,
  routingMode: TenantRoutingMode,
): string {
  if (routingMode !== "path" || !tenantSlug) return pathname;

  const prefix = `/${tenantSlug}`;
  if (pathname === prefix) return "/";
  if (pathname.startsWith(`${prefix}/`)) {
    return pathname.slice(prefix.length);
  }
  return pathname;
}

export function useTenantAwarePath() {
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  return useCallback(
    (path: string) => tenantAwarePath(path, tenantSlug, routingMode),
    [routingMode, tenantSlug],
  );
}
