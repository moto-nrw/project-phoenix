import {
  proxyDelete,
  proxyGet,
  proxyPost,
} from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Löschauftrag eines deaktivierten Trägers (#4657): lesen, mit schriftlicher
// Bestätigung und Schonfrist erfassen, während der Schonfrist zurücknehmen.
export const GET = proxyGet(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/erasure-order`,
);

export const POST = proxyPost(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/erasure-order`,
);

export const DELETE = proxyDelete(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/erasure-order`,
);
