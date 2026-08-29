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
 * Anfragearten (Stammdaten, Betreuungszeiten, Angebote, Abwesenheiten),
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
  | "enrollment"
  | "care_withdrawal";

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
  readonly child_ids?: readonly string[];
  readonly children?: readonly {
    readonly case_id: string;
    readonly student_id?: string;
    readonly name: string;
  }[];
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

/**
 * Stabiler Code, warum eine Anfrage nicht gemeinsam freigegeben werden kann
 * (#2267). Der Text kommt zusätzlich als `bulk_ineligible_text` mit; der Code
 * ist das, wonach die Oberfläche ihre Formulierung wählt.
 */
export type BulkIneligibleReason =
  | "past"
  | "stale"
  | "conflict"
  | "single_only"
  | "child_unavailable"
  | "access_revoked";

export interface RequestReviewMetadata {
  readonly student_id: string;
  readonly student_name: string;
  readonly group_name?: string;
  readonly expected_version: string;
  readonly urgent_today: boolean;
  readonly bulk_eligible: boolean;
  /** Stabiler Code; der Text steht in `bulk_ineligible_text`. */
  readonly bulk_ineligible_reason?: BulkIneligibleReason;
  /** Formulierung des Backends, Rückfallebene für unbekannte Codes. */
  readonly bulk_ineligible_text?: string;
  readonly family_protected: boolean;
  /**
   * Nur bei Abwesenheiten: was für die beantragten Tage aktuell eingetragen
   * ist, je Tag `YYYY-MM-DD` → `present | sick | excused | class_trip`. Das
   * Backend liefert das Feld auf Eintragsebene, nicht in `data`.
   */
  readonly current_status_by_date?: Readonly<Record<string, string>>;
  /**
   * Die Anfrage betrifft nur vergangene Tage. Sie ändert nichts mehr und kann
   * nur noch abgelehnt oder als erledigt markiert werden (#2267).
   */
  readonly past?: boolean;
  /**
   * Schlüssel der Wünsche, die sich mit anderen Anfragen desselben Kindes
   * widersprechen können (ein Tag, ein Wochentag, ein Feld). Vom Backend
   * berechnet — die Oberfläche gruppiert nur noch danach.
   */
  readonly conflict_key?: string;
  /**
   * Wie viele offene Anfragen des Kindes denselben Schlüssel berühren. Beide
   * Felder FEHLEN, wenn es keinen Widerspruch gibt — sie stehen nie auf 1.
   * Gezählt wird über ALLE offenen Anfragen des Kindes, eine davon kann also
   * noch auf einer nicht geladenen Seite liegen. Die Oberfläche darf deshalb
   * nie annehmen, alle Beteiligten schon zu kennen.
   */
  readonly conflict_group_size?: number;
  /**
   * Die OGS hat den Wert nach der Anfrage selbst geändert. Fehlt bei Arten
   * ohne Ausgangswert (Betreuungszeiten, Abholzeit, Angebote).
   */
  readonly current_value_changed?: boolean;
}

/** Eine Anmeldungsänderung als Zeile der gemeinsamen Liste. */
export type EnrollmentRequestItem = Occurred & {
  request_type: "enrollment";
  data: EnrollmentChangeRequest;
};

export type AggregatedOpenRequest =
  | (Occurred &
      RequestReviewMetadata &
      (
        | { request_type: "master_data"; data: StaffMasterDataChange }
        | { request_type: "care_schedule"; data: StaffCareRequest }
        | { request_type: "offering"; data: StaffOfferingRequest }
        | { request_type: "excused"; data: StaffExcusedRequest }
      ))
  | EnrollmentRequestItem;
// Direkt-Korrekturen fehlen hier bewusst: sie haben keinen offenen Zustand.

/**
 * Was an einer entschiedenen Zeile noch möglich ist (#2267): `can_correct`
 * sagt, ob die Person diese Entscheidung korrigieren darf. Die Version wird
 * beim Korrigieren mitgeschickt, damit zwei gleichzeitige Korrekturen sich
 * nicht überschreiben.
 */
interface HistoryDecisionMeta {
  readonly can_correct?: boolean;
  readonly expected_version?: string;
}

