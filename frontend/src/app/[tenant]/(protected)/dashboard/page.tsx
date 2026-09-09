"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";

import { Loading } from "~/components/ui/loading";
import {
  getSmartRedirectPath,
  isSchoolPortalHandoffPath,
} from "~/lib/redirect-utils";
import { schoolPortalLoginUrl } from "~/lib/school-url";
import { useTenantRouter } from "~/lib/tenant-router";

/**
 * /dashboard ist seit #2180 die Startseite aller Rollen unter /home.
 *
 * Die alte Adresse bleibt als Weiterleitung stehen: sie steht in Lesezeichen,
 * in älteren E-Mails und in der Adresszeile von Leuten, die sie auswendig
 * tippen. Ein toter Link wäre die schlechteste Antwort darauf.
 */
export default function DashboardRedirectPage() {
  const router = useTenantRouter();
  const { data: session, status } = useSession();

  useEffect(() => {
    if (status !== "authenticated") return;

    const destination = getSmartRedirectPath(session);
    if (isSchoolPortalHandoffPath(destination)) {
      window.location.href = schoolPortalLoginUrl();
      return;
    }
    router.replace(destination);
  }, [router, session, status]);

  return <Loading message="Startseite wird geöffnet" />;
}
