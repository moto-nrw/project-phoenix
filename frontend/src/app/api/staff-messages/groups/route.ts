import { proxyPost } from "~/lib/route-proxy.server";

// Gruppenchat anlegen (#4776): Name und Mitglieder. Wer anlegen darf,
// entscheidet das Backend.
export const POST = proxyPost("/api/staff-messages/groups");
