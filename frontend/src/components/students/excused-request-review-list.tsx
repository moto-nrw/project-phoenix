"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Loading } from "~/components/ui/loading";
import {
  RequestReviewCard,
  ReviewDiffPanel,
} from "~/components/students/request-review-card";
import { createLogger } from "~/lib/logger";
import { formatDate } from "~/lib/date-helpers";
import {
  ExcusedRequestApiError,
  type StaffExcusedRequest,
  decideExcusedAbsenceRequest,
  listExcusedAbsenceRequests,
} from "~/lib/excused-request-review-api";

const logger = createLogger({ component: "ExcusedRequestReviewList" });

// The only expected 409 here: the parent withdrew the request, or another
// staffer already decided it, between load and submit. Name the recovery
// (reload) instead of a generic failure.
function decideErrorMessage(code: string | undefined): string {
  switch (code) {
    case "change_request_not_pending":
      return "Diese Anfrage wurde bereits entschieden oder von den Eltern zurückgezogen. Bitte die Seite neu laden.";
    default:
      return "Die Entscheidung konnte nicht gespeichert werden.";
  }
}

// One-line summary for the collapsed card: the covered day (or day range).
function datesSummary(dates: StaffExcusedRequest["dates"]): string {
  if (dates.length === 0) return "Entschuldigung";
  const sorted = [...dates].sort((a, b) => a.localeCompare(b));
  const first = sorted.at(0)!;
  const last = sorted.at(-1)!;
  if (sorted.length === 1) return formatDate(first);
  return `${formatDate(first)} – ${formatDate(last)}`;
}

// German-only staff UI — hardcoded strings like CareRequestReviewList (the staff
// shell ships no full message catalog).
export function ExcusedRequestReviewList() {
  const [rows, setRows] = useState<StaffExcusedRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [reasonErrors, setReasonErrors] = useState<Record<string, boolean>>({});
  const [notice, setNotice] = useState<string | null>(null);
  // Set while THIS list dispatches change-requests-refresh so its own listener
  // doesn't refetch — it already removed the decided row optimistically.
  const suppressSelfReloadRef = useRef(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setRows(await listExcusedAbsenceRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("excused_request_review_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // Refetch in place (no full-page spinner) when a sibling queue or another
  // window decides a request. Mirrors CareRequestReviewList so the pending count
  // and rows stay in sync with the sidebar badge.
  const reloadInPlace = useCallback(async () => {
    try {
      setRows(await listExcusedAbsenceRequests());
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("excused_request_review_reload_failed", { error: message });
    }
  }, []);

  useEffect(() => {
    const handler = () => {
      if (suppressSelfReloadRef.current) return;
      void reloadInPlace();
    };
    window.addEventListener("change-requests-refresh", handler);
    window.addEventListener("messages-unread-refresh", handler);
    return () => {
      window.removeEventListener("change-requests-refresh", handler);
      window.removeEventListener("messages-unread-refresh", handler);
    };
  }, [reloadInPlace]);

  const decide = useCallback(
    async (row: StaffExcusedRequest, approve: boolean) => {
      const reason = reasons[row.id]?.trim() ?? "";
      if (!approve && reason === "") {
        // The backend requires a reason for rejections; surface that before the
        // request instead of a generic error after it.
        setReasonErrors((prev) => ({ ...prev, [row.id]: true }));
        return;
      }
      setBusyId(row.id);
      setError(null);
      setNotice(null);
      try {
        await decideExcusedAbsenceRequest(row.id, approve, reason || undefined);
        setRows((prev) => prev.filter((r) => r.id !== row.id));
        // Clear/decrement the Änderungsanfragen sidebar badge without waiting for
        // the cache or an SSE round-trip. Suppress our own listener: the decided
        // row is already removed above.
        suppressSelfReloadRef.current = true;
        window.dispatchEvent(new Event("change-requests-refresh"));
        suppressSelfReloadRef.current = false;
        setNotice(
          approve ? "Abmeldung bestätigt" : "Entschuldigungs-Anfrage abgelehnt",
        );
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        const code =
          err instanceof ExcusedRequestApiError ? err.code : undefined;
        logger.warn("excused_request_review_decide_failed", {
          error: message,
          request_id: row.id,
          ...(code ? { code } : {}),
        });
        setError(decideErrorMessage(code));
      } finally {
        setBusyId(null);
      }
    },
    [reasons],
  );

  if (loading) return <Loading fullPage={false} />;

  return (
    <div className="space-y-3">
      {error && (
        <div className="rounded-xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-3 text-sm text-[#CC2626]">
          {error}
        </div>
      )}
      {notice && (
        <div className="rounded-xl border border-gray-200 bg-gray-50 p-3 text-sm text-gray-700">
          {notice}
        </div>
      )}
      {rows.length === 0 ? (
        <p className="rounded-2xl border border-gray-200 bg-white p-4 text-sm text-gray-500 shadow-sm">
          Keine offenen Entschuldigungs-Anfragen.
        </p>
      ) : (
        rows.map((row) => (
          <RequestReviewCard
            key={row.id}
            childName={`${row.first_name} ${row.last_name}`}
            summary={datesSummary(row.dates)}
            reason={reasons[row.id] ?? ""}
            onReasonChange={(value) => {
              setReasons((prev) => ({ ...prev, [row.id]: value }));
              setReasonErrors((prev) => ({ ...prev, [row.id]: false }));
            }}
            reasonPlaceholder="Begründung (Pflicht bei Ablehnung)"
            reasonError={
              reasonErrors[row.id]
                ? "Für eine Ablehnung ist eine Begründung erforderlich."
                : undefined
            }
            busy={busyId === row.id}
            onApprove={() => void decide(row, true)}
            onReject={() => void decide(row, false)}
          >
            <ReviewDiffPanel>
              <div className="text-sm">
                <span className="text-xs text-gray-500">Tage: </span>
                <span className="font-medium text-gray-900">
                  {datesSummary(row.dates)}
                </span>
              </div>
              <div className="text-sm">
                <span className="text-xs text-gray-500">
                  Notiz der Eltern:{" "}
                </span>
                {row.note.trim() ? (
                  <span className="font-medium text-gray-900">{row.note}</span>
                ) : (
                  <span className="text-gray-400">—</span>
                )}
              </div>
            </ReviewDiffPanel>
          </RequestReviewCard>
        ))
      )}
    </div>
  );
}
