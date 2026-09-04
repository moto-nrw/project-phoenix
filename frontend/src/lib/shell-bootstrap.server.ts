import type { Session } from "next-auth";
import {
  mapAccountTenant,
  type AccountTenantBackend,
} from "~/lib/account-tenants";
import { apiGet } from "~/lib/api-helpers.server";
import {
  hasEffectiveAdminScope,
  hasPermission,
  isAdmin,
  isCaregiver,
} from "~/lib/auth-utils";
import {
  canReviewCareWithdrawals,
  canReviewEnrollmentChangeRequests,
  resolveChangeRequestAccess,
} from "~/lib/change-request-access";
import { createLogger } from "~/lib/logger";
import { mapProfileResponse, type BackendProfile } from "~/lib/profile-helpers";
import type { UnreadAnnouncement } from "~/lib/hooks/use-announcements";
import type { RemindersResult } from "~/lib/reminders-api";
import type { SettingsSchema } from "~/lib/settings-api";
import type { ShellBootstrap, ShellCounts } from "~/lib/shell-seed";
import type {
  SchulhofStatus,
  SupervisedGroupPayload,
  SupervisionSnapshot,
} from "~/lib/supervision-derive";
import type { TenantInfo } from "~/lib/tenant-api";
import { loadUserContext } from "~/lib/user-context.server";
import type { NavigationEducationalGroup } from "~/lib/usercontext-helpers";

const logger = createLogger({ component: "ShellBootstrap" });

/**
 * Upper bound per backend call. The layout must never hang on one slow
 * counter: past the budget the field stays undefined and the browser fetches
 * it as it always did.
 */
const REQUEST_BUDGET_MS = 1500;

interface Envelope<T> {
  data: T;
}

/**
 * Resolves to the loaded value, or undefined when the request failed or ran
 * out of budget. Failures are expected (a 403 for a feature this school keeps
 * off, a 404 for an unconfigured integration) and never block the page.
 */
async function optional<T>(
  name: string,
  load: (signal: AbortSignal) => Promise<T>,
): Promise<T | undefined> {
  const controller = new AbortController();
  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<undefined>((resolve) => {
    timer = setTimeout(() => {
      logger.warn("shell_bootstrap_timeout", { field: name });
      controller.abort();
      resolve(undefined);
    }, REQUEST_BUDGET_MS);
  });
  try {
    return await Promise.race([load(controller.signal), timeout]);
  } catch (error) {
    logger.debug("shell_bootstrap_skipped", {
      field: name,
      error: error instanceof Error ? error.message : String(error),
    });
    return undefined;
  } finally {
    if (timer !== undefined) clearTimeout(timer);
  }
}

async function loadSupervision(
  session: Session,
  tenant: TenantInfo,
  token: string,
): Promise<SupervisionSnapshot | null> {
  // Same gates as SupervisionProvider: the server answers every request
  // itself, these only skip calls that are guaranteed to 403.
  const adminScope = hasEffectiveAdminScope(session);
  const canReadGroups =
    adminScope ||
    hasPermission(session, "groups:read") ||
    (!Array.isArray(session.user.permissions) && isCaregiver(session));
  const mayHaveOverview =
    adminScope || tenant.operationalOverviewScope === "all_staff";

  let overviewOk = false;
  const loadSupervised = async (
    signal: AbortSignal,
  ): Promise<SupervisedGroupPayload[]> => {
    if (canReadGroups && mayHaveOverview) {
      try {
        const overview = await apiGet<
          Envelope<SupervisedGroupPayload[] | null>
        >("/api/active/supervisors/all", token, { signal });
        overviewOk = true;
        return overview.data ?? [];
      } catch (error) {
        if (signal.aborted) throw error;
        // 403 = this school keeps the caller on their own supervisions.
      }
    }
    const own = await apiGet<Envelope<SupervisedGroupPayload[] | null>>(
      "/api/me/groups/supervised",
      token,
      { signal },
    );
    return own.data ?? [];
  };

  const [groups, supervised, schulhof] = await Promise.all([
    optional("groups", async (signal) => {
      const response = await apiGet<Envelope<NavigationEducationalGroup[]>>(
        "/api/students/ogs-group-navigation",
        token,
        { signal },
      );
      return response.data;
    }),
    optional("supervised", loadSupervised),
    canReadGroups
      ? optional("schulhof", async (signal) => {
          const response = await apiGet<Envelope<SchulhofStatus>>(
            "/api/active/schulhof/status",
            token,
            { signal },
          );
          return response.data;
        })
      : Promise.resolve(undefined),
  ]);

  // SupervisionProvider treats any snapshot as a complete initial load. Do
  // not seed partial data: its ordinary browser request is the recovery path.
  if (
    groups === undefined ||
    supervised === undefined ||
    (canReadGroups && schulhof === undefined)
  ) {
    return null;
  }

  return {
    groups,
    supervised,
    schulhof: schulhof ?? null,
    overviewOk,
  };
}

async function loadCount(
  name: keyof ShellCounts,
  enabled: boolean,
  load: (signal: AbortSignal) => Promise<number>,
): Promise<number | undefined> {
  return enabled ? optional(name, load) : undefined;
}

/**
 * The badge counters, gated exactly like their hooks (use-*-pending.ts,
 * use-*-unread.ts). A gate that disagrees with its hook is harmless in both
 * directions: the hook ignores a seed it would not have fetched, and fetches
 * whatever the server left out.
 */
