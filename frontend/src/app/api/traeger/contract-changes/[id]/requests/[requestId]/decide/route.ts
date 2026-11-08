import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyPost } from "~/lib/traeger/route-wrapper.server";

// Das Träger-Büro gibt einen Änderungswunsch der Eltern frei oder lehnt ihn
// ab (#5979).
export const POST = proxyPost(
  (params) =>
    `/traeger/contract-changes/${requirePathSegmentParam(params)}/requests/${requirePathSegmentParam(params, "requestId")}/decide`,
);
