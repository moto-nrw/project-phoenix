import { proxyPut } from "~/lib/traeger/route-wrapper.server";

// Erste Dateinummer des Trägers, einmal setzbar (#4693).
export const PUT = proxyPut(
  "/traeger/finance-exports/settings/file-number-start",
);
