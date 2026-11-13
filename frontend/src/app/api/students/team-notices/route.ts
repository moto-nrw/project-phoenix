import { proxyGet } from "~/lib/route-wrapper.server";

// GET /api/students/team-notices - Notizen, die das Team mit mir geteilt hat
// und die ich noch nicht gelesen habe (#6854)
export const GET = proxyGet("/api/students/team-notices");
