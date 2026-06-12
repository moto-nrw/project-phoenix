import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PublicEnrollmentLegalRoute" });

interface RouteContext {
  params: Promise<{ tenantSlug: string }>;
}

export async function GET(_request: NextRequest, context: RouteContext) {
  const { tenantSlug } = await context.params;
  if (!tenantSlug) {
    return NextResponse.json(
      { error: "tenant slug is required" },
      { status: 400 },
    );
  }
  try {
    const phaseId = _request.nextUrl.searchParams.get("phaseId");
    const backendPath = phaseId
      ? `/api/enrollment/legal/${encodeURIComponent(
          tenantSlug,
        )}/${encodeURIComponent(phaseId)}`
      : `/api/enrollment/legal/${encodeURIComponent(tenantSlug)}`;
    const response = await fetch(`${getServerApiUrl()}${backendPath}`, {
      cache: "no-store",
    });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("public_enrollment_legal_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
