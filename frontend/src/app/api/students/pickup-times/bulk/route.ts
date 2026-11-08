import { proxyPost } from "~/lib/route-wrapper.server";

// POST /api/students/pickup-times/bulk - Get effective pickup times for multiple students
export const POST = proxyPost("/api/students/pickup-times/bulk");
