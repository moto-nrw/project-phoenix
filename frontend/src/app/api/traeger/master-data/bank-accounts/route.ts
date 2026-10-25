import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Neue Bankverbindung des Trägers (#4653).
export const POST = proxyPost("/traeger/master-data/bank-accounts");
