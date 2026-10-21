import { proxyPost } from "~/lib/route-proxy.server";

// Einladung an den Caterer: der Link geht nur per E-Mail an ihn, nie an die
// Schule (ADR 0077). meals:manage erzwingt das Backend.
export const POST = proxyPost("/api/meal-caterer/invitations");
