import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareOfferingBookingStatsRoute" });

async function GETHandler(request: NextRequest) {
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const phase = request.nextUrl.searchParams.get("phase_id");
    if (!phase) {
      return NextResponse.json(
        { error: "phase_id is required" },
        { status: 400 },
      );
    }
    const url = new URL(
      `${getServerApiUrl()}/api/enrollment/care-offerings/booking-stats`,
    );
    url.searchParams.set("phase_id", phase);

    const response = await fetch(url, {
      headers: { Authorization: `Bearer ${token}` },
      cache: "no-store",
    });
    const payload = (await response.json().catch(() => ({}))) as unknown;
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("care_offering_booking_stats_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const GET = withTenantAuth(GETHandler);
