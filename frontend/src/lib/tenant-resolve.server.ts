import "server-only";
import { unstable_cache } from "next/cache";
import type { TenantSettings } from "~/lib/tenant-api";

/** The backend's answer to `/auth/tenant/resolve?slug=…` (snake_case). */
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

/** A non-200 answer of the tenant lookup; never cached. */
export class TenantResolveStatusError extends Error {
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
export function resolveTenantCached(
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