async function loadCounts(
  session: Session,
  tenant: TenantInfo,
  token: string,
): Promise<ShellCounts> {
  const canReviewAbsences =
    isAdmin(session) || hasPermission(session, "vacation:approve");
  // Non-admins need the server-side review-access check first
  // (useChangeRequestAccess); only the admin path is known up front.
  const canReviewParentRequests =
    hasEffectiveAdminScope(session) &&
    resolveChangeRequestAccess(session, "admin").canReviewParentRequests;

  const [
    staffAbsencesPending,
    messagesUnread,
    teamChatUnread,
    staffNoticesPending,
    changeRequestsPending,
    enrollmentRequestsPending,
    careWithdrawalsPending,
  ] = await Promise.all([
    loadCount("staffAbsencesPending", canReviewAbsences, async (signal) => {
      const response = await apiGet<Envelope<unknown[] | null>>(
        "/api/staff/absences/pending",
        token,
        { signal },
      );
      return (response.data ?? []).length;
    }),
    loadCount("messagesUnread", tenant.messagingEnabled, async (signal) => {
      const response = await apiGet<Envelope<{ unread_count?: number }>>(
        "/api/messages/unread-count",
        token,
        { signal },
      );
      return response.data?.unread_count ?? 0;
    }),
    loadCount(
      "teamChatUnread",
      tenant.staffMessagingEnabled,
      async (signal) => {
        const response = await apiGet<Envelope<{ unread_count?: number }>>(
          "/api/staff-messages/unread-count",
          token,
          { signal },
        );
        return response.data?.unread_count ?? 0;
      },
    ),
    loadCount(
      "staffNoticesPending",
      hasPermission(session, "users:read"),
      async (signal) => {
        const response = await apiGet<
          Envelope<Array<{
            requires_acknowledgement: boolean;
            acknowledged_at?: string;
          }> | null>
        >("/api/staff-notices/today", token, { signal });
        return (response.data ?? []).filter(
          (n) => n.requires_acknowledgement && !n.acknowledged_at,
        ).length;
      },
    ),
    loadCount(
      "changeRequestsPending",
      canReviewParentRequests,
      async (signal) => {
        const response = await apiGet<Envelope<{ pending_count?: number }>>(
          "/api/students/change-requests/pending-count",
          token,
          { signal },
        );
        return response.data?.pending_count ?? 0;
      },
    ),
    loadCount(
      "enrollmentRequestsPending",
      canReviewEnrollmentChangeRequests(session),
      async (signal) => {
        const response = await apiGet<Envelope<{ pending_count?: number }>>(
          "/api/enrollment/admin/change-requests/pending-count",
          token,
          { signal },
        );
        return response.data?.pending_count ?? 0;
      },
    ),
    loadCount(
      "careWithdrawalsPending",
      canReviewCareWithdrawals(session),
      async (signal) => {
        const response = await apiGet<Envelope<{ total?: number }>>(
          "/api/students/care-withdrawals?page_size=1",
          token,
          { signal },
        );
        return response.data?.total ?? 0;
      },
    ),
  ]);

  return {
    staffAbsencesPending,
    messagesUnread,
    teamChatUnread,
    staffNoticesPending,
    changeRequestsPending,
    enrollmentRequestsPending,
    careWithdrawalsPending,
  };
}

/**
 * Loads, in parallel, everything the tenant app shell fetched one request at
 * a time after hydration (#2973): navigation context, settings schema,
 * profile, group and supervision navigation, the account's tenants, and the
 * sidebar badge counts. Runs once per full page load in the tenant layout;
 * client-side navigations reuse the hydrated caches.
 */
export async function loadShellBootstrap(
  session: Session,
  tenant: TenantInfo,
): Promise<ShellBootstrap> {
  const token = session.user.token ?? "";
  const canReadConfig =
    isAdmin(session) || hasPermission(session, "config:read");

  const [
    userContext,
    settingsSchema,
    profile,
    accountTenants,
    reminders,
    announcements,
    supervision,
    counts,
  ] = await Promise.all([
    optional("userContext", async (signal) => {
      const context = await loadUserContext(token, { signal });
      // An incomplete projection is an error state for useUserContext (it
      // retries); leave that path to the browser instead of caching it.
      return context.incomplete ? undefined : context;
    }),
    canReadConfig
      ? optional("settingsSchema", async (signal) => {
          const response = await apiGet<Envelope<SettingsSchema>>(
            "/api/settings/schema",
            token,
            { signal },
          );
          return response.data;
        })
      : Promise.resolve(undefined),
    optional("profile", async (signal) => {
      const response = await apiGet<Envelope<BackendProfile>>(
        "/api/me/profile",
        token,
        { signal },
      );
      return mapProfileResponse(response.data);
    }),
    optional("accountTenants", async (signal) => {
      const response = await apiGet<Envelope<AccountTenantBackend[] | null>>(
        "/auth/account/tenants",
        token,
        { signal },
      );
      return (response.data ?? []).map(mapAccountTenant);
    }),
    optional("reminders", async (signal) => {
      const response = await apiGet<Envelope<RemindersResult>>(
        "/api/reminders",
        token,
        { signal },
      );
      return response.data;
    }),
    optional("announcements", async (signal) => {
      const response = await apiGet<Envelope<UnreadAnnouncement[] | null>>(
        "/api/platform/announcements/unread",
        token,
        { signal },
      );
      return response.data ?? [];
    }),
    loadSupervision(session, tenant, token),
    loadCounts(session, tenant, token),
  ]);

  return {
    accountId: session.user.id,
    userContext,
    settingsSchema,
    profile,
    accountTenants,
    reminders,
    announcements,
    supervision,
    counts,
  };
}
