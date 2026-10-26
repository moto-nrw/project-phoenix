import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Nummernkreise der Rechnungen je Leistung (#4668).
export const GET = proxyGet("/traeger/number-ranges");
