/**
 * Tenant resolution API client.
 * Calls the backend GET /auth/tenant/resolve?slug={slug} endpoint
 * to validate and fetch tenant metadata.
 */

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