export type AggregatedHistoryRequest =
  | (Occurred &
      HistoryDecisionMeta &
      (
        | { request_type: "master_data"; data: StaffMasterDataHistoryEntry }
        | { request_type: "care_schedule"; data: StaffCareRequestHistoryEntry }
        | { request_type: "offering"; data: StaffOfferingRequestHistoryEntry }
        | { request_type: "excused"; data: StaffExcusedRequestHistoryEntry }
        | { request_type: "direct_correction"; data: DirectCorrection }
      ))
  | EnrollmentRequestItem;

/**
 * Was die abrufende Person mit Elternanfragen tun darf (#2267). „none" heißt:
 * sie sieht die Seite, darf aber nichts entscheiden — die Liste erklärt das,
 * statt leer zu bleiben.
 */
export type RequestReviewAccess = "admin" | "group_leader" | "none";

export interface AggregatedRequestPage<T> {
  readonly items: readonly T[];
  /** Fehlt auf der letzten Seite. Nur für denselben Filtersatz gültig. */
  readonly next_cursor?: string;
  /** Nur in der offenen Liste. */
  readonly review_access?: RequestReviewAccess;
}

export interface AggregatedRequestParams {
  readonly search?: string;
  /**
   * Nur die Einträge eines Kindes — das Änderungsprotokoll der Kinderkartei
   * (#2437). Die Anmeldungsänderungen kennen diesen Filter nicht; ihre Quelle
   * lässt ihn weg (siehe listEnrollmentChangeRequests).
   */
  readonly studentId?: string;
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
  if (params.studentId) query.set("student_id", params.studentId);
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
      const body = (await response.json()) as { error?: string; code?: string };
      // Nur das Recht „Abwesenheiten" reicht nicht: ohne „Kinder sehen" darf
      // niemand die Liste öffnen. Das muss dastehen, sonst sieht es aus wie
      // ein Fehler der App (#2267).
      if (body.code === "absence_read_required") {
        message = WRITE_ERROR_MESSAGES.absence_read_required!;
      } else if (body.error) message = body.error;
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
 * Die Anfrage wurde geändert, seit die Liste sie geladen hat (409
 * `change_request_stale`). Eigener Fehlertyp, damit die Oberfläche neu laden
 * kann, statt die Person eine verlorene Entscheidung wiederholen zu lassen
 * (#2267).
 */
export class ChangeRequestStaleError extends Error {
  constructor(
    message = "Die Anfrage wurde inzwischen geändert. Die neue Fassung wird geladen.",
  ) {
    super(message);
    this.name = "ChangeRequestStaleError";
  }
}

/** Die vier Arten, die über den gemeinsamen Lebenszyklus entschieden werden. */
export type ParentRequestKind =
  "master_data" | "care_schedule" | "offering" | "excused";

export interface BulkApproveRequestRef {
  readonly kind: ParentRequestKind;
  readonly id: string;
  readonly expected_version: string;
}

interface ErrorBody {
  readonly error?: string;
  readonly code?: string;
}

/**
 * Liest die Fehlerantwort einer schreibenden Anfrage. Eine veraltete Version
 * wird zum eigenen Fehlertyp; alles andere behält die verständliche Meldung
 * des Backends oder den mitgegebenen Rückfalltext.
 */
async function throwWriteError(
  response: Response,
  fallback: string,
): Promise<never> {
  let body: ErrorBody = {};
  try {
    body = (await response.json()) as ErrorBody;
  } catch {
    // Eine Nicht-JSON-Antwort behält den Rückfalltext.
  }
  if (body.code === "change_request_stale") throw new ChangeRequestStaleError();
  logger.warn("change_request_write_failed", {
    status: response.status,
    ...(body.code ? { code: body.code } : {}),
  });
  throw new Error(
    body.error ?? WRITE_ERROR_MESSAGES[body.code ?? ""] ?? fallback,
  );
}

/** Verständliche Sätze zu den stabilen Fehlercodes des Backends (#2267). */
const WRITE_ERROR_MESSAGES: Record<string, string> = {
  reason_required: "Bitte tragen Sie eine Begründung ein.",
  request_past:
    "Diese Anfrage betrifft nur vergangene Tage. Sie kann nur noch abgelehnt oder als erledigt markiert werden.",
  request_not_past:
    "Diese Anfrage betrifft noch kommende Tage. Bitte entscheiden Sie sie.",
  request_not_decided: "Diese Anfrage ist noch nicht entschieden.",
  correction_unsupported:
    "Diese Entscheidung lässt sich nicht zurücknehmen. Bitte tragen Sie den richtigen Stand direkt ein.",
  absence_read_required:
    "Sie brauchen zusätzlich das Recht „Kinder sehen“, um Elternanfragen zu entscheiden.",
  conflict_kind_unsupported:
    "Für diese Art lässt sich kein gemeinsames Ergebnis festlegen. Bitte entscheiden Sie die Anfragen einzeln.",
  staff_value_unsupported:
    "Für diese Art können Sie keinen eigenen Wert eintragen. Wählen Sie einen der Wünsche oder „Keine Änderung“.",
  staff_value_invalid:
    "Der eingetragene Wert passt nicht. Bitte prüfen Sie ihn und tragen Sie ihn erneut ein.",
};

async function postLifecycle<T>(
  url: string,
  body: unknown,
  fallback: string,
): Promise<T> {
  const response = await fetch(url, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!response.ok) await throwWriteError(response, fallback);
  const envelope = (await response.json()) as Envelope<T>;
  return envelope.data;
}

function lifecycleURL(
  kind: ParentRequestKind,
  requestID: string,
  action: string,
): string {
  return `/api/students/change-requests/${encodeURIComponent(kind)}/${encodeURIComponent(requestID)}/${action}`;
}

/**
 * Schließt eine Anfrage ab, die nur noch vergangene Tage betrifft (#2267).
 * Nichts wird übernommen; die Zeile verschwindet nur aus der Arbeitsliste.
 */
export function markRequestDone(
  kind: ParentRequestKind,
  requestID: string,
  expectedVersion: string,
  reason?: string,
): Promise<unknown> {
  return postLifecycle(
    lifecycleURL(kind, requestID, "mark-done"),
    {
      expected_version: expectedVersion,
      // Der Grund ist freiwillig; ein leerer würde nur eine leere Zeile in
      // der Historie erzeugen.
      ...(reason?.trim() ? { reason: reason.trim() } : {}),
    },
    "Die Anfrage konnte nicht abgeschlossen werden.",
  );
}

/**
 * Nimmt eine bereits gefallene Entscheidung zurück und ersetzt sie (#2267).
 *
 * `kind` ist die Art der WARTESCHLANGE, nicht die des Vorgangs: eine
 * Abholzeit-Änderung wird als `care_schedule` korrigiert, weil sie dort
 * liegt. Das Backend unterscheidet die beiden selbst und antwortet für einen
 * echten Wochenplan mit 409 `correction_unsupported` plus einem deutschen
 * Satz. Der wird unverändert durchgereicht: er sagt genauer, warum es dort
 * nicht geht, als jeder Ersatztext hier.
 */
export function correctRequestDecision(
  kind: ParentRequestKind,
  requestID: string,
  input: Readonly<{
    approve: boolean;
    reason: string;
    expectedVersion: string;
  }>,
): Promise<{ approved: boolean }> {
  return postLifecycle(
    lifecycleURL(kind, requestID, "correct"),
    {
      approve: input.approve,
      reason: input.reason,
      expected_version: input.expectedVersion,
    },
    "Die Korrektur konnte nicht gespeichert werden.",
  );
}

/**
 * Die Art, unter der ein Widerspruch aufgelöst wird. Nicht dasselbe wie die
 * Art der Anfrage: eine Abholzeit-Änderung reist als `care_schedule` durch
 * die Liste, wird aber als `pickup_change` aufgelöst. Maßgeblich ist der
 * Schlüssel der Gruppe.
 */
export type ConflictResolutionKind = ParentRequestKind | "pickup_change";

export function conflictResolutionKind(
  conflictKey: string,
  requestType: ParentRequestKind,
): ConflictResolutionKind {
  return conflictKey.startsWith("pickup:") ? "pickup_change" : requestType;
}

export interface ConflictResolution {
  readonly kind: ConflictResolutionKind;
  /** Der Schlüssel der Gruppe, die aufgelöst wird. */
  readonly conflictKey: string;
  readonly requestIDs: readonly string[];
  readonly expectedVersions: readonly string[];
  /** Genau eine Anfrage freigeben; die übrigen werden abgelehnt. */
  readonly chosenRequestID?: string;
  /**
   * Eigener Wert der OGS: alle Anfragen ablehnen und diesen Wert eintragen.
   * Die Form hängt an der Art (Status, Uhrzeit, Feldwert).
   */
  readonly staffValue?: unknown;
  /** Keine Änderung: alle Anfragen ablehnen. */
  readonly none?: boolean;
  /** Bei einem Widerspruch immer Pflicht, unabhängig von der Einstellung. */
  readonly reason: string;
}

/**
 * Legt für eine Gruppe sich widersprechender Anfragen EIN Ergebnis fest
 * (#2267). Entweder klappt alles zusammen oder nichts. Genau eines von
 * `chosenRequestID`, `staffValue` und `none` wird mitgeschickt.
 */
export async function resolveRequestConflict(
  input: ConflictResolution,
): Promise<number> {
  const result = await postLifecycle<{ resolved_count: number }>(
    "/api/students/change-requests/conflicts/resolve",
    {
      kind: input.kind,
      conflict_key: input.conflictKey,
      request_ids: input.requestIDs,
      expected_versions: input.expectedVersions,
      ...(input.chosenRequestID
        ? { chosen_request_id: input.chosenRequestID }
        : input.staffValue !== undefined
          ? { staff_value: { value: input.staffValue } }
          : { none: true }),
      reason: input.reason,
    },
    "Das Ergebnis konnte nicht gespeichert werden.",
  );
  return result.resolved_count;
}

export async function bulkApproveParentRequests(
  requests: readonly BulkApproveRequestRef[],
  reason: string,
): Promise<number> {
  const response = await fetch("/api/students/change-requests", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ requests, reason }),
  });
  if (!response.ok) {
    let code: string | undefined;
    try {
      code = ((await response.json()) as { code?: string }).code;
    } catch {
      // Eine Nicht-JSON-Antwort behält die verständliche Standardmeldung.
    }
    if (code === "change_request_stale") {
      throw new ChangeRequestStaleError(
        "Mindestens eine Anfrage wurde geändert. Die Liste wird neu geladen.",
      );
    }
    let message = "Die Sammelfreigabe konnte nicht gespeichert werden.";
    if (code === "reason_required") {
      message = "Bitte tragen Sie eine Begründung ein.";
    } else if (code === "bulk_approval_ineligible") {
      message =
        "Mindestens eine Anfrage muss einzeln geprüft werden. Es wurde nichts freigegeben.";
    } else if (response.status === 403) {
      message =
        "Sie dürfen mindestens eine ausgewählte Anfrage nicht entscheiden.";
    }
    throw new Error(message);
  }
  const envelope = (await response.json()) as Envelope<{
    approved_count: number;
  }>;
  return envelope.data.approved_count;
}

export async function setFamilyProtection(
  studentId: string,
  enabled: boolean,
  reason: string,
): Promise<void> {
  const response = await fetch(
    `/api/students/${encodeURIComponent(studentId)}/family-protection`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled, reason }),
    },
  );
  if (!response.ok) {
    throw new Error("Der Familienschutz konnte nicht gespeichert werden.");
  }
}

export interface FamilyProtectionState {
  readonly student_id: string;
  readonly enabled: boolean;
}

export async function getFamilyProtection(
  studentId: string,
): Promise<FamilyProtectionState> {
  const response = await fetch(
    `/api/students/${encodeURIComponent(studentId)}/family-protection`,
    { cache: "no-store" },
  );
  if (!response.ok) {
    throw new Error("Der Familienschutz konnte nicht geladen werden.");
  }
  const envelope = (await response.json()) as Envelope<FamilyProtectionState>;
  return envelope.data;
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
  // Ohne Art-Filter: diese Quelle kennt nur eine Art. Und ohne Kind-Filter:
  // dieser Endpunkt kennt ihn nicht, würde also ungefiltert antworten.
  const { types: _types, studentId: _studentId, ...rest } = params;
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
