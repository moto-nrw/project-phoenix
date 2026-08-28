"use client";

import { useMemo } from "react";
import type { TeamChatPortal } from "~/lib/team-chat-portal";
import { tenantStaffMessagesApi } from "~/lib/staff-messages-api";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import { useTenantRouter } from "~/lib/tenant-router";

/**
 * The OGS-portal binding of the shared Team-Chat surfaces (#2598, #2208):
 * tenant router, tenant proxy routes, and the cached feature flag from the
 * tenant metadata. The school portal has its own binding in
 * lib/hooks/use-school-team-chat-portal.ts.
 */
export function useTenantTeamChatPortal(): TeamChatPortal {
  const router = useTenantRouter();
  const { tenant } = useTenant();
  // Tenant-prefix the SWR keys so a tenant switch (multi-tab / switch-tenant)
  // can never render the previous school's cached conversations.
  const tenantSlug = useTenantSlugSafe();
  const flagSaysEnabled = tenant?.staffMessagingEnabled === true;

  return useMemo(
    () => ({
      kind: "tenant",
      api: tenantStaffMessagesApi,
      cacheScope: tenantSlug ?? "",
      inboxHref: "/team-chat",
      threadHref: (threadId) => `/team-chat/${threadId}`,
      navigate: (href) => router.push(href),
      flagSaysEnabled,
      title: "Team-Chat",
      emptyDescription:
        "Hier stehen Ihre Unterhaltungen mit dem Team. Sie schreiben nur an Personen Ihrer Schule, auch an Lehrkräfte. Eltern sehen davon nichts.",
      recipientHint:
        "Sie schreiben nur an Personen Ihrer Schule. Lehrkräfte lesen im Portal „moto schule“ mit.",
    }),
    [router, tenantSlug, flagSaysEnabled],
  );
}
