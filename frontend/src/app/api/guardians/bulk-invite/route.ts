import { proxyPost } from "@/lib/route-proxy.server";

// POST /api/guardians/bulk-invite
// Invite the guardians of many children to the parents portal in one run
// (#3378). With dry_run the backend only counts, nothing is mailed.
export const POST = proxyPost(() => "/api/guardians/bulk-invite");
