// Settings value by key API route
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiGet, apiPut, handleApiError } from "~/lib/api-helpers";
import {
  createGetHandler,
  createPutHandler,
  isStringParam,
} from "~/lib/route-wrapper";
import { auth } from "~/server/auth";
import { env } from "~/env";

export const GET = createGetHandler(
  async (request: NextRequest, token: string, params) => {
    if (!isStringParam(params.key)) {
      throw new Error("Invalid key parameter");
    }

    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });
    const queryString = queryParams.toString();
    const endpoint = `/api/settings/values/${encodeURIComponent(params.key)}${queryString ? `?${queryString}` : ""}`;

    return await apiGet(endpoint, token);
  },
);

interface SetValueBody {
  value: string;
  scope: string;
  scope_id?: number;
}

export const PUT = createPutHandler<unknown, SetValueBody>(
  async (_request: NextRequest, body: SetValueBody, token: string, params) => {
    if (!isStringParam(params.key)) {
      throw new Error("Invalid key parameter");
    }

    return await apiPut(
      `/api/settings/values/${encodeURIComponent(params.key)}`,
      token,
      body,
    );
  },
);

interface DeleteValueBody {
  scope: string;
  scope_id?: number;
}

// DELETE with body requires custom implementation since apiDelete doesn't support body
export async function DELETE(
  request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
) {
  try {
    const session = await auth();
    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const params = await context.params;
    const key = params.key;
    if (typeof key !== "string") {
      return NextResponse.json(
        { error: "Invalid key parameter" },
        { status: 400 },
      );
    }

    // Parse body for DELETE request
    let body: DeleteValueBody = { scope: "system" };
    try {
      const text = await request.text();
      if (text) {
        body = JSON.parse(text) as DeleteValueBody;
      }
    } catch {
      // Use default body
    }

    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/api/settings/values/${encodeURIComponent(key)}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${session.user.token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      },
    );

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`API error (${response.status}): ${errorText}`);
    }

    return new NextResponse(null, { status: 204 });
  } catch (error) {
    return handleApiError(error);
  }
}
