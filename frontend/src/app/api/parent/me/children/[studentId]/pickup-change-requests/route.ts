import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendPickupChangeRequest {
  id: string;
  date: string;
  pickup_time: string;
  reason: string;
  status: string;
  decision_reason?: string;
  created_at: string;
  reviewed_at?: string;
}

export const GET = proxyGet<BackendPickupChangeRequest[]>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/pickup-change-requests`,
);
