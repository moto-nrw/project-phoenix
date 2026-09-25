import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request) => {
    const query = new URLSearchParams();
    for (const key of [
      "phase_id",
      "status",
      "care_offering_id",
      "grade_level",
      "day_count",
      "weekday",
      "pickup_time",
      "search",
    ]) {
      const value = request.nextUrl.searchParams.get(key);
      if (value) query.set(key, value);
    }
    for (const value of request.nextUrl.searchParams.getAll(
      "care_offering_ids",
    )) {
      query.append("care_offering_ids", value);
    }
    const suffix = query.toString();
    return `/api/enrollment/admin/reports/care-usage${suffix ? `?${suffix}` : ""}`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
