import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { handleApiError } from "~/lib/api-helpers";
import { env } from "~/env";

// GET handler for fetching avatar images
// Note: This doesn't use createGetHandler because we need to return raw image data
export const GET = async (
  request: NextRequest,
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
    const filename = params.filename as string;

    if (!filename) {
      return NextResponse.json(
        {
          error: "Filename is required",
          success: false,
          message: "Filename is required",
        },
        { status: 400 },
      );
    }

    // Validate filename to prevent path traversal attacks
    if (
      filename.includes("..") ||
      filename.includes("/") ||
      filename.includes("\\")
    ) {
      return NextResponse.json(
        {
          error: "Invalid filename",
          success: false,
          message: "Invalid filename",
        },
        { status: 400 },
      );
    }

    // Fetch from backend
    const backendUrl = `${env.NEXT_PUBLIC_API_URL}/api/me/profile/avatar/${filename}`;
    const response = await fetch(backendUrl, {
      headers: {
        Authorization: `Bearer ${session.user.token}`,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      return NextResponse.json(
        {
          error: errorText || `Failed to fetch avatar: ${response.status}`,
          success: false,
          message: errorText || `Failed to fetch avatar: ${response.status}`,
        },
        { status: response.status },
      );
    }

    // Get the image data and content type
    const contentType = response.headers.get("content-type") ?? "image/jpeg";
    const buffer = await response.arrayBuffer();

    // Return raw image data with proper headers
    return new NextResponse(buffer, {
      headers: {
        "Content-Type": contentType,
        "Cache-Control": "private, max-age=86400", // Cache for 1 day
      },
    });
  } catch (error) {
    return handleApiError(error);
  }
};
