/** One row of GET /auth/account/tenants. */
export interface AccountTenantBackend {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
}

/**
 * Pure mapper shared by the browser client (tenant-api) and the server-side
 * shell bootstrap; lives apart from tenant-api so the server layout does not
 * pull the session cache into its module graph.
 */
export function mapAccountTenant(t: AccountTenantBackend) {
  return {
    tenantId: t.tenant_id,
    slug: t.slug,
    name: t.name,
    subdomain: t.subdomain,
    organizationId: t.organization_id,
    organizationName: t.organization_name,
  };
}
