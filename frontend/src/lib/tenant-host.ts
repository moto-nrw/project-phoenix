/**
 * Server-side helper to extract the tenant slug from a request `Host` header.
 *
 * Mirrors the logic in `src/proxy.ts` `extractTenantSlug` but lives here so
 * route handlers can call it without dragging proxy/env wiring in. Returns
 * `null` when the host is the bare TENANT_DOMAIN, a reserved subdomain, or
 * unrelated to the tenant domain at all (e.g. path-mode dev).
 *
 * Throws when `TENANT_DOMAIN` is unset — `env.js` already requires it, and
 * silently returning null on a missing var would let a settings write
 * succeed while the cache invalidation step skips, leaving tenants on
 * stale data for the rest of the cache TTL.
 */
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";

export function tenantSlugFromHost(host: string | null): string | null {
  const tenantDomain = process.env.TENANT_DOMAIN;
  if (!tenantDomain) {
    throw new Error(
      "TENANT_DOMAIN is not set — refusing to extract tenant slug without it",
    );
  }
  if (!host) return null;
  const normalizedTenantDomain = tenantDomain.toLowerCase().replace(/\.$/, "");
  const hostname = (host.split(":")[0] ?? "").toLowerCase().replace(/\.$/, "");

  if (hostname === normalizedTenantDomain) return null;
  if (!hostname.endsWith(`.${normalizedTenantDomain}`)) return null;

  const slug = hostname.slice(0, -(normalizedTenantDomain.length + 1));
  if (!slug) return null;
  if (RESERVED_SLUGS.has(slug)) return null;
  return slug;
}

/**
 * Whether the host is the bare TENANT_DOMAIN itself. Path mode
 * (<domain>/<school>/...) lives there; the tenant layout sends it to the
 * school's subdomain (#5130).
 */
export function isBareTenantDomainHost(host: string | null): boolean {
  const tenantDomain = process.env.TENANT_DOMAIN;
  if (!tenantDomain) {
    throw new Error(
      "TENANT_DOMAIN is not set — refusing to classify the host without it",
    );
  }
  return (host?.split(":")[0] ?? "") === tenantDomain;
}
