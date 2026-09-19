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
 * Accept a return path only when it is an internal path of one of the
 * collections that may open a detail page. Query parameters and child paths
 * are retained, so filters and a room-to-student drill-in still work.
 */
export function resolveDetailReferrer(
  candidate: string | null,
  fallback: string,
  allowedPrefixes: readonly string[],
): string {
  if (!candidate || !candidate.startsWith("/") || candidate.startsWith("//")) {
    return fallback;
  }

  try {
    // Browsers treat a backslash like a slash in a URL. Checking the resolved
    // origin prevents `/\\example.test` from becoming a protocol-relative URL.
    const url = new URL(candidate, "https://moto.invalid");
    if (url.origin !== "https://moto.invalid") {
      return fallback;
    }
    // Validate the URL-normalized path, not the raw spelling. Otherwise an
    // allowed prefix such as `/rooms` would also accept `/rooms/../settings`.
    const normalized = `${url.pathname}${url.search}${url.hash}`;
    return allowedPrefixes.some(
      (prefix) =>
        normalized === prefix ||
        normalized.startsWith(`${prefix}/`) ||
        normalized.startsWith(`${prefix}?`) ||
        normalized.startsWith(`${prefix}#`),
    )
      ? normalized
      : fallback;
  } catch {
    return fallback;
  }
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
