// Monatsabschluss (#1417): GET lists the active snapshots of one month for
// the whole school ([] = not closed), POST freezes the month school-wide in
// one transaction. Both gated on time_tracking:manage backend-side.
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { proxyGet } from "~/lib/route-proxy.server";
import { createPostHandler } from "~/lib/route-wrapper.server";
import type { BackendMonthCloseSnapshot } from "~/lib/time-tracking-helpers";

/** GET /api/staff/time-tracking/month-close?year=&month= */
export const GET = proxyGet<BackendMonthCloseSnapshot[]>(
  "/api/staff/time-tracking/month-close",
);

// Explicit allowlist: only these fields reach the backend.
interface MonthCloseBody {
  year?: number;
  month?: number;
  reason?: string;
}

/** POST /api/staff/time-tracking/month-close */
export const POST = createPostHandler<unknown, MonthCloseBody>(
  async (_request: NextRequest, body: MonthCloseBody, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/staff/time-tracking/month-close",
      token,
      {
        year: body.year,
        month: body.month,
        reason: body.reason,
      },
    );
    return response.data;
  },
);
