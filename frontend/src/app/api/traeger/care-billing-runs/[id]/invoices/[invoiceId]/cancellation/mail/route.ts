import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Das Storno-Schreiben einer Monatsrechnung senden, zuerst oder erneut
// (#4684). Die POST-Weiterleitung nimmt keine Abfrage mit, deshalb hat der
// Storno eine eigene Adresse.
export const POST = proxyPost(
  (params) =>
    `/traeger/care-billing-runs/${requirePathSegmentParam(params)}/invoices/${requirePathSegmentParam(params, "invoiceId")}/mail?kind=cancellation`,
);
