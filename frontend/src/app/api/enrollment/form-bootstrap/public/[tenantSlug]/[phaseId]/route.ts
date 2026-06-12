import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentFormBootstrapRoute" });

interface RouteContext {
  params: Promise<{ tenantSlug: string; phaseId: string }>;
}

export async function GET(_request: NextRequest, context: RouteContext) {
  const { tenantSlug, phaseId } = await context.params;
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/form-bootstrap/public/${encodeURIComponent(
        tenantSlug,
      )}/${encodeURIComponent(phaseId)}`,
      { cache: "no-store" },
    );
    const payload = await response.json().catch(() => ({}));
    if (!response.ok) {
      return NextResponse.json(payload, { status: response.status });
    }

    const session = await auth();
    const token = session?.user?.token;
    if (!token) {
      return NextResponse.json(payload, { status: response.status });
    }

    const profileResponse = await fetch(
      `${getServerApiUrl()}/api/enrollment/me/profile`,
      {
        headers: { Authorization: `Bearer ${token}` },
        cache: "no-store",
      },
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
    logger.error("form_bootstrap_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
