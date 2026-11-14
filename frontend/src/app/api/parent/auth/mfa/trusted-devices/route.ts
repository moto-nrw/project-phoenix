import { createParentJsonProxy } from "~/lib/parent/route-wrapper.server";

// The devices the parents portal remembered (#7051), read with the parent
// session. The shared device card reads every portal's list under its
// auth path; the parents portal keeps it with the account's settings.
export const GET = createParentJsonProxy({
  method: "GET",
  path: "/parent/me/mfa/trusted-devices",
});
