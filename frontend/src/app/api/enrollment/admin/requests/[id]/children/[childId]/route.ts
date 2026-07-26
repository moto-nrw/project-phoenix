import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentChildDeleteRoute" });

interface RouteContext {
  params: Promise<{ id: string; childId: string }>;
}

async function DELETEHandler(request: NextRequest, context: RouteContext) {
  const { id, childId } = await context.params;
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const body = (await request.json()) as { reason: string };
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/admin/requests/${encodeURIComponent(id)}/children/${encodeURIComponent(childId)}`,
      {
        method: "DELETE",
        headers: {
          Authorization: `Bearer ${token}`,
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("admin_child_delete_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const DELETE = withTenantAuth(DELETEHandler);
