import { proxyGet } from "~/lib/parent/route-wrapper.server";

/**
 * Proxy GET /api/parent/me/documents/contracts → backend
 * /parent/me/documents/contracts (#4678). The care contracts the Träger
 * sent to the calling account's address; the account comes from the parent
 * JWT server-side, never from query or body. The wire shape lives in
 * `ParentDocument` (`~/lib/parent-api`).
 */
export const GET = proxyGet<unknown>("/parent/me/documents/contracts");
