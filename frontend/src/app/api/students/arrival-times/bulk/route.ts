import { proxyPost } from "@/lib/route-proxy.server";

// POST /api/students/arrival-times/bulk - Get effective arrival times for multiple students
export const POST = proxyPost("/api/students/arrival-times/bulk");
