import { proxyGet, proxyPost } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendContractNotice {
  kind: "kuendigung" | "widerruf";
  termination_kind?: "ordentlich" | "sonderkuendigung";
  channel: "eltern_app" | "traeger_buero";
  declared_at: string;
  received_on: string;
  ends_on: string | null;
  recipient: string;
  confirmation_text: string;
}

interface BackendContractNoticeOptions {
  contract_id: number;
  organization_id: number;
  traeger_name: string;
  status: string;
  child_name: string;
  modules: string;
  school_name: string;
  terms: string;
  ordinary_ends_on: string | null;
  special_ends_on: string | null;
  withdrawal_until: string | null;
  early_start: boolean;
  notice: BackendContractNotice | null;
}

interface ContractNoticeBody {
  kind: "kuendigung" | "widerruf";
  termination_kind?: "ordentlich" | "sonderkuendigung";
}

const path = (params: Record<string, unknown>) =>
  `/parent/me/documents/${requirePathSegmentParam(params, "organizationId")}/contracts/${requirePathSegmentParam(params, "contractId")}/notice`;

/**
 * Proxy GET → backend: what the parents can declare on a care contract sent
 * to them today, or the declaration made (#4682). The account comes from
 * the parent JWT server-side; the backend checks the relationship right.
 */
export const GET = proxyGet<BackendContractNoticeOptions>(path);

/**
 * Proxy POST → backend: the parents withdraw or terminate the contract with
 * the button „jetzt kündigen“ (§ 312k BGB) and get the confirmation.
 */
export const POST = proxyPost<BackendContractNoticeOptions, ContractNoticeBody>(
  path,
);
