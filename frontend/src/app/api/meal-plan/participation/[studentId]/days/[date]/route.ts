import { proxyDelete, proxyPut } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Einen Tag an- oder abmelden bzw. zurück auf die festen Tage (#3873), auch
// nach der Frist. meals:orders erzwingt das Backend.
const endpoint = (p: Record<string, unknown>) =>
  `/api/meal-plan/participation/${requirePathSegmentParam(p, "studentId")}/days/${requirePathSegmentParam(p, "date")}`;

export const PUT = proxyPut(endpoint);
export const DELETE = proxyDelete(endpoint);
