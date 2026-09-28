import { NextResponse } from "next/server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const schoolId = params.id;
  const accountId = params.accountId;
  if (typeof schoolId !== "string" || typeof accountId !== "string") {
    return NextResponse.json(
      { error: "Invalid school or account parameter" },
      { status: 400 },
    );
  }
  return `/operator/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/mfa`;
};

export const GET = createOperatorJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  explicitGetMethod: true,
});
export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path,
  cache: "no-store",
  body: "json",
});
