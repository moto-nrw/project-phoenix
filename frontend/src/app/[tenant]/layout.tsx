import { notFound } from "next/navigation";
import { TenantProvider } from "~/components/tenant/tenant-provider";
import { TenantGuard } from "~/components/tenant/tenant-guard";
import { TenantProviders } from "./providers";
import type { TenantInfo, TenantSettings } from "~/lib/tenant-api";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";

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
      { next: { revalidate: 300, tags: [`tenant-${slug}`] } },
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

  // Block reserved slugs from being resolved as tenants. Without this,
  // Next.js [tenant] dynamic segment catches paths like "/operator" and
  // renders the tenant dashboard instead of the operator dashboard.
  if (RESERVED_SLUGS.has(tenantSlug)) {
    notFound();
  }

  const tenant = await fetchTenantInfo(tenantSlug);

  if (!tenant) {
    notFound();
  }

  return (
    <TenantProviders>
      <TenantProvider tenantSlug={tenantSlug} tenant={tenant}>
        <TenantGuard>{children}</TenantGuard>
      </TenantProvider>
    </TenantProviders>
  );
}
