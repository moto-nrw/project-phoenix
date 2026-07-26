"use client";

import { SessionProvider } from "next-auth/react";
import { ProfileProvider } from "~/lib/profile-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { TenantAuthWrapper } from "~/components/auth/tenant-auth-wrapper";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";

/**
 * Tenant-scoped providers.
 *
 * SessionProvider reads the tenant cookie ("{TENANT_DOMAIN-derived}.session-token")
 * via the default basePath "/api/auth". This cookie is shared across
 * tenant subdomains for tenant-to-tenant switching.
 */
export function TenantProviders({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <SessionProvider refetchInterval={4 * 60} refetchOnWindowFocus={false}>
      <PushSubscriptionSync portal="tenant" />
      <TenantAuthWrapper>
        <ProfileProvider>
          <SupervisionProvider>{children}</SupervisionProvider>
        </ProfileProvider>
      </TenantAuthWrapper>
    </SessionProvider>
  );
}
