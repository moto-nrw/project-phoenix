"use client";

import { useLayoutEffect, useMemo } from "react";
import type { Session } from "next-auth";
import { SessionProvider, useSession } from "next-auth/react";
// eslint-disable-next-line no-restricted-imports -- SWRConfig only; hooks stay on ~/lib/swr
import { SWRConfig } from "swr";
import { ProfileProvider } from "~/lib/profile-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";
import { ModuleBookingAudienceProvider } from "~/contexts/ToastContext";
import { TenantAuthWrapper } from "~/components/auth/tenant-auth-wrapper";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { ANNOUNCEMENTS_UNREAD_SWR_KEY } from "~/lib/hooks/use-announcements";
import { USER_CONTEXT_SWR_KEY } from "~/lib/hooks/use-user-context";
import { REMINDERS_SWR_KEY } from "~/lib/reminders-api";
import { primeSessionCache } from "~/lib/session-cache";
import { SESSION_PROVIDER_POLL_SECONDS } from "~/lib/session-provider-poll";
import { ShellSeedProvider, type ShellBootstrap } from "~/lib/shell-seed";
import { mutate } from "~/lib/swr";
import { staffNoticesTodayKey } from "~/lib/staff-notices-api";
import { TenantProvider, type TenantRoutingMode } from "~/lib/tenant-context";
import { tenantAwarePath } from "~/lib/tenant-path";
import type { TenantInfo } from "~/lib/tenant-api";
import { seesLockedModules } from "~/lib/module-register";

/**
 * Mirrors SessionProvider's response into the getSession() dedupe cache that
 * every API client reads (#2973). A layout effect in a leaf rendered BEFORE
 * the provider tree commits before any SWR hook further down starts its mount
 * revalidation; the hydrated session is therefore reused instead of fetched.
 * Later session changes (poll, update()) prime it again; between them the
 * entry holds, except where a flow that rotates tokens calls
 * clearSessionCache() or the BFF refuses another school's session (#5130).
 * An answer without access (logout here or in another tab) drops the entry.
 */
export function SessionCachePrimer() {
  const { data: session } = useSession();
  useLayoutEffect(() => {
    // undefined: the provider has no answer yet. null or an answer without
    // access: the sign-in is gone, and primeSessionCache drops the entry.
    if (session !== undefined) primeSessionCache(session);
  }, [session]);
  return null;
}

function ModuleBookingAudienceBridge({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const { data: session } = useSession();
  return (
    <ModuleBookingAudienceProvider
      seesLockedModules={seesLockedModules(session)}
    >
      {children}
    </ModuleBookingAudienceProvider>
  );
}

/**
 * Shell entries whose readers never revalidate on mount (#4989): the header
 * bell polls the reminders, the sidebar badge reloads today's staff notices
 * on focus and after an acknowledgement, and the home block only shows them.
 * The user context is immutable SWR; settings changes, SSE events and the
 * guardian-approval retry invalidate it.
 */
export function shellOwnedEntries(shell: ShellBootstrap, tenantSlug: string) {
  const entries: Record<string, unknown> = {};
  if (shell.userContext) {
    entries[`${tenantSlug}:${USER_CONTEXT_SWR_KEY}`] = shell.userContext;
  }
  if (shell.reminders) {
    entries[`${tenantSlug}:${REMINDERS_SWR_KEY}`] = shell.reminders;
  }
  // Keyed by the account the snapshot was built for, like every reader.
  if (shell.staffNoticesToday) {
    entries[`${tenantSlug}:${staffNoticesTodayKey(shell.accountId)}`] =
      shell.staffNoticesToday;
  }
  return entries;
}

/**
 * SWR entries the server preloaded, under the tenant-scoped keys useSWRAuth
 * produces. `fallback` lets the first (server and hydration) render show the
 * data; `cacheData` makes SWR consume it as the mount revalidation instead of
 * fetching, after which it is an ordinary cache entry.
 */
export function shellSwrConfig(shell: ShellBootstrap, tenantSlug: string) {
  const entries: Record<string, unknown> = {
    ...shellOwnedEntries(shell, tenantSlug),
  };
  // Platform-scoped, so its key carries no tenant prefix.
  if (shell.announcements) {
    entries[ANNOUNCEMENTS_UNREAD_SWR_KEY] = shell.announcements;
  }
  return { fallback: entries, cacheData: entries };
}

/**
 * Writes the shell-owned entries into the SWR cache. A reader with
 * `revalidateIfStale: false` shows `fallback` and skips the mount
 * revalidation, so SWR never turns `cacheData` into a cache entry, and the
 * first later revalidation (poll, mutate, focus) answered with the seed
 * instead of asking the server (#4989). Like SessionCachePrimer, a leaf
 * rendered before the provider tree: its layout effect runs before any
 * reader mounts. An entry a reader already holds stays.
 */
export function ShellCacheSeeder({
  entries,
}: Readonly<{ entries: Record<string, unknown> }>) {
  useLayoutEffect(() => {
    for (const [key, data] of Object.entries(entries)) {
      void mutate(key, (current: unknown) => current ?? data, {
        revalidate: false,
      });
    }
  }, [entries]);
  return null;
}

/**
 * Tenant-scoped providers.
 *
 * SessionProvider reads the tenant cookie ("{TENANT_DOMAIN-derived}.session-token")
 * via the default basePath "/api/auth". This cookie is shared across
 * tenant subdomains for tenant-to-tenant switching.
 *
 * The read-only server session snapshot hydrates SessionProvider without
 * running refresh-capable auth callbacks in a Server Component. The provider
 * polls its response-aware route later; SessionCachePrimer shares the current
 * value with non-React API clients. `shell` is the server preload (#2973);
 * missing fields retain their client fetch path.
 */
export function TenantProviders({
  children,
  tenantSlug,
  tenant,
  routingMode,
  session,
  shell = null,
}: Readonly<{
  children: React.ReactNode;
  tenantSlug: string;
  tenant: TenantInfo;
  routingMode: TenantRoutingMode;
  session?: Session | null;
  shell?: ShellBootstrap | null;
}>) {
  const swrSeed = useMemo(
    () =>
      shell
        ? {
            config: shellSwrConfig(shell, tenantSlug),
            owned: shellOwnedEntries(shell, tenantSlug),
          }
        : null,
    [shell, tenantSlug],
  );

  const providers = (
    <ShellSeedProvider value={shell}>
      <TenantProvider
        tenantSlug={tenantSlug}
        tenant={tenant}
        routingMode={routingMode}
      >
        <TenantAuthWrapper>
          <ProfileProvider initialProfile={shell?.profile ?? null}>
            <SupervisionProvider initial={shell?.supervision ?? null}>
              {children}
            </SupervisionProvider>
          </ProfileProvider>
        </TenantAuthWrapper>
      </TenantProvider>
    </ShellSeedProvider>
  );

  return (
    <SessionProvider
      session={session ?? undefined}
      refetchInterval={SESSION_PROVIDER_POLL_SECONDS}
      refetchOnWindowFocus={false}
    >
      <PersonalStorageGuard
        homeHref={() => tenantAwarePath("/", tenantSlug, routingMode)}
      />
      <SessionCachePrimer />
      <ModuleBookingAudienceBridge>
        <PushSubscriptionSync portal="tenant" />
        <SentrySessionContext portal="tenant" />
        {swrSeed ? (
          <SWRConfig value={swrSeed.config}>
            <ShellCacheSeeder entries={swrSeed.owned} />
            {providers}
          </SWRConfig>
        ) : (
          providers
        )}
      </ModuleBookingAudienceBridge>
    </SessionProvider>
  );
}
