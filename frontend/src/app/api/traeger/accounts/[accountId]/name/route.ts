import { proxyPut } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Namen einer Person im Träger-Büro eintragen oder ändern (#4693). Er steht
// im Exportprotokoll und im Verlauf jeder Rechnung. Das Backend verlangt das
// Recht „Konten verwalten“.
export const PUT = proxyPut(
  (params) =>
    `/traeger/accounts/${requirePathSegmentParam(params, "accountId")}/name`,
);
