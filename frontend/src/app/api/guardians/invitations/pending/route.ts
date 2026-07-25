import { proxyGet } from "@/lib/route-proxy.server";

// GET /api/guardians/invitations/pending
// Open guardian invitations. The guardian list uses it to map a guardian
// profile to its invitation, which is the key the delivery-status endpoint
// takes (#1937).
export const GET = proxyGet("/api/guardians/invitations/pending");
