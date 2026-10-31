"use client";

import { useLayoutEffect, useMemo, useRef } from "react";
import type { Session } from "next-auth";
import { SessionProvider, useSession } from "next-auth/react";
// eslint-disable-next-line no-restricted-imports -- SWRConfig only; hooks stay on ~/lib/swr
import { SWRConfig } from "swr";
import { ProfileProvider } from "~/lib/profile-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { PersonalStorageGuard } from "~/components/auth/personal-storage-guard";
import { SessionAccountBinding } from "~/components/auth/session-account-binding";
import { SentrySessionContext } from "~/components/auth/sentry-session-context";
import { ModuleBookingAudienceProvider } from "~/contexts/ToastContext";
import { TenantAuthWrapper } from "~/components/auth/tenant-auth-wrapper";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { ANNOUNCEMENTS_UNREAD_SWR_KEY } from "~/lib/hooks/use-announcements";
import { USER_CONTEXT_SWR_KEY } from "~/lib/hooks/use-user-context";
import { REMINDERS_SWR_KEY } from "~/lib/reminders-api";
import { primeSessionCache } from "~/lib/session-cache";
import { SESSION_PROVIDER_POLL_SECONDS } from "~/lib/session-provider-poll";
import { useSessionPollInterval } from "~/lib/session-end-state";
import { ShellSeedProvider, type ShellBootstrap } from "~/lib/shell-seed";
import { mutate, scopedSWRKey, tenantSWRScope } from "~/lib/swr";
import { dataGenerationOf, stampSWRValue } from "~/lib/swr/generation";
import { authGeneration, useSWRSessionRefresh } from "~/lib/swr/scope";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
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
  // The scope useSWRAuth gives the account the snapshot was built for
  // (#5006): no other account's reader finds these entries.
  const scope = tenantSWRScope(tenantSlug, shell.accountId);
  const entries: Record<string, unknown> = {};
  if (shell.userContext) {
    entries[scopedSWRKey(scope, USER_CONTEXT_SWR_KEY)] = shell.userContext;
  }
  if (shell.reminders) {
    entries[scopedSWRKey(scope, REMINDERS_SWR_KEY)] = shell.reminders;
  }
  if (shell.staffNoticesToday) {
    entries[scopedSWRKey(scope, staffNoticesTodayKey(shell.accountId))] =
      shell.staffNoticesToday;
  }
  return entries;
}

/**
 * SWR entries the server preloaded, under the tenant-scoped keys useSWRAuth
 * produces. `fallback` lets the first (server and hydration) render show the
 * data; `cacheData` makes SWR consume it as the mount revalidation instead of
 * fetching, after which it is an ordinary cache entry.
 *
 * The shell-owned entries stay out of `cacheData`: ShellCacheSeeder writes
 * them into the cache, so no reader ever consumes them there, and SWR keeps
 * a hook's `cacheData` in its revalidation for the hook's lifetime. After
 * the cache reset of a rights change under the same key (#5006), that
 * revalidation answered with the seed instead of the server.
 */
export function shellSwrConfig(shell: ShellBootstrap, tenantSlug: string) {
  const consumed: Record<string, unknown> = {};
  // Platform-scoped, so its key carries no tenant prefix.
  if (shell.announcements) {
    consumed[ANNOUNCEMENTS_UNREAD_SWR_KEY] = shell.announcements;
  }
  return {
    fallback: { ...shellOwnedEntries(shell, tenantSlug), ...consumed },
    cacheData: consumed,
  };
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

const NO_SEED = { fallback: {}, cacheData: {} };
const NO_ENTRIES: Record<string, unknown> = {};

/**
 * The server preload, only while the session is the one it was built for
 * (#5006). After a change of rights or preview, even under the same keys, no
 * reader may receive the snapshot of the session before; the cache reset of
 * SWRScopeBoundary drops what the seeder wrote, this drops the fallback. A
 * generation once left never takes the seed back.
 */
function ShellSwrSeed({
  seed,
  generation,
  children,
}: Readonly<{
  seed: {
    config: ReturnType<typeof shellSwrConfig>;
    owned: Record<string, unknown>;
  };
  generation: string | null;
  children: React.ReactNode;
}>) {
  const { data: session, status } = useSession();
  const current = status === "loading" ? generation : authGeneration(session);
  const refresh = useSWRSessionRefresh();
  const initialRevision = useRef(refresh.revision);
  const left = useRef(false);
  const invalidated =
    refresh.pending || refresh.revision !== initialRevision.current;
  const active = !left.current && !invalidated && current === generation;
  useLayoutEffect(() => {
    if (invalidated || current !== generation) left.current = true;
  }, [invalidated, current, generation]);
  return (
    <SWRConfig value={active ? seed.config : NO_SEED}>
      <ShellCacheSeeder entries={active ? seed.owned : NO_ENTRIES} />
      {children}
    </SWRConfig>
  );
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
  const swrSeed = useMemo(() => {
    if (!shell) return null;
    const config = shellSwrConfig(shell, tenantSlug);
    // The seed belongs to the generation it was built in (#5006): after a
    // drop of one of its keys no reader shows it, also not as fallback.
    for (const [key, value] of Object.entries(config.fallback)) {
      stampSWRValue(key, value, dataGenerationOf(key));
    }
    return { config, owned: shellOwnedEntries(shell, tenantSlug) };
  }, [shell, tenantSlug]);
  // The seed belongs to the session the server rendered with.
  const seedGeneration = authGeneration(session);
  // A logout stops the poll at once (#5399).
  const pollSeconds = useSessionPollInterval(SESSION_PROVIDER_POLL_SECONDS);

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
      refetchInterval={pollSeconds}
      refetchOnWindowFocus={false}
    >
      <SessionAccountBinding />
      <PersonalStorageGuard
        homeHref={() => tenantAwarePath("/", tenantSlug, routingMode)}
      />
      <SWRScopeBoundary expectedTenantId={tenant.tenantId} />
      <SessionCachePrimer />
      <ModuleBookingAudienceBridge>
        <PushSubscriptionSync portal="tenant" />
        <SentrySessionContext portal="tenant" />
        {swrSeed ? (
          <ShellSwrSeed seed={swrSeed} generation={seedGeneration}>
            {providers}
          </ShellSwrSeed>
        ) : (
          providers
        )}
      </ModuleBookingAudienceBridge>
    </SessionProvider>
  );
}
