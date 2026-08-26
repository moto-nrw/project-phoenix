/**
 * Tenant resolution and switching API client.
 * Provides public tenant resolution (pre-login) and authenticated
 * tenant listing/switching (post-login).
 */

import { clearSessionCache, sessionFetch } from "./session-cache";

const TENANT_ACCESS_DENIED_MESSAGE =
  "account does not have access to this tenant";

export interface TenantSettings {
  logoUrl?: string;
  loginImageUrl?: string;
  primaryColor?: string;
  [key: string]: unknown;
}

/** Convert a stored login image URL path to the public-serving proxy URL. */
export function loginImageSrc(storedPath: string): string {
  const filename = storedPath.split("/").pop() ?? "";
  return `/api/public/login-image/${filename}`;
}

/**
 * Tenant presence mode. "detailed" tracks rooms/activities/visits;
 * "binary" tracks only in-school/out-of-school on `active.attendance`.
 * Drives which badge component renders (LocationBadge vs PresenceBadge)
 * and which nav items appear. Defaults to "detailed" so older tenants
 * without the setting keep their current UX.
 */
export type PresenceMode = "detailed" | "binary";

export interface TenantInfo {
  tenantId: number;
  slug: string;
  name: string;
  subdomain: string;
  organizationId: number;
  organizationName: string;
  hidden?: boolean;
  settings: TenantSettings;
  presenceMode: PresenceMode;
  /**
   * Whether the tenant has the photo feature enabled. Surfaced via the
   * tenant resolve endpoint (which any authenticated session reaches)
   * rather than /api/settings/schema, because non-admin Betreuer don't
   * carry config:read but still need to know whether to render avatars.
   */
  studentPhotosEnabled: boolean;
  /**
   * Whether this tenant uses NFC devices for attendance/location capture.
   * Exposed through tenant resolve so non-admin staff can hide NFC-only
   * navigation without needing config:read.
   */
  nfcEnabled: boolean;
  /**
   * Whether parent-OGS messaging is enabled for this tenant
   * (operations.parent_notes_enabled). Surfaced via tenant resolve so non-admin
   * staff can hide the "Neue Nachricht" compose entry points when messaging is
   * off, instead of composing into a backend 403.
   */
  messagingEnabled: boolean;
  /**
   * Whether the OGS-internal Team-Chat is switched on for this school
   * (operations.staff_messaging_enabled, #2598). Defaults to FALSE: an
   * internal staff channel must never appear at a school that did not ask for
   * it, so a missing flag hides the whole area rather than showing it.
   */
  staffMessagingEnabled: boolean;
  /**
   * Whether the Info-Point Dashboard (display.enabled) is enabled for this
   * tenant. The feature is opt-in and defaults off, so the sidebar entry and
   * admin page must stay hidden until a school explicitly enables it.
   */
  displayEnabled: boolean;
  /**
   * Whether staff may correct care offerings on approved enrollments. Missing
   * metadata is treated as enabled for compatibility with older backends.
   */
  careOfferingsEnabled?: boolean;
  attendanceWebEnabled?: boolean;
  /**
   * Whether the attendance log / Tagesauswertung (gdpr.attendance_log_enabled)
   * is enabled. Opt-in and default off; the sidebar entry stays hidden until a
   * school enables it.
   */
  attendanceLogEnabled?: boolean;
  groupMode?: "fixed_groups" | "open_care";
  /**
   * Who at this school may see and operate every running module
   * (operations.operational_overview_scope, #2380). A hint the client uses to
   * decide which supervision endpoint to ask for — the server decides every
   * request on its own. Unknown values collapse to "own".
   */
  operationalOverviewScope?: OperationalOverviewScope;
  showTimetableCounts?: boolean;
  waitlistEnabled?: boolean;
  /** Highest grade offered by this tenant (enrollment.grade_level_max). */
  gradeLevelMax: number;
}

/** Identity-only tenant row returned by list/switch endpoints. Feature and
 * settings metadata is deliberately absent until resolveTenant is called. */
