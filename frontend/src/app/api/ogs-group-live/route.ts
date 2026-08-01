// app/api/ogs-group-live/route.ts
// Proxy for the aggregated OGS group live projection (#2056).
// One backend request returns groups, students, room status, pickup times,
// tracking indicators, and transfers for one supervised group — replacing the
// former /api/ogs-dashboard BFF fan-out (5 backend calls) and the per-group
// client fan-out (4 more). Errors are forwarded as-is: the backend fails the
// whole request instead of degrading sections to empty arrays.
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";
import type { OgsGroupLiveWireResponse } from "~/lib/ogs-group-live-api";

export const GET = createGetHandler<OgsGroupLiveWireResponse>(
  async (request: NextRequest, token: string) => {
    const groupId = request.nextUrl.searchParams.get("group_id");
    const query = groupId ? `?group_id=${encodeURIComponent(groupId)}` : "";
    const response = await apiGet<{ data: OgsGroupLiveWireResponse }>(
      `/api/students/ogs-group-live${query}`,
      token,
    );
    return response.data;
  },
);
