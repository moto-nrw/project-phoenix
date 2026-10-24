import { headers } from "next/headers";
import { notFound, redirect } from "next/navigation";
import type { Session } from "next-auth";
import { TenantGuard } from "~/components/tenant/tenant-guard";
import { TenantProviders } from "./providers";
import { loadShellBootstrap } from "~/lib/shell-bootstrap.server";
import type { ShellBootstrap } from "~/lib/shell-seed";
import { readTenantSessionSnapshot } from "~/lib/tenant-session-snapshot.server";
import type { TenantInfo, TenantSettings } from "~/lib/tenant-api";
import {
  bookedModuleFields,
  normalizeAttendanceEditScope,
  normalizeEarlyCheckoutTolerance,
  normalizeOverviewScope,
  normalizePresenceMode,
  normalizeRecordingSamplePercent,
} from "~/lib/tenant-api";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";
import { isValidTenantSlug } from "~/lib/tenant-slug";
import { env } from "~/env";
import { tenantResolutionRateLimitedError } from "~/sentry.shared";

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
  booked_modules?: string[];
  parent_messaging_enabled?: boolean;
  staff_messaging_enabled?: boolean;
  display_enabled?: boolean;
  caldav_enabled?: boolean;
  care_offerings_enabled?: boolean;
  attendance_web_enabled?: boolean;
  attendance_log_enabled?: boolean;
  group_mode?: string;
  operational_overview_scope?: string;
  attendance_edit_scope?: string;
  timetable_enabled?: boolean;
  show_timetable_counts?: boolean;
  meal_plan_enabled?: boolean;
  meal_registration_enabled?: boolean;
  waitlist_enabled?: boolean;
  grade_level_max: number;
  analytics_enabled?: boolean;
  analytics_freigabe?: boolean;
  analytics_recording_sample_percent?: number;
  early_checkout_note_enabled?: boolean;
  early_checkout_note_tolerance_minutes?: number;
  weekend_follows_friday?: boolean;
  child_search_only_navigation?: boolean;
}

// Next's data cache keeps only 200 responses, so a 404 for an unknown slug
// was never cached: every scanner request for a made-up subdomain reached the
// backend. Its anonymous rate limit counts all of them under this server's IP,
// so a scan burst answered real tenant lookups with 429 (FRONTEND-1B). Unknown
// slugs are remembered briefly; a school created meanwhile resolves after at
// most this TTL.
const UNKNOWN_TENANT_TTL_MS = 60_000;
const UNKNOWN_TENANT_LIMIT = 1_000;
const unknownTenantUntil = new Map<string, number>();

function isRememberedUnknownTenant(slug: string): boolean {
  const until = unknownTenantUntil.get(slug);
  if (until === undefined) return false;
  if (until > Date.now()) return true;
  unknownTenantUntil.delete(slug);
  return false;
}

function rememberUnknownTenant(slug: string): void {
  if (unknownTenantUntil.size >= UNKNOWN_TENANT_LIMIT) {
    const oldest = unknownTenantUntil.keys().next().value;
    if (oldest !== undefined) unknownTenantUntil.delete(oldest);
  }
  unknownTenantUntil.set(slug, Date.now() + UNKNOWN_TENANT_TTL_MS);
}

/**
 * Resolve tenant metadata on the server side.
 * Uses the backend API to validate the tenant slug.
 */
async function fetchTenantInfo(slug: string): Promise<TenantInfo | null> {
  if (isRememberedUnknownTenant(slug)) return null;

  const { getServerApiUrl } = await import("~/lib/server-api-url");
  const res = await fetch(
    `${getServerApiUrl()}/auth/tenant/resolve?slug=${encodeURIComponent(slug)}`,
    { next: { revalidate: 300, tags: [`tenant-${slug}`] } },
  );

  if (res.status === 404) {
    rememberUnknownTenant(slug);
    return null;
  }
  if (res.status === 429) {
    const error = new Error("Tenant resolution was rate limited (HTTP 429)");
    error.name = tenantResolutionRateLimitedError;
    throw error;
  }
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
    ...bookedModuleFields(data.booked_modules),
    messagingEnabled: data.parent_messaging_enabled === true,
    staffMessagingEnabled: data.staff_messaging_enabled === true,
    displayEnabled: data.display_enabled === true,
    caldavEnabled: data.caldav_enabled === true,
    careOfferingsEnabled: data.care_offerings_enabled !== false,
    attendanceWebEnabled: data.attendance_web_enabled === true,
    attendanceLogEnabled: data.attendance_log_enabled === true,
    groupMode: data.group_mode === "open_care" ? "open_care" : "fixed_groups",
    operationalOverviewScope: normalizeOverviewScope(
      data.operational_overview_scope,
    ),
    attendanceEditScope: normalizeAttendanceEditScope(
      data.attendance_edit_scope,
    ),
    timetableEnabled: data.timetable_enabled !== false,
    showTimetableCounts: data.show_timetable_counts !== false,
    // The meal plan pages read both from the tenant context on the first
    // render; the client resolve only refreshes them later (#3873).
    mealPlanEnabled: data.meal_plan_enabled === true,
    mealRegistrationEnabled: data.meal_registration_enabled === true,
    waitlistEnabled: data.waitlist_enabled !== false,
    gradeLevelMax: data.grade_level_max,
    analyticsEnabled: data.analytics_enabled === true,
    analyticsFreigabe: data.analytics_freigabe === true,
    analyticsRecordingSamplePercent: normalizeRecordingSamplePercent(
      data.analytics_recording_sample_percent,
    ),
    earlyCheckoutNoteToleranceMinutes: normalizeEarlyCheckoutTolerance(
      data.early_checkout_note_enabled,
      data.early_checkout_note_tolerance_minutes,
    ),
    weekendFollowsFriday: data.weekend_follows_friday === true,
    childSearchOnlyNavigation: data.child_search_only_navigation === true,
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

/**
 * Preload the app shell for a session that belongs to this tenant (#2973).
 * A mismatched tenant is TenantGuard's job (auto-switch), an errored session
 * ends in a sign-out; both fetch nothing here and keep the client path.
 */
async function loadShell(
  session: Session | null,
  tenant: TenantInfo,
): Promise<ShellBootstrap | null> {
  // Only a tenant-staff scope ("" or "org") may read these endpoints; every
  // other scope is rejected by TenantMiddleware, so asking would produce a
  // burst of guaranteed 401s.
  const scope = session?.user?.scope ?? "";
  if (
    !session?.user?.token ||
    session.error ||
    (scope !== "" && scope !== "org") ||
    session.user.tenantId !== tenant.tenantId
  ) {
    return null;
  }
  return loadShellBootstrap(session, tenant);
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

  // The dynamic segment also catches scanner probes such as
  // /wp-trackback.php. Reject values that cannot be tenant subdomains before
  // they consume the shared tenant-resolution cache or backend rate limit.
  if (!isValidTenantSlug(tenantSlug) || RESERVED_SLUGS.has(tenantSlug)) {
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

  // Read-only JWT decode: auth() may rotate a refresh token, but a Server
  // Component cannot persist the replacement cookie (#1938). The snapshot
  // safely hydrates SessionProvider; later polls and refreshes use its
  // response-aware route handler.
  const session = await readTenantSessionSnapshot();
  const shell = await loadShell(session, tenant);

  return (
    <TenantProviders
      tenantSlug={tenantSlug}
      tenant={tenant}
      routingMode={routingMode}
      session={session}
      shell={shell}
    >
      <TenantGuard>{children}</TenantGuard>
    </TenantProviders>
  );
}
