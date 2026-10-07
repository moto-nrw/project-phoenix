import { operatorErrorResponse } from "~/lib/operator/route-wrapper.server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const schoolId = params.id;
  const accountId = params.accountId;
  if (typeof schoolId !== "string" || typeof accountId !== "string") {
    return operatorErrorResponse(
      400,
      "general.input",
      "Invalid school or account parameter",
    );
  }
  return `/operator/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/caregiver-capability`;
};

export const GET = createOperatorJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  explicitGetMethod: true,
});
export const POST = createOperatorJsonProxy({
  method: "POST",
  path,
  cache: "no-store",
});
export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path,
  cache: "no-store",
});
