"use client";

import { useEffect } from "react";

import { Loading } from "~/components/ui/loading";
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

  useEffect(() => {
    router.replace("/home");
  }, [router]);

  return <Loading message="Startseite wird geöffnet" />;
}
