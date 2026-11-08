import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Schulen für einen neuen Monatslauf, je Schule der offene Lauf des
// Monats, der sie schon abrechnet (#5916). Die Abfrage `month` geht mit.
export const GET = proxyGet("/traeger/care-billing-runs/schools", {
  forwardQuery: ["month"],
});
