import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, getCookieHeader } from "~/server/auth";
import { apiGet } from "~/lib/api-client";
import { handleApiError } from "~/lib/api-helpers";

export async function GET(
  _request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
): Promise<NextResponse> {
  try {
    const session = await auth();

    if (!session?.user) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    const cookieHeader = await getCookieHeader();
    const params = await context.params;
    const groupId = params?.id;

    if (!groupId || typeof groupId !== "string") {
      return NextResponse.json(
        { error: "Group ID is required" },
        { status: 400 },
      );
    }

    // Call backend endpoint to get students in the group
    const response = await apiGet(
      `/api/groups/${groupId}/students`,
      cookieHeader,
    );

    // The backend returns a wrapped response with status, data, and message
    // Extract the data array and return it directly
    if (response && typeof response === "object" && "data" in response) {
      return NextResponse.json(response.data);
    }

    // If response is already an array, return it directly
    return NextResponse.json(response);
  } catch (error) {
    return handleApiError(error);
  }
}
