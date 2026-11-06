import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Angenommene Anmeldungen ohne Betreuungsvertrag (#4678).
export const GET = proxyGet("/traeger/contracts/candidates");
