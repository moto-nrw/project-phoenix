import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

export const POST = createCatererJsonProxy({
  method: "POST",
  path: "/caterer/auth/passkeys/enrollment/challenge",
  forwardClientHeaders: true,
});
