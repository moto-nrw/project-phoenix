import { proxyGet, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Preisliste des Monatsbeitrags (#4684): Module, Preise je Wochentag und
// Schule, ab einem Monat gültig.
export const GET = proxyGet("/traeger/care-tariffs");

export const PUT = proxyPut("/traeger/care-tariffs");
