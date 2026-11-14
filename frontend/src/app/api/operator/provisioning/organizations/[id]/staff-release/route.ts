import {
  proxyDelete,
  proxyGet,
  proxyPost,
} from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Freigabe „Personal beim Träger“ (#4705, ADR 0089 Abschnitt 2): lesen,
// mit der bestätigten Erklärung erteilen und zurücknehmen.
export const GET = proxyGet(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/staff-release`,
);

export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/staff-release`,
);

export const DELETE = proxyDelete(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/staff-release`,
);
