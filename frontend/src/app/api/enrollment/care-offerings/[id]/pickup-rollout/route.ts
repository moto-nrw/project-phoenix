import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareOfferingPickupRolloutRoute" });

interface RouteContext {
  params: Promise<{ id: string }>;
}

async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

// GET = Rollout-Vorschau für den Bestätigungsdialog (#2290)
async function GETHandler(_request: NextRequest, context: RouteContext) {
  const { id } = await context.params;
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/care-offerings/${encodeURIComponent(id)}/pickup-rollout`,
      { headers: { Authorization: authHeader }, cache: "no-store" },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("care_offering_pickup_preview_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);

// POST = Rollout ausführen; skip_student_ids bleiben unangetastet
async function POSTHandler(request: NextRequest, context: RouteContext) {
  const { id } = await context.params;
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const body = (await request.json()) as { skip_student_ids?: string[] };
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/care-offerings/${encodeURIComponent(id)}/pickup-rollout`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: authHeader,
        },
        body: JSON.stringify({
          skip_student_ids: Array.isArray(body.skip_student_ids)
            ? body.skip_student_ids
            : [],
        }),
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("care_offering_pickup_rollout_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
