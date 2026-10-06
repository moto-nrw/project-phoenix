import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createOperatorJsonProxy({
  method: "GET",
  path: (_request, params) => {
    const { accountId, tenantId } = params;
    if (typeof accountId !== "string" || typeof tenantId !== "string") {
      return operatorErrorResponse(
        400,
        "general.input",
        "Invalid account or school parameter",
      );
    }
    return `/operator/accounts/${encodeURIComponent(accountId)}/tenants/${encodeURIComponent(tenantId)}/roles`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
