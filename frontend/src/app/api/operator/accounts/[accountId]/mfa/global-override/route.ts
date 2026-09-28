import { NextResponse } from "next/server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

// Global MFA emergency override is operator-only, not a school-admin setting.
const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const accountId = params.accountId;
  if (typeof accountId !== "string") {
    return NextResponse.json(
      { error: "Invalid account parameter" },
      { status: 400 },
    );
  }
  return `/operator/accounts/${encodeURIComponent(accountId)}/mfa/global-override`;
};

export const GET = createOperatorJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  contentTypeOnGet: false,
  explicitGetMethod: true,
});
export const PUT = createOperatorJsonProxy({
  method: "PUT",
  path,
  cache: "no-store",
});
