import { createLogger } from "~/lib/logger";
import type {
  StaffCareRequest,
  StaffCareRequestHistoryEntry,
} from "~/lib/care-request-review-api";
import type {
  StaffExcusedRequest,
  StaffExcusedRequestHistoryEntry,
} from "~/lib/excused-request-review-api";
import type {
  StaffMasterDataChange,
  StaffMasterDataHistoryEntry,
} from "~/lib/master-data-review-api";
import type {
  StaffOfferingRequest,
  StaffOfferingRequestHistoryEntry,
} from "~/lib/offering-request-review-api";

const logger = createLogger({ component: "ChangeRequestListApi" });

/**
 * Aggregierte Eltern-Anfragenliste (#2432): ein Endpunkt über die vier
 * Anfragearten (Stammdaten, Betreuungszeiten, Angebote, Entschuldigungen),
 * offen oder Historie, mit serverseitiger Namenssuche, Filtern und
 * Keyset-Pagination. Jedes Item trägt unter `data` die unveränderte
 * Art-Projektion, diskriminiert über `request_type` — die bestehenden
 * Entscheiden-Aufrufe der Art-APIs bleiben unverändert.
 */

export type AggregatedRequestType =
  | "master_data"
  | "care_schedule"
  | "offering"
  | "excused"
  | "direct_correction";

/**
 * Eine Direkt-Korrektur der Verwaltung an den Angebots-Buchungen eines Kindes
 * (#2436). Keine Anfrage: kein Status, keine Entscheidung, nur in der
 * Historie.
 */
export interface DirectCorrection {
  readonly id: string;
  readonly student_id: string;
  readonly student_name: string;
  readonly changed_at: string;
  readonly changed_by_name: string;
  readonly reason: string;
  readonly diff: readonly {
    readonly offering_id: string;
    readonly label: string;
    readonly old: string;
    readonly new: string;
  }[];
}

export type AggregatedRequestStatus = "approved" | "rejected" | "withdrawn";

export type AggregatedOpenRequest =
  | { request_type: "master_data"; data: StaffMasterDataChange }
  | { request_type: "care_schedule"; data: StaffCareRequest }
  | { request_type: "offering"; data: StaffOfferingRequest }
  | { request_type: "excused"; data: StaffExcusedRequest };
// Direkt-Korrekturen fehlen hier bewusst: sie haben keinen offenen Zustand.

export type AggregatedHistoryRequest =
  | { request_type: "master_data"; data: StaffMasterDataHistoryEntry }
  | { request_type: "care_schedule"; data: StaffCareRequestHistoryEntry }
  | { request_type: "offering"; data: StaffOfferingRequestHistoryEntry }
  | { request_type: "excused"; data: StaffExcusedRequestHistoryEntry }
  | { request_type: "direct_correction"; data: DirectCorrection };

export interface AggregatedRequestPage<T> {
  readonly items: readonly T[];
  /** Fehlt auf der letzten Seite. Nur für denselben Filtersatz gültig. */
  readonly next_cursor?: string;
}

export interface AggregatedRequestParams {
  readonly search?: string;
  readonly types?: readonly AggregatedRequestType[];
  /** Nur Historie. */
  readonly statuses?: readonly AggregatedRequestStatus[];
  /** Nur Historie, YYYY-MM-DD. */
  readonly from?: string;
  /** Nur Historie, YYYY-MM-DD. */
  readonly to?: string;
  readonly cursor?: string;
}

interface Envelope<T> {
  data: T;
}

function buildQuery(
  view: "open" | "history",
  params: AggregatedRequestParams,
): string {
  const query = new URLSearchParams();
  query.set("view", view);
  if (params.search?.trim()) query.set("search", params.search.trim());
  if (params.types && params.types.length > 0)
    query.set("types", params.types.join(","));
  if (params.statuses && params.statuses.length > 0)
    query.set("status", params.statuses.join(","));
  if (params.from) query.set("from", params.from);
  if (params.to) query.set("to", params.to);
  if (params.cursor) query.set("cursor", params.cursor);
  return `?${query.toString()}`;
}

async function fetchPage<T>(
  view: "open" | "history",
  params: AggregatedRequestParams,
): Promise<AggregatedRequestPage<T>> {
  const response = await fetch(
    `/api/students/change-requests${buildQuery(view, params)}`,
    {
      cache: "no-store",
    },
  );
  if (!response.ok) {
    let message = "Anfragen konnten nicht geladen werden.";
    try {
      const body = (await response.json()) as { error?: string };
      if (body.error) message = body.error;
    } catch {
      // Nicht-JSON-Fehlerantworten behalten die generische Meldung.
    }
    logger.warn("change_request_list_load_failed", {
      status: response.status,
      view,
    });
    throw new Error(message);
  }
  const envelope = (await response.json()) as Envelope<
    AggregatedRequestPage<T>
  >;
  return envelope.data;
}

export function listAggregatedOpenRequests(
  params: AggregatedRequestParams = {},
): Promise<AggregatedRequestPage<AggregatedOpenRequest>> {
  return fetchPage<AggregatedOpenRequest>("open", params);
}

export function listAggregatedRequestHistory(
  params: AggregatedRequestParams = {},
): Promise<AggregatedRequestPage<AggregatedHistoryRequest>> {
  return fetchPage<AggregatedHistoryRequest>("history", params);
}
