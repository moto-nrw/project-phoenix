"use client";

import { useLayoutEffect, useMemo } from "react";
import type { Session } from "next-auth";
import { SessionProvider, useSession } from "next-auth/react";
// eslint-disable-next-line no-restricted-imports -- SWRConfig only; hooks stay on ~/lib/swr
import { SWRConfig } from "swr";
import { ProfileProvider } from "~/lib/profile-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { TenantAuthWrapper } from "~/components/auth/tenant-auth-wrapper";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { ANNOUNCEMENTS_UNREAD_SWR_KEY } from "~/lib/hooks/use-announcements";
import { USER_CONTEXT_SWR_KEY } from "~/lib/hooks/use-user-context";
import { REMINDERS_SWR_KEY } from "~/lib/reminders-api";
import { primeSessionCache } from "~/lib/session-cache";
import { SETTINGS_SCHEMA_SWR_KEY } from "~/lib/settings-api";
import { ShellSeedProvider, type ShellBootstrap } from "~/lib/shell-seed";
import { TenantProvider, type TenantRoutingMode } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";

/**
 * Mirrors SessionProvider's response into the getSession() dedupe cache that
 * every API client reads (#2973). A layout effect in a leaf rendered BEFORE
 * the provider tree commits before any SWR hook further down starts its mount
 * revalidation; the hydrated session is therefore reused instead of fetched.
 * Later session changes (poll, update()) go through the cache's own TTL and
 * the explicit clearSessionCache() calls of the flows that rotate tokens.
 */
function SessionCachePrimer() {
  const { data: session } = useSession();
  useLayoutEffect(() => {
    if (session?.user?.token) primeSessionCache(session);
  }, [session]);
  return null;
}

/**
 * SWR entries the server preloaded, under the tenant-scoped keys useSWRAuth
 * produces. `fallback` lets the first (server and hydration) render show the
 * data; `cacheData` makes SWR consume it as the mount revalidation instead of
 * fetching, after which it is an ordinary cache entry.
 */
function shellSwrConfig(shell: ShellBootstrap, tenantSlug: string) {
  const entries: Record<string, unknown> = {};
  if (shell.userContext) {
    entries[`${tenantSlug}:${USER_CONTEXT_SWR_KEY}`] = shell.userContext;
  }
  if (shell.settingsSchema) {
    entries[`${tenantSlug}:${SETTINGS_SCHEMA_SWR_KEY}`] = shell.settingsSchema;
  }
  if (shell.reminders) {
    entries[`${tenantSlug}:${REMINDERS_SWR_KEY}`] = shell.reminders;
  }
  // Platform-scoped, so its key carries no tenant prefix.
  if (shell.announcements) {
    entries[ANNOUNCEMENTS_UNREAD_SWR_KEY] = shell.announcements;
  }
  return { fallback: entries, cacheData: entries };
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
  const swrConfig = useMemo(
    () => (shell ? shellSwrConfig(shell, tenantSlug) : null),
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
      refetchInterval={4 * 60}
      refetchOnWindowFocus={false}
    >
      <SessionCachePrimer />
      <PushSubscriptionSync portal="tenant" />
      {swrConfig ? (
        <SWRConfig value={swrConfig}>{providers}</SWRConfig>
      ) : (
        providers
      )}
    </SessionProvider>
  );
}
