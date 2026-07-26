import { proxyPost } from "~/lib/route-proxy.server";

interface VacationRequestBody {
  date_start: string;
  date_end: string;
  start_half_day?: boolean;
  end_half_day?: boolean;
  note?: string;
  substitute_staff_id?: number;
}

export const POST = proxyPost<unknown, VacationRequestBody>(
  "/api/time-tracking/vacation/request",
);
