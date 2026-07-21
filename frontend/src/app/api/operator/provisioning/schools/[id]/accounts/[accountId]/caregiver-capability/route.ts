import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

async function proxyOperatorCaregiverCapability(
  request: NextRequest,
  context: RouteContext,
  method: "GET" | "POST" | "DELETE",
) {
  const session = await operatorAuth();
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const params = await context.params;
  const schoolId = params?.id;
  const accountId = params?.accountId;

  if (typeof schoolId !== "string" || typeof accountId !== "string") {
    return NextResponse.json(
      { error: "Invalid school or account parameter" },
      { status: 400 },
    );
  }

  const requestBody =
    method === "POST" ? JSON.stringify(await request.json()) : undefined;

  const makeRequest = async (token: string) =>
    fetch(
      `${getServerApiUrl()}/operator/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
      {
        method,
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: requestBody,
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
  return proxyOperatorCaregiverCapability(request, context, "GET");
}

export const GET = withOperatorAuth(GETHandler);

async function POSTHandler(request: NextRequest, context: RouteContext) {
  return proxyOperatorCaregiverCapability(request, context, "POST");
}

export const POST = withOperatorAuth(POSTHandler);

async function DELETEHandler(request: NextRequest, context: RouteContext) {
  return proxyOperatorCaregiverCapability(request, context, "DELETE");
}

export const DELETE = withOperatorAuth(DELETEHandler);
