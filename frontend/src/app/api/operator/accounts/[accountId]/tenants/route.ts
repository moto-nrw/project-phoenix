import { NextResponse } from "next/server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

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
