import { problemResponse } from "~/lib/api-helpers.server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

// Beitrag der öffentlichen Ferienanmeldung (#4664) für die Zusammenfassung
// vor dem Absenden: ohne Sitzung, nur die Buchungen, nichts wird gespeichert.
export const POST = createPublicJsonProxy({
  method: "POST",
  path: (request, params) => {
    const token = params.token;
    if (typeof token !== "string" || !token) {
      return problemResponse(400, "token required", request);
    }
    return `/traeger/holiday-quotes/${encodeURIComponent(token)}`;
  },
});
