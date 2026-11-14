import { createParentJsonProxy } from "~/lib/parent/route-wrapper.server";

// The parents' sign-in with a code (#7051): whether it is on, switching it on
// with an e-mailed code and switching it off with the password.

export const GET = createParentJsonProxy({
  method: "GET",
  path: "/parent/me/mfa",
});
