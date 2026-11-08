import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

// Einladung ins Träger-Büro prüfen (#4650). Öffentlich: Der Link aus der Mail
// ist der Nachweis, eine Sitzung gibt es noch nicht.
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (_request, params) =>
    `/traeger/invitations/${encodeURIComponent(String(params.token))}`,
});
