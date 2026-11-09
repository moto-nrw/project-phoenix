import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Ein Schließtag des Trägers (#4702): ändern oder löschen.
const closingDayPath = (params: Record<string, unknown>) =>
  `/traeger/closing-days/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(closingDayPath);

export const DELETE = proxyDelete(closingDayPath);
