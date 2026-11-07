import { proxyGet } from "~/lib/parent/route-wrapper.server";

interface BackendParentContractDocument {
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
  replaced?: boolean;
  contract_status?: string;
  ends_on?: string;
}

/**
 * Proxy GET /api/parent/me/documents/contracts → backend
 * /parent/me/documents/contracts (#4678). The care contracts the Träger
 * sent to the calling account's address; the account comes from the parent
 * JWT server-side, never from query or body.
 */
export const GET = proxyGet<BackendParentContractDocument[]>(
  "/parent/me/documents/contracts",
);
