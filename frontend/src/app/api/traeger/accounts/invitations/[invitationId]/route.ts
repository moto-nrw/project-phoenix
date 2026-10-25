import { proxyDelete } from "~/lib/traeger/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Offene Einladung ins Träger-Büro zurücknehmen (#4650).
export const DELETE = proxyDelete(
  (params) =>
    `/traeger/accounts/invitations/${requirePathSegmentParam(params, "invitationId")}`,
);
