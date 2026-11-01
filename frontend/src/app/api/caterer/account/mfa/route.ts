import { createCatererJsonProxy } from "~/lib/caterer/route-wrapper.server";

// The caterer's two-factor sign-in (#4497): whether it is on, switching it
// on with an e-mailed code and switching it off with the password.

export const GET = createCatererJsonProxy({
  method: "GET",
  path: "/caterer/account/mfa",
  forwardClientHeaders: true,
});
