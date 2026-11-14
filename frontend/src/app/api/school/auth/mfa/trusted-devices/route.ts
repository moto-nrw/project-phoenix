import { school } from "~/lib/school/route-wrapper.server";

// Die im Schul-Portal gemerkten Geräte des eigenen Kontos (#6955), gelesen mit
// der Sitzung der Lehrkraft.

export const GET = school.jsonProxy({
  method: "GET",
  path: "/school/auth/mfa/trusted-devices",
});
