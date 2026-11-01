import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

// Zwei-Faktor-Anmeldung eines Caterer-Kontos zurücksetzen (#4497): mit Grund,
// Audit-Eintrag und E-Mail an den Caterer. Danach meldet er sich mit dem
// Passwort an und kann sie im Konto wieder einschalten.
export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path: (_request, params) => {
    const catererId = params.id;
    const accountId = params.accountId;
    if (typeof catererId !== "string" || typeof accountId !== "string") {
      return operatorErrorResponse(
        400,
        "general.input",
        "Invalid caterer parameter",
      );
    }
    return `/operator/caterers/${encodeURIComponent(catererId)}/accounts/${encodeURIComponent(accountId)}/mfa`;
  },
  cache: "no-store",
  body: "json",
});