export type TenantSummary = Pick<
  TenantInfo,
  | "tenantId"
  | "slug"
  | "name"
  | "subdomain"
  | "organizationId"
  | "organizationName"
  | "hidden"
>;

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
  staff_messaging_enabled?: boolean;
  display_enabled?: boolean;
  care_offerings_enabled?: boolean;
  attendance_web_enabled?: boolean;
  attendance_log_enabled?: boolean;
  group_mode?: string;
  operational_overview_scope?: string;
  show_timetable_counts?: boolean;
  waitlist_enabled?: boolean;
  grade_level_max: number;
}

/** Scope values of operations.operational_overview_scope (#2380). */
export type OperationalOverviewScope = "own" | "admins" | "all_staff";

/**
 * Normalize the backend's operational_overview_scope string. Anything the
 * client does not recognise collapses to the restrictive "own" — an older
 * backend that omits the field must never widen the UI.
 */
export function normalizeOverviewScope(raw: unknown): OperationalOverviewScope {
  return raw === "admins" || raw === "all_staff" ? raw : "own";
}

/**
 * Normalize the backend's presence_mode string into our union type.
 * Anything other than "binary" collapses to "detailed" — the safe default
 * used by the backend itself when the setting is missing or errors.
 */
export function normalizePresenceMode(raw: unknown): PresenceMode {
  return raw === "binary" ? "binary" : "detailed";
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

    const json = (await response.json()) as {
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
      staffMessagingEnabled: data.staff_messaging_enabled === true,
      displayEnabled: data.display_enabled === true,
      careOfferingsEnabled: data.care_offerings_enabled !== false,
      attendanceWebEnabled: data.attendance_web_enabled === true,
      attendanceLogEnabled: data.attendance_log_enabled === true,
      groupMode: data.group_mode === "open_care" ? "open_care" : "fixed_groups",
      operationalOverviewScope: normalizeOverviewScope(
        data.operational_overview_scope,
      ),
      showTimetableCounts: data.show_timetable_counts !== false,
      waitlistEnabled: data.waitlist_enabled !== false,
      gradeLevelMax: data.grade_level_max,
    };
  } catch {
    return null;
  }
}

/** Public tenant response (no internal IDs) */
interface PublicTenantBackend {
  slug: string;
  name: string;
  subdomain: string;
  organization_name: string;
}

export interface TenantListResult {
  tenants: TenantSummary[];
  /** "ok" = backend responded successfully, "error" = network or server failure */
  status: "ok" | "error";
}

/**
 * List all active tenants with retry logic.
 * This is a public (no-auth) call used on the root tenant selector page.
 * The backend omits internal IDs from this public endpoint.
 *
 * Retries up to {@link maxAttempts} times with exponential backoff to
 * handle transient failures (e.g. after registration redirect).
 */
export async function listAllTenants(
  maxAttempts = 3,
): Promise<TenantListResult> {
  let lastError: unknown;

  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const response = await fetch("/api/tenant/list");
      if (!response.ok) {
        lastError = new Error(`HTTP ${response.status}`);
        if (attempt < maxAttempts) {
          await delay(1000 * 2 ** (attempt - 1));
          continue;
        }
        return { tenants: [], status: "error" };
      }

      const json = (await response.json()) as {
        data?: PublicTenantBackend[];
      };
      const items = json.data ?? [];
      return {
        tenants: items.map((t) => ({
          tenantId: 0,
          slug: t.slug,
          name: t.name,
          subdomain: t.subdomain,
          organizationId: 0,
          organizationName: t.organization_name,
          hidden: false,
        })),
        status: "ok",
      };
    } catch (error) {
      lastError = error;
      if (attempt < maxAttempts) {
        await delay(1000 * 2 ** (attempt - 1));
        continue;
      }
    }
  }

  // All attempts exhausted
  void lastError; // acknowledged but not rethrown — callers check status
  return { tenants: [], status: "error" };
}

