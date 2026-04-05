import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { handleApiError } from "~/lib/api-helpers";
import { getServerApiUrl } from "~/lib/server-api-url";

// GET handler for fetching staff avatar images
// Returns raw image data, not JSON — does not use createGetHandler
export const GET = async (
  _request: NextRequest,
  context: { params: Promise<Record<string, string | string[] | undefined>> },
): Promise<NextResponse> => {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json(
        { error: "Unauthorized", success: false, message: "Unauthorized" },
        { status: 401 },
      );
    }

    const params = await context.params;
    const id = params.id as string;

    if (!id) {
      return NextResponse.json(
        { error: "Staff ID is required", success: false },
        { status: 400 },
      );
    }

    const backendUrl = `${getServerApiUrl()}/api/staff/${id}/avatar`;
    const response = await fetch(backendUrl, {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
      },
    });

    if (!response.ok) {
      return new NextResponse(null, { status: response.status });
    }

    const contentType = response.headers.get("content-type") ?? "image/jpeg";
    const buffer = await response.arrayBuffer();

    return new NextResponse(buffer, {
      headers: {
        "Content-Type": contentType,
        "Cache-Control": "private, max-age=86400",
      },
    });
  } catch (error) {
    return handleApiError(error);
  }
};
