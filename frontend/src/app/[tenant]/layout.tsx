import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import { TenantProvider } from "~/lib/tenant-context";
import { TenantGuard } from "~/components/tenant/tenant-guard";
import { TenantProviders } from "./providers";
import type { TenantInfo, TenantSettings } from "~/lib/tenant-api";
import { normalizePresenceMode } from "~/lib/tenant-api";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";
import { env } from "~/env";

interface TenantResolveResponse {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
  hidden?: boolean;
  settings: TenantSettings;
  presence_mode?: string;
  student_photos_enabled?: boolean;
  nfc_enabled?: boolean;
  parent_messaging_enabled?: boolean;
  display_enabled?: boolean;
  care_offerings_enabled?: boolean;
  attendance_web_enabled?: boolean;
  attendance_log_enabled?: boolean;
  group_mode?: string;
  show_timetable_counts?: boolean;
  waitlist_enabled?: boolean;
  grade_level_max: number;
}

/**
 * Resolve tenant metadata on the server side.
 * Uses the backend API to validate the tenant slug.
 */
async function fetchTenantInfo(slug: string): Promise<TenantInfo | null> {
  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const res = await fetch(
    `${getServerApiUrl()}/auth/tenant/resolve?slug=${encodeURIComponent(slug)}`,
    { next: { revalidate: 300, tags: [`tenant-${slug}`] } },
  );

  if (res.status === 404) return null;
  if (!res.ok) {
    throw new Error(`Tenant resolution failed with HTTP ${res.status}`);
  }

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
    hidden: data.hidden === true,
    settings: data.settings ?? {},
    presenceMode: normalizePresenceMode(data.presence_mode),
    studentPhotosEnabled: data.student_photos_enabled === true,
    nfcEnabled: data.nfc_enabled === true,
    messagingEnabled: data.parent_messaging_enabled === true,
    displayEnabled: data.display_enabled === true,
    careOfferingsEnabled: data.care_offerings_enabled !== false,
    attendanceWebEnabled: data.attendance_web_enabled === true,
    attendanceLogEnabled: data.attendance_log_enabled === true,
    groupMode: data.group_mode === "open_care" ? "open_care" : "fixed_groups",
    showTimetableCounts: data.show_timetable_counts !== false,
    waitlistEnabled: data.waitlist_enabled !== false,
    gradeLevelMax: data.grade_level_max,
  };
}

export function bareTenantHost(currentHost: string): string {
  const [hostname = "", port] = currentHost.split(":");
  const portSuffix = port ? `:${port}` : "";

  if (hostname === env.TENANT_DOMAIN) {
    return currentHost;
  }

  if (hostname.endsWith(`.${env.TENANT_DOMAIN}`)) {
    return `${env.TENANT_DOMAIN}${portSuffix}`;
  }

  return env.TENANT_DOMAIN;
}

function isTenantSubdomainHost(currentHost: string | null, subdomain: string) {
  if (!currentHost) return false;
  const hostname = currentHost.split(":")[0] ?? "";
  return hostname === `${subdomain}.${env.TENANT_DOMAIN}`;
}

async function redirectToTenantSelection(): Promise<never> {
  const requestHeaders = await headers();
  const currentHost =
    requestHeaders.get("x-moto-original-host") ??
    requestHeaders.get("x-forwarded-host") ??
    requestHeaders.get("host");

  if (!currentHost) {
    throw new Error("Cannot redirect invalid tenant without a Host header");
  }

  const protocol =
    requestHeaders.get("x-forwarded-proto") ??
    (env.NODE_ENV === "production" ? "https" : "http");

  redirect(`${protocol}://${bareTenantHost(currentHost)}/`);
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
    await redirectToTenantSelection();
    throw new Error("Tenant redirect did not complete");
  }
  const requestHeaders = await headers();
  const currentHost =
    requestHeaders.get("x-moto-original-host") ??
    requestHeaders.get("x-forwarded-host") ??
    requestHeaders.get("host");
  const routingMode = isTenantSubdomainHost(currentHost, tenant.subdomain)
    ? "subdomain"
    : "path";

  return (
    <TenantProviders>
      <TenantProvider
        tenantSlug={tenantSlug}
        tenant={tenant}
        routingMode={routingMode}
      >
        <TenantGuard>{children}</TenantGuard>
      </TenantProvider>
    </TenantProviders>
  );
}
