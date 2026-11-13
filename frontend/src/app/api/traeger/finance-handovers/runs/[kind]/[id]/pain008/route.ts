import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Die Lastschriftdatei eines ausgestellten Laufs an den Empfänger übergeben
// (#6824, ADR 0218). Jede Übergabe bekommt die nächste Dateinummer und steht
// im Exportprotokoll; ein Fehlschlag ist ein Ergebnis, kein Fehler.
export const POST = proxyPost(
  (params) =>
    `/traeger/finance-handovers/runs/${requirePathSegmentParam(params, "kind")}/${requirePathSegmentParam(params)}/pain008`,
);
