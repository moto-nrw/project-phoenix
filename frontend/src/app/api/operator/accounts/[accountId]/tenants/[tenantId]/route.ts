import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

// Rolle ändern (PUT) und Zugang entziehen (DELETE) für eine einzelne Schule
// eines Kontos (Issue #1021).

async function forward(
  request: NextRequest,
  context: RouteContext,
  method: "PUT" | "DELETE",
) {
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

  const body =
    method === "PUT" ? JSON.stringify(await request.json()) : undefined;

  const makeRequest = async (token: string) =>
    fetch(
      `${getServerApiUrl()}/operator/accounts/${encodeURIComponent(accountId)}/tenants/${encodeURIComponent(tenantId)}`,
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

  const text = await response.text();
  return new NextResponse(text, {
    status: response.status,
    headers: {
      "Content-Type":
        response.headers.get("Content-Type") ?? "application/json",
    },
  });
}

async function PUTHandler(request: NextRequest, context: RouteContext) {
  return forward(request, context, "PUT");
}

export const PUT = withOperatorAuth(PUTHandler);

async function DELETEHandler(request: NextRequest, context: RouteContext) {
  return forward(request, context, "DELETE");
}

export const DELETE = withOperatorAuth(DELETEHandler);
