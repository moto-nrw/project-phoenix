"use client";

import { redirect } from "next/navigation";

import { useTenantAwarePath } from "~/lib/tenant-path";

/**
 * Alt-Route der Anmeldungs-Änderungsanfragen. Sie erscheinen seit #2435 im
 * Anfragen-Modul (/anfragen, Reiter „Eltern") als Anfrageart „Anmeldung";
 * gespeicherte Links und Gewohnheiten landen hier und werden weitergeleitet.
 * Die Detailansicht darunter bleibt und wird aus der Liste heraus verlinkt.
 */
export default function AdminEnrollmentChangeRequestsRedirect() {
  const tenantPath = useTenantAwarePath();
  return redirect(tenantPath("/anfragen"));
}
