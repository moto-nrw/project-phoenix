import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Abrechnungsläufe des Trägers (#4668): Liste und neuen Lauf starten.
export const GET = proxyGet("/traeger/billing-runs");

export const POST = proxyPost("/traeger/billing-runs");
