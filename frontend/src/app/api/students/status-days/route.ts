import { proxyGet } from "~/lib/route-proxy.server";

// Abwesenheits-Übersicht (#2288): the forward-looking status days of every
// child in the caller's permitted groups.
export const GET = proxyGet("/api/students/status-days");
