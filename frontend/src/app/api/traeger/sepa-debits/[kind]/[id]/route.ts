import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyGet } from "~/lib/traeger/route-wrapper.server";

// Die Lastschriftdatei eines ausgestellten Laufs (#4697): was in die Datei
// geht, was draußen bleibt und warum, ohne Bankdaten der Familien. Die Art
// des Laufs (care-runs, holiday-runs) prüft das Backend.
export const GET = proxyGet(
  (params) =>
    `/traeger/sepa-debits/${requirePathSegmentParam(params, "kind")}/${requirePathSegmentParam(params)}`,
);
