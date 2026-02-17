/**
 * Tenant resolution API client.
 * Calls the backend GET /auth/tenant/resolve?slug={slug} endpoint
 * to validate and fetch tenant metadata.
 */

export interface TenantInfo {
  tenantId: number;
  slug: string;
  name: string;
  subdomain: string;
  organizationName: string;
}

interface TenantResolveResponse {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_name: string;
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
      organizationName: data.organization_name,
    };
  } catch {
    return null;
  }
}
