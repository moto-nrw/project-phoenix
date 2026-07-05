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

export function useTenantAwarePath() {
  const tenantSlug = useTenantSlugSafe();
  const routingMode = useTenantRoutingModeSafe();
  return useCallback(
    (path: string) => tenantAwarePath(path, tenantSlug, routingMode),
    [routingMode, tenantSlug],
  );
}
