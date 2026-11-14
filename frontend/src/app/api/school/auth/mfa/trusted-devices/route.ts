import { school } from "~/lib/school/route-wrapper.server";

// The devices moto schule remembered at the session's school (#6955), read
// with the school session.
export const GET = school.jsonProxy({
  method: "GET",
  path: "/school/auth/mfa/trusted-devices",
});
