import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

// /operator/accounts/{accountId}/mfa/global-override is the platform-
// wide ("mailbox lockout emergency switch") surface — distinct from
// /operator/provisioning/schools/{id}/accounts/{accountId}/mfa/override
// which is per-school. Tenant admins intentionally cannot reach this.

async function forward(
  request: NextRequest,
  context: RouteContext,
  method: "GET" | "PUT",
) {
  const session = await operatorAuth();
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const params = await context.params;
  const accountId = params?.accountId;
  if (typeof accountId !== "string") {
    return NextResponse.json(
      { error: "Invalid account parameter" },
      { status: 400 },
    );
  }

  let body: string | undefined;
  if (method === "PUT") {
    body = JSON.stringify(await request.json());
  }

  const makeRequest = async (token: string) =>
    fetch(
      `${getServerApiUrl()}/operator/accounts/${encodeURIComponent(accountId)}/mfa/global-override`,
      {
        method,
        headers: {
          Authorization: `Bearer ${token}`,
          ...(method === "PUT" ? { "Content-Type": "application/json" } : {}),
        },
        body,
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

  if (response.status === 204) {
    return new NextResponse(null, { status: 204 });
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

async function GETHandler(request: NextRequest, context: RouteContext) {
  return forward(request, context, "GET");
}

export const GET = withOperatorAuth(GETHandler);

async function PUTHandler(request: NextRequest, context: RouteContext) {
  return forward(request, context, "PUT");
}

export const PUT = withOperatorAuth(PUTHandler);
