import { createTenantJsonProxy } from "~/lib/route-wrapper.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request, params) => {
    const bumpsGrade =
      request.nextUrl.searchParams.get("bumps_grade") === "false"
        ? "false"
        : "true";
    return `/api/enrollment/phases/${encodeURIComponent(String(params.id))}/rollover-preview?bumps_grade=${bumpsGrade}`;
  },
});
