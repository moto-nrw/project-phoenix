import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PublicEnrollmentSchemaRoute" });

interface RouteContext {
  params: Promise<{ tenantSlug: string; phaseId: string }>;
}

export async function GET(_request: NextRequest, context: RouteContext) {
  const { tenantSlug, phaseId } = await context.params;
  if (!tenantSlug || !phaseId) {
    return NextResponse.json(
      { error: "tenant slug and phaseId are required" },
      { status: 400 },
    );
  }
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/schema/public/${encodeURIComponent(
        tenantSlug,
      )}/${encodeURIComponent(phaseId)}`,
      { cache: "no-store" },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("public_enrollment_schema_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