function delay(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** Backend response shape for account tenants (snake_case) */
interface AccountTenantBackend {
  tenant_id: number;
  slug: string;
  name: string;
  subdomain: string;
  organization_id: number;
  organization_name: string;
}

/**
 * List tenants the current user has access to.
 * Requires an authenticated session.
 */
export async function listAvailableTenants(): Promise<TenantSummary[]> {
  const response = await sessionFetch("/api/auth/account-tenants");
  if (!response.ok) {
    return [];
  }
  const json = (await response.json()) as { data?: AccountTenantBackend[] };
  const items = json.data ?? [];
  return items.map((t) => ({
    tenantId: t.tenant_id,
    slug: t.slug,
    name: t.name,
    subdomain: t.subdomain,
    organizationId: t.organization_id,
    organizationName: t.organization_name,
  }));
}

/** Response shape from the switch-tenant endpoint */
interface SwitchTenantResponse {
  access_token: string;
  refresh_token: string;
}

interface ErrorResponseBody {
  error?: string;
  message?: string;
  code?: string;
}

type TenantSwitchErrorCode = "access_denied" | "use_school_portal" | "unknown";

export class TenantSwitchError extends Error {
  status: number;
  code: TenantSwitchErrorCode;

  constructor(
    message: string,
    status: number,
    code: TenantSwitchErrorCode = "unknown",
  ) {
    super(message);
    this.name = "TenantSwitchError";
    this.status = status;
    this.code = code;
  }
}

async function parseErrorResponse(
  response: Response,
): Promise<{ message: string; code?: string }> {
  const text = await response.text();

  if (!text) {
    return { message: "Failed to switch tenant" };
  }

  try {
    const payload = JSON.parse(text) as ErrorResponseBody;
    return {
      message: payload.error ?? payload.message ?? text,
      code: payload.code,
    };
  } catch {
    return { message: text };
  }
}

/**
 * Perform the full tenant switch sequence (steps 1-4 of the switch spec).
 * Returns the new tokens. Callers handle the final navigation/update (step 5).
 *
 * 1. Get new JWT tokens from backend
 * 2. Update NextAuth session via signIn("credentials", { internalRefresh })
 * 3. Clear SWR cache to prevent stale cross-tenant data
 * 4. Clear session cache for fresh token resolution
 */
export async function performTenantSwitch(
  subdomain: string,
  signIn: (
    provider: string,
    options: Record<string, unknown>,
  ) => Promise<unknown>,
  swrMutate: (
    matcher: (key: unknown) => boolean,
    data: undefined,
    opts: { revalidate: boolean },
  ) => Promise<unknown>,
): Promise<SwitchTenantResponse> {
  const tokens = await switchTenant(subdomain);

  await signIn("credentials", {
    redirect: false,
    internalRefresh: true,
    token: tokens.access_token,
    refreshToken: tokens.refresh_token,
  });

  await swrMutate(() => true, undefined, { revalidate: false });

  clearSessionCache();

  return tokens;
}

/**
 * Switch the current session to a different tenant.
 * Returns new JWT tokens scoped to the target tenant.
 *
 * Takes the tenant SUBDOMAIN, not the slug column: the backend resolves the
 * wire field `tenant_slug` via FindBySubdomain (same as login), so passing
 * the slug 404s for schools where slug != subdomain (#1975).
 */
export async function switchTenant(
  subdomain: string,
): Promise<SwitchTenantResponse> {
  const response = await sessionFetch("/api/auth/switch-tenant", {
    method: "POST",
    body: JSON.stringify({ tenant_slug: subdomain }),
  });
  if (!response.ok) {
    const { message, code: responseCode } = await parseErrorResponse(response);
    const code =
      response.status === 401 && message === TENANT_ACCESS_DENIED_MESSAGE
        ? "access_denied"
        : response.status === 403 && responseCode === "use_school_portal"
          ? "use_school_portal"
          : "unknown";
    throw new TenantSwitchError(message, response.status, code);
  }
  return (await response.json()) as SwitchTenantResponse;
}
