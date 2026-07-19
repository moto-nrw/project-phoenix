import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentAdminDecideRoute" });

// Flattened proxy. The backend handler lives at
// /api/enrollment/admin/requests/{id}/children/{childId}/decide, but
// Turbopack dev (Next.js 16) loses that route's registration after
// its periodic filesystem-cache compaction, returning 404 until the
// dev server restarts. Moving the proxy to a single non-dynamic path
// and reading the IDs from the body sidesteps the bug.
async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

interface DecideBody {
  request_id?: string | number;
  child_id?: string | number;
  status?: string;
  reason?: string;
}

async function POSTHandler(request: NextRequest) {
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  let body: DecideBody;
  try {
    body = (await request.json()) as DecideBody;
  } catch {
    return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });
  }

  const requestID = body.request_id != null ? String(body.request_id) : "";
  const childID = body.child_id != null ? String(body.child_id) : "";
  if (!requestID || !childID) {
    return NextResponse.json(
      { error: "request_id and child_id are required" },
      { status: 400 },
    );
  }

  try {
    const upstream = await fetch(
      `${getServerApiUrl()}/api/enrollment/admin/requests/${encodeURIComponent(
        requestID,
      )}/children/${encodeURIComponent(childID)}/decide`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: authHeader,
        },
        body: JSON.stringify({
          status: body.status,
          reason: body.reason ?? "",
        }),
      },
    );
    const payload = await upstream.json().catch(() => ({}));
    return NextResponse.json(payload, { status: upstream.status });
  } catch (error) {
    logger.error("admin_request_decide_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
