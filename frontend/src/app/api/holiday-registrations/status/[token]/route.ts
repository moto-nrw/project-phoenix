import { problemResponse } from "~/lib/api-helpers.server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

// Stand einer Ferienanmeldung (#6676): der geheime Link aus der
// Eingangsmail, ohne Sitzung. Der Token ist der einzige Schlüssel.
export const GET = createPublicJsonProxy({
  method: "GET",
  path: (request, params) => {
    const token = params.token;
    if (typeof token !== "string" || !token) {
      return problemResponse(400, "token required", request);
    }
    return `/traeger/holiday-registrations/status/${encodeURIComponent(token)}`;
  },
});
