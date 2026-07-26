import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PublicCareOfferingsRoute" });

interface RouteContext {
  params: Promise<{ tenantSlug: string; phaseId: string }>;
}

export async function GET(request: NextRequest, context: RouteContext) {
  const { tenantSlug, phaseId } = await context.params;
  if (!tenantSlug || !phaseId) {
    return NextResponse.json(
      { error: "tenant slug and phaseId are required" },
      { status: 400 },
    );
  }
  try {
    const lateInvite = request.nextUrl.searchParams.get("late_invite")?.trim();
    const backendUrl = new URL(
      `${getServerApiUrl()}/api/enrollment/care-offerings/public/${encodeURIComponent(
        tenantSlug,
      )}/${encodeURIComponent(phaseId)}`,
    );
    if (lateInvite) backendUrl.searchParams.set("late_invite", lateInvite);
    const response = await fetch(backendUrl, { cache: "no-store" });
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("public_care_offerings_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
