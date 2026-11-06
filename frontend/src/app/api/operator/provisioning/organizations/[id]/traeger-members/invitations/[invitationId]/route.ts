import { proxyDelete } from "~/lib/operator/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

// Offene Einladung ins Träger-Büro zurücknehmen (#5749): der Link in der
// E-Mail funktioniert danach nicht mehr.
export const DELETE = proxyDelete(
  (params) =>
    `/operator/organizations/${requirePathSegmentParam(params)}/traeger-members/invitations/${requirePathSegmentParam(params, "invitationId")}`,
);
