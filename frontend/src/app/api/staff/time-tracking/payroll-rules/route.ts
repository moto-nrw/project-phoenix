import { proxyGet } from "~/lib/route-proxy.server";

// Eigene Lohnarten, Zuschlagsregeln und die Zuordnung der Abwesenheitsarten
// der Schule (#4807) für /payroll.
export const GET = proxyGet(() => "/api/staff/time-tracking/payroll-rules");
