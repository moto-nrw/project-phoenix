import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Feldzuordnung der Sollstellung speichern oder zurücksetzen (#4693).
export const PUT = proxyPut("/traeger/finance-exports/settings/sollstellung");
export const DELETE = proxyDelete(
  "/traeger/finance-exports/settings/sollstellung",
);
