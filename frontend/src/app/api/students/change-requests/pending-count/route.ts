import { proxyGet } from "~/lib/route-wrapper.server";

// Open parent requests (master data + care schedule) for the sidebar badge.
export const GET = proxyGet("/api/students/change-requests/pending-count");
