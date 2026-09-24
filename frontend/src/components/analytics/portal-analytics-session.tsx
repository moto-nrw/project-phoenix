"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { clearPortalSession, registerPortalSession } from "~/lib/analytics";

const PORTALS = {
  parents: { scope: "parent", role: "guardian" },
  school: { scope: "school", role: "lehrkraft" },
} as const;

/**
 * Registers the analytics context of the parents or school portal at login
 * and clears it at logout, like TenantAuthWrapper does for the OGS portal.
 * Mounted inside the portal's SessionProvider; renders nothing.
 */
export function PortalAnalyticsSession({
  surface,
}: {
  readonly surface: keyof typeof PORTALS;
}) {
  const { data: session, status } = useSession();
  const { scope, role } = PORTALS[surface];
  const inScope = session?.user?.scope === scope;
  // Parent tokens carry no school: a guardian can have children in several.
  const schoolId = session?.user?.tenantId?.toString() ?? null;
  const registeredRef = useRef<string | null>(null);

  useEffect(() => {
    if (status === "authenticated" && inScope) {
      const key = schoolId ?? "";
      registerPortalSession(
        { surface, schoolId, role },
        registeredRef.current !== null && registeredRef.current !== key,
      );
      registeredRef.current = key;
    } else if (status === "unauthenticated" || status === "authenticated") {
      clearPortalSession();
      registeredRef.current = null;
    }
  }, [status, inScope, schoolId, surface, role]);

  return null;
}
