import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const accountId = params.accountId;
  if (typeof accountId !== "string") {
    return operatorErrorResponse(
      400,
      "general.input",
      "Invalid account parameter",
    );
  }
  return `/operator/accounts/${encodeURIComponent(accountId)}/tenants`;
};

export const GET = createOperatorJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  contentTypeOnGet: false,
  explicitGetMethod: true,
});
export const POST = createOperatorJsonProxy({
  method: "POST",
  path,
  cache: "no-store",
  invalidJsonMessage: "Invalid JSON body",
});
