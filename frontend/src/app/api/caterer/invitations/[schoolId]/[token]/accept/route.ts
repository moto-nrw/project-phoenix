import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { invitationPath } from "../invitation-path";

/** Public: the caterer accepts the mailed invitation with its password. */
export const POST = createPublicJsonProxy({
  method: "POST",
  path: (_request, params) => invitationPath(params, "/accept"),
  forwardClientHeaders: true,
});
