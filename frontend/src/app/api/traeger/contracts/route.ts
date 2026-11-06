import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Betreuungsverträge des Trägers (#4678): Liste mit Filter und Entwürfe aus
// angenommenen Anmeldungen anlegen.
export const GET = proxyGet("/traeger/contracts");

export const POST = proxyPost("/traeger/contracts");
