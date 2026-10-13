import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";
import { problemResponse } from "~/lib/api-helpers.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const { token, changeRequestId } = params;
  if (
    typeof token !== "string" ||
    !token ||
    typeof changeRequestId !== "string" ||
    !changeRequestId
  ) {
    return problemResponse(
      400,
      "token and change request id required",
      _request,
    );
  }
  return `/api/enrollment/requests/${encodeURIComponent(token)}/change-requests/${encodeURIComponent(changeRequestId)}/messages`;
};

export const POST = createPublicJsonProxy({
  method: "POST",
  path,
});
