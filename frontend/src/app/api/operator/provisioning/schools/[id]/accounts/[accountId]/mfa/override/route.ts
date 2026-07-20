import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

async function PUTHandler(request: NextRequest, context: RouteContext) {
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

  const body = JSON.stringify(await request.json());

  const makeRequest = async (token: string) =>
    fetch(
      `${getServerApiUrl()}/operator/schools/${encodeURIComponent(schoolId)}/accounts/${encodeURIComponent(accountId)}/mfa/override`,
      {
        method: "PUT",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
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

  // 204 must not include a body — see comment in the sibling /mfa route.
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

export const PUT = withOperatorAuth(PUTHandler);
