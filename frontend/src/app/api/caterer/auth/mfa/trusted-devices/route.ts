import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

// The caterer's remembered devices (#4497), read with the caterer session.

export const GET = createCatererJsonProxy({
  method: "GET",
  path: "/caterer/auth/mfa/trusted-devices",
  forwardClientHeaders: true,
});
