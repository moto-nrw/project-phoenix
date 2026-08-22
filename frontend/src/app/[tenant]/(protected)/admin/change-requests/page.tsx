"use client";

import { redirect } from "next/navigation";

import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Alt-Route der Freigabeansicht für Elternanfragen. Der Inhalt lebt seit
 * #2429 im Anfragen-Modul (/anfragen, Reiter "Eltern"); gespeicherte Links
 * und Gewohnheiten landen hier und werden weitergeleitet. Der History-Eintrag
 * wird ersetzt, damit "Zurück" nicht in einer Redirect-Schleife endet.
 */
export default function AdminChangeRequestsRedirect() {
  const tenantPath = useTenantAwarePath();
  return redirect(tenantPath("/anfragen"));
}
