import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

export const PUT = createOperatorJsonProxy({
  method: "PUT",
  path: (_request, params) => {
    const schoolId = params.id;
    const accountId = params.accountId;
    if (typeof schoolId !== "string" || typeof accountId !== "string") {
      return operatorErrorResponse(
        400,
        "general.input",
        "Invalid school or account parameter",
      );
    }
    return `/operator/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/mfa/override`;
  },
  cache: "no-store",
});
