import { proxyGet, proxyPost } from "@/lib/route-proxy.server";

// GET /api/guardians - List guardians
export const GET = proxyGet("/api/guardians", { raw: true });

// POST /api/guardians - Create guardian
export const POST = proxyPost("/api/guardians");
