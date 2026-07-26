import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiDelete, handleApiError } from "~/lib/api-helpers.server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";

interface RouteContext {
  params: Promise<{ filename?: string }>;
}

function isStaleTokenError(error: unknown): boolean {
  return error instanceof Error && error.message.includes("API error (401)");
}

function tokenExpiredResponse() {
  return NextResponse.json(
    { error: "Token expired", code: "TOKEN_EXPIRED" },
    { status: 401 },
  );
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
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  const { filename } = await context.params;
  if (!filename) {
    return NextResponse.json({ error: "Invalid filename" }, { status: 400 });
  }

  try {
    const deleted = await deleteWithRetry(
      `/api/enrollment/legal-documents/${encodeURIComponent(filename)}`,
      token,
    );
    if (!deleted) {
      return tokenExpiredResponse();
    }
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    return handleApiError(error);
  }
}

export const DELETE = withTenantAuth(DELETEHandler);
