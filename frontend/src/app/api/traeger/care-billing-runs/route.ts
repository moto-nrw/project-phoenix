import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Monatsläufe des Trägers (#4684): Liste und neuen Lauf starten.
export const GET = proxyGet("/traeger/care-billing-runs");

export const POST = proxyPost("/traeger/care-billing-runs");
