import { proxyGet, proxyPost } from "~/lib/route-wrapper.server";

// Aggregated parent requests across the four kinds, open or history,
// cursor-paginated (#2432).
export const GET = proxyGet("/api/students/change-requests", {
  forwardQuery: [
    "view",
    "search",
    "student_id",
    "types",
    "status",
    "from",
    "to",
    "cursor",
    "limit",
  ],
});

export const POST = proxyPost("/api/students/change-requests/bulk-approve");
