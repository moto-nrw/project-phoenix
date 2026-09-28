import { NextResponse } from "next/server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const { accountId, tenantId } = params;
  if (typeof accountId !== "string" || typeof tenantId !== "string") {
    return NextResponse.json(
      { error: "Invalid account or school parameter" },
      { status: 400 },
    );
  }
  return `/operator/accounts/${encodeURIComponent(accountId)}/tenants/${encodeURIComponent(tenantId)}`;
};

export const PUT = createOperatorJsonProxy({
  method: "PUT",
  path,
  cache: "no-store",
  invalidJsonMessage: "Invalid JSON body",
});
export const DELETE = createOperatorJsonProxy({
  method: "DELETE",
  path,
  cache: "no-store",
  contentTypeOnDelete: false,
});
