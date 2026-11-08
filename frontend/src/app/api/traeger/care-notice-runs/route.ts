import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Läufe der Jahresbescheide des Trägers (#6219): Liste und neuen Lauf starten.
export const GET = proxyGet("/traeger/care-notice-runs");

export const POST = proxyPost("/traeger/care-notice-runs");
