/**
 * Tenant resolution and switching API client.
 * Provides public tenant resolution (pre-login) and authenticated
 * tenant listing/switching (post-login).
 */

import { sessionFetch } from "./session-cache";

export interface TenantSettings {
  logoUrl?: string;
  primaryColor?: string;
  [key: string]: unknown;
}

export interface TenantInfo {
  tenantId: number;
  slug: string;
  name: string;
  subdomain: string;
  organizationId: number;
  organizationName: string;
  settings: TenantSettings;
}

interface TenantResolveResponse {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
  settings: TenantSettings;
}

/**
 * Resolve a tenant slug to its metadata.
 * This is a public (no-auth) call used before login.
 */
export async function resolveTenant(slug: string): Promise<TenantInfo | null> {
  try {
    const response = await fetch(
      `/api/tenant/resolve?slug=${encodeURIComponent(slug)}`,
    );

    if (!response.ok) {
      return null;
    }

    const data = (await response.json()) as TenantResolveResponse;
    return {
      tenantId: data.tenant_id,
      slug: data.slug,
      name: data.name,
      subdomain: data.subdomain,
      organizationId: data.organization_id,
      organizationName: data.organization_name,
      settings: data.settings ?? {},
    };
  } catch {
    return null;
  }
}

/** Backend response shape for account tenants (snake_case) */
interface AccountTenantBackend {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
}

/**
 * List tenants the current user has access to.
 * Requires an authenticated session.
 */
export async function listAvailableTenants(): Promise<TenantInfo[]> {
  const response = await sessionFetch("/api/auth/account-tenants");
  if (!response.ok) {
    return [];
  }
  const json = (await response.json()) as { data?: AccountTenantBackend[] };
  const items = json.data ?? [];
  return items.map((t) => ({
    tenantId: t.tenant_id,
    slug: t.slug,
    name: t.name,
    subdomain: t.subdomain,
    organizationId: t.organization_id,
    organizationName: t.organization_name,
    settings: {},
  }));
}

/** Response shape from the switch-tenant endpoint */
interface SwitchTenantResponse {
  access_token: string;
  refresh_token: string;
}

/**
 * Switch the current session to a different tenant.
 * Returns new JWT tokens scoped to the target tenant.
 */
export async function switchTenant(
  slug: string,
): Promise<SwitchTenantResponse> {
  const response = await sessionFetch("/api/auth/switch-tenant", {
    method: "POST",
    body: JSON.stringify({ tenant_slug: slug }),
  });
  if (!response.ok) {
    const text = await response.text();
    throw new Error(text || "Failed to switch tenant");
  }
  return (await response.json()) as SwitchTenantResponse;
}
