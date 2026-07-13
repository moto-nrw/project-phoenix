"use client";

/**
 * PlanungRedirect — Client-Redirect der alten Planungs-Routen (#1886).
 *
 * /timetables, /vertretungsplan und /staff/dienstplan sind Tabs von /planung
 * geworden. Der Redirect reicht alle bestehenden Query-Params durch, damit
 * Deep-Links (z. B. ?instance=…, ?week=…) weiter funktionieren, und ersetzt
 * den History-Eintrag, damit "Zurück" nicht in einer Redirect-Schleife endet.
 */

import { useEffect } from "react";
import { useSearchParams } from "next/navigation";

import { useTenantRouter } from "~/lib/tenant-router";

export function PlanungRedirect({
  tab,
}: {
  tab: "betreuung" | "dienstplan" | "vertretung";
}) {
  const router = useTenantRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("tab", tab);
    router.replace(`/planung?${params.toString()}`);
  }, [router, searchParams, tab]);

  return null;
}
