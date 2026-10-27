import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

// Zweiten Faktor einer Person im Träger-Büro zurücksetzen (#4651). Die
// Person richtet ihn bei der nächsten Anmeldung im Träger-Portal neu ein.
export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path: (_request, params) => {
    const organizationId = params.id;
    const accountId = params.accountId;
    if (typeof organizationId !== "string" || typeof accountId !== "string") {
      return operatorErrorResponse(
        400,
        "general.input",
        "Invalid organization or account parameter",
      );
    }
    return `/operator/organizations/${encodeURIComponent(organizationId)}/traeger-members/${encodeURIComponent(accountId)}/mfa`;
  },
  cache: "no-store",
  body: "json",
});
