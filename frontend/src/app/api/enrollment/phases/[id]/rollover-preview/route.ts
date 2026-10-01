import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: (request, params) => {
    const bumpsGrade =
      request.nextUrl.searchParams.get("bumps_grade") === "false"
        ? "false"
        : "true";
    return `/api/enrollment/phases/${encodeURIComponent(String(params.id))}/rollover-preview?bumps_grade=${bumpsGrade}`;
  },
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
