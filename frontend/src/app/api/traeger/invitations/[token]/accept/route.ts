import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

// Einladung ins Träger-Büro annehmen (#4650): neues Passwort oder das
// Passwort des vorhandenen moto-Kontos.
export const POST = createPublicJsonProxy({
  method: "POST",
  path: (_request, params) =>
    `/traeger/invitations/${encodeURIComponent(String(params.token))}/accept`,
  body: "optional-json",
});
