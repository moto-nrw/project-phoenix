import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentAdminDataCorrectionRoute",
});

interface DataCorrectionBody {
  request_id?: string | number;
  child_id?: string | number;
  first_name?: string;
  last_name?: string;
  date_of_birth?: string;
  target_grade_level?: number;
  target_school_class?: string;
  reason?: string;
}

async function PUTHandler(request: NextRequest) {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }

  let body: DataCorrectionBody;
  try {
    body = (await request.json()) as DataCorrectionBody;
  } catch {
    return NextResponse.json({ error: "Invalid JSON" }, { status: 400 });
  }
  const requestID = body.request_id == null ? "" : String(body.request_id);
  const childID = body.child_id == null ? "" : String(body.child_id);
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
      )}/children/${encodeURIComponent(childID)}/data-correction`,
      {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          first_name: body.first_name,
          last_name: body.last_name,
          date_of_birth: body.date_of_birth,
          target_grade_level: body.target_grade_level,
          target_school_class: body.target_school_class,
          reason: body.reason,
        }),
      },
    );
    const payload = await upstream.json().catch(() => ({}));
    return NextResponse.json(payload, { status: upstream.status });
  } catch (error) {
    logger.error("admin_child_data_correction_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const PUT = withTenantAuth(PUTHandler);
