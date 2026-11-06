import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Rechnungen und Beträge je Familie (#4691): Liste der Familien.
export const GET = proxyGet("/traeger/billing-families");
