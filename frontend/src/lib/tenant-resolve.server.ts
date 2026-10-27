import { unstable_cache } from "next/cache";
import type { TenantSettings } from "~/lib/tenant-api";

export interface TenantResolveResponse {
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
export const TENANT_RATE_LIMITED = "rate-limited";

/**
 * The school a tenant subdomain belongs to, from the cached lookup the tenant
 * layout and the tenant BFF routes share (#5130). `null` means no school has
 * this subdomain; any other failure throws.
 */
export async function lookupTenant(
  slug: string,
  forwardHeaders: Record<string, string>,
): Promise<TenantResolveResponse | typeof TENANT_RATE_LIMITED | null> {
  if (isRememberedUnknownTenant(slug)) return null;

  try {
    return await resolveTenantCached(slug, forwardHeaders);
  } catch (error) {
    if (!(error instanceof TenantResolveStatusError)) throw error;
    if (error.status === 404) {
      rememberUnknownTenant(slug);
      return null;
    }
    if (error.status === 429) return TENANT_RATE_LIMITED;
    throw error;
  }
}
