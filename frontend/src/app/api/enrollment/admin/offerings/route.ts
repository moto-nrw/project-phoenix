import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentAdminOfferingsRoute" });

async function bearerHeader() {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) return null;
  return `Bearer ${token}`;
}

interface UpdateOfferingsBody {
  request_id?: string | number;
  child_id?: string | number;
  offerings?: Array<{
    offering_id?: string | number;
    selected_days?: string[];
  }>;
  reason?: string;
  complete_withdrawal_confirmed?: boolean;
}

async function PUTHandler(request: NextRequest) {
  const authHeader = await bearerHeader();
  if (!authHeader) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  let body: UpdateOfferingsBody;
  try {
    body = (await request.json()) as UpdateOfferingsBody;
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
      )}/children/${encodeURIComponent(childID)}/offerings`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: authHeader,
        },
        body: JSON.stringify({
          offerings: body.offerings ?? [],
          reason: body.reason ?? "",
          complete_withdrawal_confirmed:
            body.complete_withdrawal_confirmed === true,
        }),
      },
    );
    const payload = await upstream.json().catch(() => ({}));
    return NextResponse.json(payload, { status: upstream.status });
  } catch (error) {
    logger.error("admin_child_offerings_update_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const PUT = withTenantAuth(PUTHandler);
