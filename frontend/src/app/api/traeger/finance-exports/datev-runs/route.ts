import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Neue DATEV-Buchungen und Wirtschaftsjahre der Läufe einer Seite (#6433).
export const GET = proxyGet("/traeger/finance-exports/datev-runs");
