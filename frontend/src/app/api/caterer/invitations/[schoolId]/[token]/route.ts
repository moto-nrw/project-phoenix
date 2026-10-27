import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { invitationPath } from "./invitation-path";

/** Public: what the mailed link offers. The link itself is the proof. */
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (_request, params) => invitationPath(params),
  forwardClientHeaders: true,
  cache: "no-store",
});
