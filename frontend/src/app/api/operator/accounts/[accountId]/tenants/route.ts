import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

// Schulzugänge eines Kontos (Issue #1021). Keyed by account, not by school:
// the whole point is seeing every school one account can reach, which only the
// operator may do. The backend response is forwarded verbatim so the modal can
// show the concrete reason a grant was rejected.

async function forward(
  request: NextRequest,
  context: RouteContext,
  method: "GET" | "POST",
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
  if (method === "POST") {
    try {
      body = JSON.stringify(await request.json());
    } catch {
      return NextResponse.json({ error: "Invalid JSON body" }, { status: 400 });
    }
  }

  const makeRequest = async (token: string) =>
    fetch(
      `${getServerApiUrl()}/operator/accounts/${encodeURIComponent(accountId)}/tenants`,
      {
        method,
        headers: {
          Authorization: `Bearer ${token}`,
          ...(method === "POST" ? { "Content-Type": "application/json" } : {}),
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

async function GETHandler(request: NextRequest, context: RouteContext) {
  return forward(request, context, "GET");
}

export const GET = withOperatorAuth(GETHandler);

async function POSTHandler(request: NextRequest, context: RouteContext) {
  return forward(request, context, "POST");
}

export const POST = withOperatorAuth(POSTHandler);
