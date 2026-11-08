import { proxyGet } from "~/lib/route-wrapper.server";

// Abwesenheits-Übersicht (#2288): the forward-looking status days of every
// child in the caller's permitted groups.
export const GET = proxyGet("/api/students/status-days");
