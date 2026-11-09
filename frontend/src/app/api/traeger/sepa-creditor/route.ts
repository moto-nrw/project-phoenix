import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Gläubiger-ID, Vorlauf der Vorabankündigung und Verhalten ohne Mandat
// (#4695).
export const GET = proxyGet("/traeger/sepa-creditor");

export const PUT = proxyPut("/traeger/sepa-creditor");
