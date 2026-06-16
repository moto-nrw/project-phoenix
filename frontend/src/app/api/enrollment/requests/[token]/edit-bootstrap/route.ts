import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentEditBootstrapRoute" });

interface RouteContext {
  params: Promise<{ token: string }>;
}

export async function GET(_request: NextRequest, context: RouteContext) {
  const { token } = await context.params;
  if (!token) {
    return NextResponse.json({ error: "token required" }, { status: 400 });
  }
  try {
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/requests/${encodeURIComponent(token)}/edit-bootstrap`,
      { cache: "no-store" },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("edit_bootstrap_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
