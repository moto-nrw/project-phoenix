import { captureBffException } from "~/lib/sentry-bff.server";
import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { tenantPortal } from "~/server/auth";
import { sendBackend } from "~/lib/backend-client.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "EnrollmentFormBootstrapRoute" });

interface RouteContext {
  params: Promise<{ tenantSlug: string; phaseId: string }>;
}

async function GETHandler(request: NextRequest, context: RouteContext) {
  const { tenantSlug, phaseId } = await context.params;
  try {
    const lateInvite = request.nextUrl.searchParams.get("late_invite")?.trim();
    const query = lateInvite
      ? `?${new URLSearchParams({ late_invite: lateInvite })}`
      : "";
    const response = await sendBackend(
      `/api/enrollment/form-bootstrap/public/${encodeURIComponent(
        tenantSlug,
      )}/${encodeURIComponent(phaseId)}${query}`,
      null,
      { request },
    );
    if (!response.ok) {
      return forwardBackendResponse(response, request);
    }
    const payload = await response.json().catch(() => ({}));

    // The parents host never reaches this route (proxy allow-list), so the
    // tenant session's profile only enriches the tenant's own public form.
    const session = await tenantPortal.auth().catch((error: unknown) => {
      logger.warn("profile_auth_prefetch_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    });
    const token = session?.user?.token;
    if (!token) {
      return NextResponse.json(payload, { status: response.status });
    }

    const profileResponse = await sendBackend(
      "/api/enrollment/me/profile",
      token,
      { request },
    ).catch((error: unknown) => {
      logger.warn("profile_prefetch_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      return null;
    });
    if (!profileResponse?.ok) {
      return NextResponse.json(payload, { status: response.status });
    }

    const profilePayload = await profileResponse.json().catch(() => null);
    const data =
      payload && typeof payload === "object" && "data" in payload
        ? (payload.data as Record<string, unknown>)
        : (payload as Record<string, unknown>);
    return NextResponse.json(
      {
        ...payload,
        data: {
          ...data,
          profile:
            profilePayload &&
            typeof profilePayload === "object" &&
            "data" in profilePayload
              ? profilePayload.data
              : profilePayload,
        },
      },
      { status: response.status },
    );
  } catch (error) {
    captureBffException(error, request);
    logger.error("form_bootstrap_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", request);
  }
}

export const GET = tenantPortal.withAuth(GETHandler);
