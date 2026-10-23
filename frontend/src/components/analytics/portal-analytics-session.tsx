"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { setAnalyticsContext } from "~/lib/posthog-client";
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
    let cancelled = false;
    setAnalyticsContext({ enabled: false });
    if (status === "authenticated" && inScope) {
      // Resolve on each session/school change. A failed read leaves collection off.
      void fetch(
        `/api/${scope === "parent" ? "parent" : "school"}/analytics-policy`,
        { cache: "no-store" },
      )
        .then(async (response) =>
          response.ok
            ? (response.json() as Promise<{ data?: { enabled?: boolean } }>)
            : null,
        )
        .then((result) => {
          if (cancelled) return;
          const key = schoolId ?? "";
          registerPortalSession(
            {
              surface,
              schoolId,
              role,
              enabled: result?.data?.enabled === true,
            },
            registeredRef.current !== null && registeredRef.current !== key,
          );
          registeredRef.current = key;
        })
        .catch(() => {
          /* Privacy lookup failures intentionally keep collection off. */
        });
    } else if (status === "unauthenticated" || status === "authenticated") {
      clearPortalSession();
      registeredRef.current = null;
    }
    return () => {
      cancelled = true;
      setAnalyticsContext({ enabled: false });
    };
  }, [status, inScope, schoolId, surface, scope, role]);

  return null;
}
