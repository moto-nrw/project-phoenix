import { notFound } from "next/navigation";
import { TenantProvider } from "~/components/tenant/tenant-provider";
import { TenantGuard } from "~/components/tenant/tenant-guard";
import type { TenantInfo, TenantSettings } from "~/lib/tenant-api";

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
 * Resolve tenant metadata on the server side.
 * Uses the backend API to validate the tenant slug.
 */
async function fetchTenantInfo(slug: string): Promise<TenantInfo | null> {
  try {
    const { getServerApiUrl } = await import("~/lib/server-api-url");
    const res = await fetch(
      `${getServerApiUrl()}/auth/tenant/resolve?slug=${encodeURIComponent(slug)}`,
      { next: { revalidate: 300 } }, // Cache for 5 minutes
    );

    if (!res.ok) return null;

    const json = (await res.json()) as {
      status: string;
      data: TenantResolveResponse;
    };
    const data = json.data;
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

/**
 * Layout for all tenant-scoped routes.
 * Validates the tenant slug via the backend and provides tenant context.
 * Returns 404 if the tenant slug is invalid.
 */
export default async function TenantLayout({
  children,
  params,
}: {
  children: React.ReactNode;
  params: Promise<{ tenant: string }>;
}) {
  const { tenant: tenantSlug } = await params;

  const tenant = await fetchTenantInfo(tenantSlug);

  if (!tenant) {
    notFound();
  }

  return (
    <TenantProvider tenantSlug={tenantSlug} tenant={tenant}>
      <TenantGuard>{children}</TenantGuard>
    </TenantProvider>
  );
}
