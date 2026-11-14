import { traeger } from "~/lib/traeger/route-wrapper.server";

// The devices remembered at the Träger of the session (#6955), read with
// the Träger session.
export const GET = traeger.jsonProxy({
  method: "GET",
  path: "/traeger/auth/mfa/trusted-devices",
});
