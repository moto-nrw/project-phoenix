/**
 * Client of the review of a monthly run before its release (#5916): the
 * schools a new run may bill, the invoice date, the exclusion of single
 * invoices and the review list. Backend int64 IDs leave here as strings;
 * calendar dates stay "YYYY-MM-DD".
 */

import { traegerRequest } from "./traeger-api";
import { type BillingFile, billingFile } from "./traeger-billing-api";
import {
  type BackendRunDetail,
  type CareRunDetail,
  detailFromBackend,
} from "./traeger-care-billing-api";

const RUNS_PATH = "/api/traeger/care-billing-runs";

/** A school of the Träger for a new run, with the open run that bills it. */
export interface CareRunSchool {
  readonly id: string;
  readonly name: string;
  /** The open run of the month that already bills the school. */
  readonly openRunId: string | null;
}

export type CareReviewFormat = "xlsx" | "csv";

function runPath(runId: string): string {
  return `${RUNS_PATH}/${encodeURIComponent(runId)}`;
}

/** The schools a new run of `month` ("YYYY-MM-01") may bill. */
export async function fetchCareRunSchools(
  month: string,
): Promise<CareRunSchool[]> {
  const schools = await traegerRequest<
    { id: number; name: string; open_run_id: number | null }[] | null
  >(`${RUNS_PATH}/schools?month=${encodeURIComponent(month)}`);
  return (schools ?? []).map((school) => ({
    id: String(school.id),
    name: school.name,
    openRunId: school.open_run_id === null ? null : String(school.open_run_id),
  }));
}

/** Sets the invoice date of a calculated run; null takes the release day. */
export async function setCareRunInvoiceDate(
  runId: string,
  invoiceDate: string | null,
): Promise<CareRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(`${runPath(runId)}/invoice-date`, {
      method: "PUT",
      body: { invoice_date: invoiceDate },
    }),
  );
}

/** Leaves the invoice of a contract out of the release, with a reason. */
export async function excludeCareInvoice(
  runId: string,
  contractId: string,
  reason: string,
): Promise<CareRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(`${runPath(runId)}/exclusions`, {
      method: "POST",
      body: { contract_id: Number(contractId), reason },
    }),
  );
}

/** Takes an excluded invoice back into the release. */
export async function includeCareInvoice(
  runId: string,
  contractId: string,
): Promise<CareRunDetail> {
  return detailFromBackend(
    await traegerRequest<BackendRunDetail>(
      `${runPath(runId)}/exclusions/${encodeURIComponent(contractId)}`,
      { method: "DELETE" },
    ),
  );
}

/** The review list of the preview, without invoice numbers. */
export async function downloadCareRunReview(
  runId: string,
  format: CareReviewFormat,
): Promise<BillingFile> {
  return billingFile(
    `${runPath(runId)}/review-export?format=${format}`,
    {},
    `pruefliste.${format}`,
  );
}
