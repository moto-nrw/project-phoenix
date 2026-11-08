import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Eine Regel des Monatsbeitrags (#4685): ändern oder löschen.
const rulePath = (params: Record<string, unknown>) =>
  `/traeger/care-rules/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(rulePath);

export const DELETE = proxyDelete(rulePath);
