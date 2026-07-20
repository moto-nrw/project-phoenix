// src/app/api/activities/[id]/supervisors/[supervisorId]/route.ts
import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { apiPut, handleApiError } from "~/lib/api-helpers.server";
import { createPutHandler } from "~/lib/route-wrapper.server";
import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import {
  createUnauthorizedResponse,
  encodePathSegment,
  extractParams,
  isStringParam,
  type RouteContext,
} from "~/lib/route-wrapper-utils.server";

function createTokenExpiredResponse(): NextResponse {
  return NextResponse.json(
    { error: "Token expired", code: "TOKEN_EXPIRED" },
    { status: 401 },
  );
}

/**
 * PUT handler for updating a supervisor's role for an activity (primarily is_primary status)
 * @route PUT /api/activities/:id/supervisors/:supervisorId
 * Request body: { is_primary: boolean }
 */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: { is_primary: boolean },
    token: string,
    params: Record<string, unknown>,
  ) => {
    // Extract parameters
    const activityId = String(params.id);
    const supervisorId = String(params.supervisorId);

    if (!activityId) {
      throw new Error("Activity ID is required");
    }

    if (!supervisorId) {
      throw new Error("Supervisor ID is required");
    }

    if (body.is_primary === undefined) {
      throw new Error("is_primary parameter is required");
    }

    await apiPut(
      `/api/activities/${activityId}/supervisors/${supervisorId}`,
      token,
      {
        is_primary: body.is_primary,
      },
    );

    return { success: true };
  },
);

/**
 * DELETE handler for removing a supervisor from an activity
 * @route DELETE /api/activities/:id/supervisors/:supervisorId
 */
export const DELETE = withTenantAuth(
  async (
    request: NextRequest,
    context: RouteContext,
  ): Promise<NextResponse> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return createUnauthorizedResponse();
      }

      const params = await extractParams(request, context);
      const activityId = params.id;
      const supervisorId = params.supervisorId;

      if (!isStringParam(activityId) || activityId.length === 0) {
        throw new Error("Activity ID is required");
      }

      if (!isStringParam(supervisorId) || supervisorId.length === 0) {
        throw new Error("Supervisor ID is required");
      }

      const backendUrl = new URL(
        `${getServerApiUrl()}/api/activities/${encodePathSegment(activityId)}/supervisors/${encodePathSegment(supervisorId)}`,
      );

      const replacementStaffId = request.nextUrl.searchParams.get(
        "replacement_staff_id",
      );
      if (replacementStaffId) {
        backendUrl.searchParams.set("replacement_staff_id", replacementStaffId);
      }

      const makeRequest = (token: string) =>
        fetch(backendUrl.toString(), {
          method: "DELETE",
          headers: {
            Authorization: `Bearer ${token}`,
          },
          cache: "no-store",
        });

      let response = await makeRequest(session.user.token);
      if (response.status === 401) {
        const refreshed = await uncachedAuth();
        if (
          refreshed?.user?.token &&
          refreshed.user.token !== session.user.token
        ) {
          response = await makeRequest(refreshed.user.token);
        } else {
          return createTokenExpiredResponse();
        }
      }

      if (!response.ok) {
        if (response.status === 401) {
          return createTokenExpiredResponse();
        }

        const contentType = response.headers.get("content-type") ?? "";

        if (contentType.includes("application/json")) {
          const payload = (await response.json()) as Record<string, unknown>;
          return NextResponse.json(payload, { status: response.status });
        }

        const errorMessage = await response.text();
        return NextResponse.json(
          { error: errorMessage || `API error (${response.status})` },
          { status: response.status },
        );
      }

      return NextResponse.json({ success: true });
    } catch (error) {
      return handleApiError(error);
    }
  },
);
