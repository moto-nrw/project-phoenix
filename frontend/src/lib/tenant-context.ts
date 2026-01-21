/**
 * Tenant context utilities for multi-tenant subdomain support.
 *
 * The middleware sets tenant context headers on each request:
 * - x-tenant-slug: The subdomain (e.g., "ogs-musterstadt")
 * - x-tenant-id: The BetterAuth organization ID
 * - x-tenant-name: The organization display name
 *
 * Server components and API routes can read these headers to determine
 * the current tenant context.
 */

import { headers } from "next/headers";

export interface TenantContext {
  slug: string | null;
  id: string | null;
  name: string | null;
  isMultiTenant: boolean;
}

/**
 * Get the current tenant context from request headers.
 * Call this in Server Components or API Route handlers.
 *
 * @returns TenantContext with slug, id, and name (all nullable if no tenant)
 */
export async function getTenantContext(): Promise<TenantContext> {
  const headersList = await headers();

  const slug = headersList.get("x-tenant-slug");
  const id = headersList.get("x-tenant-id");
  const name = headersList.get("x-tenant-name");

  return {
    slug,
    id,
    name,
    isMultiTenant: slug !== null,
  };
}

/**
 * Check if the request is for a specific tenant (subdomain).
 * Returns false for main domain requests.
 */
export async function isTenantRequest(): Promise<boolean> {
  const headersList = await headers();
  return headersList.get("x-tenant-slug") !== null;
}

/**
 * Get the tenant slug from request headers.
 * Returns null for main domain requests.
 */
export async function getTenantSlug(): Promise<string | null> {
  const headersList = await headers();
  return headersList.get("x-tenant-slug");
}

/**
 * Get the tenant organization ID from request headers.
 * Returns null for main domain requests.
 */
export async function getTenantId(): Promise<string | null> {
  const headersList = await headers();
  return headersList.get("x-tenant-id");
}

/**
 * Require a tenant context for the current request.
 * Throws an error if not in a tenant context.
 *
 * @throws Error if not in a tenant context
 */
export async function requireTenantContext(): Promise<
  Required<Omit<TenantContext, "isMultiTenant">> & { isMultiTenant: true }
> {
  const context = await getTenantContext();

  if (!context.isMultiTenant || !context.slug) {
    throw new Error("This page requires a tenant context (subdomain)");
  }

  return {
    slug: context.slug,
    id: context.id ?? "",
    name: context.name ?? "",
    isMultiTenant: true,
  };
}
