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
  const accountId = session?.user?.id;
  // Parent tokens carry no school: a guardian can have children in several.
  const schoolId = session?.user?.tenantId?.toString() ?? null;
  const registeredRef = useRef<string | null>(null);

  useEffect(() => {
    let requestSequence = 0;
    let cancelled = false;
    setAnalyticsContext({ surface, enabled: false });
    if (status === "authenticated" && inScope) {
      const refresh = () => {
        setAnalyticsContext({ surface, enabled: false });
        const sequence = ++requestSequence;
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
            if (cancelled || sequence !== requestSequence) return;
            // Account IDs stay local; a different account starts a fresh SDK session.
            const key = JSON.stringify([accountId, schoolId]);
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
      };
      const refreshVisible = () => {
        if (document.visibilityState === "visible") refresh();
      };
      refresh();
      document.addEventListener("visibilitychange", refreshVisible);
      const interval = window.setInterval(refreshVisible, 5 * 60 * 1_000);
      return () => {
        cancelled = true;
        document.removeEventListener("visibilitychange", refreshVisible);
        window.clearInterval(interval);
        setAnalyticsContext({ surface, enabled: false });
      };
    } else if (status === "unauthenticated" || status === "authenticated") {
      clearPortalSession();
      registeredRef.current = null;
    }
    return () => {
      cancelled = true;
      setAnalyticsContext({ surface, enabled: false });
    };
  }, [status, inScope, accountId, schoolId, surface, scope, role]);

  return null;
}
