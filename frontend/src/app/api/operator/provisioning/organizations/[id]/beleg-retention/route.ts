import { proxyGet } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Belegaufbewahrung eines Trägers (#4720): Weisungen, Aussetzungen,
// Klärungsliste und Löschprotokoll lesen.
export const GET = proxyGet(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/beleg-retention`,
);
