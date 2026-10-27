import { headers } from "next/headers";
import { unstable_cache } from "next/cache";
import { notFound, redirect } from "next/navigation";
import type { Session } from "next-auth";
import { TenantGuard } from "~/components/tenant/tenant-guard";
import { TenantProviders } from "./providers";
import { loadShellBootstrap } from "~/lib/shell-bootstrap.server";
import type { ShellBootstrap } from "~/lib/shell-seed";
import {
  readTenantSessionSnapshot,
  type TenantSessionSnapshot,
} from "~/lib/tenant-session-snapshot.server";
import type { TenantInfo, TenantSettings } from "~/lib/tenant-api";
import {
  bookedModuleFields,
  normalizeAttendanceEditScope,
  normalizeReasonPolicy,
  normalizeEarlyCheckoutTolerance,
  normalizeOverviewScope,
  normalizePresenceMode,
  normalizeRecordingSamplePercent,
} from "~/lib/tenant-api";
import { RESERVED_SLUGS } from "~/lib/reserved-slugs";
import { isValidTenantSlug } from "~/lib/tenant-slug";
import { env } from "~/env";
import { TenantBusyPage } from "~/components/tenant/tenant-busy-page";
import { incomingClientForwardHeaders } from "~/lib/client-headers.server";

interface TenantResolveResponse {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
  traeger_name?: string;
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
  parent_request_reason_policy?: string;
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
// backend. Its anonymous rate limit counted all of them under this server's
// IP, so a scan burst answered real tenant lookups with 429 (FRONTEND-1B); the
// lookup now forwards the visitor's IP as well. Unknown slugs are remembered
// briefly; a school created meanwhile resolves after at most this TTL.
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

/** A non-200 answer of the tenant lookup; never cached. */
class TenantResolveStatusError extends Error {
  constructor(readonly status: number) {
    super(`Tenant resolution failed with HTTP ${status}`);
  }
}

/**
 * The lookup is cached per slug for all visitors and invalidated by
 * revalidateTag(`tenant-${slug}`). Its key holds the slug only, so the
 * client headers travel in the closure, not in the key: a miss asks the
 * backend under the IP of the visitor who caused it, a hit asks nobody.
 * fetch() itself cannot do this, because Next keys its data cache by the
 * request headers, which would split the cache per client IP. Only a 200 is
 * stored; any other status throws and stays uncached.
 */
function resolveTenantCached(
  slug: string,
  forwardHeaders: Record<string, string>,
): Promise<TenantResolveResponse> {
  return unstable_cache(
    async () => {
      const { getServerApiUrl } = await import("~/lib/server-api-url");
      const res = await fetch(
        `${getServerApiUrl()}/auth/tenant/resolve?slug=${encodeURIComponent(slug)}`,
        { headers: forwardHeaders, cache: "no-store" },
      );
      if (!res.ok) throw new TenantResolveStatusError(res.status);
      const json = (await res.json()) as { data: TenantResolveResponse };
      return json.data;
    },
    ["tenant-resolve", slug],
    { revalidate: 300, tags: [`tenant-${slug}`] },
  )();
}

/**
 * The backend rate limit rejected the lookup (HTTP 429). Scanner bursts cause
 * it, not a defect, and the backend counts every rejection in its rate-limit
 * metric. The layout shows a waiting page instead of throwing (#5090): an
 * error boundary would show a generic crash, report it to Sentry and retry
 * only on the client, without running this layout again.
 */
const TENANT_RATE_LIMITED = "rate-limited";

/**
 * Resolve tenant metadata on the server side.
 * Uses the backend API to validate the tenant slug.
 */
async function fetchTenantInfo(
  slug: string,
): Promise<TenantInfo | typeof TENANT_RATE_LIMITED | null> {
  if (isRememberedUnknownTenant(slug)) return null;

  let data: TenantResolveResponse;
  try {
    data = await resolveTenantCached(
      slug,
      await incomingClientForwardHeaders(),
    );
  } catch (error) {
    if (!(error instanceof TenantResolveStatusError)) throw error;
    if (error.status === 404) {
      rememberUnknownTenant(slug);
      return null;
    }
    if (error.status === 429) return TENANT_RATE_LIMITED;
    throw error;
  }

  return {
    tenantId: data.tenant_id,
    slug: data.slug,
    name: data.name,
    subdomain: data.subdomain,
    organizationId: data.organization_id,
    organizationName: data.organization_name,
    traegerName: data.traeger_name,
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
    // „Anfragen“ reads it on the first render; without it the strict
    // fallback asked for a reason on every approval (#5024).
    parentRequestReasonPolicy: normalizeReasonPolicy(
      data.parent_request_reason_policy,
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
  snapshot: TenantSessionSnapshot | null,
  tenant: TenantInfo,
): Promise<ShellBootstrap | null> {
  // Only a tenant-staff scope ("" or "org") may read these endpoints; every
  // other scope is rejected by TenantMiddleware, so asking would produce a
  // burst of guaranteed 401s.
  const session: Session | undefined = snapshot?.session;
  const scope = session?.user?.scope ?? "";
  if (
    !snapshot?.accessToken ||
    !session ||
    session.error ||
    (scope !== "" && scope !== "org") ||
    session.user.tenantId !== tenant.tenantId
  ) {
    return null;
  }
  return loadShellBootstrap(session, tenant, snapshot.accessToken);
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

  if (tenant === TENANT_RATE_LIMITED) {
    return <TenantBusyPage />;
  }
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
  // safely hydrates SessionProvider without any token (#5005); later polls
  // and refreshes use its response-aware route handler.
  const snapshot = await readTenantSessionSnapshot();
  const session = snapshot?.session ?? null;
  const shell = await loadShell(snapshot, tenant);

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
