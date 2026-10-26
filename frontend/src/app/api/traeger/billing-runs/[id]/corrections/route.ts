import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Einen Betrag im Abrechnungslauf mit Begründung korrigieren (#4668).
export const POST = proxyPost(
  (params) =>
    `/traeger/billing-runs/${requirePathSegmentParam(params)}/corrections`,
);
