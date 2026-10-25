import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { proxyDelete, proxyPut } from "~/lib/traeger/route-wrapper.server";

// Eine Beitragsregel des Trägers (#4664): ändern oder löschen.
const rulePath = (params: Record<string, unknown>) =>
  `/traeger/contribution-rules/${requirePathSegmentParam(params)}`;

export const PUT = proxyPut(rulePath);

export const DELETE = proxyDelete(rulePath);
