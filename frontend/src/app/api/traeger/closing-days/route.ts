import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";

// Schließtage des Trägers und Feiertage seiner Schulen (#4702).
export const GET = proxyGet("/traeger/closing-days/", {
  forwardQuery: ["from", "to"],
});

export const POST = proxyPost("/traeger/closing-days/");
