import { traeger } from "~/lib/traeger/route-wrapper.server";

// Die im Träger-Portal gemerkten Geräte des eigenen Kontos (#6955), gelesen mit
// der Träger-Sitzung.

export const GET = traeger.jsonProxy({
  method: "GET",
  path: "/traeger/auth/mfa/trusted-devices",
});
