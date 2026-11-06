import { proxyGet } from "~/lib/route-proxy.server";

// student_id asks whether the review scope reaches one child (#3886).
export const GET = proxyGet("/api/students/change-requests/access", {
  forwardQuery: ["student_id"],
});
