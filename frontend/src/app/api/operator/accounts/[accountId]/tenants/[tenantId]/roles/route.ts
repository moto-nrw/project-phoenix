import { NextResponse } from "next/server";
import { createOperatorJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createOperatorJsonProxy({
  method: "GET",
  path: (_request, params) => {
    const { accountId, tenantId } = params;
    if (typeof accountId !== "string" || typeof tenantId !== "string") {
      return NextResponse.json(
        { error: "Invalid account or school parameter" },
        { status: 400 },
      );
    }
    return `/operator/accounts/${encodeURIComponent(accountId)}/tenants/${encodeURIComponent(tenantId)}/roles`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
});
