import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { handleApiError } from "~/lib/api-helpers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import {
  encodePathSegment,
  isStringParam,
} from "~/lib/route-wrapper-utils.server";

// GET handler for fetching staff avatar images
// Returns raw image data, not JSON — does not use createGetHandler
export const GET = withTenantAuth(
  async (
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
      const rawId = params.id;

      if (!isStringParam(rawId) || !/^\d+$/.test(rawId)) {
        return NextResponse.json(
          { error: "Valid staff ID is required", success: false },
          { status: 400 },
        );
      }

      const backendUrl = `${getServerApiUrl()}/api/staff/${encodePathSegment(rawId)}/avatar`;
      const makeRequest = (token: string) =>
        fetch(backendUrl, {
          cache: "no-store",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

      let response = await makeRequest(session.user.token);
      if (response.status === 401) {
        const refreshed = await uncachedAuth();
        if (
          refreshed?.user?.token &&
          refreshed.user.token !== session.user.token
        ) {
          response = await makeRequest(refreshed.user.token);
        }
      }

      if (!response.ok) {
        return NextResponse.json(
          { error: "Avatar not found", success: false },
          { status: response.status },
        );
      }

      const contentType = response.headers.get("content-type") ?? "image/jpeg";
      const buffer = await response.arrayBuffer();

      return new NextResponse(buffer, {
        headers: {
          "Content-Type": contentType,
          "Cache-Control": "private, max-age=300",
        },
      });
    } catch (error) {
      return handleApiError(error);
    }
  },
);
