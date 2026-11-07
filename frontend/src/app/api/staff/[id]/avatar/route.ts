import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";
import { auth, uncachedAuth, withTenantAuth } from "~/server/auth";
import { handleApiError, problemResponse } from "~/lib/api-helpers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import {
  encodePathSegment,
  isStringParam,
} from "~/lib/route-wrapper-utils.server";

// GET handler for fetching staff avatar images
// Returns raw image data, not JSON — does not use createGetHandler

// `staffAvatarUrl` adds `?v=<stored filename>`. Each upload gets a new
// filename, so a versioned address never changes its bytes and the browser
// may keep it without revalidation. Unversioned addresses stay short-lived.
const VERSIONED_CACHE_CONTROL = "private, max-age=86400, immutable";
const UNVERSIONED_CACHE_CONTROL = "private, max-age=300";
export const GET = withTenantAuth(
  async (
    _request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> },
  ): Promise<NextResponse> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return problemResponse(401, "Unauthorized", _request);
      }

      const params = await context.params;
      const rawId = params.id;

      if (!isStringParam(rawId) || !/^\d+$/.test(rawId)) {
        return problemResponse(400, "Valid staff ID is required", _request);
      }

      const backendUrl = `${getServerApiUrl()}/api/staff/${encodePathSegment(rawId)}/avatar`;
      const makeRequest = (token: string) =>
        fetch(backendUrl, {
          cache: "no-store",
          headers: {
            Authorization: `Bearer ${token}`,
          },
        });

      const response = await sendWithTokenRefresh(
        session.user.token,
        makeRequest,
        () => uncachedAuth(),
      );

      if (!response.ok) {
        return forwardBackendResponse(response, _request);
      }

      const contentType = response.headers.get("content-type") ?? "image/jpeg";
      const buffer = await response.arrayBuffer();

      return new NextResponse(buffer, {
        headers: {
          "Content-Type": contentType,
          "Cache-Control": _request.nextUrl.searchParams.get("v")
            ? VERSIONED_CACHE_CONTROL
            : UNVERSIONED_CACHE_CONTROL,
        },
      });
    } catch (error) {
      return handleApiError(error, _request);
    }
  },
);
