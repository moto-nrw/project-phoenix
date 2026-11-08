import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Zahlungsweg, Zahlungsziel und Bündelung je Leistung (#4689).
export const GET = proxyGet("/traeger/billing-terms");
