import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { handleApiError, problemResponse } from "~/lib/api-helpers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";

// GET handler for fetching avatar images
// Note: This doesn't use createGetHandler because we need to return raw image data
export const GET = withTenantAuth(
  async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> },
  ): Promise<NextResponse> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return problemResponse(401, "Unauthorized", request);
      }

      const params = await context.params;
      const filename = params.filename as string;

      if (!filename) {
        return problemResponse(400, "Filename is required", request);
      }

      // Validate filename to prevent path traversal attacks
      if (
        filename.includes("..") ||
        filename.includes("/") ||
        filename.includes("\\")
      ) {
        return problemResponse(400, "Invalid filename", request);
      }

      // Fetch from backend
      const backendUrl = `${getServerApiUrl()}/api/me/profile/avatar/${filename}`;
      const response = await fetch(backendUrl, {
        headers: {
          Authorization: `Bearer ${session.user.token}`,
        },
      });

      if (!response.ok) {
        return forwardBackendResponse(response, request);
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
      return handleApiError(error, request);
    }
  },
);
