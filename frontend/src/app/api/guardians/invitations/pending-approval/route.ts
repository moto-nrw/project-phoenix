import { proxyGet } from "~/lib/route-wrapper.server";

// GET /api/guardians/invitations/pending-approval
// Staff approval queue: parent-initiated invites awaiting approval.
export const GET = proxyGet("/api/guardians/invitations/pending-approval");
