import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// SEPA-Mandate der Familien (#4695): Liste (auf Wunsch einer Familie) und
// neues Mandat. Die IBAN kommt nur maskiert zurück.
export const GET = proxyGet("/traeger/sepa-mandates");

export const POST = proxyPost("/traeger/sepa-mandates");
