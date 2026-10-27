import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

export const POST = createCatererJsonProxy({
  method: "POST",
  path: "/caterer/account/mfa/confirm",
  forwardClientHeaders: true,
});
