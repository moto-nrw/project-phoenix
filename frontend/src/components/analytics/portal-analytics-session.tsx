"use client";

import { useEffect, useRef } from "react";
import { useSession } from "next-auth/react";
import { setAnalyticsContext } from "~/lib/posthog-client";
import { clearPortalSession, registerPortalSession } from "~/lib/analytics";
import {
  setAnalyticsConsentSchool,
  startAnalyticsConsent,
  stopAnalyticsConsent,
} from "~/lib/analytics-consent";
import type { AnalyticsConsentCopy } from "~/components/analytics/analytics-consent-copy";
import { AnalyticsConsentPromptGate } from "~/components/analytics/analytics-consent-prompt-gate";

const PORTALS = {
  parents: { scope: "parent", role: "guardian", consent: "parent" },
  school: { scope: "school", role: "lehrkraft", consent: "school" },
} as const;

/**
 * Registers the analytics context of the parents or school portal at login
 * and clears it at logout, like TenantAuthWrapper does for the OGS portal.
 * Mounted inside the portal's SessionProvider. It also reads the person's
 * consent (#4901) and renders the consent prompt after login, in the texts
 * the portal hands in (translated in the parents portal).
 */
export function PortalAnalyticsSession({
  surface,
  consentCopy,
}: {
  readonly surface: keyof typeof PORTALS;
  readonly consentCopy: AnalyticsConsentCopy;
}) {
  const { data: session, status } = useSession();
  const { scope, role, consent } = PORTALS[surface];
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
      if (accountId) startAnalyticsConsent(consent, accountId);
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
            setAnalyticsConsentSchool(consent, result?.data?.enabled === true);
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
      stopAnalyticsConsent(consent);
      registeredRef.current = null;
    }
    return () => {
      cancelled = true;
      setAnalyticsContext({ surface, enabled: false });
    };
  }, [status, inScope, accountId, schoolId, surface, scope, role, consent]);

  return <AnalyticsConsentPromptGate portal={consent} copy={consentCopy} />;
}
