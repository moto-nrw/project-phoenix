import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Neue Person ins Träger-Büro einladen (#4650). Das Backend schickt die
// Einladung per Mail.
export const POST = proxyPost("/traeger/accounts/invitations");
