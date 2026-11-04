import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendParentDocument {
  id: string;
  organization_id: string;
  traeger_name: string;
  kind: string;
  number: string;
  period_name: string;
  service_from: string;
  service_until: string;
  issued_on: string;
  total_cents: number;
  child_names: string[];
  sent_at: string;
}

/**
 * Proxy GET /api/parent/me/documents → backend /parent/me/documents (#4671).
 * The documents the Träger sent to the calling account's address; the
 * account comes from the parent JWT server-side, never from query or body.
 */
export const GET = proxyGet<BackendParentDocument[]>("/parent/me/documents");
