import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Ein Nachweis eines Vertrags (#4685): ändern oder löschen.
const evidencePath = (params: Record<string, unknown>) =>
  `/traeger/care-evidences/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(evidencePath);

export const DELETE = proxyDelete(evidencePath);
