import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Ausgestellte Läufe mit ihrem letzten Export (#4693).
export const GET = proxyGet("/traeger/finance-exports/runs");
