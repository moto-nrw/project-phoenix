"use client";

import { SessionProvider } from "next-auth/react";
import { ProfileProvider } from "~/lib/profile-context";
import { SupervisionProvider } from "~/lib/supervision-context";
import { TenantAuthWrapper } from "~/components/auth/tenant-auth-wrapper";
import { PushSubscriptionSync } from "~/components/notifications/service-worker-registrar";
import { TenantProvider, type TenantRoutingMode } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";

/**
 * Tenant-scoped providers.
 *
 * SessionProvider reads the tenant cookie ("{TENANT_DOMAIN-derived}.session-token")
 * via the default basePath "/api/auth". This cookie is shared across
 * tenant subdomains for tenant-to-tenant switching.
 */
export function TenantProviders({
  children,
  tenantSlug,
  tenant,
  routingMode,
}: Readonly<{
  children: React.ReactNode;
  tenantSlug: string;
  tenant: TenantInfo;
  routingMode: TenantRoutingMode;
}>) {
  return (
    <SessionProvider refetchInterval={4 * 60} refetchOnWindowFocus={false}>
      <PushSubscriptionSync portal="tenant" />
      <TenantProvider
        tenantSlug={tenantSlug}
        tenant={tenant}
        routingMode={routingMode}
      >
        <TenantAuthWrapper>
          <ProfileProvider>
            <SupervisionProvider>{children}</SupervisionProvider>
          </ProfileProvider>
        </TenantAuthWrapper>
      </TenantProvider>
    </SessionProvider>
  );
}
