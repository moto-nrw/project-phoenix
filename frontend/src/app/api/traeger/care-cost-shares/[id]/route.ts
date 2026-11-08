import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Ein Kostenträger-Anteil eines Vertrags (#4687): ändern oder löschen.
const sharePath = (params: Record<string, unknown>) =>
  `/traeger/care-cost-shares/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(sharePath);

export const DELETE = proxyDelete(sharePath);
