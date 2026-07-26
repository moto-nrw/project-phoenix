import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { canonicalForwardedFor } from "~/lib/client-headers.server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentPublicSubmitRoute" });

interface RouteContext {
  params: Promise<{ tenantSlug: string }>;
}

export async function POST(request: NextRequest, context: RouteContext) {
  const { tenantSlug } = await context.params;
  if (!tenantSlug) {
    return NextResponse.json(
      { error: "tenant slug is required" },
      { status: 400 },
    );
  }
  try {
    const body = (await request.json()) as Record<string, unknown>;
    const forwardedFor = canonicalForwardedFor(request.headers);
    // This proxies to the tenant-aware backend API, not static/object storage;
    // encodeURIComponent keeps the slug inside one path segment.
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/${encodeURIComponent(tenantSlug)}/submit`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...(forwardedFor && { "X-Forwarded-For": forwardedFor }),
        },
        body: JSON.stringify(body),
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("public_submit_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
