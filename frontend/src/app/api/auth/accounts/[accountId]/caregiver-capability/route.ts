import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

async function proxyCaregiverCapability(
  request: NextRequest,
  context: RouteContext,
  method: "GET" | "POST" | "DELETE",
) {
  const session = await auth();
  if (!session?.user?.token) {
    return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
  }

  const params = await context.params;
  const accountId = params?.accountId;
  if (typeof accountId !== "string" || accountId.length === 0) {
    return NextResponse.json(
      { error: "Invalid accountId parameter" },
      { status: 400 },
    );
  }

  const requestBody =
    method === "POST" ? JSON.stringify(await request.json()) : undefined;

  const makeRequest = async (token: string) =>
    fetch(
      `${getServerApiUrl()}/auth/accounts/${encodeURIComponent(accountId)}/caregiver-capability`,
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
    const refreshed = await uncachedAuth();
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

export async function GET(request: NextRequest, context: RouteContext) {
  return proxyCaregiverCapability(request, context, "GET");
}

export async function POST(request: NextRequest, context: RouteContext) {
  return proxyCaregiverCapability(request, context, "POST");
}

export async function DELETE(request: NextRequest, context: RouteContext) {
  return proxyCaregiverCapability(request, context, "DELETE");
}
