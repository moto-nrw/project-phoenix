import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Dokumentart je Leistung (#4688).
export const GET = proxyGet("/traeger/billing-document-kinds");
