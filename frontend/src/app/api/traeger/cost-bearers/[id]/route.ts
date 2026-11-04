import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Ein Kostenträger des Trägers (#5627): ändern oder löschen.
const costBearerPath = (params: Record<string, unknown>) =>
  `/traeger/cost-bearers/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(costBearerPath);

export const DELETE = proxyDelete(costBearerPath);
