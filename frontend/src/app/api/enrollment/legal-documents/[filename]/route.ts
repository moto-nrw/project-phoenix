import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import {
  apiDelete,
  handleApiError,
  problemResponse,
} from "~/lib/api-helpers.server";
import { isApiResponseError } from "~/lib/api-response-error.server";
import { createTokenExpiredResponse } from "~/lib/route-wrapper-utils.server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

interface RouteContext {
  params: Promise<{ filename?: string }>;
}

function isStaleTokenError(error: unknown): boolean {
  return isApiResponseError(error, 401);
}

async function deleteWithRetry(endpoint: string, token: string) {
  try {
    await apiDelete(endpoint, token);
    return true;
  } catch (error) {
    if (!isStaleTokenError(error)) {
      throw error;
    }

    const refreshed = await uncachedAuth();
    if (!refreshed?.user?.token || refreshed.user.token === token) {
      return false;
    }

    try {
      await apiDelete(endpoint, refreshed.user.token);
      return true;
    } catch (retryError) {
      if (isStaleTokenError(retryError)) {
        return false;
      }
      throw retryError;
    }
  }
}

async function DELETEHandler(_request: NextRequest, context: RouteContext) {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return problemResponse(401, "Unauthenticated", _request);
  }

  const { filename } = await context.params;
  if (!filename) {
    return problemResponse(400, "Invalid filename", _request);
  }

  try {
    const deleted = await deleteWithRetry(
      `/api/enrollment/legal-documents/${encodeURIComponent(filename)}`,
      token,
    );
    if (!deleted) {
      return createTokenExpiredResponse(_request);
    }
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    return handleApiError(error, _request);
  }
}

export const DELETE = withTenantAuth(DELETEHandler);
