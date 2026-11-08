import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/operator/route-wrapper.server";

// Global MFA emergency override is operator-only, not a school-admin setting.
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
  return `/operator/accounts/${encodeURIComponent(accountId)}/mfa/global-override`;
};

export const GET = createOperatorJsonProxy({
  method: "GET",
  path,
});
export const PUT = createOperatorJsonProxy({
  method: "PUT",
  path,
});
