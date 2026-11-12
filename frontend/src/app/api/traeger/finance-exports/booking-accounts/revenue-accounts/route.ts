import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Erlöskonto einer Leistung oder eines Angebots setzen oder entfernen (#4821).
export const PUT = proxyPut(
  "/traeger/finance-exports/booking-accounts/revenue-accounts",
);
