"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { clearPortalSession } from "~/lib/analytics";
import { setAnalyticsContext } from "~/lib/posthog-client";

/**
 * The analytics context of the Träger portal (#4649). The portal collects
 * nothing yet (see `analyticsCollectionAllowed`), so a signed-in session
 * only pins the surface with collection off, and a logout clears the
 * context like in every other portal. Renders nothing.
 */
export function TraegerAnalyticsSession() {
  const { status } = useSession();

  useEffect(() => {
    if (status === "authenticated") {
      setAnalyticsContext({ surface: "traeger", enabled: false });
    } else if (status === "unauthenticated") {
      clearPortalSession();
    }
  }, [status]);

  return null;
}
