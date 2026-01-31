import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import type { ApiErrorResponse } from "~/lib/api-helpers";
import { handleApiError } from "~/lib/api-helpers";
import { env } from "~/env";

interface UpdateBreakRequest {
  minutes: number;
}

/**
 * POST /api/time-tracking/break
 * Update break duration for current session (proxies PATCH to backend)
 */
export async function POST(request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" } as ApiErrorResponse, {
        status: 401,
      });
    }

    const body = (await request.json()) as UpdateBreakRequest;

    // Make PATCH request to backend
    const response = await fetch(
      `${env.NEXT_PUBLIC_API_URL}/api/time-tracking/break`,
      {
        method: "PATCH",
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

    const data = (await response.json()) as { data: unknown };
    return NextResponse.json({
      success: true,
      message: "Success",
      data: data.data,
    });
  } catch (error) {
    return handleApiError(error);
  }
}
