import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Abweichungen der Familien (#4689): Empfänger, Zahlungsweg und Zahltag.
// PUT setzt die Abweichung der Familie eines Vertrags; weicht nichts mehr
// ab, ist sie entfernt.
export const GET = proxyGet("/traeger/family-billing");

export const PUT = proxyPut("/traeger/family-billing");
