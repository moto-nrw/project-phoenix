import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import {
  createOperatorJsonProxy,
  operatorJsonErrors,
} from "~/lib/operator/route-wrapper.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const { accountId, tenantId } = params;
  if (typeof accountId !== "string" || typeof tenantId !== "string") {
    return operatorErrorResponse(
      400,
      "general.input",
      "Invalid account or school parameter",
    );
  }
  return `/operator/accounts/${encodeURIComponent(accountId)}/tenants/${encodeURIComponent(tenantId)}`;
};

export const PUT = createOperatorJsonProxy({
  method: "PUT",
  path,
  invalidJsonResponse: operatorJsonErrors.invalidJsonResponse,
});
export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path,
});
