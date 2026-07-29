import { NextResponse } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

// Assignable system and school-specific roles for one operator-managed access.
async function GETHandler(_request: Request, context: RouteContext) {
  const session = await operatorAuth();
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const params = await context.params;
  const accountId = params?.accountId;
  const tenantId = params?.tenantId;
  if (typeof accountId !== "string" || typeof tenantId !== "string") {
    return NextResponse.json(
      { error: "Invalid account or school parameter" },
      { status: 400 },
    );
  }

  const makeRequest = (token: string) =>
    fetch(
      `${getServerApiUrl()}/operator/accounts/${encodeURIComponent(accountId)}/tenants/${encodeURIComponent(tenantId)}/roles`,
      {
        headers: { Authorization: `Bearer ${token}` },
        cache: "no-store",
      },
    );

  let response = await makeRequest(session.user.token);
  if (response.status === 401) {
    const refreshed = await uncachedOperatorAuth();
    if (refreshed?.user?.token && refreshed.user.token !== session.user.token) {
      response = await makeRequest(refreshed.user.token);
    }
  }

  const text = await response.text();
  return new NextResponse(text, {
    status: response.status,
    headers: {
      "Content-Type":
        response.headers.get("Content-Type") ?? "application/json",
    },
  });
}

export const GET = withOperatorAuth(GETHandler);
