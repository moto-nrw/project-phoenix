import { proxyGet, proxyPost } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Geschwister aus getrennten Ferienanmeldungen (#5628): die Gruppen eines
// Zeitraums und das Zusammenführen. Die Elternabrechnung hält die Gruppen,
// darum gehen beide an ihre Route.
export const GET = proxyGet(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/sibling-groups`,
);

export const POST = proxyPost(
  (params) =>
    `/traeger/holiday-contributions/${requirePathSegmentParam(params)}/sibling-groups`,
);
