import { proxyGet } from "@/lib/route-proxy.server";

// GET /api/guardians/invitations/[invitationId]/delivery
// Per-invitation email delivery status (#1937): one entry per dispatch
// attempt, separating "handed to the mail server" from "arrived".
export const GET = proxyGet(
  (params) =>
    `/api/guardians/invitations/${String(params.invitationId)}/delivery`,
);
