import { proxyDelete } from "~/lib/route-proxy.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Eine offene Einladung zurücknehmen. meals:manage erzwingt das Backend.
export const DELETE = proxyDelete(
  (p: Record<string, unknown>) =>
    `/api/meal-caterer/invitations/${requirePathSegmentParam(p, "invitationId")}`,
);
