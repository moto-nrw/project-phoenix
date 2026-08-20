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
  | "direct_correction"
  | "enrollment";

/**
 * Eine Direkt-Korrektur der Verwaltung an den Angebots-Buchungen eines Kindes
 * (#2436). Keine Anfrage: kein Status, keine Entscheidung, nur in der
 * Historie.
 */
interface DirectCorrection {
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

/**
 * Eine Anmeldungsänderung (#2435): der Wunsch einer Familie, ihre Anmeldung
 * nach dem Absenden noch zu ändern. Eigener Endpunkt, weil die Freigabe an
 * config:manage hängt statt an users:update.
 */
export interface EnrollmentChangeRequest {
  readonly id: string;
  readonly request_id: string;
  /** "parent" = von der Familie eingereicht, "admin" = Korrektur der OGS. */
  readonly origin: string;
  readonly status: string;
  readonly child_names: readonly string[];
  readonly guardian_name?: string;
  readonly parent_note?: string;
  readonly decision_note?: string;
  /**
   * Die Anmeldung wie eingereicht und wie beantragt, plus die Liste der
   * geänderten Schlüssel — dieselben drei Felder, aus denen die Detailansicht
   * ihren Vergleich baut. Die Zeile zeigt daraus das echte vorher → nachher.
   */
  readonly base_snapshot: Record<string, unknown>;
  readonly proposed_snapshot: Record<string, unknown>;
  readonly diff: Record<string, unknown>;
  readonly created_at: string;
  readonly decided_at?: string;
  readonly decided_by_name?: string;
}

export type AggregatedRequestStatus = "approved" | "rejected" | "withdrawn";

/**
 * Der Zeitpunkt, nach dem alle Quellen des Eltern-Reiters gemeinsam sortiert
 * werden: die Einreichung in der Arbeitsliste, die Entscheidung in der
 * Historie.
 */
interface Occurred {
  readonly occurred_at: string;
}

/** Eine Anmeldungsänderung als Zeile der gemeinsamen Liste. */
export type EnrollmentRequestItem = Occurred & {
  request_type: "enrollment";
  data: EnrollmentChangeRequest;
};

export type AggregatedOpenRequest =
  | (Occurred &
      (
        | { request_type: "master_data"; data: StaffMasterDataChange }
        | { request_type: "care_schedule"; data: StaffCareRequest }
        | { request_type: "offering"; data: StaffOfferingRequest }
        | { request_type: "excused"; data: StaffExcusedRequest }
      ))
  | EnrollmentRequestItem;
// Direkt-Korrekturen fehlen hier bewusst: sie haben keinen offenen Zustand.

export type AggregatedHistoryRequest =
  | (Occurred &
      (
        | { request_type: "master_data"; data: StaffMasterDataHistoryEntry }
        | { request_type: "care_schedule"; data: StaffCareRequestHistoryEntry }
        | { request_type: "offering"; data: StaffOfferingRequestHistoryEntry }
        | { request_type: "excused"; data: StaffExcusedRequestHistoryEntry }
        | { request_type: "direct_correction"; data: DirectCorrection }
      ))
  | EnrollmentRequestItem;

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

/**
 * Zahl der offenen Anmeldungsänderungen für das Badge (#2435). Wie die übrigen
 * Badge-Abrufe: ein Fehler ergibt 0, damit ein Zähler nie die Seitenleiste
 * beschädigt.
 */
export async function fetchPendingEnrollmentChangeRequestCount(): Promise<number> {
  try {
    const response = await fetch(
      "/api/enrollment/admin/change-requests/pending-count",
      { cache: "no-store" },
    );
    if (!response.ok) return 0;
    const envelope = (await response.json()) as Envelope<{
      pending_count?: number;
    }>;
    return envelope.data?.pending_count ?? 0;
  } catch {
    return 0;
  }
}

/**
 * Anmeldungsänderungen, offen oder in der Historie (#2435). Gleiche Antwort-
 * form wie oben, eigener Endpunkt hinter config:manage. Der Art-Filter wirkt
 * hier clientseitig: die Quelle wird schlicht weggelassen, wenn die Art nicht
 * gewählt ist.
 */
export async function listEnrollmentChangeRequests(
  view: "open" | "history",
  params: AggregatedRequestParams = {},
): Promise<AggregatedRequestPage<EnrollmentRequestItem>> {
  // Ohne Art-Filter: diese Quelle kennt nur eine Art.
  const { types: _types, ...rest } = params;
  const response = await fetch(
    `/api/enrollment/admin/change-requests/list${buildQuery(view, rest)}`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    logger.warn("enrollment_change_request_list_load_failed", {
      status: response.status,
      view,
    });
    throw new Error("Anmeldungsänderungen konnten nicht geladen werden.");
  }
  const envelope = (await response.json()) as Envelope<
    AggregatedRequestPage<EnrollmentRequestItem>
  >;
  return envelope.data;
}
