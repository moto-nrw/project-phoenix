import { NextResponse } from "next/server";
import { createPublicJsonProxy } from "~/lib/backend-proxy-route.server";

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
    return NextResponse.json(
      { error: "token and change request id required" },
      { status: 400 },
    );
  }
  return `/api/enrollment/requests/${encodeURIComponent(token)}/change-requests/${encodeURIComponent(changeRequestId)}/messages`;
};

export const POST = createPublicJsonProxy({
  method: "POST",
  path,
});
